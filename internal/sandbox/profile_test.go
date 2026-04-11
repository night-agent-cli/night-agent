package sandbox_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pietroperona/agent-guardian/internal/sandbox"
)

// --- TestLoadProfile ---

func TestLoadProfile_FileNotFound_ReturnsNil(t *testing.T) {
	profile, err := sandbox.LoadProfile("/nonexistent/dir")
	if err != nil {
		t.Fatalf("atteso nil error per file mancante, ottenuto: %v", err)
	}
	if profile != nil {
		t.Error("atteso nil profile quando .guardian.yaml non esiste")
	}
}

func TestLoadProfile_ValidFile(t *testing.T) {
	dir := t.TempDir()
	content := `sandbox:
  default_image: "node:20-alpine"
  default_network: "bridge"
  env:
    - "NODE_ENV=test"
`
	writeGuardianYAML(t, dir, content)

	profile, err := sandbox.LoadProfile(dir)
	if err != nil {
		t.Fatalf("errore caricamento profilo: %v", err)
	}
	if profile == nil {
		t.Fatal("atteso profilo non nil")
	}
	if profile.DefaultImage != "node:20-alpine" {
		t.Errorf("DefaultImage: atteso %q, ottenuto %q", "node:20-alpine", profile.DefaultImage)
	}
	if profile.DefaultNetwork != "bridge" {
		t.Errorf("DefaultNetwork: atteso %q, ottenuto %q", "bridge", profile.DefaultNetwork)
	}
	if len(profile.Env) != 1 || profile.Env[0] != "NODE_ENV=test" {
		t.Errorf("Env: atteso [NODE_ENV=test], ottenuto %v", profile.Env)
	}
}

func TestLoadProfile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeGuardianYAML(t, dir, "sandbox: {invalid yaml [[[")

	_, err := sandbox.LoadProfile(dir)
	if err == nil {
		t.Fatal("atteso errore per YAML non valido")
	}
}

func TestLoadProfile_WithMounts(t *testing.T) {
	dir := t.TempDir()
	content := `sandbox:
  default_image: "alpine:3.20"
  default_network: "none"
  mounts:
    - source: "./data"
      target: "/data"
      readonly: true
    - source: "./config"
      target: "/config"
      readonly: false
`
	writeGuardianYAML(t, dir, content)

	profile, err := sandbox.LoadProfile(dir)
	if err != nil {
		t.Fatalf("errore: %v", err)
	}
	if len(profile.Mounts) != 2 {
		t.Fatalf("attesi 2 mount, ottenuti %d", len(profile.Mounts))
	}
	if profile.Mounts[0].Source != "./data" {
		t.Errorf("Mount[0].Source: atteso %q, ottenuto %q", "./data", profile.Mounts[0].Source)
	}
	if !profile.Mounts[0].Readonly {
		t.Error("Mount[0].Readonly: atteso true")
	}
}

// --- TestMergeConfig ---

func TestMergeConfig_NoProfile_ConfigUnchanged(t *testing.T) {
	cfg := sandbox.Config{Image: "alpine:3.20", Network: "none"}
	merged := sandbox.MergeConfig(cfg, nil)

	if merged.Image != "alpine:3.20" {
		t.Errorf("Image: atteso %q, ottenuto %q", "alpine:3.20", merged.Image)
	}
}

func TestMergeConfig_ProfileDefaultsApplied(t *testing.T) {
	cfg := sandbox.Config{} // nessuna preferenza dalla regola
	profile := &sandbox.Profile{
		DefaultImage:   "node:20-alpine",
		DefaultNetwork: "bridge",
	}
	merged := sandbox.MergeConfig(cfg, profile)

	if merged.Image != "node:20-alpine" {
		t.Errorf("Image: atteso profilo %q, ottenuto %q", "node:20-alpine", merged.Image)
	}
	if merged.Network != "bridge" {
		t.Errorf("Network: atteso profilo %q, ottenuto %q", "bridge", merged.Network)
	}
}

func TestMergeConfig_RuleOverridesProfile(t *testing.T) {
	cfg := sandbox.Config{Image: "python:3.12-alpine", Network: "none"}
	profile := &sandbox.Profile{
		DefaultImage:   "node:20-alpine",
		DefaultNetwork: "bridge",
	}
	merged := sandbox.MergeConfig(cfg, profile)

	// la regola ha priorità sul profilo
	if merged.Image != "python:3.12-alpine" {
		t.Errorf("Image: la regola deve avere priorità, ottenuto %q", merged.Image)
	}
	if merged.Network != "none" {
		t.Errorf("Network: la regola deve avere priorità, ottenuto %q", merged.Network)
	}
}

func TestMergeConfig_EnvFromProfileAppended(t *testing.T) {
	cfg := sandbox.Config{Image: "alpine:3.20", Network: "none"}
	profile := &sandbox.Profile{
		DefaultImage: "alpine:3.20",
		Env:          []string{"NODE_ENV=test", "DEBUG=1"},
	}
	merged := sandbox.MergeConfig(cfg, profile)

	if len(merged.Env) != 2 {
		t.Errorf("Env: attesi 2 elementi, ottenuti %d", len(merged.Env))
	}
}

func TestMergeConfig_ExtraMountsFromProfileAppended(t *testing.T) {
	cfg := sandbox.Config{Image: "alpine:3.20", Network: "none", WorkDir: "/tmp/proj"}
	profile := &sandbox.Profile{
		DefaultImage: "alpine:3.20",
		Mounts: []sandbox.ProfileMount{
			{Source: "./data", Target: "/data", Readonly: true},
		},
	}
	merged := sandbox.MergeConfig(cfg, profile)

	if len(merged.ExtraMounts) != 1 {
		t.Errorf("ExtraMounts: atteso 1, ottenuto %d", len(merged.ExtraMounts))
	}
	if merged.ExtraMounts[0].Target != "/data" {
		t.Errorf("ExtraMounts[0].Target: atteso /data, ottenuto %q", merged.ExtraMounts[0].Target)
	}
	if !merged.ExtraMounts[0].Readonly {
		t.Error("ExtraMounts[0].Readonly: atteso true")
	}
}

// --- TestBuildDockerArgs con ExtraMounts ---

func TestBuildDockerArgs_ExtraMountsReadOnly(t *testing.T) {
	cfg := sandbox.Config{
		Image:   "alpine:3.20",
		Network: "none",
		WorkDir: "/tmp/proj",
		ExtraMounts: []sandbox.Mount{
			{Source: "/tmp/proj/data", Target: "/data", Readonly: true},
		},
	}
	args := sandbox.BuildDockerArgs("ls /data", cfg)

	found := false
	for _, a := range args {
		if a == "/tmp/proj/data:/data:ro" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("atteso mount read-only /tmp/proj/data:/data:ro, ottenuto: %v", args)
	}
}

func TestBuildDockerArgs_ContainsGuardianLabel(t *testing.T) {
	cfg := sandbox.Config{Image: "alpine:3.20", Network: "none"}
	args := sandbox.BuildDockerArgs("echo hello", cfg)

	foundLabel := false
	for i, a := range args {
		if a == "--label" && i+1 < len(args) && args[i+1] == "guardian.sandbox=true" {
			foundLabel = true
			break
		}
	}
	if !foundLabel {
		t.Errorf("args devono contenere --label guardian.sandbox=true, ottenuto: %v", args)
	}
}

// --- helpers ---

func writeGuardianYAML(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, sandbox.ProfileFileName)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("errore scrittura .guardian.yaml: %v", err)
	}
}
