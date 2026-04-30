package audit

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

// SignFunc è la funzione iniettata nel Logger per firmare eventi.
// Riceve l'evento da firmare, restituisce la firma (sig), la sorgente ("local"/"remote") e un eventuale errore.
type SignFunc func(e Event) (sig, source string, err error)

// LocalSignFunc restituisce una SignFunc che firma con il Signer locale.
func LocalSignFunc(s *Signer) SignFunc {
	return func(e Event) (string, string, error) {
		signed, err := s.Sign(e)
		if err != nil {
			return "", "", err
		}
		return signed.Sig, "local", nil
	}
}

// Signer gestisce la chiave HMAC-SHA256 per firmare e verificare eventi.
// La chiave è un segreto locale a 32 byte conservato in ~/.night-agent/signing.key.
// Con la futura cloud dashboard la chiave pubblica potrà essere caricata per
// verifica server-side (upgrade a Ed25519 pianificato).
type Signer struct {
	key []byte
}

// GenerateKey crea una nuova chiave casuale a 32 byte nel file keyPath.
// Se il file esiste già non fa nulla (idempotente).
func GenerateKey(keyPath string) error {
	if _, err := os.Stat(keyPath); err == nil {
		return nil // già presente
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("generazione chiave: %w", err)
	}

	encoded := hex.EncodeToString(key)
	return os.WriteFile(keyPath, []byte(encoded), 0600)
}

// NewSigner carica la chiave da keyPath e restituisce un Signer pronto.
func NewSigner(keyPath string) (*Signer, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("lettura chiave: %w", err)
	}

	key, err := hex.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("chiave non valida: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("chiave deve essere 32 byte, trovati %d", len(key))
	}

	return &Signer{key: key}, nil
}

// Sign aggiunge la firma HMAC-SHA256 all'evento e lo restituisce.
// La firma copre la serializzazione JSON dell'evento con Sig="" e SigSource=""
// (sig_source è campo informativo, non incluso nel payload firmato).
func (s *Signer) Sign(e Event) (Event, error) {
	e.Sig = ""        // azzera prima di firmare
	e.SigSource = "" // non incluso nel payload firmato
	payload, err := json.Marshal(e)
	if err != nil {
		return e, fmt.Errorf("serializzazione evento: %w", err)
	}
	e.Sig = s.computeHMAC(payload)
	return e, nil
}

// Verify controlla che la firma dell'evento sia valida.
// sig_source non fa parte del payload firmato: viene azzerato prima della verifica.
func (s *Signer) Verify(e Event) error {
	if e.Sig == "" {
		return fmt.Errorf("evento senza firma (sig assente)")
	}
	if len(s.key) == 0 {
		return fmt.Errorf("signer senza chiave")
	}

	sig := e.Sig
	e.Sig = ""
	e.SigSource = "" // non incluso nel payload firmato
	payload, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("serializzazione evento: %w", err)
	}

	expected := s.computeHMAC(payload)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return fmt.Errorf("firma non valida — evento potenzialmente manomesso")
	}
	return nil
}

func (s *Signer) computeHMAC(data []byte) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyResult è il risultato della verifica di un singolo evento.
type VerifyResult struct {
	EventID string
	Index   int
	Err     error
}

// VerifyAll legge tutti gli eventi da logPath e ne verifica:
// 1. la firma HMAC-SHA256 (integrità del contenuto)
// 2. la catena prev_hash (nessun evento eliminato o riordinato)
// Restituisce un risultato per ogni evento (Err nil = tutto valido).
func VerifyAll(logPath string, signer *Signer) ([]VerifyResult, error) {
	events, err := ReadAll(logPath)
	if err != nil {
		return nil, err
	}

	results := make([]VerifyResult, len(events))
	var prevSig string

	for i, e := range events {
		r := VerifyResult{EventID: e.ID, Index: i}

		// 1. verifica firma
		if sigErr := signer.Verify(e); sigErr != nil {
			r.Err = sigErr
			results[i] = r
			prevSig = e.Sig
			continue
		}

		// 2. verifica catena: prev_hash deve corrispondere alla firma dell'evento precedente
		if i > 0 && e.PrevHash != "" && e.PrevHash != prevSig {
			r.Err = fmt.Errorf("catena hash spezzata — evento precedente mancante o modificato")
		}

		prevSig = e.Sig
		results[i] = r
	}
	return results, nil
}
