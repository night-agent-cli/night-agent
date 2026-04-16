package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pietroperona/night-agent/internal/audit"
	"github.com/pietroperona/night-agent/internal/cloudconfig"
	"github.com/pietroperona/night-agent/internal/configdir"
	"github.com/pietroperona/night-agent/internal/daemon"
	"github.com/pietroperona/night-agent/internal/policy"
	nightsync "github.com/pietroperona/night-agent/internal/sync"
	"github.com/spf13/cobra"
)

var localPolicyOnly bool

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Avvia il daemon Guardian in foreground",
	RunE:  runStart,
}

func init() {
	startCmd.Flags().BoolVar(&localPolicyOnly, "local-policy-only", false, "ignora la policy cloud, usa solo locale/globale")
	rootCmd.AddCommand(startCmd)
}

func runStart(_ *cobra.Command, _ []string) error {
	cfgDir, err := resolveConfigDir()
	if err != nil {
		return err
	}

	socketPath := filepath.Join(cfgDir, "night-agent.sock")
	logPath := filepath.Join(cfgDir, "audit.jsonl")
	cloudCfgPath := filepath.Join(cfgDir, "cloud.yaml")

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// costruisci cloud client se configurato e non --local-policy-only
	var cloudClient policy.CloudClient
	var machineID string
	if !localPolicyOnly {
		if cfg, err := cloudconfig.Load(cloudCfgPath); err == nil && cfg.Connected && cfg.Token != "" {
			cloudClient = &policy.HTTPCloudClient{
				Endpoint: cfg.Endpoint,
				Token:    cfg.Token,
			}
			machineID = cfg.MachineID
		}
	}

	// carica policy con priorità cloud → locale → globale → none
	lp, err := policy.Load(cwd, cloudClient, machineID)
	if err != nil {
		return fmt.Errorf("errore caricamento policy: %w", err)
	}
	fmt.Println(policy.FormatSource(lp))

	// policy permissiva se SourceNone (nessuna policy trovata)
	var p *policy.Policy
	if lp.Policy != nil {
		p = lp.Policy
	} else {
		p = &policy.Policy{}
	}

	// usa signed logger se la chiave esiste, altrimenti logger base
	keyPath := filepath.Join(cfgDir, "signing.key")
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

	policyPath := lp.Path
	if lp.Source == policy.SourceNone {
		policyPath = filepath.Join(cfgDir, "policy.yaml")
	}

	srv, err := daemon.NewServerWithPolicyPath(socketPath, policyPath, p, logger)
	if err != nil {
		return fmt.Errorf("errore avvio daemon: %w", err)
	}
	srv.WithLogPath(logPath)

	// imposta hash iniziale per il trust checker (file già presente su disco)
	if lp.Source != policy.SourceNone && lp.Path != "" {
		if data, readErr := os.ReadFile(lp.Path); readErr == nil {
			srv.SetInitialHash(data)
		}
	}

	fmt.Printf("night-agent in ascolto su %s\n", socketPath)

	go srv.Serve()

	// hot-reload: watch cwd per nightagent-policy.yaml.
	// isTrustedFileContent impedisce al daemon di ricaricare modifiche esterne non autorizzate.
	stopWatch, watchErr := policy.Watch(cwd, cloudClient, machineID, func(reloaded *policy.LoadedPolicy) {
		if reloaded.Policy != nil {
			srv.UpdatePolicy(reloaded.Policy)
		} else {
			srv.UpdatePolicy(&policy.Policy{})
		}
		fmt.Printf("[policy] reloaded from %s\n", reloaded.Path)
	}, srv.IsTrustedFileContent)
	if watchErr != nil {
		fmt.Fprintf(os.Stderr, "  avviso: hot-reload non disponibile: %v\n", watchErr)
		stopWatch = func() {}
	}
	defer stopWatch()

	// sync cloud periodico ogni 30s — fail-open, errori ignorati
	if _, err := os.Stat(cloudCfgPath); err == nil {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			agent := nightsync.NewAgent(cloudCfgPath, logPath)
			// includi policy nel payload solo se locale o globale (non cloud)
			if lp.Source == policy.SourceLocal || lp.Source == policy.SourceGlobal {
				agent.WithPolicyPath(lp.Path)
			}
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

// resolvePolicyPath e fetchCloudPolicy non più necessari — sostituiti da policy.Load()
// Mantenute per compatibilità con altri comandi che potrebbero usarle.

func globalPolicyPath() (string, error) {
	dir, err := configdir.Global()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "policy.yaml"), nil
}
