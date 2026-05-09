package shell_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/night-agent-cli/night-agent/internal/shell"
)

const testSocketPath = "/tmp/nightagent-test.sock"

func TestInject_AddsHookToZshrc(t *testing.T) {
	zshrc := writeTempRC(t, "# existing config\nexport PATH=$PATH:/usr/local/bin\n")

	injected, err := shell.Inject(zshrc, testSocketPath)
	if err != nil {
		t.Fatalf("errore iniezione: %v", err)
	}
	if !injected {
		t.Error("atteso injected=true alla prima iniezione")
	}

	content, _ := os.ReadFile(zshrc)
	if !strings.Contains(string(content), "nightagent") {
		t.Error("atteso hook nightagent nel file")
	}
	if !strings.Contains(string(content), "preexec") {
		t.Error("atteso uso di preexec nell'hook")
	}
	// contenuto originale preservato
	if !strings.Contains(string(content), "existing config") {
		t.Error("il contenuto originale deve essere preservato")
	}
}

func TestInject_Idempotent(t *testing.T) {
	zshrc := writeTempRC(t, "# existing config\n")

	if _, err := shell.Inject(zshrc, testSocketPath); err != nil {
		t.Fatalf("prima iniezione fallita: %v", err)
	}
	injected, err := shell.Inject(zshrc, testSocketPath)
	if err != nil {
		t.Fatalf("seconda iniezione fallita: %v", err)
	}
	if injected {
		t.Error("atteso injected=false alla seconda iniezione (già presente)")
	}

	content, _ := os.ReadFile(zshrc)
	count := strings.Count(string(content), "# BEGIN nightagent")
	if count != 1 {
		t.Errorf("atteso 1 blocco nightagent, trovati %d", count)
	}
}

func TestInject_FileNotFound(t *testing.T) {
	_, err := shell.Inject("/nonexistent/path/.zshrc", testSocketPath)
	if err == nil {
		t.Fatal("atteso errore per file mancante, ottenuto nil")
	}
}

func TestRemove_RemovesHookFromZshrc(t *testing.T) {
	zshrc := writeTempRC(t, "# existing config\n")

	_, _ = shell.Inject(zshrc, testSocketPath)
	if err := shell.Remove(zshrc); err != nil {
		t.Fatalf("errore rimozione: %v", err)
	}

	content, _ := os.ReadFile(zshrc)
	if strings.Contains(string(content), "nightagent") {
		t.Error("nightagent non dovrebbe essere presente dopo la rimozione")
	}
	if !strings.Contains(string(content), "existing config") {
		t.Error("il contenuto originale deve essere preservato dopo la rimozione")
	}
}

func TestRemove_NoopIfNotInjected(t *testing.T) {
	zshrc := writeTempRC(t, "# existing config\n")

	if err := shell.Remove(zshrc); err != nil {
		t.Fatalf("rimozione su file senza hook non deve fallire: %v", err)
	}
}

func TestIsInjected_True(t *testing.T) {
	zshrc := writeTempRC(t, "")
	_, _ = shell.Inject(zshrc, testSocketPath)

	if !shell.IsInjected(zshrc) {
		t.Error("atteso IsInjected=true dopo l'iniezione")
	}
}

func TestIsInjected_False(t *testing.T) {
	zshrc := writeTempRC(t, "# clean config\n")

	if shell.IsInjected(zshrc) {
		t.Error("atteso IsInjected=false su file senza hook")
	}
}

// TestHookTemplate_SinglePythonInvocation verifica che il template iniettato
// usi al massimo una invocazione di python3 (invece delle 3 precedenti).
func TestHookTemplate_SinglePythonInvocation(t *testing.T) {
	zshrc := writeTempRC(t, "")
	_, err := shell.Inject(zshrc, testSocketPath)
	if err != nil {
		t.Fatalf("iniezione fallita: %v", err)
	}
	content, _ := os.ReadFile(zshrc)
	count := strings.Count(string(content), "python3")
	if count > 1 {
		t.Errorf("hook contiene %d invocazioni di python3, atteso ≤1", count)
	}
}

// TestHookTemplate_NoDependencyOnNc verifica che l'hook non dipenda da nc.
func TestHookTemplate_NoDependencyOnNc(t *testing.T) {
	zshrc := writeTempRC(t, "")
	_, _ = shell.Inject(zshrc, testSocketPath)
	content, _ := os.ReadFile(zshrc)
	if strings.Contains(string(content), "nc -U") {
		t.Error("hook non deve dipendere da 'nc -U' (rimpiazzato da socket Python)")
	}
}

func writeTempRC(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		t.Fatalf("errore scrittura file temporaneo: %v", err)
	}
	return f
}
