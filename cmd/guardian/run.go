package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pietroperona/night-agent/internal/intercept"
	"github.com/pietroperona/night-agent/internal/shim"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <agente> [args...]",
	Short: "Avvia un agente AI sotto la protezione di Night Agent",
	Long: `Avvia un agente AI con protezione attiva via due meccanismi:

  1. PATH shims — intercetta i comandi eseguiti dall'agente via shell
     (funziona con tutti gli agenti, incluso Claude Code con Hardened Runtime)

  2. DYLD_INSERT_LIBRARIES — intercetta syscall di processo a livello C
     (funziona per agenti senza Hardened Runtime: node, python3, ecc.)

Esempi:
  night-agent run claude
  night-agent run python3 my_agent.py
  night-agent run node agent.js`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAgent,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runAgent(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	guardianDir := filepath.Join(home, ".night-agent")
	socketPath := filepath.Join(guardianDir, "night-agent.sock")

	if !isDaemonRunning(socketPath) {
		return fmt.Errorf("daemon non in esecuzione — avvia prima 'night-agent start' in un altro terminale")
	}

	shimDir := shim.ShimDir(guardianDir)
	shimBinary := filepath.Join(shimDir, shim.ShimBinaryName)

	env := os.Environ()
	env = append(env, "NIGHTAGENT_AGENT="+args[0])

	// PATH shims: funzionano con tutti gli agenti indipendentemente da Hardened Runtime
	if _, err := os.Stat(shimBinary); err == nil {
		env = shim.PrependPath(env, shimDir)
		env = append(env, "GUARDIAN_SHIM_DIR="+shimDir)
		env = append(env, "GUARDIAN_SOCKET="+socketPath)
		fmt.Printf("night-agent: shims    → %s\n", shimDir)
	} else {
		fmt.Printf("night-agent: avviso — shim dir non trovata (%s)\n", shimDir)
		fmt.Printf("night-agent: esegui 'make shim' per abilitare l'interception PATH\n")
	}

	// DYLD: copertura aggiuntiva per agenti senza Hardened Runtime (node, python3...)
	binaryDir := filepath.Dir(os.Args[0])
	if dylibPath, err := findDylibCandidates(binaryDir); err == nil {
		env = intercept.BuildEnv(env, dylibPath, socketPath)
		fmt.Printf("night-agent: dylib    → %s\n", dylibPath)
	}

	agentBinary, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("agente '%s' non trovato nel PATH: %w", args[0], err)
	}

	fmt.Printf("night-agent: avvio '%s' con interception attiva\n", args[0])
	fmt.Printf("night-agent: socket   → %s\n\n", socketPath)

	agentCmd := exec.Command(agentBinary, args[1:]...)
	agentCmd.Env = env
	agentCmd.Stdin = os.Stdin
	agentCmd.Stdout = os.Stdout
	agentCmd.Stderr = os.Stderr

	return agentCmd.Run()
}

func findDylibCandidates(binaryDir string) (string, error) {
	candidates := []string{
		binaryDir,
		".", // directory corrente (sviluppo locale)
	}
	for _, dir := range candidates {
		if path, err := intercept.FindDylib(dir); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf(
		"guardian-intercept.dylib non trovata — esegui 'make dylib' nella root del progetto",
	)
}
