package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Event rappresenta un singolo evento di audit nel log JSONL.
type Event struct {
	ID           string    `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	SessionID    string    `json:"session_id,omitempty"`
	AgentName    string    `json:"agent_name,omitempty"`
	ProjectPath  string    `json:"project_path,omitempty"`
	ActionType   string    `json:"action_type,omitempty"`
	Command      string    `json:"command,omitempty"`
	Path         string    `json:"path,omitempty"`
	WorkDir      string    `json:"work_dir,omitempty"`
	Decision     string    `json:"decision"`
	RuleID       string    `json:"rule_id,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	UserOverride bool      `json:"user_override,omitempty"`
	// Campi sandbox (Ciclo 2)
	Sandboxed       bool   `json:"sandboxed,omitempty"`
	SandboxImage    string `json:"sandbox_image,omitempty"`
	SandboxExitCode *int   `json:"sandbox_exit_code,omitempty"`
	// Campi risk scoring (Ciclo 3)
	RiskScore       float64  `json:"risk_score,omitempty"`
	RiskLevel       string   `json:"risk_level,omitempty"`
	RiskSignals     []string `json:"risk_signals,omitempty"`
	AnomalyDetected bool     `json:"anomaly_detected,omitempty"`
	Suggestions     []string `json:"suggestions,omitempty"`
	// Signed audit trail — catena hash (blockchain-like)
	PrevHash  string `json:"prev_hash,omitempty"`   // hash SHA256 dell'evento precedente
	Sig       string `json:"sig,omitempty"`          // HMAC-SHA256 di tutto l'evento (incluso prev_hash)
	SigSource string `json:"sig_source,omitempty"`  // "local" | "remote"
}

// Filter specifica criteri di filtro per ReadFiltered.
type Filter struct {
	Decision   string
	ActionType string
}

// Logger scrive eventi in formato JSONL su file.
type Logger struct {
	file     *os.File
	enc      *json.Encoder
	signFn   SignFunc // nil = nessuna firma
	lastHash string   // hash dell'ultimo evento scritto (per catena)
	signer   *Signer  // mantenuto per retrocompatibilità con VerifyAll
}

// NewLogger apre (o crea) il file di log e restituisce un Logger senza firma.
func NewLogger(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("impossibile aprire il file di log: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return &Logger{file: f, enc: enc}, nil
}

// NewSignedLogger apre (o crea) il file di log con firma HMAC-SHA256 attiva.
func NewSignedLogger(path string, signer *Signer) (*Logger, error) {
	l, err := NewSignedLoggerWithFunc(path, LocalSignFunc(signer))
	if err != nil {
		return nil, err
	}
	l.signer = signer // mantieni per VerifyAll
	return l, nil
}

// NewSignedLoggerWithFunc apre il file di log e usa la SignFunc fornita per firmare.
func NewSignedLoggerWithFunc(path string, fn SignFunc) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("impossibile aprire il file di log: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return &Logger{file: f, enc: enc, signFn: fn}, nil
}

// Write scrive un evento nel log. Se l'evento non ha timestamp, lo imposta ora.
// Se il logger ha una signFn, aggiunge prev_hash (catena) e firma l'evento.
func (l *Logger) Write(event Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if l.signFn != nil {
		event.PrevHash = l.lastHash
		event.Sig = "" // azzera prima di firmare
		sig, source, err := l.signFn(event)
		if err != nil {
			return fmt.Errorf("firma evento: %w", err)
		}
		event.Sig = sig
		event.SigSource = source
		l.lastHash = sig
	}
	if err := l.enc.Encode(event); err != nil {
		return fmt.Errorf("errore scrittura evento: %w", err)
	}
	return nil
}

// Close chiude il file di log.
func (l *Logger) Close() error {
	return l.file.Close()
}

// ReadAll legge tutti gli eventi dal file JSONL.
func ReadAll(path string) ([]Event, error) {
	events, _, err := ReadFilteredWithStats(path, Filter{})
	return events, err
}

// ReadFiltered legge gli eventi applicando un filtro opzionale.
// Le righe JSON malformate vengono saltate senza fallire.
func ReadFiltered(path string, filter Filter) ([]Event, error) {
	events, _, err := ReadFilteredWithStats(path, filter)
	return events, err
}

// ReadFilteredWithStats legge gli eventi come ReadFiltered e restituisce
// anche il numero di righe JSON malformate incontrate nel file.
func ReadFilteredWithStats(path string, filter Filter) (events []Event, corrupted int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("impossibile aprire il log: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			corrupted++
			continue
		}
		if matchesFilter(e, filter) {
			events = append(events, e)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, corrupted, fmt.Errorf("errore lettura log: %w", err)
	}
	return events, corrupted, nil
}

func matchesFilter(e Event, f Filter) bool {
	if f.Decision != "" && e.Decision != f.Decision {
		return false
	}
	if f.ActionType != "" && e.ActionType != f.ActionType {
		return false
	}
	return true
}
