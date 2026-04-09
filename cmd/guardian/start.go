package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pietroperona/agent-guardian/internal/audit"
	"github.com/pietroperona/agent-guardian/internal/daemon"
	"github.com/pietroperona/agent-guardian/internal/policy"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Avvia il daemon Guardian in foreground",
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	guardianDir := filepath.Join(home, ".guardian")
	policyPath := filepath.Join(guardianDir, "policy.yaml")
	socketPath := filepath.Join(guardianDir, "guardian.sock")
	logPath := filepath.Join(guardianDir, "audit.jsonl")

	p, err := policy.Load(policyPath)
	if err != nil {
		return fmt.Errorf("errore caricamento policy: %w", err)
	}

	logger, err := audit.NewLogger(logPath)
	if err != nil {
		return fmt.Errorf("errore apertura log: %w", err)
	}
	defer logger.Close()

	srv, err := daemon.NewServer(socketPath, p, logger)
	if err != nil {
		return fmt.Errorf("errore avvio daemon: %w", err)
	}

	fmt.Printf("guardian in ascolto su %s\n", socketPath)

	go srv.Serve()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nguardian fermato")
	srv.Stop()
	return nil
}
