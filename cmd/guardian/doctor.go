package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pietroperona/agent-guardian/internal/shell"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Verifica lo stato dell'installazione di Guardian",
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	guardianDir := filepath.Join(home, ".guardian")
	policyPath := filepath.Join(guardianDir, "policy.yaml")
	socketPath := filepath.Join(guardianDir, "guardian.sock")

	allOK := true

	check := func(label string, ok bool, detail string) {
		status := "✓"
		if !ok {
			status = "✗"
			allOK = false
		}
		if detail != "" {
			fmt.Printf("  %s %s — %s\n", status, label, detail)
		} else {
			fmt.Printf("  %s %s\n", status, label)
		}
	}

	fmt.Println("Guardian — diagnostica:")

	_, errDir := os.Stat(guardianDir)
	check("directory ~/.guardian", errDir == nil, guardianDir)

	_, errPolicy := os.Stat(policyPath)
	check("policy.yaml", errPolicy == nil, policyPath)

	rcPath := filepath.Join(home, ".zshrc")
	check("hook shell (.zshrc)", shell.IsInjected(rcPath), "")

	daemonRunning := isDaemonRunning(socketPath)
	check("daemon in esecuzione", daemonRunning, socketPath)

	fmt.Println()
	if allOK {
		fmt.Println("tutto ok — guardian è operativo")
	} else {
		fmt.Println("alcuni controlli falliti — esegui 'guardian init' per configurare")
	}
	return nil
}

func isDaemonRunning(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
