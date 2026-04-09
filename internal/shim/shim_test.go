package shim_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pietroperona/agent-guardian/internal/shim"
)

func TestShimDir(t *testing.T) {
	dir := shim.ShimDir("/home/user/.guardian")
	expected := "/home/user/.guardian/shims"
	if dir != expected {
		t.Errorf("atteso %s, ottenuto %s", expected, dir)
	}
}

func TestPrependPath_NoExistingPath(t *testing.T) {
	shimDir := "/tmp/guardian-shims"
	env := []string{"HOME=/home/user"}
	result := shim.PrependPath(env, shimDir)

	for _, e := range result {
		if e == "PATH="+shimDir {
			return
		}
	}
	t.Errorf("PATH=%s non trovato nell'env risultante: %v", shimDir, result)
}

func TestPrependPath_ExistingPath(t *testing.T) {
	shimDir := "/tmp/guardian-shims"
	env := []string{"PATH=/usr/bin:/bin"}
	result := shim.PrependPath(env, shimDir)

	expected := "PATH=" + shimDir + ":/usr/bin:/bin"
	for _, e := range result {
		if e == expected {
			return
		}
	}
	t.Errorf("atteso %s nell'env, non trovato. env: %v", expected, result)
}

func TestPrependPath_AlreadyFirst(t *testing.T) {
	shimDir := "/tmp/guardian-shims"
	env := []string{"PATH=" + shimDir + ":/usr/bin:/bin"}
	result := shim.PrependPath(env, shimDir)

	count := 0
	for _, e := range result {
		if len(e) > 5 && e[:5] == "PATH=" {
			count++
			expected := "PATH=" + shimDir + ":/usr/bin:/bin"
			if e != expected {
				t.Errorf("atteso %s, ottenuto %s", expected, e)
			}
		}
	}
	if count != 1 {
		t.Errorf("attesa esattamente 1 occorrenza di PATH, trovate %d", count)
	}
}

func TestPrependPath_PreservesOtherVars(t *testing.T) {
	shimDir := "/tmp/guardian-shims"
	env := []string{"HOME=/home/user", "PATH=/usr/bin", "TERM=xterm"}
	result := shim.PrependPath(env, shimDir)

	foundHome := false
	foundTerm := false
	for _, e := range result {
		if e == "HOME=/home/user" {
			foundHome = true
		}
		if e == "TERM=xterm" {
			foundTerm = true
		}
	}
	if !foundHome {
		t.Error("HOME non preservata nell'env risultante")
	}
	if !foundTerm {
		t.Error("TERM non preservata nell'env risultante")
	}
}

func TestCreateSymlinks(t *testing.T) {
	shimDir := t.TempDir()
	fakeBinary := filepath.Join(shimDir, "guardian-shim")
	if err := os.WriteFile(fakeBinary, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := shim.CreateSymlinks(shimDir, fakeBinary); err != nil {
		t.Fatalf("CreateSymlinks fallita: %v", err)
	}

	for _, cmd := range shim.ShimmedCommands {
		linkPath := filepath.Join(shimDir, cmd)
		target, err := os.Readlink(linkPath)
		if err != nil {
			t.Errorf("symlink per %s non trovata: %v", cmd, err)
			continue
		}
		if target != fakeBinary {
			t.Errorf("symlink %s punta a %s invece di %s", cmd, target, fakeBinary)
		}
	}
}

func TestCreateSymlinks_Idempotent(t *testing.T) {
	shimDir := t.TempDir()
	fakeBinary := filepath.Join(shimDir, "guardian-shim")
	if err := os.WriteFile(fakeBinary, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}

	// prima installazione
	if err := shim.CreateSymlinks(shimDir, fakeBinary); err != nil {
		t.Fatalf("prima CreateSymlinks fallita: %v", err)
	}
	// seconda installazione — deve sovrascrivere senza errori
	if err := shim.CreateSymlinks(shimDir, fakeBinary); err != nil {
		t.Fatalf("seconda CreateSymlinks fallita: %v", err)
	}
}

func TestShimmedCommands_NotEmpty(t *testing.T) {
	if len(shim.ShimmedCommands) == 0 {
		t.Error("ShimmedCommands non deve essere vuota")
	}
	// verifica che sudo e rm siano sempre presenti
	found := map[string]bool{}
	for _, cmd := range shim.ShimmedCommands {
		found[cmd] = true
	}
	for _, required := range []string{"sudo", "rm", "git", "curl"} {
		if !found[required] {
			t.Errorf("comando richiesto '%s' non presente in ShimmedCommands", required)
		}
	}
}
