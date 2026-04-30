package cloudconfig_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/night-agent-cli/night-agent/internal/audit"
	"github.com/night-agent-cli/night-agent/internal/cloudconfig"
)

func TestRemoteSigner_RemoteSuccess(t *testing.T) {
	// Mock HTTP server che restituisce sig
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/sign" {
			t.Errorf("path=%s", r.URL.Path)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"sig": "fakesig123"})
	}))
	defer srv.Close()

	cfg := &cloudconfig.Config{
		Endpoint:  srv.URL,
		Token:     "testtoken",
		MachineID: "machine-1",
		Connected: true,
	}
	rs := cloudconfig.NewRemoteSigner(cfg)
	fn := rs.SignFunc(nil) // nil = nessun fallback locale

	e := audit.Event{ID: "evt-1", Decision: "allow"}
	sig, source, err := fn(e)
	if err != nil {
		t.Fatal(err)
	}
	if sig != "fakesig123" {
		t.Errorf("sig=%s, want fakesig123", sig)
	}
	if source != "remote" {
		t.Errorf("source=%s, want remote", source)
	}
}

func TestRemoteSigner_TimeoutFallback(t *testing.T) {
	// Server che non risponde mai → fallback locale
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second) // > 3s timeout
	}))
	defer srv.Close()

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signing.key")
	audit.GenerateKey(keyPath)
	localSigner, _ := audit.NewSigner(keyPath)

	cfg := &cloudconfig.Config{Endpoint: srv.URL, Token: "t", MachineID: "m1", Connected: true}
	rs := cloudconfig.NewRemoteSigner(cfg)
	fn := rs.SignFunc(localSigner)

	e := audit.Event{ID: "evt-2", Decision: "block"}
	_, source, err := fn(e)
	if err != nil {
		t.Fatal(err) // fallback non deve restituire errore
	}
	if source != "local" {
		t.Errorf("source=%s, want local (fallback)", source)
	}
}

func TestRemoteSigner_NoCloudFallback(t *testing.T) {
	// Server che risponde 500 → fallback locale
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signing.key")
	audit.GenerateKey(keyPath)
	localSigner, _ := audit.NewSigner(keyPath)

	cfg := &cloudconfig.Config{Endpoint: srv.URL, Token: "t", MachineID: "m1", Connected: true}
	rs := cloudconfig.NewRemoteSigner(cfg)
	fn := rs.SignFunc(localSigner)

	_, source, err := fn(audit.Event{ID: "evt-3", Decision: "allow"})
	if err != nil {
		t.Fatal(err)
	}
	if source != "local" {
		t.Errorf("source=%s, want local", source)
	}
}

func TestIsConnected(t *testing.T) {
	tests := []struct {
		name string
		cfg  *cloudconfig.Config
		want bool
	}{
		{"nil config", nil, false},
		{"empty config", &cloudconfig.Config{}, false},
		{"connected=true but no token", &cloudconfig.Config{Connected: true, MachineID: "m"}, false},
		{"connected=true but no machine_id", &cloudconfig.Config{Connected: true, Token: "t"}, false},
		{"fully connected", &cloudconfig.Config{Connected: true, Token: "t", MachineID: "m"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cfg.IsConnected()
			if got != tc.want {
				t.Errorf("IsConnected()=%v, want %v", got, tc.want)
			}
		})
	}
}
