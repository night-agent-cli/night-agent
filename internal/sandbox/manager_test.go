package sandbox_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/night-agent-cli/night-agent/internal/sandbox"
)

// dockerAvailable verifica se Docker è installato e il daemon è in esecuzione.
func dockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

// --- Unit tests (non richiedono Docker) ---

func TestNew_ReturnsManager(t *testing.T) {
	m := sandbox.New()
	if m == nil {
		t.Fatal("New() ha restituito nil")
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := sandbox.Config{}
	cfg.ApplyDefaults()

	if cfg.Image == "" {
		t.Error("ApplyDefaults() deve impostare Image")
	}
	if cfg.Network == "" {
		t.Error("ApplyDefaults() deve impostare Network")
	}
}

func TestConfig_DefaultImage(t *testing.T) {
	cfg := sandbox.Config{}
	cfg.ApplyDefaults()

	if cfg.Image != sandbox.DefaultImage {
		t.Errorf("DefaultImage atteso %q, ottenuto %q", sandbox.DefaultImage, cfg.Image)
	}
}

func TestConfig_DefaultNetwork(t *testing.T) {
	cfg := sandbox.Config{}
	cfg.ApplyDefaults()

	if cfg.Network != sandbox.DefaultNetwork {
		t.Errorf("DefaultNetwork atteso %q, ottenuto %q", sandbox.DefaultNetwork, cfg.Network)
	}
}

func TestConfig_UserValuesNotOverridden(t *testing.T) {
	cfg := sandbox.Config{
		Image:   "alpine:3.20",
		Network: "bridge",
	}
	cfg.ApplyDefaults()

	if cfg.Image != "alpine:3.20" {
		t.Errorf("Image utente non deve essere sovrascritta: ottenuto %q", cfg.Image)
	}
	if cfg.Network != "bridge" {
		t.Errorf("Network utente non deve essere sovrascritto: ottenuto %q", cfg.Network)
	}
}

func TestBuildDockerArgs_ContainsImage(t *testing.T) {
	cfg := sandbox.Config{
		Image:   "alpine:3.20",
		Network: "none",
	}
	args := sandbox.BuildDockerArgs("echo hello", cfg)

	found := false
	for _, a := range args {
		if a == "alpine:3.20" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("args Docker devono contenere l'immagine, ottenuto: %v", args)
	}
}

func TestBuildDockerArgs_ContainsNetwork(t *testing.T) {
	cfg := sandbox.Config{
		Image:   "alpine:3.20",
		Network: "none",
	}
	args := sandbox.BuildDockerArgs("echo hello", cfg)

	foundFlag := false
	foundValue := false
	for i, a := range args {
		if a == "--network" {
			foundFlag = true
			if i+1 < len(args) && args[i+1] == "none" {
				foundValue = true
			}
		}
	}
	if !foundFlag || !foundValue {
		t.Errorf("args devono contenere --network none, ottenuto: %v", args)
	}
}

func TestBuildDockerArgs_ContainsRm(t *testing.T) {
	cfg := sandbox.Config{Image: "alpine:3.20", Network: "none"}
	args := sandbox.BuildDockerArgs("echo hello", cfg)

	found := false
	for _, a := range args {
		if a == "--rm" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("args devono contenere --rm per pulizia automatica: %v", args)
	}
}

func TestBuildDockerArgs_WithWorkDir(t *testing.T) {
	cfg := sandbox.Config{
		Image:   "alpine:3.20",
		Network: "none",
		WorkDir: "/tmp/myproject",
	}
	args := sandbox.BuildDockerArgs("ls", cfg)

	foundMount := false
	for _, a := range args {
		if a == "/tmp/myproject:/workspace:rw" {
			foundMount = true
			break
		}
	}
	if !foundMount {
		t.Errorf("args devono contenere il mount workspace, ottenuto: %v", args)
	}
}

func TestBuildDockerArgs_WithoutWorkDir_NoWorkspaceMount(t *testing.T) {
	cfg := sandbox.Config{Image: "alpine:3.20", Network: "none"}
	args := sandbox.BuildDockerArgs("ls", cfg)

	// senza WorkDir non deve esserci il mount /workspace
	for i, a := range args {
		if a == "-v" && i+1 < len(args) && strings.Contains(args[i+1], "/workspace") {
			t.Errorf("senza WorkDir non deve esserci mount /workspace: %v", args)
		}
	}
}

func TestBuildDockerArgs_CommandWrappedInShell(t *testing.T) {
	cfg := sandbox.Config{Image: "alpine:3.20", Network: "none"}
	args := sandbox.BuildDockerArgs("echo hello world", cfg)

	// ultimi 3 elementi devono essere: sh -c "echo hello world"
	n := len(args)
	if n < 3 || args[n-3] != "sh" || args[n-2] != "-c" || args[n-1] != "echo hello world" {
		t.Errorf("comando deve essere wrappato in sh -c, ultimi args: %v", args[max(0, n-3):])
	}
}

// TestBuildDockerArgs_CredentialShielding verifica che i path credenziali
// vengano oscurati via tmpfs quando è presente un WorkDir.
func TestBuildDockerArgs_CredentialShielding(t *testing.T) {
	cfg := sandbox.Config{
		Image:   "alpine:3.20",
		Network: "none",
		WorkDir: "/home/user/myproject",
	}
	args := sandbox.BuildDockerArgs("ls", cfg)

	sshShielded := false
	awsShielded := false
	for i, a := range args {
		if a == "--tmpfs" && i+1 < len(args) {
			if args[i+1] == "/workspace/.ssh" {
				sshShielded = true
			}
			if args[i+1] == "/workspace/.aws" {
				awsShielded = true
			}
		}
	}
	if !sshShielded {
		t.Errorf("--tmpfs /workspace/.ssh mancante, args: %v", args)
	}
	if !awsShielded {
		t.Errorf("--tmpfs /workspace/.aws mancante, args: %v", args)
	}
}

// TestBuildDockerArgs_NoCredentialShielding_WithoutWorkDir verifica che
// senza WorkDir non vengano aggiunti tmpfs (nessun workspace montato).
func TestBuildDockerArgs_NoCredentialShielding_WithoutWorkDir(t *testing.T) {
	cfg := sandbox.Config{Image: "alpine:3.20", Network: "none"}
	args := sandbox.BuildDockerArgs("ls", cfg)

	for i, a := range args {
		if a == "--tmpfs" && i+1 < len(args) {
			t.Errorf("--tmpfs inatteso senza WorkDir: args[%d+1]=%s, tutti args: %v", i, args[i+1], args)
		}
	}
}

// --- Integration tests (richiedono Docker attivo) ---

func TestExecute_SimpleCommand(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker non disponibile, skip integration test")
	}

	m := sandbox.New()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := sandbox.Config{
		Image:   "alpine:3.20",
		Network: "none",
	}

	result, err := m.Execute(ctx, "echo hello", cfg)
	if err != nil {
		t.Fatalf("Execute() errore inatteso: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode atteso 0, ottenuto %d", result.ExitCode)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("Stdout atteso %q, ottenuto %q", "hello\n", result.Stdout)
	}
}

