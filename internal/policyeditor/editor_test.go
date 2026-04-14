package policyeditor_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pietroperona/night-agent/internal/policyeditor"
	"github.com/pietroperona/night-agent/internal/policy"
)

func writePolicy(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

var sampleYAML = `version: 1
rules:
  - id: block_sudo
    when:
      action_type: shell
      command_matches: ["sudo *"]
    match_type: glob
    decision: block
    reason: "sudo disabilitato"
  - id: block_rm_rf
    when:
      action_type: shell
      command_matches: ["rm -rf *"]
    match_type: glob
    decision: allow
    reason: "consentito dall'utente"
  - id: ask_git_push_main
    when:
      action_type: git
      command_matches: ["git push * main"]
    match_type: glob
    decision: block
    reason: "push su main richiede conferma"
`

func TestToggleRule_BlockToAllow(t *testing.T) {
	path := writePolicy(t, sampleYAML)

	if err := policyeditor.ToggleRule(path, "block_sudo"); err != nil {
		t.Fatalf("ToggleRule fallita: %v", err)
	}

	p, _ := policy.LoadFile(path)
	for _, r := range p.Rules {
		if r.ID == "block_sudo" {
			if r.Decision != policy.DecisionAllow {
				t.Errorf("atteso allow dopo toggle, ottenuto %s", r.Decision)
			}
			return
		}
	}
	t.Error("regola block_sudo non trovata dopo toggle")
}

func TestToggleRule_AllowToBlock(t *testing.T) {
	path := writePolicy(t, sampleYAML)

	if err := policyeditor.ToggleRule(path, "block_rm_rf"); err != nil {
		t.Fatalf("ToggleRule fallita: %v", err)
	}

	p, _ := policy.LoadFile(path)
	for _, r := range p.Rules {
		if r.ID == "block_rm_rf" {
			if r.Decision != policy.DecisionBlock {
				t.Errorf("atteso block dopo toggle, ottenuto %s", r.Decision)
			}
			return
		}
	}
	t.Error("regola block_rm_rf non trovata dopo toggle")
}

func TestToggleRule_NotFound(t *testing.T) {
	path := writePolicy(t, sampleYAML)
	err := policyeditor.ToggleRule(path, "nonexistent_rule")
	if err == nil {
		t.Error("atteso errore per regola inesistente")
	}
}

func TestAddRule_NewRule(t *testing.T) {
	path := writePolicy(t, sampleYAML)

	rule := policyeditor.NewRuleSpec{
		ID:          "block_chmod",
		ActionType:  "shell",
		Pattern:     "chmod 777 *",
		Decision:    "block",
		Reason:      "chmod 777 non consentito",
	}
	if err := policyeditor.AddRule(path, rule); err != nil {
		t.Fatalf("AddRule fallita: %v", err)
	}

	p, _ := policy.LoadFile(path)
	for _, r := range p.Rules {
		if r.ID == "block_chmod" {
			if r.Decision != policy.DecisionBlock {
				t.Errorf("atteso block, ottenuto %s", r.Decision)
			}
			return
		}
	}
	t.Error("regola block_chmod non trovata dopo AddRule")
}

func TestAddRule_DuplicateID(t *testing.T) {
	path := writePolicy(t, sampleYAML)

	rule := policyeditor.NewRuleSpec{
		ID:         "block_sudo",
		ActionType: "shell",
		Pattern:    "sudo *",
		Decision:   "block",
		Reason:     "duplicato",
	}
	err := policyeditor.AddRule(path, rule)
	if err == nil {
		t.Error("atteso errore per ID duplicato")
	}
}

func TestRemoveRule(t *testing.T) {
	path := writePolicy(t, sampleYAML)

	if err := policyeditor.RemoveRule(path, "block_sudo"); err != nil {
		t.Fatalf("RemoveRule fallita: %v", err)
	}

	p, _ := policy.LoadFile(path)
	for _, r := range p.Rules {
		if r.ID == "block_sudo" {
			t.Error("regola block_sudo ancora presente dopo RemoveRule")
		}
	}
}

func TestRemoveRule_NotFound(t *testing.T) {
	path := writePolicy(t, sampleYAML)
	err := policyeditor.RemoveRule(path, "nonexistent")
	if err == nil {
		t.Error("atteso errore per regola inesistente")
	}
}

func TestRenderTable_NotEmpty(t *testing.T) {
	path := writePolicy(t, sampleYAML)
	p, _ := policy.LoadFile(path)

	out := policyeditor.RenderTable(p)
	if out == "" {
		t.Error("RenderTable non deve restituire stringa vuota")
	}
	if !containsStr(out, "block_sudo") {
		t.Error("tabella non contiene block_sudo")
	}
	if !containsStr(out, "block_rm_rf") {
		t.Error("tabella non contiene block_rm_rf")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || findSubstr(s, sub))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
