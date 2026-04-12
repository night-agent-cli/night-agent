package wizard

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var ansiRe = regexp.MustCompile(`\033\[[0-9;]*m`)

// StripANSI rimuove i codici escape ANSI da una stringa.
func StripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// ANSI color codes
const (
	reset     = "\033[0m"
	bold      = "\033[1m"
	dim       = "\033[2m"
	red       = "\033[31m"
	green     = "\033[32m"
	yellow    = "\033[33m"
	blue      = "\033[34m"
	magenta   = "\033[35m"
	cyan      = "\033[36m"
	white     = "\033[37m"
	bgRed     = "\033[41m"
	bgGreen   = "\033[42m"
	boldRed   = "\033[1;31m"
	boldGreen = "\033[1;32m"
	boldCyan  = "\033[1;36m"
	boldWhite = "\033[1;37m"
)

var logo = bold + cyan + `
  ███╗   ██╗██╗ ██████╗ ██╗  ██╗████████╗
  ████╗  ██║██║██╔════╝ ██║  ██║╚══██╔══╝
  ██╔██╗ ██║██║██║  ███╗███████║   ██║
  ██║╚██╗██║██║██║   ██║██╔══██║   ██║
  ██║ ╚████║██║╚██████╔╝██║  ██║   ██║
  ╚═╝  ╚═══╝╚═╝ ╚═════╝ ╚═╝  ╚═╝   ╚═╝

   █████╗  ██████╗ ███████╗███╗   ██╗████████╗
  ██╔══██╗██╔════╝ ██╔════╝████╗  ██║╚══██╔══╝
  ███████║██║  ███╗█████╗  ██╔██╗ ██║   ██║
  ██╔══██║██║   ██║██╔══╝  ██║╚██╗██║   ██║
  ██║  ██║╚██████╔╝███████╗██║ ╚████║   ██║
  ╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝  ╚═══╝   ╚═╝
` + reset

// Question rappresenta una domanda del wizard di configurazione.
type Question struct {
	Label        string // identificatore interno
	RuleID       string // ID della regola nella policy
	Description  string // testo mostrato all'utente
	Detail       string // spiegazione aggiuntiva del rischio
	Icon         string // emoji/simbolo per la regola
	DefaultBlock bool   // true = default blocca, false = default consenti
}

// Prompt restituisce la stringa da mostrare all'utente per questa domanda.
func (q Question) Prompt() string {
	var hint string
	if q.DefaultBlock {
		hint = bold + red + "S" + reset + dim + "/n" + reset
	} else {
		hint = dim + "s/" + reset + bold + green + "N" + reset
	}
	return fmt.Sprintf("  %s Bloccare? [%s] ", white+">"+reset, hint)
}

// ParseAnswer interpreta la risposta dell'utente.
// Accetta y/Y/s/S/si/yes come "blocca", n/N/no come "non bloccare".
// Stringa vuota → usa il default.
func ParseAnswer(input string, defaultBlock bool) bool {
	input = strings.TrimSpace(strings.ToLower(input))
	switch input {
	case "y", "s", "si", "yes":
		return true
	case "n", "no":
		return false
	default:
		return defaultBlock
	}
}

// DefaultQuestions restituisce le domande standard del wizard.
func DefaultQuestions() []Question {
	return []Question{
		{
			Label:        "sudo",
			RuleID:       "block_sudo",
			Icon:         "🔐",
			Description:  "sudo — escalation privilegi",
			Detail:       "Permette all'agente di eseguire comandi come root",
			DefaultBlock: true,
		},
		{
			Label:        "rm_rf",
			RuleID:       "block_rm_rf",
			Icon:         "🗑️",
			Description:  "rm -rf — cancellazione ricorsiva",
			Detail:       "Elimina file e directory in modo irreversibile",
			DefaultBlock: true,
		},
		{
			Label:        "curl_pipe",
			RuleID:       "block_curl_pipe",
			Icon:         "🌐",
			Description:  "curl/wget | bash — esecuzione script remoti",
			Detail:       "Scarica ed esegue codice arbitrario da internet",
			DefaultBlock: true,
		},
		{
			Label:        "sensitive_paths",
			RuleID:       "block_sensitive_paths",
			Icon:         "🔑",
			Description:  "File sensibili — ~/.ssh, ~/.aws, .env",
			Detail:       "Accesso a chiavi SSH, credenziali cloud e segreti",
			DefaultBlock: true,
		},
		{
			Label:        "git_push_main",
			RuleID:       "ask_git_push_main",
			Icon:         "🚀",
			Description:  "git push su main/master o --force",
			Detail:       "Push diretto su branch protetti o riscrittura storia",
			DefaultBlock: true,
		},
	}
}

