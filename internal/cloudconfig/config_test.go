package cloudconfig_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pietroperona/night-agent/internal/cloudconfig"
)

func TestLoad_FileNotExist_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	cfg, err := cloudconfig.Load(filepath.Join(dir, "cloud.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Token != "" {
		t.Errorf("token atteso vuoto, got %q", cfg.Token)
	}
	if cfg.Endpoint == "" {
		t.Error("endpoint default atteso non vuoto")
	}
	if cfg.Connected {
		t.Error("connected atteso false su config vuota")
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cloud.yaml")

	cfg := &cloudconfig.Config{
		Token:     "tok-abc",
		Endpoint:  "https://api.example.com", // ignorato: Load usa sempre defaultEndpoint
		MachineID: "machine-123",
		Connected: true,
	}
	if err := cloudconfig.Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := cloudconfig.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Token != cfg.Token {
		t.Errorf("token: want %q, got %q", cfg.Token, loaded.Token)
	}
	// Endpoint sempre dal valore compilato, non dal file.
	if loaded.Endpoint == "" {
		t.Error("endpoint atteso non vuoto")
	}
	if loaded.MachineID != cfg.MachineID {
		t.Errorf("machine_id: want %q, got %q", cfg.MachineID, loaded.MachineID)
	}
	if !loaded.Connected {
		t.Error("connected atteso true")
	}
}

func TestSave_CreatesDirectoryIfMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "cloud.yaml")

	cfg := &cloudconfig.Config{Token: "tok", Connected: true}
	if err := cloudconfig.Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file non creato: %v", err)
	}
}

func TestSave_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cloud.yaml")

	if err := cloudconfig.Save(path, &cloudconfig.Config{Token: "tok"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("permessi attesi 0600, got %o", info.Mode().Perm())
	}
}

func TestConnect_SetsTokenAndMachineID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cloud.yaml")

	cfg, err := cloudconfig.Connect(path, "my-token")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if cfg.Token != "my-token" {
		t.Errorf("token: want 'my-token', got %q", cfg.Token)
	}
	if cfg.MachineID == "" {
		t.Error("machine_id non generato")
	}
	if !cfg.Connected {
		t.Error("connected atteso true dopo Connect")
	}
}

func TestConnect_PreservesMachineID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cloud.yaml")

	// primo connect — genera machine_id
	first, err := cloudconfig.Connect(path, "tok1")
	if err != nil {
		t.Fatalf("primo Connect: %v", err)
	}

	// secondo connect (rinnovo token) — machine_id invariato
	second, err := cloudconfig.Connect(path, "tok2")
	if err != nil {
		t.Fatalf("secondo Connect: %v", err)
	}
	if first.MachineID != second.MachineID {
		t.Errorf("machine_id cambiato: %q → %q", first.MachineID, second.MachineID)
	}
	if second.Token != "tok2" {
		t.Errorf("token non aggiornato: got %q", second.Token)
	}
}

func TestDisconnect_ClearsToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cloud.yaml")

	cloudconfig.Connect(path, "tok")
	if err := cloudconfig.Disconnect(path); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}

	cfg, _ := cloudconfig.Load(path)
	if cfg.Token != "" {
		t.Errorf("token atteso vuoto dopo Disconnect, got %q", cfg.Token)
	}
	if cfg.Connected {
		t.Error("connected atteso false dopo Disconnect")
	}
}

func TestUpdateCursor_SetsCursorAndLastSync(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cloud.yaml")

	cloudconfig.Save(path, &cloudconfig.Config{Token: "tok", Connected: true})
	before := time.Now()

	if err := cloudconfig.UpdateCursor(path, "event-99"); err != nil {
		t.Fatalf("UpdateCursor: %v", err)
	}

	cfg, _ := cloudconfig.Load(path)
	if cfg.Cursor != "event-99" {
		t.Errorf("cursor: want 'event-99', got %q", cfg.Cursor)
	}
	if cfg.LastSync.Before(before) {
		t.Error("LastSync non aggiornato")
	}
}
