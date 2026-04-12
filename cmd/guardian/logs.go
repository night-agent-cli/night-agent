package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/pietroperona/night-agent/internal/audit"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Visualizza l'audit trail degli eventi",
	RunE:  runLogs,
}

var (
	flagDecision   string
	flagActionType string
	flagJSONOutput bool
	flagLimit      int
)

func init() {
	logsCmd.Flags().StringVar(&flagDecision, "decision", "", "filtra per decisione (allow, block, ask)")
	logsCmd.Flags().StringVar(&flagActionType, "type", "", "filtra per tipo azione (shell, git, file)")
	logsCmd.Flags().BoolVar(&flagJSONOutput, "json", false, "output in formato JSON raw")
	logsCmd.Flags().IntVar(&flagLimit, "limit", 50, "numero massimo di eventi da mostrare")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	logPath, err := guardianLogPath()
	if err != nil {
		return err
	}

	events, err := audit.ReadFiltered(logPath, audit.Filter{
		Decision:   flagDecision,
		ActionType: flagActionType,
	})
	if err != nil {
		return fmt.Errorf("impossibile leggere il log: %w", err)
	}

	if len(events) > flagLimit {
		events = events[len(events)-flagLimit:]
	}

	if flagJSONOutput {
		return printJSON(events)
	}
	return printTable(events)
}

func printTable(events []audit.Event) error {
	if len(events) == 0 {
		fmt.Println("nessun evento trovato")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIMESTAMP\tDECISIONE\tRISCHIO\tTIPO\tCOMANDO\tMOTIVO")
	fmt.Fprintln(w, "---------\t---------\t-------\t----\t-------\t------")
	for _, e := range events {
		ts := e.Timestamp.Format(time.DateTime)
		cmd := e.Command
		if len(cmd) > 45 {
			cmd = cmd[:42] + "..."
		}
		risk := riskLabel(e.RiskLevel, e.RiskScore, e.AnomalyDetected)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", ts, e.Decision, risk, e.ActionType, cmd, e.Reason)
		// stampa suggerimenti inline se presenti
		for _, h := range e.Suggestions {
			fmt.Fprintf(w, "\t\t\t\t→ %s\t\n", h)
		}
	}
	return w.Flush()
}

// riskLabel formatta il livello di rischio per la tabella.
func riskLabel(level string, score float64, anomaly bool) string {
	if level == "" {
		return "-"
	}
	label := fmt.Sprintf("%s(%.2f)", level, score)
	if anomaly {
		label += "!"
	}
	return label
}

func printJSON(events []audit.Event) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(events)
}

func guardianLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, ".night-agent", "audit.jsonl")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("log non trovato in %s — esegui prima 'night-agent init'", path)
	}
	return path, nil
}
