package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// ANSI — riutilizza le costanti già definite in policy.go
// (stesso package main, stesso file di build)

var helpCmd = &cobra.Command{
	Use:   "help",
	Short: "Mostra tutti i comandi disponibili",
	Run:   runHelp,
}

func init() {
	// sostituisce il comando help di default di cobra
	rootCmd.SetHelpCommand(helpCmd)
	rootCmd.AddCommand(helpCmd)
}

func runHelp(cmd *cobra.Command, args []string) {
	w := os.Stdout

	fmt.Fprintln(w, ansiBold+ansiBoldCyan+`
  ███████╗ ██████╗ ██████╗ ████████╗██╗███████╗██╗   ██╗
  ██╔════╝██╔═══██╗██╔══██╗╚══██╔══╝██║██╔════╝╚██╗ ██╔╝
  █████╗  ██║   ██║██████╔╝   ██║   ██║█████╗   ╚████╔╝
  ██╔══╝  ██║   ██║██╔══██╗   ██║   ██║██╔══╝    ╚██╔╝
  ██║     ╚██████╔╝██║  ██║   ██║   ██║██║        ██║
  ╚═╝      ╚═════╝ ╚═╝  ╚═╝   ╚═╝   ╚═╝╚═╝        ╚═╝`+ansiReset)

	fmt.Fprintln(w, ansiBold+ansiBoldWhite+"  Runtime security layer per agenti AI"+ansiReset)
	fmt.Fprintln(w, ansiDim+"  ─────────────────────────────────────────────────────────"+ansiReset)
	fmt.Fprintln(w)

	section := func(title string) {
		fmt.Fprintf(w, "  %s%s%s\n", ansiBold+ansiCyan, title, ansiReset)
	}
	command := func(name, desc string) {
		fmt.Fprintf(w, "  %s%-34s%s%s%s\n", ansiBold, name, ansiReset, ansiDim+desc, ansiReset)
	}
	sub := func(name, desc string) {
		fmt.Fprintf(w, "    %s%-32s%s%s%s\n", ansiDim, name, ansiReset, ansiDim+desc, ansiReset)
	}

	// Setup
	section("Setup")
	command("guardian init", "Installa Guardian, esegui il wizard di policy")
	command("guardian init --yes", "Installa con tutti i default senza wizard")
	command("guardian uninstall", "Rimuovi Guardian dal sistema")
	fmt.Fprintln(w)

	// Runtime
	section("Runtime")
	command("guardian start", "Avvia il daemon in foreground (terminale dedicato)")
	command("guardian run <agente>", "Avvia un agente AI sotto protezione")
	sub("guardian run claude", "Claude Code")
	sub("guardian run python3 agent.py", "Script Python")
	sub("guardian run node agent.js", "Script Node.js")
	fmt.Fprintln(w)

	// Policy
	section("Policy")
	command("guardian policy list", "Mostra tutte le regole e il loro stato")
	command("guardian policy toggle <id>", "Attiva/disattiva una regola (block ↔ allow)")
	command("guardian policy add", "Aggiungi una nuova regola in modo interattivo")
	command("guardian policy remove <id>", "Rimuovi una regola dalla policy")
	fmt.Fprintln(w)

	// Logs
	section("Logs")
	command("guardian logs", "Mostra l'audit trail degli ultimi eventi")
	sub("guardian logs --limit 20", "Ultimi N eventi")
	sub("guardian logs --decision block", "Filtra per decisione (block/allow)")
	sub("guardian logs --type shell", "Filtra per tipo (shell/git/file)")
	sub("guardian logs --json", "Output raw JSONL")
	fmt.Fprintln(w)

	// Diagnostica
	section("Diagnostica")
	command("guardian doctor", "Verifica che tutto sia configurato correttamente")
	fmt.Fprintln(w)

	fmt.Fprintln(w, ansiDim+"  ─────────────────────────────────────────────────────────"+ansiReset)
	fmt.Fprintf(w, "  %sDocumentazione:%s github.com/pietroperona/agent-guardian\n\n",
		ansiDim, ansiReset)
}
