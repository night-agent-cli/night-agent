package shell_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pietroperona/agent-guardian/internal/shell"
)

const testSocketPath = "/tmp/guardian-test.sock"

func TestInject_AddsHookToZshrc(t *testing.T) {
	zshrc := writeTempRC(t, "# existing config\nexport PATH=$PATH:/usr/local/bin\n")

	if err := shell.Inject(zshrc, testSocketPath); err != nil {
		t.Fatalf("errore iniezione: %v", err)
	}

	content, _ := os.ReadFile(zshrc)
	if !strings.Contains(string(content), "guardian") {
		t.Error("atteso hook guardian nel file")
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

	if err := shell.Inject(zshrc, testSocketPath); err != nil {
		t.Fatalf("prima iniezione fallita: %v", err)
	}
	if err := shell.Inject(zshrc, testSocketPath); err != nil {
		t.Fatalf("seconda iniezione fallita: %v", err)
	}

	content, _ := os.ReadFile(zshrc)
	count := strings.Count(string(content), "# BEGIN guardian")
	if count != 1 {
		t.Errorf("atteso 1 blocco guardian, trovati %d", count)
	}
}

func TestInject_FileNotFound(t *testing.T) {
	err := shell.Inject("/nonexistent/path/.zshrc", testSocketPath)
	if err == nil {
		t.Fatal("atteso errore per file mancante, ottenuto nil")
	}
}

func TestRemove_RemovesHookFromZshrc(t *testing.T) {
	zshrc := writeTempRC(t, "# existing config\n")

	_ = shell.Inject(zshrc, testSocketPath)
	if err := shell.Remove(zshrc); err != nil {
		t.Fatalf("errore rimozione: %v", err)
	}

	content, _ := os.ReadFile(zshrc)
	if strings.Contains(string(content), "guardian") {
		t.Error("guardian non dovrebbe essere presente dopo la rimozione")
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
	_ = shell.Inject(zshrc, testSocketPath)

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

func writeTempRC(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		t.Fatalf("errore scrittura file temporaneo: %v", err)
	}
	return f
}
