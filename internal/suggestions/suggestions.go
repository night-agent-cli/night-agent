// Package suggestions implementa il policy suggestion engine (Cycle 3).
// Analizza l'azione corrente, il risk score e la storia degli eventi per
// suggerire all'utente modifiche utili alla policy. I suggerimenti sono
// informativi — non alterano mai la decisione del daemon.
package suggestions

import (
	"fmt"
	"strings"

	"github.com/pietroperona/night-agent/internal/audit"
	"github.com/pietroperona/night-agent/internal/scorer"
)

// Engine genera suggerimenti di policy contestuali.
type Engine struct{}

// New crea un Engine.
func New() *Engine { return &Engine{} }

// Suggest restituisce una lista di suggerimenti testuali (può essere vuota).
// I suggerimenti vengono inclusi nel log e stampati a terminale quando presenti.
func (e *Engine) Suggest(action scorer.Action, result scorer.Result, events []audit.Event) []string {
	var hints []string

	// Nessun suggerimento per rischio basso senza anomalie
	if result.Level == scorer.LevelLow && !result.AnomalyDetected {
		return nil
	}

	// Suggerimento: path sensibile → considera read-only in policy
	for _, sig := range result.Signals {
		if strings.Contains(sig, "path sensibile") {
			path := extractSensitivePath(sig)
			hints = append(hints, fmt.Sprintf("considera di aggiungere '%s' come path read-only nella policy", path))
		}
	}

	// Suggerimento: comandi ripetuti con override manuale → rendi allow permanente
	if countRepeatedOverrides(action.Command, events) >= 3 {
		hints = append(hints, fmt.Sprintf("'%s' è stato approvato manualmente più volte — vuoi renderlo allow permanente nella policy?", truncate(action.Command, 50)))
	}

	// Suggerimento: anomalia burst → considera sandbox per questo tipo di azione
	if result.AnomalyDetected {
		hints = append(hints, fmt.Sprintf("rilevato burst anomalo di azioni — considera di eseguire '%s' in sandbox", truncate(action.Command, 40)))
	}

	// Suggerimento: rischio alto → suggerisci regola block esplicita
	if result.Level == scorer.LevelHigh {
		hints = append(hints, fmt.Sprintf("rischio alto rilevato — considera di aggiungere una regola block esplicita per questo pattern"))
	}

	// Suggerimento: script remoto via pipe → suggerisci sandbox obbligatoria
	for _, sig := range result.Signals {
		if strings.Contains(sig, "pipe") {
			hints = append(hints, "script eseguito via pipe: considera decision: sandbox nella policy per questo pattern")
		}
	}

	return deduplicate(hints)
}

// countRepeatedOverrides conta quante volte command è stato approvato manualmente
// nella storia degli eventi (user_override = true).
func countRepeatedOverrides(command string, events []audit.Event) int {
	count := 0
	for _, e := range events {
		if e.Command == command && e.UserOverride {
			count++
		}
	}
	return count
}

// extractSensitivePath estrae il nome del path dal segnale di scorer.
// Formato atteso: "accesso path sensibile: <path>"
func extractSensitivePath(signal string) string {
	parts := strings.SplitN(signal, ": ", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return "path"
}

// truncate accorcia la stringa a maxLen caratteri aggiungendo "..." se necessario.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// deduplicate rimuove stringhe duplicate mantenendo l'ordine.
func deduplicate(hints []string) []string {
	seen := make(map[string]struct{}, len(hints))
	out := hints[:0]
	for _, h := range hints {
		if _, ok := seen[h]; !ok {
			seen[h] = struct{}{}
			out = append(out, h)
		}
	}
	return out
}
