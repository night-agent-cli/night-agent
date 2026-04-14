package policy_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pietroperona/night-agent/internal/policy"
)

// mockClient implementa CloudClient per i test.
type mockClient struct {
	yaml []byte
	err  error
}

func (m *mockClient) FetchPolicy(_ string) ([]byte, error) {
	return m.yaml, m.err
}

var validPolicy = []byte(`
version: 1
rules:
  - id: test_block
    when:
      action_type: shell
      command_matches: ["sudo *"]
    match_type: glob
    decision: block
    reason: test
`)

func TestLoadCloud(t *testing.T) {
	client := &mockClient{yaml: validPolicy}
	lp, err := policy.Load(t.TempDir(), client, "machine-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if lp.Source != policy.SourceCloud {
		t.Errorf("source: want Cloud, got %v", lp.Source)
	}
	if lp.Path != "cloud:machine-1" {
		t.Errorf("path: want 'cloud:machine-1', got %q", lp.Path)
	}
}

func TestLoadCloudFallbackLocal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nightagent-policy.yaml"), validPolicy, 0600); err != nil {
		t.Fatal(err)
	}

	client := &mockClient{err: fmt.Errorf("404")}
	lp, err := policy.Load(dir, client, "machine-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if lp.Source != policy.SourceLocal {
		t.Errorf("source: want Local, got %v", lp.Source)
	}
}

func TestLoadLocal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nightagent-policy.yaml"), validPolicy, 0600); err != nil {
		t.Fatal(err)
	}

	lp, err := policy.Load(dir, nil, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if lp.Source != policy.SourceLocal {
		t.Errorf("source: want Local, got %v", lp.Source)
	}
	if lp.Path != filepath.Join(dir, "nightagent-policy.yaml") {
		t.Errorf("path: want %q, got %q", filepath.Join(dir, "nightagent-policy.yaml"), lp.Path)
	}
}

func TestLoadLocalParent(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "subdir")
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "nightagent-policy.yaml"), validPolicy, 0600); err != nil {
		t.Fatal(err)
	}

	lp, err := policy.Load(child, nil, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if lp.Source != policy.SourceLocal {
		t.Errorf("source: want Local, got %v", lp.Source)
	}
	if lp.Path != filepath.Join(parent, "nightagent-policy.yaml") {
		t.Errorf("path: want parent policy, got %q", lp.Path)
	}
}

func TestLoadGlobal(t *testing.T) {
	dir := t.TempDir() // no local policy

	lp, err := policy.Load(dir, nil, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// può essere Global o None a seconda di ~/.night-agent/policy.yaml
	if lp.Source != policy.SourceGlobal && lp.Source != policy.SourceNone {
		t.Errorf("source: want Global or None, got %v", lp.Source)
	}
}

func TestLoadNone(t *testing.T) {
	// dir isolata senza policy, nessun cloud, home senza ~/.night-agent/policy.yaml
	// Non possiamo garantire assenza di ~/.night-agent/policy.yaml nell'ambiente CI,
	// quindi verifichiamo solo che non restituisca errore.
	lp, err := policy.Load(t.TempDir(), nil, "")
	if err != nil {
		t.Fatalf("Load non deve restituire errore: %v", err)
	}
	if lp == nil {
		t.Fatal("LoadedPolicy non deve essere nil")
	}
}

func TestLoadPriorityCloudOverLocal(t *testing.T) {
	dir := t.TempDir()
	// crea policy locale
	if err := os.WriteFile(filepath.Join(dir, "nightagent-policy.yaml"), validPolicy, 0600); err != nil {
		t.Fatal(err)
	}

	// cloud risponde con policy valida
	client := &mockClient{yaml: validPolicy}
	lp, err := policy.Load(dir, client, "machine-cloud")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if lp.Source != policy.SourceCloud {
		t.Errorf("cloud deve vincere su locale: got source %v", lp.Source)
	}
}
