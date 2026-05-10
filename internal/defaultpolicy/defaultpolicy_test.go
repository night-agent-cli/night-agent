package defaultpolicy_test

import (
	"testing"

	"github.com/night-agent-cli/night-agent/internal/defaultpolicy"
	"gopkg.in/yaml.v3"
)

func TestDefaultPolicyBytes_NonEmpty(t *testing.T) {
	if len(defaultpolicy.DefaultPolicyBytes) == 0 {
		t.Fatal("DefaultPolicyBytes è vuoto — go:embed non ha funzionato")
	}
}

func TestDefaultPolicyBytes_ValidYAML(t *testing.T) {
	var v struct {
		Version int `yaml:"version"`
	}
	if err := yaml.Unmarshal(defaultpolicy.DefaultPolicyBytes, &v); err != nil {
		t.Fatalf("DefaultPolicyBytes non è YAML valido: %v", err)
	}
	if v.Version == 0 {
		t.Error("policy YAML deve avere version > 0")
	}
}
