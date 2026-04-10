package prompt

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Response rappresenta la scelta dell'utente al prompt interattivo.
type Response int

const (
	ResponseBlock        Response = iota // blocca questa volta
	ResponseAllowOnce                    // consenti solo questa volta
	ResponseAllowSession                 // consenti per tutta la sessione
	ResponseAllowAlways                  // scrivi regola allow permanente
)

func (r Response) String() string {
	switch r {
	case ResponseBlock:
		return "block"
	case ResponseAllowOnce:
		return "allow_once"
	case ResponseAllowSession:
		return "allow_session"
	case ResponseAllowAlways:
		return "allow_always"
	default:
		return "block"
	}
}

// SessionAllowlist mantiene in memoria i comandi consentiti per la sessione corrente.
// Thread-safe.
type SessionAllowlist struct {
	mu      sync.RWMutex
	entries map[string]map[string]struct{} // agentName → set di comandi
}

func NewSessionAllowlist() *SessionAllowlist {
	return &SessionAllowlist{
		entries: make(map[string]map[string]struct{}),
	}
}

// Add aggiunge un comando all'allowlist di sessione per l'agente dato.
func (s *SessionAllowlist) Add(agentName, command string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries[agentName] == nil {
		s.entries[agentName] = make(map[string]struct{})
	}
	s.entries[agentName][command] = struct{}{}
}

// IsAllowed verifica se il comando è nella allowlist di sessione per l'agente.
func (s *SessionAllowlist) IsAllowed(agentName, command string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cmds, ok := s.entries[agentName]
	if !ok {
		return false
	}
	_, allowed := cmds[command]
	return allowed
}

// BuildDialogScript genera lo script osascript per il dialog macOS.
func BuildDialogScript(agentName, command, reason string) string {
	// escape delle virgolette per non rompere lo script AppleScript
	command = strings.ReplaceAll(command, `"`, `'`)
	reason = strings.ReplaceAll(reason, `"`, `'`)

	return fmt.Sprintf(`display dialog "Agente: %s\nComando: %s\nMotivo: %s" `+
		`with title "Guardian — Azione richiesta" `+
		`buttons {"Blocca", "Consenti", "Sessione", "Sempre"} `+
		`default button "Blocca" `+
		`with icon caution`,
		agentName, command, reason)
}

// ParseDialogResult interpreta l'output di osascript e restituisce la Response.
// In caso di output non riconosciuto (incluso errore/dismiss), restituisce ResponseBlock (safe failure).
func ParseDialogResult(output string) Response {
	output = strings.TrimSpace(output)
	switch {
	case strings.Contains(output, "Consenti") && strings.Contains(output, "Sessione"):
		return ResponseAllowSession
	case strings.Contains(output, "Sessione"):
		return ResponseAllowSession
	case strings.Contains(output, "Sempre"):
		return ResponseAllowAlways
	case strings.Contains(output, "Consenti"):
		return ResponseAllowOnce
	case strings.Contains(output, "Blocca"):
		return ResponseBlock
	default:
		return ResponseBlock // safe failure
	}
}

// Ask mostra il dialog macOS e attende la risposta dell'utente.
// Blocca fino a quando l'utente non risponde o chiude il dialog.
// Safe failure: se osascript non è disponibile o fallisce → ResponseBlock.
func Ask(agentName, command, reason string) Response {
	script := BuildDialogScript(agentName, command, reason)
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return ResponseBlock // safe failure
	}
	return ParseDialogResult(string(out))
}