// Run esegue il wizard interattivo su reader/writer e restituisce
// la lista di RuleID da mantenere abilitati (decision=block).
func Run(r io.Reader, w io.Writer) (blocked []string, err error) {
	questions := DefaultQuestions()
	total := len(questions)

	// header
	fmt.Fprint(w, logo)
	fmt.Fprintln(w, dim+"  ─────────────────────────────────────────────────────────"+reset)
	fmt.Fprintln(w, bold+white+"  Runtime security layer per agenti AI"+reset)
	fmt.Fprintln(w, dim+"  ─────────────────────────────────────────────────────────"+reset)
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold+"  Configurazione Policy"+reset)
	fmt.Fprintln(w, dim+"  Scegli quali azioni bloccare. Premi Invio per il default ("+
		bold+red+"S"+reset+dim+"=blocca)."+reset)
	fmt.Fprintln(w)

	scanner := bufio.NewScanner(r)
	results := make([]bool, 0, total)

	for i, q := range questions {
		// progress bar
		progressBar := renderProgress(i+1, total)
		fmt.Fprintf(w, "  %s  %s%d/%d%s\n",
			progressBar,
			dim, i+1, total, reset)

		// domanda
		fmt.Fprintf(w, "\n  %s  %s%s%s\n",
			q.Icon,
			bold+white, q.Description, reset)
		fmt.Fprintf(w, "     %s%s%s\n", dim, q.Detail, reset)
		fmt.Fprint(w, q.Prompt())

		scanner.Scan()
		answer := scanner.Text()
		block := ParseAnswer(answer, q.DefaultBlock)
		results = append(results, block)

		// feedback inline
		if block {
			fmt.Fprintf(w, "  %s✗ bloccato%s\n\n", boldRed, reset)
			blocked = append(blocked, q.RuleID)
		} else {
			fmt.Fprintf(w, "  %s✓ consentito%s\n\n", boldGreen, reset)
		}
	}

	// summary
	printSummary(w, questions, results)

	return blocked, nil
}

func renderProgress(current, total int) string {
	width := 20
	filled := (current * width) / total
	bar := "["
	for i := 0; i < width; i++ {
		if i < filled {
			bar += cyan + "█" + reset
		} else {
			bar += dim + "░" + reset
		}
	}
	bar += "]"
	return bar
}

func printSummary(w io.Writer, questions []Question, results []bool) {
	fmt.Fprintln(w, dim+"  ─────────────────────────────────────────────────────────"+reset)
	fmt.Fprintln(w, bold+"  Riepilogo configurazione"+reset)
	fmt.Fprintln(w)

	for i, q := range questions {
		if results[i] {
			fmt.Fprintf(w, "  %s %-42s %s✗ BLOCCATO%s\n",
				q.Icon, q.Description, boldRed, reset)
		} else {
			fmt.Fprintf(w, "  %s %-42s %s✓ CONSENTITO%s\n",
				q.Icon, q.Description, boldGreen, reset)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, dim+"  ─────────────────────────────────────────────────────────"+reset)
	fmt.Fprintln(w, bold+green+"  Night Agent è pronto. "+reset+
		dim+"Avvia il daemon con: night-agent start"+reset)
	fmt.Fprintln(w)
}
