package policyeditor

import (
	"fmt"
	"strings"

	"github.com/pietroperona/night-agent/internal/policy"
)

// NewRuleSpec contiene i dati per creare una nuova regola.
type NewRuleSpec struct {
	ID         string
	ActionType string
	Pattern    string
	Decision   string
	Reason     string
}

// ToggleRule inverte la decisione di una regola (block↔allow).
func ToggleRule(policyPath, ruleID string) error {
	p, err := policy.LoadFile(policyPath)
	if err != nil {
		return err
	}

	found := false
	for i, r := range p.Rules {
		if r.ID == ruleID {
			if p.Rules[i].Decision == policy.DecisionBlock || p.Rules[i].Decision == policy.DecisionAsk {
				p.Rules[i].Decision = policy.DecisionAllow
				p.Rules[i].Reason = "consentito dall'utente"
			} else {
				p.Rules[i].Decision = policy.DecisionBlock
				p.Rules[i].Reason = r.Reason
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("regola '%s' non trovata nella policy", ruleID)
	}

	return policy.Save(policyPath, p)
}

// AddRule aggiunge una nuova regola alla policy.
// Restituisce errore se esiste già una regola con lo stesso ID.
func AddRule(policyPath string, spec NewRuleSpec) error {
	p, err := policy.LoadFile(policyPath)
	if err != nil {
		return err
	}

	for _, r := range p.Rules {
		if r.ID == spec.ID {
			return fmt.Errorf("regola con ID '%s' già esistente", spec.ID)
		}
	}

	decision := policy.Decision(spec.Decision)
	if decision == "" {
		decision = policy.DecisionBlock
	}

	newRule := policy.Rule{
		ID: spec.ID,
		When: policy.Condition{
			ActionType:     spec.ActionType,
			CommandMatches: []string{spec.Pattern},
		},
		MatchType: policy.MatchGlob,
		Decision:  decision,
		Reason:    spec.Reason,
	}

	p.Rules = append(p.Rules, newRule)
	return policy.Save(policyPath, p)
}

// RemoveRule rimuove una regola dalla policy per ID.
func RemoveRule(policyPath, ruleID string) error {
	p, err := policy.LoadFile(policyPath)
	if err != nil {
		return err
	}

	newRules := make([]policy.Rule, 0, len(p.Rules))
	found := false
	for _, r := range p.Rules {
		if r.ID == ruleID {
			found = true
			continue
		}
		newRules = append(newRules, r)
	}
	if !found {
		return fmt.Errorf("regola '%s' non trovata nella policy", ruleID)
	}

	p.Rules = newRules
	return policy.Save(policyPath, p)
}

// ANSI
const (
	reset     = "\033[0m"
	bold      = "\033[1m"
	dim       = "\033[2m"
	red       = "\033[31m"
	green     = "\033[32m"
	yellow    = "\033[33m"
	cyan      = "\033[36m"
	boldRed   = "\033[1;31m"
	boldGreen = "\033[1;32m"
	boldCyan  = "\033[1;36m"
	boldWhite = "\033[1;37m"
)

// RenderTable restituisce la rappresentazione colorata della policy corrente.
func RenderTable(p *policy.Policy) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\n%s  Policy attiva%s  %s(v%d, %d regole)%s\n",
		bold+boldCyan, reset, dim, p.Version, len(p.Rules), reset))
	sb.WriteString(dim + "  ─────────────────────────────────────────────────────────────────\n" + reset)
	sb.WriteString(fmt.Sprintf("  %s%-4s  %-30s  %-6s  %-8s  %s%s\n",
		bold, "#", "ID", "TIPO", "DECISIONE", "MOTIVO", reset))
	sb.WriteString(dim + "  ─────────────────────────────────────────────────────────────────\n" + reset)

	for i, r := range p.Rules {
		decisionStr := formatDecision(r.Decision)
		actionType := string(r.When.ActionType)
		reason := r.Reason
		if len(reason) > 35 {
			reason = reason[:32] + "..."
		}
		sb.WriteString(fmt.Sprintf("  %s%-4d%s  %-30s  %-6s  %s  %s%s%s\n",
			dim, i+1, reset,
			r.ID,
			actionType,
			decisionStr,
			dim, reason, reset,
		))
	}

	sb.WriteString(dim + "  ─────────────────────────────────────────────────────────────────\n" + reset)
	return sb.String()
}

func formatDecision(d policy.Decision) string {
	switch d {
	case policy.DecisionBlock:
		return boldRed + "✗ block " + reset
	case policy.DecisionAllow:
		return boldGreen + "✓ allow " + reset
	case policy.DecisionAsk:
		return yellow + "? ask   " + reset
	default:
		return dim + string(d) + reset
	}
}
