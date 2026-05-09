package policy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/gobwas/glob"
	"gopkg.in/yaml.v3"
)

type MatchType string
type Decision string
type ActionType string

const (
	MatchGlob  MatchType = "glob"
	MatchRegex MatchType = "regex"

	DecisionAllow   Decision = "allow"
	DecisionBlock   Decision = "block"
	DecisionAsk     Decision = "ask"
	DecisionSandbox Decision = "sandbox"

	ActionTypeShell ActionType = "shell"
	ActionTypeGit   ActionType = "git"
	ActionTypeFile  ActionType = "file"
)

type Condition struct {
	ActionType     string   `yaml:"action_type"`
	CommandMatches []string `yaml:"command_matches"`
	PathMatches    []string `yaml:"path_matches"`
}

// SandboxConfig configura il container Docker per le regole con decision: sandbox.
type SandboxConfig struct {
	Image   string `yaml:"image"`   // immagine Docker, es: "alpine:3.20"
	Network string `yaml:"network"` // "none" (default) o "bridge"
}

type Rule struct {
	ID        string         `yaml:"id"`
	When      Condition      `yaml:"when"`
	MatchType MatchType      `yaml:"match_type"`
	Decision  Decision       `yaml:"decision"`
	Reason    string         `yaml:"reason"`
	Sandbox   *SandboxConfig `yaml:"sandbox,omitempty"`

	// pre-compilate da LoadBytes per regole match_type: regex
	compiledCommandRe []*regexp.Regexp
	compiledPathRe    []*regexp.Regexp
}

type Policy struct {
	Version int    `yaml:"version"`
	Rules   []Rule `yaml:"rules"`
}

type Action struct {
	Type    string
	Command string
	Path    string
}

type EvalResult struct {
	Decision Decision
	RuleID   string
	Reason   string
	Sandbox  *SandboxConfig // non nil se la regola ha decision: sandbox
}

// LoadFile carica la policy da un file YAML.
func LoadFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("impossibile leggere il file di policy: %w", err)
	}
	return LoadBytes(data)
}

// LoadBytes parsa e valida una policy da slice di byte YAML.
// Per le regole con match_type: regex, pre-compila le pattern e restituisce
// errore immediatamente se una regex è malformata.
func LoadBytes(data []byte) (*Policy, error) {
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("YAML non valido: %w", err)
	}
	if p.Version == 0 {
		return nil, fmt.Errorf("campo 'version' mancante o zero nella policy")
	}
	if err := compileRegexRules(p.Rules); err != nil {
		return nil, err
	}
	return &p, nil
}

// compileRegexRules pre-compila le pattern regex di tutte le regole.
// Restituisce errore al primo pattern non valido.
func compileRegexRules(rules []Rule) error {
	for i := range rules {
		if rules[i].MatchType != MatchRegex {
			continue
		}
		for _, pat := range rules[i].When.CommandMatches {
			re, err := regexp.Compile(pat)
			if err != nil {
				return fmt.Errorf("regex non valida nella regola %q (command_matches %q): %w", rules[i].ID, pat, err)
			}
			rules[i].compiledCommandRe = append(rules[i].compiledCommandRe, re)
		}
		for _, pat := range rules[i].When.PathMatches {
			re, err := regexp.Compile(pat)
			if err != nil {
				return fmt.Errorf("regex non valida nella regola %q (path_matches %q): %w", rules[i].ID, pat, err)
			}
			rules[i].compiledPathRe = append(rules[i].compiledPathRe, re)
		}
	}
	return nil
}

// hardcodedRules sono regole immutabili che proteggono l'integrità del sistema.
// Vengono valutate PRIMA delle regole YAML e non possono essere sovrascritte.
var hardcodedRules = []Rule{
	{
		ID:        "_protect_policy_write_shell",
		MatchType: MatchGlob,
		When: Condition{
			ActionType: string(ActionTypeShell),
			CommandMatches: []string{
				"* > *nightagent*",
				"* >> *nightagent*",
				"* > *.nightagent*",
				"* >> *.nightagent*",
				"tee *nightagent*",
				"tee *.nightagent*",
				"sed * nightagent*",
				"chflags nouchg *nightagent*",
				"chflags nouchg *night-agent*",
			},
		},
		Decision: DecisionBlock,
		Reason:   "modifica diretta dei file policy non consentita — usa 'nightagent policy edit'",
	},
	{
		ID:        "_protect_policy_write_file",
		MatchType: MatchGlob,
		When: Condition{
			ActionType: string(ActionTypeFile),
			PathMatches: []string{
				"*nightagent-policy*",
				"*.nightagent*policy*",
				"*night-agent*policy*",
			},
		},
		Decision: DecisionBlock,
		Reason:   "modifica diretta dei file policy non consentita — usa 'nightagent policy edit'",
	},
}

