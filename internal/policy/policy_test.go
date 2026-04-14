package policy_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pietroperona/night-agent/internal/policy"
)

func TestLoadPolicy_ValidFile(t *testing.T) {
	yaml := `
version: 1
rules:
  - id: block_sudo
    when:
      action_type: shell
      command_matches: ["sudo *"]
    match_type: glob
    decision: block
    reason: "sudo disabilitato"
`
	f := writeTempYAML(t, yaml)
	p, err := policy.LoadFile(f)
	if err != nil {
		t.Fatalf("atteso nessun errore, ottenuto: %v", err)
	}
	if p.Version != 1 {
		t.Errorf("atteso version=1, ottenuto %d", p.Version)
	}
	if len(p.Rules) != 1 {
		t.Fatalf("atteso 1 regola, ottenute %d", len(p.Rules))
	}
	if p.Rules[0].ID != "block_sudo" {
		t.Errorf("atteso id=block_sudo, ottenuto %s", p.Rules[0].ID)
	}
}

func TestLoadPolicy_FileNotFound(t *testing.T) {
	_, err := policy.LoadFile("/nonexistent/path/policy.yaml")
	if err == nil {
		t.Fatal("atteso errore per file mancante, ottenuto nil")
	}
}

func TestLoadPolicy_InvalidYAML(t *testing.T) {
	f := writeTempYAML(t, "{ invalid yaml ::::")
	_, err := policy.LoadFile(f)
	if err == nil {
		t.Fatal("atteso errore per YAML invalido, ottenuto nil")
	}
}

func TestLoadPolicy_MissingVersion(t *testing.T) {
	yaml := `
rules:
  - id: test_rule
    when:
      action_type: shell
      command_matches: ["sudo *"]
    decision: block
    reason: "test"
`
	f := writeTempYAML(t, yaml)
	_, err := policy.LoadFile(f)
	if err == nil {
		t.Fatal("atteso errore per version mancante, ottenuto nil")
	}
}

// --- Evaluate tests ---

func TestEvaluate_BlockGlob(t *testing.T) {
	p := &policy.Policy{
		Version: 1,
		Rules: []policy.Rule{
			{
				ID:        "block_sudo",
				MatchType: policy.MatchGlob,
				When:      policy.Condition{ActionType: "shell", CommandMatches: []string{"sudo *"}},
				Decision:  policy.DecisionBlock,
				Reason:    "sudo disabilitato",
			},
		},
	}

	result := p.Evaluate(policy.Action{Type: "shell", Command: "sudo rm -rf /"})
	if result.Decision != policy.DecisionBlock {
		t.Errorf("atteso block, ottenuto %s", result.Decision)
	}
	if result.RuleID != "block_sudo" {
		t.Errorf("atteso rule_id=block_sudo, ottenuto %s", result.RuleID)
	}
}

func TestEvaluate_AllowByDefault(t *testing.T) {
	p := &policy.Policy{
		Version: 1,
		Rules: []policy.Rule{
			{
				ID:        "block_sudo",
				MatchType: policy.MatchGlob,
				When:      policy.Condition{ActionType: "shell", CommandMatches: []string{"sudo *"}},
				Decision:  policy.DecisionBlock,
				Reason:    "sudo disabilitato",
			},
		},
	}

	result := p.Evaluate(policy.Action{Type: "shell", Command: "ls -la"})
	if result.Decision != policy.DecisionAllow {
		t.Errorf("atteso allow per default, ottenuto %s", result.Decision)
	}
}

func TestEvaluate_AskGlob(t *testing.T) {
	p := &policy.Policy{
		Version: 1,
		Rules: []policy.Rule{
			{
				ID:        "ask_push_main",
				MatchType: policy.MatchGlob,
				When:      policy.Condition{ActionType: "git", CommandMatches: []string{"git push * main"}},
				Decision:  policy.DecisionAsk,
				Reason:    "push su main richiede conferma",
			},
		},
	}

	result := p.Evaluate(policy.Action{Type: "git", Command: "git push origin main"})
	if result.Decision != policy.DecisionAsk {
		t.Errorf("atteso ask, ottenuto %s", result.Decision)
	}
}

func TestEvaluate_FirstMatchWins(t *testing.T) {
	p := &policy.Policy{
		Version: 1,
		Rules: []policy.Rule{
			{
				ID:        "first_rule",
				MatchType: policy.MatchGlob,
				When:      policy.Condition{ActionType: "shell", CommandMatches: []string{"rm *"}},
				Decision:  policy.DecisionBlock,
				Reason:    "prima regola",
			},
			{
				ID:        "second_rule",
				MatchType: policy.MatchGlob,
				When:      policy.Condition{ActionType: "shell", CommandMatches: []string{"rm -rf *"}},
				Decision:  policy.DecisionAsk,
				Reason:    "seconda regola",
			},
		},
	}

	result := p.Evaluate(policy.Action{Type: "shell", Command: "rm -rf /tmp/test"})
	if result.RuleID != "first_rule" {
		t.Errorf("atteso first_rule (first-match-wins), ottenuto %s", result.RuleID)
	}
}