func TestExecute_CapturesExitCode(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker non disponibile, skip integration test")
	}

	m := sandbox.New()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := sandbox.Config{Image: "alpine:3.20", Network: "none"}
	result, err := m.Execute(ctx, "exit 42", cfg)
	if err != nil {
		t.Fatalf("Execute() errore inatteso: %v", err)
	}
	if result.ExitCode != 42 {
		t.Errorf("ExitCode atteso 42, ottenuto %d", result.ExitCode)
	}
}

func TestExecute_CapturesStderr(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker non disponibile, skip integration test")
	}

	m := sandbox.New()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := sandbox.Config{Image: "alpine:3.20", Network: "none"}
	result, err := m.Execute(ctx, "echo errore >&2", cfg)
	if err != nil {
		t.Fatalf("Execute() errore inatteso: %v", err)
	}
	if result.Stderr != "errore\n" {
		t.Errorf("Stderr atteso %q, ottenuto %q", "errore\n", result.Stderr)
	}
}

func TestExecute_NetworkIsolated(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker non disponibile, skip integration test")
	}

	m := sandbox.New()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Con network "none" non deve poter fare ping esterno
	cfg := sandbox.Config{Image: "alpine:3.20", Network: "none"}
	result, err := m.Execute(ctx, "ping -c 1 -W 1 8.8.8.8", cfg)
	if err != nil {
		t.Fatalf("Execute() errore inatteso: %v", err)
	}
	if result.ExitCode == 0 {
		t.Error("ping verso esterno non deve riuscire con network=none")
	}
}

func TestIsAvailable_ReturnsBool(t *testing.T) {
	m := sandbox.New()
	// non testiamo il valore (dipende dall'ambiente), solo che non panica
	_ = m.IsAvailable()
}

// max è helper per Go < 1.21
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
