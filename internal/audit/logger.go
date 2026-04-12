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
}

// Filter specifica criteri di filtro per ReadFiltered.
type Filter struct {
	Decision   string
	ActionType string
}

// Logger scrive eventi in formato JSONL su file.
type Logger struct {
	file *os.File
	enc  *json.Encoder
}

// NewLogger apre (o crea) il file di log e restituisce un Logger.
func NewLogger(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("impossibile aprire il file di log: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return &Logger{file: f, enc: enc}, nil
}

// Write scrive un evento nel log. Se l'evento non ha timestamp, lo imposta ora.
func (l *Logger) Write(event Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
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
	return ReadFiltered(path, Filter{})
}

// ReadFiltered legge gli eventi applicando un filtro opzionale.
func ReadFiltered(path string, filter Filter) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("impossibile aprire il log: %w", err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			continue // riga corrotta: salta senza fallire
		}
		if matchesFilter(e, filter) {
			events = append(events, e)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("errore lettura log: %w", err)
	}
	return events, nil
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
