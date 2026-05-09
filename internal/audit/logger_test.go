package audit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/night-agent-cli/night-agent/internal/audit"
	"github.com/night-agent-cli/night-agent/internal/policy"
)

func TestLogger_WriteAndRead(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	logger, err := audit.NewLogger(logPath)
	if err != nil {
		t.Fatalf("errore creazione logger: %v", err)
	}
	defer logger.Close()

	event := audit.Event{
		ID:         "evt-001",
		Timestamp:  time.Now().UTC(),
		AgentName:  "claude-code",
		ActionType: "shell",
		Command:    "sudo rm -rf /tmp",
		WorkDir:    "/home/user/project",
		Decision:   string(policy.DecisionBlock),
		RuleID:     "block_sudo",
		Reason:     "sudo disabilitato",
	}

	if err := logger.Write(event); err != nil {
		t.Fatalf("errore scrittura evento: %v", err)
	}

	events, err := audit.ReadAll(logPath)
	if err != nil {
		t.Fatalf("errore lettura log: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("atteso 1 evento, ottenuti %d", len(events))
	}
	if events[0].ID != "evt-001" {
		t.Errorf("atteso id=evt-001, ottenuto %s", events[0].ID)
	}
	if events[0].Decision != string(policy.DecisionBlock) {
		t.Errorf("atteso decision=block, ottenuto %s", events[0].Decision)
	}
}

func TestLogger_MultipleEvents(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	logger, err := audit.NewLogger(logPath)
	if err != nil {
		t.Fatalf("errore creazione logger: %v", err)
	}
	defer logger.Close()

	for i := range 3 {
		_ = logger.Write(audit.Event{
			ID:       "evt-00" + string(rune('1'+i)),
			Decision: string(policy.DecisionAllow),
		})
	}

	events, err := audit.ReadAll(logPath)
	if err != nil {
		t.Fatalf("errore lettura: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("attesi 3 eventi, ottenuti %d", len(events))
	}
}

func TestLogger_ValidJSONL(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	logger, _ := audit.NewLogger(logPath)
	defer logger.Close()
	_ = logger.Write(audit.Event{ID: "evt-001", Decision: "block"})

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("errore lettura file: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Errorf("ogni riga deve essere JSON valido: %v", err)
	}
}

func TestReadAll_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	_ = os.WriteFile(logPath, []byte{}, 0600)

	events, err := audit.ReadAll(logPath)
	if err != nil {
		t.Fatalf("atteso nessun errore per file vuoto, ottenuto: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("attesi 0 eventi, ottenuti %d", len(events))
	}
}

func TestReadAll_FileNotFound(t *testing.T) {
	_, err := audit.ReadAll("/nonexistent/path/audit.jsonl")
	if err == nil {
		t.Fatal("atteso errore per file mancante, ottenuto nil")
	}
}

func TestLogger_FilterByDecision(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	logger, _ := audit.NewLogger(logPath)
	defer logger.Close()

	_ = logger.Write(audit.Event{ID: "1", Decision: "block"})
	_ = logger.Write(audit.Event{ID: "2", Decision: "allow"})
	_ = logger.Write(audit.Event{ID: "3", Decision: "block"})

	events, err := audit.ReadFiltered(logPath, audit.Filter{Decision: "block"})
	if err != nil {
		t.Fatalf("errore lettura filtrata: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("attesi 2 eventi block, ottenuti %d", len(events))
	}
}

func TestLogger_FilterByActionType(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	logger, _ := audit.NewLogger(logPath)
	defer logger.Close()

	_ = logger.Write(audit.Event{ID: "1", ActionType: "shell", Decision: "block"})
	_ = logger.Write(audit.Event{ID: "2", ActionType: "git", Decision: "ask"})
	_ = logger.Write(audit.Event{ID: "3", ActionType: "shell", Decision: "allow"})

	events, err := audit.ReadFiltered(logPath, audit.Filter{ActionType: "git"})
	if err != nil {
		t.Fatalf("errore lettura filtrata: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("atteso 1 evento git, ottenuti %d", len(events))
	}
}

// TestReadFilteredWithStats_CountsCorruptLines verifica che le righe JSON
// malformate vengano conteggiate e segnalate anziché ignorate silenziosamente.
func TestReadFilteredWithStats_CountsCorruptLines(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "corrupt.jsonl")

	// scrivi 1 riga valida + 2 corrotte + 1 valida
	content := `{"id":"1","decision":"allow","action_type":"shell"}` + "\n" +
		`{invalid json` + "\n" +
		`not json at all` + "\n" +
		`{"id":"2","decision":"block","action_type":"git"}` + "\n"

	if err := os.WriteFile(logPath, []byte(content), 0600); err != nil {
		t.Fatalf("errore scrittura file test: %v", err)
	}

	events, corrupted, err := audit.ReadFilteredWithStats(logPath, audit.Filter{})
	if err != nil {
		t.Fatalf("errore inatteso: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("attesi 2 eventi validi, ottenuti %d", len(events))
	}
	if corrupted != 2 {
		t.Errorf("attese 2 righe corrotte, ottenute %d", corrupted)
	}
}

// TestReadFiltered_BackwardCompatible verifica che la firma esistente di
// ReadFiltered non sia cambiata e continui a funzionare sui file con corruzione.
func TestReadFiltered_BackwardCompatible(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "mixed.jsonl")

	content := `{"id":"ok","decision":"allow","action_type":"shell"}` + "\n" +
		`CORRUPT` + "\n"

	if err := os.WriteFile(logPath, []byte(content), 0600); err != nil {
		t.Fatalf("errore scrittura file test: %v", err)
	}

	// deve restituire solo gli eventi validi, senza errore
	events, err := audit.ReadFiltered(logPath, audit.Filter{})
	if err != nil {
		t.Fatalf("ReadFiltered non deve fallire con righe corrotte: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("atteso 1 evento valido, ottenuti %d", len(events))
	}
}
