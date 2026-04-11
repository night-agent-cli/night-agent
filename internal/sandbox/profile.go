package sandbox

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProfileFileName è il nome del file di configurazione sandbox per progetto.
const ProfileFileName = ".guardian.yaml"

// Profile contiene la configurazione sandbox specifica per un progetto.
// Viene letta da <workDir>/.guardian.yaml e ha priorità sui default globali,
// ma le SandboxConfig delle singole regole hanno priorità sul profilo.
type Profile struct {
	DefaultImage   string         `yaml:"default_image"`
	DefaultNetwork string         `yaml:"default_network"`
	Mounts         []ProfileMount `yaml:"mounts"`
	Env            []string       `yaml:"env"`
}

// ProfileMount descrive un mount aggiuntivo oltre al workspace principale.
type ProfileMount struct {
	Source   string `yaml:"source"`   // path relativo o assoluto sull'host
	Target   string `yaml:"target"`   // path nel container
	Readonly bool   `yaml:"readonly"` // true → montato :ro
}

// profileFile è la struttura YAML con la chiave radice "sandbox".
type profileFile struct {
	Sandbox *Profile `yaml:"sandbox"`
}

// LoadProfile carica il profilo sandbox da <workDir>/.guardian.yaml.
// Restituisce nil, nil se il file non esiste (nessun profilo di progetto).
func LoadProfile(workDir string) (*Profile, error) {
	path := filepath.Join(workDir, ProfileFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("errore lettura %s: %w", ProfileFileName, err)
	}

	var f profileFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("YAML non valido in %s: %w", ProfileFileName, err)
	}

	return f.Sandbox, nil
}

// MergeConfig unisce la Config della regola con il profilo di progetto.
// Ordine di priorità (dal più alto): regola > profilo > default globale.
func MergeConfig(cfg Config, profile *Profile) Config {
	if profile == nil {
		return cfg
	}

	// image e network: il profilo si applica solo se la regola non ha preferenze
	if cfg.Image == "" && profile.DefaultImage != "" {
		cfg.Image = profile.DefaultImage
	}
	if cfg.Network == "" && profile.DefaultNetwork != "" {
		cfg.Network = profile.DefaultNetwork
	}

	// env: aggiunti sempre dal profilo
	cfg.Env = append(cfg.Env, profile.Env...)

	// mount extra dal profilo → convertiti in Mount
	for _, pm := range profile.Mounts {
		source := pm.Source
		// path relativi risolti rispetto al workDir (se disponibile)
		if cfg.WorkDir != "" && !filepath.IsAbs(source) {
			source = filepath.Join(cfg.WorkDir, source)
		}
		cfg.ExtraMounts = append(cfg.ExtraMounts, Mount{
			Source:   source,
			Target:   pm.Target,
			Readonly: pm.Readonly,
		})
	}

	return cfg
}
