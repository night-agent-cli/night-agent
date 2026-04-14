package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pietroperona/night-agent/internal/policy"
	"github.com/pietroperona/night-agent/internal/policyeditor"
	"github.com/spf13/cobra"
)

// ANSI
const (
	ansiReset     = "\033[0m"
	ansiBold      = "\033[1m"
	ansiDim       = "\033[2m"
	ansiRed       = "\033[31m"
	ansiGreen     = "\033[32m"
	ansiCyan      = "\033[36m"
	ansiBoldCyan  = "\033[1;36m"
	ansiBoldWhite = "\033[1;37m"
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Gestisci le regole di policy",
}

var policyListCmd = &cobra.Command{
	Use:   "list",
	Short: "Mostra tutte le regole attive",
	RunE:  runPolicyList,
}

var policyToggleCmd = &cobra.Command{
	Use:   "toggle <rule-id>",
	Short: "Attiva/disattiva una regola (block ↔ allow)",
	Args:  cobra.ExactArgs(1),
	RunE:  runPolicyToggle,
}

var policyAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Aggiungi una nuova regola in modo interattivo",
	RunE:  runPolicyAdd,
}

var policyRemoveCmd = &cobra.Command{
	Use:   "remove <rule-id>",
	Short: "Rimuovi una regola dalla policy",
	Args:  cobra.ExactArgs(1),
	RunE:  runPolicyRemove,
}

func init() {
	policyCmd.AddCommand(policyListCmd)
	policyCmd.AddCommand(policyToggleCmd)
	policyCmd.AddCommand(policyAddCmd)
	policyCmd.AddCommand(policyRemoveCmd)
	rootCmd.AddCommand(policyCmd)
}

func policyPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".night-agent", "policy.yaml"), nil
}

func runPolicyList(cmd *cobra.Command, args []string) error {
	path, err := policyPath()
	if err != nil {
		return err
	}
	p, err := policy.LoadFile(path)
	if err != nil {
		return fmt.Errorf("errore caricamento policy: %w", err)
	}
	fmt.Print(policyeditor.RenderTable(p))
	fmt.Printf("  %snight-agent policy toggle <id>%s  per attivare/disattivare\n",
		ansiDim, ansiReset)
	fmt.Printf("  %snight-agent policy add%s          per aggiungere una regola\n",
		ansiDim, ansiReset)
	fmt.Printf("  %snight-agent policy remove <id>%s  per rimuovere una regola\n\n",
		ansiDim, ansiReset)
	return nil
}

func runPolicyToggle(cmd *cobra.Command, args []string) error {
	path, err := policyPath()
	if err != nil {
		return err
	}
	ruleID := args[0]

	// leggi stato attuale per dare feedback
	p, err := policy.LoadFile(path)
	if err != nil {
		return err
	}
	var current policy.Decision
	for _, r := range p.Rules {
		if r.ID == ruleID {
			current = r.Decision
			break
		}
	}

	if err := policyeditor.ToggleRule(path, ruleID); err != nil {
		return err
	}

	var arrow string
	if current == policy.DecisionBlock || current == policy.DecisionAsk {
		arrow = fmt.Sprintf("%s✗ block%s → %s✓ allow%s", ansiRed, ansiReset, ansiGreen, ansiReset)
	} else {
		arrow = fmt.Sprintf("%s✓ allow%s → %s✗ block%s", ansiGreen, ansiReset, ansiRed, ansiReset)
	}
	fmt.Printf("\n  %s%s%s  %s\n\n", ansiBold, ruleID, ansiReset, arrow)
	fmt.Printf("  %sriavvia il daemon per applicare le modifiche: night-agent start%s\n\n",
		ansiDim, ansiReset)
	return nil
}

func runPolicyAdd(cmd *cobra.Command, args []string) error {
	path, err := policyPath()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(os.Stdin)
	ask := func(label, placeholder string) string {
		fmt.Printf("  %s%s%s %s(%s)%s: ", ansiBold, label, ansiReset, ansiDim, placeholder, ansiReset)
		scanner.Scan()
		v := strings.TrimSpace(scanner.Text())
		if v == "" {
			return placeholder
		}
		return v
	}

	fmt.Printf("\n%s  Nuova regola%s\n", ansiBold+ansiBoldCyan, ansiReset)
	fmt.Printf("%s  ──────────────────────────────────────%s\n\n", ansiDim, ansiReset)

	id := ask("ID regola", "custom_rule")
	actionType := ask("Tipo azione", "shell")
	pattern := ask("Pattern glob", "comando *")
	decision := ask("Decisione (block/allow)", "block")
	reason := ask("Motivo", "regola personalizzata")

	spec := policyeditor.NewRuleSpec{
		ID:         id,
		ActionType: actionType,
		Pattern:    pattern,
		Decision:   decision,
		Reason:     reason,
	}

	if err := policyeditor.AddRule(path, spec); err != nil {
		return err
	}

	decColor := ansiRed
	decIcon := "✗"
	if decision == "allow" {
		decColor = ansiGreen
		decIcon = "✓"
	}
	fmt.Printf("\n  %s%s %s%s  regola %s'%s'%s aggiunta\n\n",
		decColor, decIcon, decision, ansiReset,
		ansiBold, id, ansiReset)
	return nil
}

func runPolicyRemove(cmd *cobra.Command, args []string) error {
	path, err := policyPath()
	if err != nil {
		return err
	}
	ruleID := args[0]

	fmt.Printf("\n  Rimuovere la regola %s'%s'%s? [s/N] ", ansiBold, ruleID, ansiReset)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "s" && answer != "y" && answer != "si" && answer != "yes" {
		fmt.Println("  annullato")
		return nil
	}

	if err := policyeditor.RemoveRule(path, ruleID); err != nil {
		return err
	}

	fmt.Printf("  %s✓%s regola '%s' rimossa\n\n", ansiGreen, ansiReset, ruleID)
	return nil
}
