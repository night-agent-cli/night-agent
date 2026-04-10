package prompt_test

import (
	"testing"

	"github.com/pietroperona/agent-guardian/internal/prompt"
)

// --- PromptResponse ---

func TestPromptResponse_String(t *testing.T) {
	cases := []struct {
		r    prompt.Response
		want string
	}{
		{prompt.ResponseBlock, "block"},
		{prompt.ResponseAllowOnce, "allow_once"},
		{prompt.ResponseAllowSession, "allow_session"},
		{prompt.ResponseAllowAlways, "allow_always"},
	}
	for _, c := range cases {
		if got := c.r.String(); got != c.want {
			t.Errorf("Response(%d).String() = %q, atteso %q", c.r, got, c.want)
		}
	}
}

// --- SessionAllowlist ---

func TestSessionAllowlist_Empty(t *testing.T) {
	sa := prompt.NewSessionAllowlist()
	if sa.IsAllowed("claude", "sudo ls") {
		t.Error("atteso false per allowlist vuota")
	}
}

func TestSessionAllowlist_AddAndCheck(t *testing.T) {
	sa := prompt.NewSessionAllowlist()
	sa.Add("claude", "sudo ls")
	if !sa.IsAllowed("claude", "sudo ls") {
		t.Error("atteso true dopo Add")
	}
}

func TestSessionAllowlist_DifferentAgent(t *testing.T) {
	sa := prompt.NewSessionAllowlist()
	sa.Add("claude", "sudo ls")
	if sa.IsAllowed("codex", "sudo ls") {
		t.Error("allowlist per 'claude' non deve applicarsi a 'codex'")
	}
}

func TestSessionAllowlist_DifferentCommand(t *testing.T) {
	sa := prompt.NewSessionAllowlist()
	sa.Add("claude", "sudo ls")
	if sa.IsAllowed("claude", "sudo rm -rf /") {
		t.Error("allowlist per 'sudo ls' non deve applicarsi a 'sudo rm -rf /'")
	}
}

func TestSessionAllowlist_MultipleCommands(t *testing.T) {
	sa := prompt.NewSessionAllowlist()
	sa.Add("claude", "sudo ls")
	sa.Add("claude", "git push origin main")

	if !sa.IsAllowed("claude", "sudo ls") {
		t.Error("atteso true per 'sudo ls'")
	}
	if !sa.IsAllowed("claude", "git push origin main") {
		t.Error("atteso true per 'git push origin main'")
	}
}

func TestSessionAllowlist_Idempotent(t *testing.T) {
	sa := prompt.NewSessionAllowlist()
	sa.Add("claude", "sudo ls")
	sa.Add("claude", "sudo ls") // duplicato
	if !sa.IsAllowed("claude", "sudo ls") {
		t.Error("atteso true dopo Add duplicato")
	}
}

// --- BuildOsascriptArgs ---

func TestBuildOsascriptArgs_ContainsCommand(t *testing.T) {
	script := prompt.BuildDialogScript("claude", "sudo rm -rf /tmp", "sudo disabilitato")
	if script == "" {
		t.Error("script non deve essere vuoto")
	}
	// verifica che contenga i testi chiave
	checkContains(t, script, "sudo rm -rf /tmp", "comando")
	checkContains(t, script, "claude", "nome agente")
	checkContains(t, script, "sudo disabilitato", "motivo")
}

func TestBuildOsascriptArgs_ContainsAllButtons(t *testing.T) {
	script := prompt.BuildDialogScript("claude", "sudo ls", "test")
	checkContains(t, script, "Blocca", "bottone Blocca")
	checkContains(t, script, "Consenti", "bottone Consenti")
	checkContains(t, script, "Sessione", "bottone Sessione")
	checkContains(t, script, "Sempre", "bottone Sempre")
}

// --- ParseDialogResult ---

func TestParseDialogResult(t *testing.T) {
	cases := []struct {
		input string
		want  prompt.Response
	}{
		{"button returned:Blocca", prompt.ResponseBlock},
		{"button returned:Consenti", prompt.ResponseAllowOnce},
		{"button returned:Sessione", prompt.ResponseAllowSession},
		{"button returned:Sempre", prompt.ResponseAllowAlways},
		{"", prompt.ResponseBlock},                  // safe failure
		{"button returned:unknown", prompt.ResponseBlock}, // safe failure
	}
	for _, c := range cases {
		got := prompt.ParseDialogResult(c.input)
		if got != c.want {
			t.Errorf("ParseDialogResult(%q) = %v, atteso %v", c.input, got, c.want)
		}
	}
}

func checkContains(t *testing.T, s, sub, label string) {
	t.Helper()
	if !containsStr(s, sub) {
		t.Errorf("script non contiene %s (%q)", label, sub)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
