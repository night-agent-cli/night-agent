package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pietroperona/agent-guardian/internal/launchagent"
	"github.com/pietroperona/agent-guardian/internal/shell"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Rimuove Guardian dal sistema",
	Long:  "Ferma il daemon, rimuove il LaunchAgent e rimuove l'hook dallo shell profile.",
	RunE:  runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// rimuovi LaunchAgent
	if launchagent.IsInstalled(home) {
		if err := launchagent.Uninstall(home); err != nil {
			fmt.Printf("avviso: errore rimozione LaunchAgent: %v\n", err)
		} else {
			fmt.Println("LaunchAgent rimosso")
		}
	}

	// rimuovi hook dallo shell profile
	rcPath := filepath.Join(home, ".zshrc")
	if shell.IsInjected(rcPath) {
		if err := shell.Remove(rcPath); err != nil {
			fmt.Printf("avviso: errore rimozione hook da %s: %v\n", rcPath, err)
		} else {
			fmt.Printf("hook rimosso da: %s\n", rcPath)
		}
	}

	fmt.Println("\nguardian disinstallato.")
	fmt.Printf("I dati in ~/.guardian/ sono stati preservati. Per rimuoverli: rm -rf ~/.guardian\n")
	return nil
}