func (p *Policy) Evaluate(action Action) EvalResult {
	// Regole immutabili — valutate prima di qualsiasi regola YAML
	for _, rule := range hardcodedRules {
		if rule.When.ActionType != action.Type {
			continue
		}
		if matches(rule, action) {
			return EvalResult{
				Decision: rule.Decision,
				RuleID:   rule.ID,
				Reason:   rule.Reason,
			}
		}
	}

	for _, rule := range p.Rules {
		if rule.When.ActionType != action.Type {
			continue
		}

		if matches(rule, action) {
			return EvalResult{
				Decision: rule.Decision,
				RuleID:   rule.ID,
				Reason:   rule.Reason,
				Sandbox:  rule.Sandbox,
			}
		}
	}

	return EvalResult{Decision: DecisionAllow}
}

func matches(rule Rule, action Action) bool {
	mt := rule.MatchType
	if mt == "" {
		mt = MatchGlob
	}

	// match su command
	if len(rule.When.CommandMatches) > 0 && action.Command != "" {
		for i, pattern := range rule.When.CommandMatches {
			var compiled *regexp.Regexp
			if mt == MatchRegex && i < len(rule.compiledCommandRe) {
				compiled = rule.compiledCommandRe[i]
			}
			if matchPattern(mt, pattern, action.Command, false, compiled) {
				return true
			}
		}
		return false
	}

	// match su path
	if len(rule.When.PathMatches) > 0 && action.Path != "" {
		for i, pattern := range rule.When.PathMatches {
			var compiled *regexp.Regexp
			if mt == MatchRegex && i < len(rule.compiledPathRe) {
				compiled = rule.compiledPathRe[i]
			}
			if matchPattern(mt, pattern, action.Path, true, compiled) {
				return true
			}
		}
		return false
	}

	return false
}

// Save serializza la policy e la scrive su file.
func Save(path string, p *Policy) error {
	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("errore serializzazione policy: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// LockFile imposta il flag user-immutable (chflags uchg) sul file.
// Blocca scrittura da qualsiasi processo inclusi subprocess non-interattivi.
func LockFile(path string) error {
	return exec.Command("chflags", "uchg", path).Run()
}

// UnlockFile rimuove il flag user-immutable prima di una scrittura autorizzata.
func UnlockFile(path string) error {
	return exec.Command("chflags", "nouchg", path).Run()
}

// RelockFile re-imposta il flag user-immutable dopo una scrittura autorizzata.
func RelockFile(path string) error {
	return exec.Command("chflags", "uchg", path).Run()
}

// AppendAllowRule aggiunge una regola allow permanente nella policy YAML per
// il comando esatto dato. Se una regola identica esiste già, non duplica.
func AppendAllowRule(policyPath, agentName, command string) error {
	p, err := LoadFile(policyPath)
	if err != nil {
		return err
	}

	// genera un ID univoco basato su agente e comando
	id := "allow_" + sanitizeID(agentName) + "_" + sanitizeID(command)

	// controlla se esiste già una regola identica
	for _, r := range p.Rules {
		if r.ID == id {
			return nil // già presente, niente da fare
		}
	}

	newRule := Rule{
		ID: id,
		When: Condition{
			ActionType:     string(ActionTypeShell),
			CommandMatches: []string{command},
		},
		MatchType: MatchGlob,
		Decision:  DecisionAllow,
		Reason:    fmt.Sprintf("consentito sempre per %s", agentName),
	}

	// inserisci come prima regola (priorità massima — first-match-wins)
	p.Rules = append([]Rule{newRule}, p.Rules...)

	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("errore serializzazione policy: %w", err)
	}
	return os.WriteFile(policyPath, data, 0600)
}

// sanitizeID trasforma una stringa in un identificatore YAML-safe.
func sanitizeID(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	if len(result) > 40 {
		result = result[:40]
	}
	return string(result)
}

// matchPattern valuta un pattern contro un valore.
// Per command_matches usa glob senza separatori (il * matcha spazi e /).
// Per path_matches usa glob con filepath.Separator (il * non matcha /).
// compiled è opzionale: se non nil, viene usata al posto della compilazione runtime.
func matchPattern(mt MatchType, pattern, value string, isPath bool, compiled *regexp.Regexp) bool {
	switch mt {
	case MatchRegex:
		if compiled != nil {
			return compiled.MatchString(value)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(value)
	default: // glob
		var g glob.Glob
		var err error
		if isPath {
			g, err = glob.Compile(pattern, filepath.Separator)
		} else {
			// per comandi shell: * deve matchare spazi, slash e qualsiasi carattere
			g, err = glob.Compile(pattern)
		}
		if err != nil {
			return false
		}
		return g.Match(value)
	}
}
