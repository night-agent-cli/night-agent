package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/night-agent-cli/night-agent/internal/mcphook"
	"github.com/spf13/cobra"
)

var mcpHookCmd = &cobra.Command{
	Use:   "mcp-hook",
	Short: "Hook PreToolUse per Claude Code — valuta tool call prima dell'esecuzione",
	Long: `Invocato automaticamente da Claude Code via PreToolUse hook.
Legge il contesto della tool call da stdin (JSON), invia al daemon e
restituisce exit code 0 (allow) o 2 (block).

Configurazione in ~/.claude/settings.json:
  {
    "hooks": {
      "PreToolUse": [{
        "matcher": "*",
        "hooks": [{"type": "command", "command": "/path/to/nightagent mcp-hook"}]
      }]
    }
  }`,
	RunE: runMCPHook,
}

func init() {
	rootCmd.AddCommand(mcpHookCmd)
}

func runMCPHook(cmd *cobra.Command, args []string) error {
	parsed, err := mcphook.ParseStdin(os.Stdin)
	if err != nil {
		// fail-open: input malformato non deve bloccare il workflow
		fmt.Fprintf(os.Stderr, "[night-agent] errore parsing input: %v — consento\n", err)
		os.Exit(0)
	}

	req := mcphook.BuildDaemonRequest(parsed)

	cfgDir, _ := resolveConfigDir()
	socketPath := filepath.Join(cfgDir, "night-agent.sock")

	decision, reason := mcphook.QueryDaemon(socketPath, req)

	code := mcphook.ExitCode(decision)
	if code != 0 {
		fmt.Fprintf(os.Stderr, "[guardian] bloccato: %s — %s\n", parsed.Command, reason)
	} else if decision == "sandbox" {
		fmt.Fprintf(os.Stderr, "[guardian] sandbox: %s — %s\n", parsed.Command, reason)
	}

	os.Exit(code)
	return nil
}

