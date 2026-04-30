package audit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/night-agent-cli/night-agent/internal/audit"
)

func TestGenerateKey_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signing.key")

	if err := audit.GenerateKey(keyPath); err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("key file non creato: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("permessi attesi 0600, got %o", info.Mode().Perm())
	}
}

func TestGenerateKey_Idempotent(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signing.key")

	if err := audit.GenerateKey(keyPath); err != nil {
		t.Fatal(err)
	}
	data1, _ := os.ReadFile(keyPath)

	// seconda chiamata non deve sovrascrivere
	if err := audit.GenerateKey(keyPath); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(keyPath)

	if string(data1) != string(data2) {
		t.Error("GenerateKey sovrascrive chiave esistente")
	}
}

func TestSignAndVerify_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signing.key")
	audit.GenerateKey(keyPath)

	signer, err := audit.NewSigner(keyPath)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	event := audit.Event{
		ID:       "test-123",
		Command:  "git push origin main",
		Decision: "block",
		Reason:   "test",
	}

	signed, err := signer.Sign(event)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if signed.Sig == "" {
		t.Error("firma assente dopo Sign")
	}

	if err := signer.Verify(signed); err != nil {
		t.Errorf("Verify fallito su evento valido: %v", err)
	}
}

func TestVerify_DetectsTampering(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signing.key")
	audit.GenerateKey(keyPath)

	signer, _ := audit.NewSigner(keyPath)

	event := audit.Event{
		ID:       "test-456",
		Command:  "go build ./...",
		Decision: "allow",
	}
	signed, _ := signer.Sign(event)

	// manomissione: cambia il comando dopo la firma
	signed.Command = "sudo rm -rf /"

	if err := signer.Verify(signed); err == nil {
		t.Error("Verify dovrebbe fallire su evento manomesso")
	}
}

func TestVerify_NoKey(t *testing.T) {
	event := audit.Event{ID: "x", Decision: "allow"}
	// evento senza sig → Verify restituisce errore specifico
	signer := &audit.Signer{}
	if err := signer.Verify(event); err == nil {
		t.Error("Verify dovrebbe fallire su evento senza firma")
	}
}

func TestLoggerWithSigning_WritesAndVerifies(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signing.key")
	logPath := filepath.Join(dir, "audit.jsonl")

	audit.GenerateKey(keyPath)
	signer, _ := audit.NewSigner(keyPath)

	logger, err := audit.NewSignedLogger(logPath, signer)
	if err != nil {
		t.Fatalf("NewSignedLogger: %v", err)
	}

	events := []audit.Event{
		{ID: "1", Command: "git status", Decision: "allow"},
		{ID: "2", Command: "sudo su", Decision: "block"},
	}
	for _, e := range events {
		if err := logger.Write(e); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	logger.Close()

	// leggi e verifica tutti
	results, err := audit.VerifyAll(logPath, signer)
	if err != nil {
		t.Fatalf("VerifyAll: %v", err)
	}
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("evento %s: verifica fallita: %v", r.EventID, r.Err)
		}
	}
}

func TestChain_DetectsDeletedEvent(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signing.key")
	logPath := filepath.Join(dir, "audit.jsonl")

	audit.GenerateKey(keyPath)
	signer, _ := audit.NewSigner(keyPath)

	logger, _ := audit.NewSignedLogger(logPath, signer)
	logger.Write(audit.Event{ID: "1", Command: "ls", Decision: "allow"})
	logger.Write(audit.Event{ID: "2", Command: "git status", Decision: "allow"})
	logger.Write(audit.Event{ID: "3", Command: "sudo su", Decision: "block"})
	logger.Close()

	// leggi le righe, rimuovi la riga 2 (evento nel mezzo)
	data, _ := os.ReadFile(logPath)
	lines := splitLines(data)
	if len(lines) < 3 {
		t.Skip("meno di 3 eventi nel log")
	}
	// rimuovi riga 2 (indice 1)
	trimmed := append(lines[:1], lines[2:]...)
	os.WriteFile(logPath, joinLines(trimmed), 0600)

	results, _ := audit.VerifyAll(logPath, signer)
	chainBroken := false
	for _, r := range results {
		if r.Err != nil {
			chainBroken = true
		}
	}
	if !chainBroken {
		t.Error("VerifyAll dovrebbe rilevare evento eliminato (catena hash spezzata)")
	}
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	return lines
}

func joinLines(lines [][]byte) []byte {
	var out []byte
	for _, l := range lines {
		out = append(out, l...)
		out = append(out, '\n')
	}
	return out
}

func TestVerifyAll_DetectsTamperedLine(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signing.key")
	logPath := filepath.Join(dir, "audit.jsonl")

	audit.GenerateKey(keyPath)
	signer, _ := audit.NewSigner(keyPath)

	logger, _ := audit.NewSignedLogger(logPath, signer)
	logger.Write(audit.Event{ID: "1", Command: "ls", Decision: "allow"})
	logger.Close()

	// manometti il file JSONL direttamente
	data, _ := os.ReadFile(logPath)
	tampered := append(data[:len(data)-2], []byte(`,"decision":"block"}`)...)
	os.WriteFile(logPath, append(tampered, '\n'), 0600)

	results, _ := audit.VerifyAll(logPath, signer)
	if len(results) == 0 || results[0].Err == nil {
		t.Error("VerifyAll dovrebbe rilevare manomissione")
	}
}

func TestSignFunc_Local(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signing.key")
	audit.GenerateKey(keyPath)
	signer, _ := audit.NewSigner(keyPath)

	fn := audit.LocalSignFunc(signer)
	e := audit.Event{ID: "test-1", Decision: "allow"}
	sig, source, err := fn(e)
	if err != nil {
		t.Fatal(err)
	}
	if sig == "" {
		t.Error("sig vuota")
	}
	if source != "local" {
		t.Errorf("source=%s, want local", source)
	}
}

func TestLogger_WriteSigSource(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signing.key")
	logPath := filepath.Join(dir, "audit.jsonl")
	audit.GenerateKey(keyPath)
	signer, _ := audit.NewSigner(keyPath)

	logger, _ := audit.NewSignedLoggerWithFunc(logPath, audit.LocalSignFunc(signer))
	logger.Write(audit.Event{ID: "1", Decision: "allow"})
	logger.Close()

	events, _ := audit.ReadAll(logPath)
	if len(events) == 0 {
		t.Fatal("nessun evento")
	}
	if events[0].SigSource != "local" {
		t.Errorf("sig_source=%q, want local", events[0].SigSource)
	}
}
