package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pietroperona/night-agent/internal/audit"
	"github.com/pietroperona/night-agent/internal/daemon"
	"github.com/pietroperona/night-agent/internal/policy"
	nightsync "github.com/pietroperona/night-agent/internal/sync"
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

	guardianDir := filepath.Join(home, ".night-agent")
	policyPath := filepath.Join(guardianDir, "policy.yaml")
	socketPath := filepath.Join(guardianDir, "night-agent.sock")
	logPath := filepath.Join(guardianDir, "audit.jsonl")

	p, err := policy.Load(policyPath)
	if err != nil {
		return fmt.Errorf("errore caricamento policy: %w", err)
	}

	// usa signed logger se la chiave esiste, altrimenti logger base
	keyPath := filepath.Join(guardianDir, "signing.key")
	var logger *audit.Logger
	if signer, sigErr := audit.NewSigner(keyPath); sigErr == nil {
		logger, err = audit.NewSignedLogger(logPath, signer)
	} else {
		logger, err = audit.NewLogger(logPath)
	}
	if err != nil {
		return fmt.Errorf("errore apertura log: %w", err)
	}
	defer logger.Close()

	srv, err := daemon.NewServerWithPolicyPath(socketPath, policyPath, p, logger)
	if err != nil {
		return fmt.Errorf("errore avvio daemon: %w", err)
	}
	srv.WithLogPath(logPath)

	fmt.Printf("night-agent in ascolto su %s\n", socketPath)

	go srv.Serve()

	// sync cloud periodico ogni 30s — fail-open, errori ignorati
	cloudCfgPath := filepath.Join(guardianDir, "cloud.yaml")
	if _, err := os.Stat(cloudCfgPath); err == nil {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			agent := nightsync.NewAgent(cloudCfgPath, logPath)
			for range ticker.C {
				_ = agent.SyncOnce()
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nnight-agent fermato")
	srv.Stop()
	return nil
}
