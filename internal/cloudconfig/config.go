package cloudconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// defaultEndpoint è iniettato a compile time via ldflags:
//
//	go build -ldflags "-X github.com/pietroperona/night-agent/internal/cloudconfig.defaultEndpoint=https://..."
//
// Non esporre mai questo valore come flag CLI — l'utente non deve poterlo cambiare.
var defaultEndpoint = "https://api.nightagent.dev"

// Config contiene la configurazione per la connessione cloud.
// Salvata in ~/.night-agent/cloud.yaml.
type Config struct {
	Token      string    `yaml:"token"`
	Endpoint   string    `yaml:"endpoint"`
	MachineID  string    `yaml:"machine_id"`
	Cursor     string    `yaml:"cursor,omitempty"`      // ID ultimo evento sincronizzato
	LastSync   time.Time `yaml:"last_sync,omitempty"`   // timestamp ultimo sync riuscito
	Connected  bool      `yaml:"connected"`
}

// Load legge la configurazione da path. Se il file non esiste, restituisce
// una Config vuota senza errore.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{Endpoint: defaultEndpoint}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lettura cloud.yaml: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing cloud.yaml: %w", err)
	}
	// Endpoint sempre dal valore compilato — non modificabile dall'utente.
	cfg.Endpoint = defaultEndpoint
	return &cfg, nil
}

// Save scrive la configurazione su path (crea directory se mancante).
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creazione directory: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serializzazione cloud.yaml: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// Connect imposta il token e genera un machine_id se non già presente.
// Aggiorna il file di configurazione su disco.
func Connect(path, token string) (*Config, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}

	cfg.Token = token
	cfg.Connected = true

	if cfg.MachineID == "" {
		cfg.MachineID = uuid.New().String()
	}

	if err := Save(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Disconnect rimuove il token e segna la connessione come inattiva.
func Disconnect(path string) error {
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	cfg.Token = ""
	cfg.Connected = false
	return Save(path, cfg)
}

// UpdateCursor aggiorna il cursore e il timestamp dell'ultimo sync.
func UpdateCursor(path, cursor string) error {
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	cfg.Cursor = cursor
	cfg.LastSync = time.Now().UTC()
	return Save(path, cfg)
}
