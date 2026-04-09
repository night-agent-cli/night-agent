package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pietroperona/agent-guardian/internal/shell"
	"github.com/pietroperona/agent-guardian/internal/shim"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Inizializza Guardian nel progetto corrente",
	Long:  "Crea la directory di configurazione, copia la policy di default e inietta l'hook nel profilo shell.",
	RunE:  runInit,
}

var flagAdvanced bool

func init() {
	initCmd.Flags().BoolVar(&flagAdvanced, "advanced", false, "modalità avanzata con sintassi regex per le regole")
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
	fmt.Printf("policy creata: %s\n", policyPath)

	rcPath, err := detectZshrc()
	if err != nil {
		return err
	}

	socketPath := filepath.Join(guardianDir, "guardian.sock")
	if err := shell.Inject(rcPath, socketPath); err != nil {
		return fmt.Errorf("errore iniezione hook shell: %w", err)
	}
	fmt.Printf("hook iniettato in: %s\n", rcPath)

	// installa PATH shims — copertura agent-agnostica (funziona anche con Claude Code)
	shimDir := shim.ShimDir(guardianDir)
	shimBinary := filepath.Join(shimDir, shim.ShimBinaryName)
	if err := installShims(guardianDir, shimBinary); err != nil {
		// non bloccare l'init: gli shims sono opzionali se il binario non è ancora compilato
		fmt.Printf("avviso: shims non installati (%v)\n", err)
		fmt.Printf("        esegui 'make shim && guardian init' per abilitarli\n")
	} else {
		fmt.Printf("shims installati in: %s\n", shimDir)
	}

	fmt.Println("\nguardian inizializzato. Riavvia il terminale o esegui: source " + rcPath)
	return nil
}

func installShims(guardianDir, shimBinaryPath string) error {
	// cerca il binario guardian-shim nella stessa dir del binario guardian o nel cwd
	candidates := []string{
		shimBinaryPath,
		filepath.Join(filepath.Dir(os.Args[0]), shim.ShimBinaryName),
		filepath.Join(".", shim.ShimBinaryName),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return shim.Install(guardianDir, candidate)
		}
	}
	return fmt.Errorf("binario %s non trovato — esegui 'make shim'", shim.ShimBinaryName)
}

func ensureGuardianDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("impossibile determinare la home directory: %w", err)
	}
	dir := filepath.Join(home, ".guardian")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("impossibile creare %s: %w", dir, err)
	}
	return dir, nil
}

func copyDefaultPolicy(dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return nil // già esiste, non sovrascrivere
	}

	// cerca la policy di default relativa al binario o nel path di sviluppo
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

	// preferisci .zshrc se zsh è la shell corrente
	if isZsh() {
		return filepath.Join(home, ".zshrc"), nil
	}
	return filepath.Join(home, ".bashrc"), nil
}

func isZsh() bool {
	shell := os.Getenv("SHELL")
	if shell != "" {
		return filepath.Base(shell) == "zsh"
	}
	_, err := exec.LookPath("zsh")
	return err == nil
}
