package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pietroperona/night-agent/internal/audit"
	"github.com/pietroperona/night-agent/internal/claudehook"
	"github.com/pietroperona/night-agent/internal/launchagent"
	"github.com/pietroperona/night-agent/internal/policy"
	"github.com/pietroperona/night-agent/internal/shell"
	"github.com/pietroperona/night-agent/internal/shim"
	"github.com/pietroperona/night-agent/internal/wizard"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Inizializza Night Agent e configura la policy",
	Long:  "Crea la directory di configurazione, esegue il wizard di policy e avvia il daemon automatico.",
	RunE:  runInit,
}

var flagYes bool

func init() {
	initCmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "accetta tutti i default senza wizard interattivo")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	guardianDir, err := ensureGuardianDir()
	if err != nil {
		return err
	}

	policyPath := filepath.Join(guardianDir, "policy.yaml")
	if err := copyDefaultPolicy(policyPath); err != nil {
		return err
	}

	// wizard di configurazione policy (saltabile con --yes)
	if !flagYes {
		if err := runPolicyWizard(policyPath); err != nil {
			return err
		}
	} else {
		fmt.Println("policy: tutte le regole di default attive (--yes)")
	}
	fmt.Printf("policy: %s\n", policyPath)

	// lock policy file: imposta user-immutable flag (chflags uchg)
	// impedisce scrittura da qualsiasi processo inclusi subprocess non-interattivi
	if err := lockPolicyFile(policyPath); err != nil {
		fmt.Printf("avviso: lock policy file fallito (%v)\n", err)
	} else {
		fmt.Printf("policy lock: immutabile (usa 'nightagent policy edit' per modificare)\n")
	}

	rcPath, err := detectZshrc()
	if err != nil {
		return err
	}

	socketPath := filepath.Join(guardianDir, "night-agent.sock")
	injected, err := shell.Inject(rcPath, socketPath)
	if err != nil {
		return fmt.Errorf("errore iniezione hook shell: %w", err)
	}
	if injected {
		fmt.Printf("hook iniettato in: %s\n", rcPath)
	} else {
		fmt.Printf("hook shell già presente in: %s\n", rcPath)
	}

	// installa PATH shims
	shimDir := shim.ShimDir(guardianDir)
	shimBinary := filepath.Join(shimDir, shim.ShimBinaryName)
	if err := installShims(guardianDir, shimBinary); err != nil {
		fmt.Printf("avviso: shims non installati (%v)\n", err)
		fmt.Printf("        esegui 'make shim && night-agent init' per abilitarli\n")
	} else {
		fmt.Printf("shims installati in: %s\n", shimDir)
	}

	// installa LaunchAgent
	home, _ := os.UserHomeDir()
	binaryPath, err := resolveAbsBinary()
	if err != nil {
		fmt.Printf("avviso: LaunchAgent non installato (%v)\n", err)
	} else if err := launchagent.Install(home, binaryPath, guardianDir); err != nil {
		fmt.Printf("avviso: LaunchAgent non installato (%v)\n", err)
	} else {
		fmt.Printf("LaunchAgent installato: avvio automatico al login attivo\n")
	}

	// configura hook Claude Code (solo se Claude Code è installato)
	if claudehook.IsClaudeInstalled() {
		settingsPath, err := claudehook.SettingsPath()
		if err == nil {
			binaryPath, err := resolveAbsBinary()
			if err == nil {
				if err := claudehook.Install(settingsPath, binaryPath); err != nil {
					fmt.Printf("avviso: hook Claude Code non configurato (%v)\n", err)
				} else if claudehook.IsConfigured(settingsPath) {
					fmt.Printf("hook Claude Code: %s\n", settingsPath)
				}
			}
		}
	}

	// genera la chiave di firma (idempotente — non sovrascrive se esiste già)
	keyPath := filepath.Join(guardianDir, "signing.key")
	if err := audit.GenerateKey(keyPath); err != nil {
		fmt.Printf("avviso: chiave di firma non generata (%v)\n", err)
	} else {
		fmt.Printf("firma audit: %s\n", keyPath)
	}

	if injected {
		fmt.Println("\nnight-agent inizializzato. Riavvia il terminale o esegui: source " + rcPath)
	} else {
		fmt.Println("\nnight-agent aggiornato.")
	}
	return nil
}

// runPolicyWizard esegue il wizard interattivo e aggiorna la policy.
// Le regole non selezionate dall'utente vengono impostate su "allow".
func runPolicyWizard(policyPath string) error {
	blockedRuleIDs, err := wizard.Run(os.Stdin, os.Stdout)
	if err != nil {
		return err
	}

	p, err := policy.LoadFile(policyPath)
	if err != nil {
		return err
	}

	blockedSet := make(map[string]bool, len(blockedRuleIDs))
	for _, id := range blockedRuleIDs {
		blockedSet[id] = true
	}

	for i, rule := range p.Rules {
		if !blockedSet[rule.ID] {
			p.Rules[i].Decision = policy.DecisionAllow
			p.Rules[i].Reason = "consentito dall'utente durante init"
		}
	}

	return policy.Save(policyPath, p)
}

func resolveAbsBinary() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

func installShims(guardianDir, shimBinaryPath string) error {
	candidates := []string{
		filepath.Join(filepath.Dir(os.Args[0]), shim.ShimBinaryName),
		filepath.Join(".", shim.ShimBinaryName),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return shim.Install(guardianDir, candidate)
		}
	}
	if info, err := os.Stat(shimBinaryPath); err == nil && info.Size() > 0 {
		return shim.CreateSymlinks(shim.ShimDir(guardianDir), shimBinaryPath)
	}
	return fmt.Errorf("binario %s non trovato — esegui 'make shim'", shim.ShimBinaryName)
}

func ensureGuardianDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("impossibile determinare la home directory: %w", err)
	}
	dir := filepath.Join(home, ".night-agent")

	// Migrazione automatica da ~/.guardian (installazione precedente)
	oldDir := filepath.Join(home, ".guardian")
	if _, errOld := os.Stat(oldDir); errOld == nil {
		if _, errNew := os.Stat(dir); os.IsNotExist(errNew) {
			if errRename := os.Rename(oldDir, dir); errRename == nil {
				fmt.Printf("migrazione: ~/.guardian → ~/.night-agent\n")
			} else {
				fmt.Printf("avviso: migrazione ~/.guardian fallita (%v) — procedo con nuova directory\n", errRename)
			}
		}
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("impossibile creare %s: %w", dir, err)
	}
	return dir, nil
}

func copyDefaultPolicy(dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return nil // già esiste, non sovrascrivere
	}
	candidates := []string{
		"configs/default_policy.yaml",
		filepath.Join(filepath.Dir(os.Args[0]), "configs", "default_policy.yaml"),
	}
	for _, src := range candidates {
		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		return os.WriteFile(dest, data, 0600)
	}
	return fmt.Errorf("policy di default non trovata")
}

func detectZshrc() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if isZsh() {
		return filepath.Join(home, ".zshrc"), nil
	}
	return filepath.Join(home, ".bashrc"), nil
}

func isZsh() bool {
	s := os.Getenv("SHELL")
	if s != "" {
		return filepath.Base(s) == "zsh"
	}
	_, err := exec.LookPath("zsh")
	return err == nil
}

// lockPolicyFile imposta il flag user-immutable (chflags uchg) sul file policy.
// Blocca scrittura da qualsiasi processo, inclusi subprocess non-interattivi.
func lockPolicyFile(path string) error {
	return policy.LockFile(path)
}
