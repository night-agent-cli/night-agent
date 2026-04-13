package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pietroperona/night-agent/internal/cloudconfig"
	cloudsync "github.com/pietroperona/night-agent/internal/sync"
	"github.com/spf13/cobra"
)

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Gestisci connessione cloud Night Agent",
}

var cloudConnectCmd = &cobra.Command{
	Use:   "connect <TOKEN>",
	Short: "Connetti al cloud Night Agent con il token fornito",
	Args:  cobra.ExactArgs(1),
	RunE:  runCloudConnect,
}

var cloudStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Mostra stato connessione cloud",
	RunE:  runCloudStatus,
}

var cloudDisconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "Disconnetti dal cloud Night Agent",
	RunE:  runCloudDisconnect,
}

var cloudSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sincronizza manualmente gli eventi con il cloud",
	RunE:  runCloudSync,
}

func init() {
	cloudCmd.AddCommand(cloudConnectCmd)
	cloudCmd.AddCommand(cloudStatusCmd)
	cloudCmd.AddCommand(cloudDisconnectCmd)
	cloudCmd.AddCommand(cloudSyncCmd)
	rootCmd.AddCommand(cloudCmd)
}

func cloudConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".night-agent", "cloud.yaml"), nil
}

func cloudLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".night-agent", "audit.jsonl"), nil
}

func runCloudConnect(_ *cobra.Command, args []string) error {
	token := args[0]

	cfgPath, err := cloudConfigPath()
	if err != nil {
		return err
	}

	cfg, err := cloudconfig.Connect(cfgPath, token)
	if err != nil {
		return fmt.Errorf("connessione fallita: %w", err)
	}

	fmt.Println("  ✓ connesso al cloud Night Agent")
	fmt.Printf("  endpoint : %s\n", cfg.Endpoint)
	fmt.Printf("  machine  : %s\n", cfg.MachineID)
	fmt.Println()
	fmt.Println("  sync automatico: avvia il daemon con 'nightagent start'")
	fmt.Println("  sync manuale   : 'nightagent cloud sync'")
	return nil
}

func runCloudStatus(_ *cobra.Command, _ []string) error {
	cfgPath, err := cloudConfigPath()
	if err != nil {
		return err
	}

	cfg, err := cloudconfig.Load(cfgPath)
	if err != nil {
		return err
	}

	if !cfg.Connected || cfg.Token == "" {
		fmt.Println("  cloud: non connesso")
		fmt.Println("  usa 'nightagent cloud connect <TOKEN>' per connetterti")
		return nil
	}

	fmt.Println("  cloud: connesso")
	fmt.Printf("  endpoint  : %s\n", cfg.Endpoint)
	fmt.Printf("  machine   : %s\n", cfg.MachineID)

	if cfg.Cursor != "" {
		fmt.Printf("  cursore   : %s\n", cfg.Cursor)
	} else {
		fmt.Println("  cursore   : nessun sync effettuato")
	}

	if !cfg.LastSync.IsZero() {
		ago := time.Since(cfg.LastSync).Round(time.Second)
		fmt.Printf("  ultimo sync: %s fa (%s)\n", ago, cfg.LastSync.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Println("  ultimo sync: mai")
	}
	return nil
}

func runCloudDisconnect(_ *cobra.Command, _ []string) error {
	cfgPath, err := cloudConfigPath()
	if err != nil {
		return err
	}

	if err := cloudconfig.Disconnect(cfgPath); err != nil {
		return fmt.Errorf("disconnessione fallita: %w", err)
	}

	fmt.Println("  ✓ disconnesso dal cloud Night Agent")
	fmt.Println("  token rimosso da ~/.night-agent/cloud.yaml")
	return nil
}

func runCloudSync(_ *cobra.Command, _ []string) error {
	cfgPath, err := cloudConfigPath()
	if err != nil {
		return err
	}
	logPath, err := cloudLogPath()
	if err != nil {
		return err
	}

	cfg, err := cloudconfig.Load(cfgPath)
	if err != nil {
		return err
	}
	if !cfg.Connected || cfg.Token == "" {
		return fmt.Errorf("non connesso — esegui 'nightagent cloud connect <TOKEN>'")
	}

	fmt.Print("  sincronizzazione in corso... ")
	agent := cloudsync.NewAgent(cfgPath, logPath)
	if err := agent.SyncOnce(); err != nil {
		fmt.Println("✗")
		return err
	}

	// rileggi config aggiornata per mostrare cursore
	updated, _ := cloudconfig.Load(cfgPath)
	fmt.Println("✓")
	if updated != nil && updated.Cursor != "" {
		fmt.Printf("  ultimo evento sincronizzato: %s\n", updated.Cursor)
	}
	return nil
}
