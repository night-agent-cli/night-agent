package launchagent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pietroperona/agent-guardian/internal/launchagent"
)

func TestPlistPath(t *testing.T) {
	path := launchagent.PlistPath("/home/user")
	expected := "/home/user/Library/LaunchAgents/com.guardian.daemon.plist"
	if path != expected {
		t.Errorf("atteso %s, ottenuto %s", expected, path)
	}
}

func TestGeneratePlist_ContainsLabel(t *testing.T) {
	plist := launchagent.GeneratePlist("/usr/local/bin/guardian", "/home/user/.guardian")
	if !strings.Contains(plist, "com.guardian.daemon") {
		t.Error("plist non contiene il label com.guardian.daemon")
	}
}

func TestGeneratePlist_ContainsBinaryPath(t *testing.T) {
	binaryPath := "/usr/local/bin/guardian"
	plist := launchagent.GeneratePlist(binaryPath, "/home/user/.guardian")
	if !strings.Contains(plist, binaryPath) {
		t.Errorf("plist non contiene il path del binario %s", binaryPath)
	}
}

func TestGeneratePlist_ContainsStartCommand(t *testing.T) {
	plist := launchagent.GeneratePlist("/usr/local/bin/guardian", "/home/user/.guardian")
	if !strings.Contains(plist, "start") {
		t.Error("plist non contiene il sottocomando 'start'")
	}
}

func TestGeneratePlist_ContainsLogPaths(t *testing.T) {
	guardianDir := "/home/user/.guardian"
	plist := launchagent.GeneratePlist("/usr/local/bin/guardian", guardianDir)
	if !strings.Contains(plist, guardianDir) {
		t.Errorf("plist non contiene la guardian dir %s", guardianDir)
	}
}

func TestGeneratePlist_RunAtLoad(t *testing.T) {
	plist := launchagent.GeneratePlist("/usr/local/bin/guardian", "/home/user/.guardian")
	if !strings.Contains(plist, "RunAtLoad") {
		t.Error("plist non contiene RunAtLoad")
	}
	if !strings.Contains(plist, "<true/>") {
		t.Error("plist non ha RunAtLoad impostato a true")
	}
}

func TestInstall_WritesFile(t *testing.T) {
	dir := t.TempDir()
	launchAgentsDir := filepath.Join(dir, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	plistPath := filepath.Join(launchAgentsDir, "com.guardian.daemon.plist")
	plist := launchagent.GeneratePlist("/usr/local/bin/guardian", dir+"/.guardian")
	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		t.Fatalf("errore scrittura plist: %v", err)
	}

	data, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("errore lettura plist: %v", err)
	}
	if !strings.Contains(string(data), "com.guardian.daemon") {
		t.Error("file plist scritto non contiene il label atteso")
	}
}

func TestIsInstalled_True(t *testing.T) {
	dir := t.TempDir()
	plistPath := launchagent.PlistPath(dir)
	launchAgentsDir := filepath.Dir(plistPath)
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plistPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	if !launchagent.IsInstalled(dir) {
		t.Error("atteso IsInstalled=true, ottenuto false")
	}
}

func TestIsInstalled_False(t *testing.T) {
	dir := t.TempDir()
	if launchagent.IsInstalled(dir) {
		t.Error("atteso IsInstalled=false, ottenuto true")
	}
}
