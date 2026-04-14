package policy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// Source indica da dove è stata caricata la policy.
type Source int

const (
	SourceNone   Source = iota // nessuna policy trovata — tutto consentito
	SourceCloud                // scaricata dal cloud
	SourceLocal                // file locale nel progetto (walk-up)
	SourceGlobal               // ~/.night-agent/policy.yaml
)

func (s Source) String() string {
	switch s {
	case SourceCloud:
		return "cloud"
	case SourceLocal:
		return "local"
	case SourceGlobal:
		return "global"
	default:
		return "none"
	}
}

// LoadedPolicy è il risultato del caricamento con metadati sulla sorgente.
type LoadedPolicy struct {
	*Policy
	Source Source
	Path   string // path file oppure "cloud:<machine_id>"
}

// CloudClient è l'interfaccia per scaricare la policy dal cloud.
// Facilita il mock nei test.
type CloudClient interface {
	FetchPolicy(machineID string) ([]byte, error)
}

// HTTPCloudClient implementa CloudClient con chiamate HTTP reali.
type HTTPCloudClient struct {
	Endpoint  string
	Token     string
}

type cloudPolicyResponse struct {
	MachineID  string  `json:"machine_id"`
	PolicyYAML *string `json:"policy_yaml"`
}

func (c *HTTPCloudClient) FetchPolicy(machineID string) ([]byte, error) {
	url := c.Endpoint + "/api/policy?machine_id=" + machineID
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cloud policy: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var r cloudPolicyResponse
	if err := json.Unmarshal(body, &r); err != nil {
		// risposta non JSON — prova a trattarla come YAML diretto
		return body, nil
	}
	if r.PolicyYAML == nil {
		return nil, fmt.Errorf("cloud policy: policy_yaml è null")
	}
	return []byte(*r.PolicyYAML), nil
}

// localPolicyNames sono i nomi file cercati risalendo i parent.
var localPolicyNames = []string{"nightagent-policy.yaml", ".nightagent/policy.yaml"}

// Load carica la policy con la seguente priorità:
//  1. Cloud (se client != nil e machineID != "")
//  2. File locale (nightagent-policy.yaml, walk-up fino a home)
//  3. ~/.night-agent/policy.yaml (globale)
//  4. SourceNone — nessun errore, tutto consentito
//
// Se il cloud fallisce, logga il warning e scende alla priorità successiva.
func Load(workDir string, client CloudClient, machineID string) (*LoadedPolicy, error) {
	// 1. Cloud
	if client != nil && machineID != "" {
		if yamlBytes, err := client.FetchPolicy(machineID); err == nil {
			if p, err := LoadBytes(yamlBytes); err == nil {
				return &LoadedPolicy{
					Policy: p,
					Source: SourceCloud,
					Path:   "cloud:" + machineID,
				}, nil
			}
		}
		// fallthrough silenzioso — rete down, 404, policy non valida
	}

	// 2. Locale — walk-up da workDir fino a home
	home, _ := os.UserHomeDir()
	dir := workDir
	for {
		for _, name := range localPolicyNames {
			candidate := filepath.Join(dir, name)
			if data, err := os.ReadFile(candidate); err == nil {
				if p, err := LoadBytes(data); err == nil {
					return &LoadedPolicy{Policy: p, Source: SourceLocal, Path: candidate}, nil
				}
			}
		}
		if dir == home || dir == filepath.Dir(dir) {
			break
		}
		dir = filepath.Dir(dir)
	}

	// 3. Globale
	if home != "" {
		globalPath := filepath.Join(home, ".night-agent", "policy.yaml")
		if data, err := os.ReadFile(globalPath); err == nil {
			if p, err := LoadBytes(data); err == nil {
				return &LoadedPolicy{Policy: p, Source: SourceGlobal, Path: globalPath}, nil
			}
		}
	}

	// 4. Nessuna policy — permissive
	return &LoadedPolicy{Source: SourceNone, Path: ""}, nil
}

// FormatSource restituisce la stringa di log per la policy caricata.
func FormatSource(lp *LoadedPolicy) string {
	switch lp.Source {
	case SourceCloud:
		machineID := lp.Path
		if len(machineID) > len("cloud:") {
			machineID = machineID[len("cloud:"):]
		}
		return fmt.Sprintf("[policy] loaded from cloud (machine: %s)", machineID)
	case SourceLocal:
		return fmt.Sprintf("[policy] loaded from %s", lp.Path)
	case SourceGlobal:
		home, _ := os.UserHomeDir()
		path := lp.Path
		if home != "" {
			if rel, err := filepath.Rel(home, lp.Path); err == nil {
				path = "~/" + rel
			}
		}
		return fmt.Sprintf("[policy] loaded from %s (global)", path)
	default:
		return "[policy] no policy found — all actions allowed"
	}
}

// Watch avvia un watcher fs su workDir per nightagent-policy.yaml.
// Quando il file viene creato, modificato o eliminato, chiama onChange con
// la policy ricalcolata (usando Load con gli stessi parametri).
// Ritorna una funzione stop per il cleanup. Fail-open: errori del watcher
// vengono ignorati silenziosamente.
func Watch(workDir string, client CloudClient, machineID string, onChange func(*LoadedPolicy)) (func(), error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return func() {}, fmt.Errorf("fsnotify: %w", err)
	}

	if err := watcher.Add(workDir); err != nil {
		watcher.Close()
		return func() {}, fmt.Errorf("watch %s: %w", workDir, err)
	}

	stop := make(chan struct{})

	go func() {
		defer watcher.Close()
		for {
			select {
			case <-stop:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				base := filepath.Base(event.Name)
				if base != "nightagent-policy.yaml" {
					continue
				}
				if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					if lp, err := Load(workDir, client, machineID); err == nil {
						onChange(lp)
					}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
				// errori watcher ignorati silenziosamente
			}
		}
	}()

	return func() { close(stop) }, nil
}
