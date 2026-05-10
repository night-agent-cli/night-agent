package configdir_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/night-agent-cli/night-agent/internal/configdir"
)

func TestResolve_LocalExists_ReturnsLocal(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, configdir.LocalDirName)
	if err := os.Mkdir(local, 0700); err != nil {
		t.Fatal(err)
	}
	// deve esistere policy.yaml per essere considerata config locale valida
	if err := os.WriteFile(filepath.Join(local, "policy.yaml"), []byte("version: 1\nrules: []\n"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := configdir.Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != local {
		t.Errorf("want %q, got %q", local, got)
	}
}

// TestResolve_IgnoresDirWithoutPolicy verifica che una dir .nightagent senza
// policy.yaml (es. quella creata dal daemon per il socket) non venga scambiata
// per una config locale di progetto.
func TestResolve_IgnoresDirWithoutPolicy(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, configdir.LocalDirName)
	if err := os.Mkdir(local, 0700); err != nil {
		t.Fatal(err)
	}
	// solo daemon.sock, nessuna policy.yaml

	got, err := configdir.Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	global, _ := configdir.Global()
	if got != global {
		t.Errorf("atteso fallback global %q, got %q", global, got)
	}
}

func TestResolve_NoLocal_ReturnsGlobal(t *testing.T) {
	dir := t.TempDir() // no .nightagent/ inside

	got, err := configdir.Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	global, _ := configdir.Global()
	if got != global {
		t.Errorf("want global %q, got %q", global, got)
	}
}

func TestResolve_LocalFile_NotDir_FallsBackToGlobal(t *testing.T) {
	dir := t.TempDir()
	// crea un file (non dir) col nome .nightagent
	localPath := filepath.Join(dir, configdir.LocalDirName)
	if err := os.WriteFile(localPath, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := configdir.Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	global, _ := configdir.Global()
	if got != global {
		t.Errorf("atteso fallback global %q, got %q", global, got)
	}
}

func TestCreateLocal_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	got, err := configdir.CreateLocal(dir)
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	expected := filepath.Join(dir, configdir.LocalDirName)
	if got != expected {
		t.Errorf("want %q, got %q", expected, got)
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatalf("dir non creata: %v", err)
	}
	if !info.IsDir() {
		t.Error("atteso directory")
	}
}

func TestCreateLocal_Idempotent(t *testing.T) {
	dir := t.TempDir()
	if _, err := configdir.CreateLocal(dir); err != nil {
		t.Fatal(err)
	}
	// seconda chiamata non deve dare errore
	if _, err := configdir.CreateLocal(dir); err != nil {
		t.Errorf("seconda CreateLocal: %v", err)
	}
}

func TestIsLocal(t *testing.T) {
	if !configdir.IsLocal("/some/project/.nightagent") {
		t.Error("atteso true per .nightagent")
	}
	if configdir.IsLocal("/home/user/.night-agent") {
		t.Error("atteso false per .night-agent")
	}
}