func TestEvaluate_RegexMatch(t *testing.T) {
	p := &policy.Policy{
		Version: 1,
		Rules: []policy.Rule{
			{
				ID:        "regex_sudo",
				MatchType: policy.MatchRegex,
				When:      policy.Condition{ActionType: "shell", CommandMatches: []string{`^sudo\s+.*`}},
				Decision:  policy.DecisionBlock,
				Reason:    "sudo disabilitato (regex)",
			},
		},
	}

	result := p.Evaluate(policy.Action{Type: "shell", Command: "sudo apt-get install curl"})
	if result.Decision != policy.DecisionBlock {
		t.Errorf("atteso block con regex, ottenuto %s", result.Decision)
	}
}

func TestEvaluate_ActionTypeMismatch(t *testing.T) {
	p := &policy.Policy{
		Version: 1,
		Rules: []policy.Rule{
			{
				ID:        "block_sudo",
				MatchType: policy.MatchGlob,
				When:      policy.Condition{ActionType: "shell", CommandMatches: []string{"sudo *"}},
				Decision:  policy.DecisionBlock,
				Reason:    "sudo disabilitato",
			},
		},
	}

	// azione tipo "git" non deve matchare una regola shell
	result := p.Evaluate(policy.Action{Type: "git", Command: "sudo rm -rf /"})
	if result.Decision != policy.DecisionAllow {
		t.Errorf("atteso allow (action_type mismatch), ottenuto %s", result.Decision)
	}
}

func TestEvaluate_PathMatch(t *testing.T) {
	p := &policy.Policy{
		Version: 1,
		Rules: []policy.Rule{
			{
				ID:        "block_ssh",
				MatchType: policy.MatchGlob,
				When:      policy.Condition{ActionType: "file", PathMatches: []string{"~/.ssh/*"}},
				Decision:  policy.DecisionBlock,
				Reason:    "accesso ssh vietato",
			},
		},
	}

	result := p.Evaluate(policy.Action{Type: "file", Path: "~/.ssh/id_rsa"})
	if result.Decision != policy.DecisionBlock {
		t.Errorf("atteso block per path ~/.ssh/id_rsa, ottenuto %s", result.Decision)
	}
}

// --- AppendAllowRule ---

func TestAppendAllowRule_AddsRule(t *testing.T) {
	path := writeTempYAML(t, "version: 1\nrules: []\n")

	if err := policy.AppendAllowRule(path, "claude", "sudo ls"); err != nil {
		t.Fatalf("AppendAllowRule fallita: %v", err)
	}

	p, err := policy.LoadFile(path)
	if err != nil {
		t.Fatalf("errore caricamento policy dopo append: %v", err)
	}

	result := p.Evaluate(policy.Action{Type: "shell", Command: "sudo ls"})
	if result.Decision != policy.DecisionAllow {
		t.Errorf("atteso allow dopo AppendAllowRule, ottenuto %s", result.Decision)
	}
}

func TestAppendAllowRule_Idempotent(t *testing.T) {
	path := writeTempYAML(t, "version: 1\nrules: []\n")

	if err := policy.AppendAllowRule(path, "claude", "sudo ls"); err != nil {
		t.Fatalf("prima AppendAllowRule fallita: %v", err)
	}
	if err := policy.AppendAllowRule(path, "claude", "sudo ls"); err != nil {
		t.Fatalf("seconda AppendAllowRule fallita: %v", err)
	}

	p, err := policy.LoadFile(path)
	if err != nil {
		t.Fatalf("errore caricamento policy: %v", err)
	}
	// conta regole allow per questo comando
	count := 0
	for _, r := range p.Rules {
		if r.Decision == policy.DecisionAllow {
			count++
		}
	}
	if count != 1 {
		t.Errorf("attesa 1 regola allow, trovate %d", count)
	}
}

func TestAppendAllowRule_PreservesExistingRules(t *testing.T) {
	path := writeTempYAML(t, `version: 1
rules:
  - id: block_sudo
    when:
      action_type: shell
      command_matches: ["sudo *"]
    match_type: glob
    decision: block
    reason: "sudo disabilitato"
`)

	if err := policy.AppendAllowRule(path, "claude", "git status"); err != nil {
		t.Fatalf("AppendAllowRule fallita: %v", err)
	}

	p, err := policy.LoadFile(path)
	if err != nil {
		t.Fatalf("errore caricamento policy: %v", err)
	}
	// la regola block_sudo deve ancora esistere
	result := p.Evaluate(policy.Action{Type: "shell", Command: "sudo rm -rf /"})
	if result.Decision != policy.DecisionBlock {
		t.Errorf("regola block_sudo rimossa dopo AppendAllowRule, atteso block ottenuto %s", result.Decision)
	}
}

// --- helpers ---

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		t.Fatalf("errore scrittura file temporaneo: %v", err)
	}
	return f
}
