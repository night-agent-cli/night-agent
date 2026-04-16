package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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

var policyEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Modifica la policy nell'editor di sistema ($EDITOR)",
	RunE:  runPolicyEdit,
}

func init() {
	policyCmd.AddCommand(policyListCmd)
	policyCmd.AddCommand(policyToggleCmd)
	policyCmd.AddCommand(policyAddCmd)
	policyCmd.AddCommand(policyRemoveCmd)
	policyCmd.AddCommand(policyEditCmd)
	rootCmd.AddCommand(policyCmd)
}

func policyPath() (string, error) {
	dir, err := resolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "policy.yaml"), nil
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

func runPolicyEdit(cmd *cobra.Command, args []string) error {
	path, err := policyPath()
	if err != nil {
		return err
	}

	// leggi policy corrente (crea file vuoto se non esiste)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte("version: 1\nrules: []\n")
		} else {
			return fmt.Errorf("impossibile leggere la policy: %w", err)
		}
	}

	// scrivi in temp file
	tmpFile, err := os.CreateTemp("", "nightagent-policy-*.yaml")
	if err != nil {
		return fmt.Errorf("impossibile creare file temporaneo: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	// apri editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}

	editorCmd := exec.Command(editor, tmpPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("editor terminato con errore: %w", err)
	}

	// leggi contenuto modificato
	newData, err := os.ReadFile(tmpPath)
	if err != nil {
		return err
	}

	// valida YAML prima di applicare
	if _, err := policy.LoadBytes(newData); err != nil {
		return fmt.Errorf("%spolicy non valida%s: %w", ansiRed, ansiReset, err)
	}

	// invia al daemon via socket (canale autorizzato)
	cfgDir, err := resolveConfigDir()
	if err != nil {
		return err
	}
	socketPath := filepath.Join(cfgDir, "night-agent.sock")

	if err := sendPolicyUpdate(socketPath, newData); err != nil {
		// daemon non disponibile: scrivi direttamente con avviso
		fmt.Fprintf(os.Stderr, "  %savviso%s: daemon non raggiungibile, scrivo direttamente su disco\n",
			ansiDim, ansiReset)
		if err2 := os.WriteFile(path, newData, 0600); err2 != nil {
			return fmt.Errorf("errore scrittura policy: %w", err2)
		}
	}

	fmt.Printf("\n  %s✓%s policy aggiornata\n\n", ansiGreen, ansiReset)
	return nil
}

// policyWriteRequest è il payload inviato al daemon per aggiornare la policy.
type policyWriteRequest struct {
	Type       string `json:"type"`
	PolicyYAML string `json:"policy_yaml"`
}

// policyWriteResponse è la risposta del daemon alla richiesta policy_write.
type policyWriteResponse struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// sendPolicyUpdate invia il nuovo YAML al daemon via Unix socket.
// Restituisce errore se il daemon non è raggiungibile o rifiuta la policy.
func sendPolicyUpdate(socketPath string, yamlContent []byte) error {
	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("daemon non raggiungibile: %w", err)
	}
	defer conn.Close()

	req := policyWriteRequest{
		Type:       "policy_write",
		PolicyYAML: string(yamlContent),
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return fmt.Errorf("errore invio richiesta: %w", err)
	}

	var resp policyWriteResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return fmt.Errorf("errore lettura risposta: %w", err)
	}

	if resp.Decision == "block" {
		return fmt.Errorf("daemon ha rifiutato: %s", resp.Reason)
	}
	return nil
}
