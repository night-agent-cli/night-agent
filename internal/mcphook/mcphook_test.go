package mcphook_test

import (
	"strings"
	"testing"

	"github.com/night-agent-cli/night-agent/internal/mcphook"
)

func TestParseInput_BashCommand(t *testing.T) {
	input := `{"command":"sudo rm -rf /tmp","workdir":"/home/user"}`
	parsed, err := mcphook.ParseInput("Bash", input)
	if err != nil {
		t.Fatalf("ParseInput: %v", err)
	}
	if parsed.Command != "sudo rm -rf /tmp" {
		t.Errorf("command: got %q", parsed.Command)
	}
	if parsed.WorkDir != "/home/user" {
		t.Errorf("workdir: got %q", parsed.WorkDir)
	}
}

func TestParseInput_EditFile(t *testing.T) {
	input := `{"file_path":"/etc/passwd","old_string":"foo","new_string":"bar"}`
	parsed, err := mcphook.ParseInput("Edit", input)
	if err != nil {
		t.Fatalf("ParseInput: %v", err)
	}
	if parsed.Path != "/etc/passwd" {
		t.Errorf("path: got %q", parsed.Path)
	}
	if parsed.Command == "" {
		t.Error("command non costruito per Edit")
	}
}

func TestParseInput_WriteFile(t *testing.T) {
	input := `{"file_path":"/home/user/.ssh/authorized_keys","content":"..."}`
	parsed, err := mcphook.ParseInput("Write", input)
	if err != nil {
		t.Fatalf("ParseInput: %v", err)
	}
	if parsed.Path != "/home/user/.ssh/authorized_keys" {
		t.Errorf("path: got %q", parsed.Path)
	}
}

func TestParseInput_UnknownTool(t *testing.T) {
	input := `{"anything":"value"}`
	parsed, err := mcphook.ParseInput("UnknownTool", input)
	if err != nil {
		t.Fatalf("ParseInput su tool sconosciuto non deve fallire: %v", err)
	}
	if parsed.ToolName != "UnknownTool" {
		t.Errorf("tool_name: got %q", parsed.ToolName)
	}
}

func TestBuildDaemonRequest(t *testing.T) {
	parsed := mcphook.ParsedCall{
		ToolName:  "Bash",
		Command:   "git push origin main",
		WorkDir:   "/home/user/project",
		AgentName: "claude-code",
	}
	req := mcphook.BuildDaemonRequest(parsed)
	if req.Command != "git push origin main" {
		t.Errorf("command: got %q", req.Command)
	}
	if req.AgentName != "claude-code" {
		t.Errorf("agent_name: got %q", req.AgentName)
	}
}

func TestExitCode_AllowIsZero(t *testing.T) {
	if mcphook.ExitCode("allow") != 0 {
		t.Error("allow deve restituire exit code 0")
	}
}

func TestExitCode_BlockIsNonZero(t *testing.T) {
	if mcphook.ExitCode("block") == 0 {
		t.Error("block deve restituire exit code non zero")
	}
}

func TestExitCode_SandboxIsZero(t *testing.T) {
	// sandbox = eseguito in isolamento, Claude Code può continuare
	if mcphook.ExitCode("sandbox") != 0 {
		t.Error("sandbox deve restituire exit code 0")
	}
}

func TestParseStdin_BashCommand(t *testing.T) {
	stdin := `{"tool_name":"Bash","tool_input":{"command":"sudo rm -rf /tmp","workdir":"/home/user"}}`
	parsed, err := mcphook.ParseStdin(strings.NewReader(stdin))
	if err != nil {
		t.Fatalf("ParseStdin: %v", err)
	}
	if parsed.ToolName != "Bash" {
		t.Errorf("tool_name: got %q", parsed.ToolName)
	}
	if parsed.Command != "sudo rm -rf /tmp" {
		t.Errorf("command: got %q", parsed.Command)
	}
}

func TestParseStdin_EditFile(t *testing.T) {
	stdin := `{"tool_name":"Edit","tool_input":{"file_path":"/etc/passwd","old_string":"a","new_string":"b"}}`
	parsed, err := mcphook.ParseStdin(strings.NewReader(stdin))
	if err != nil {
		t.Fatalf("ParseStdin: %v", err)
	}
	if parsed.Path != "/etc/passwd" {
		t.Errorf("path: got %q", parsed.Path)
	}
}

func TestParseStdin_MalformedJSON(t *testing.T) {
	_, err := mcphook.ParseStdin(strings.NewReader("not json"))
	if err == nil {
		t.Error("atteso errore su JSON malformato")
	}
}

func TestQueryDaemon_DaemonNotRunning_Blocks(t *testing.T) {
	decision, reason := mcphook.QueryDaemon("/nonexistent/night-agent.sock", mcphook.DaemonRequest{
		Command:   "sudo rm -rf /",
		AgentName: "claude-code",
	})
	if decision != "block" {
		t.Errorf("daemon non raggiungibile: atteso block, got %q", decision)
	}
	if !strings.Contains(reason, "daemon") {
		t.Errorf("reason deve menzionare 'daemon': got %q", reason)
	}
}

func TestQueryDaemon_DaemonNotRunning_ExitCodeIsTwo(t *testing.T) {
	decision, _ := mcphook.QueryDaemon("/nonexistent/night-agent.sock", mcphook.DaemonRequest{
		Command: "ls /etc",
	})
	if mcphook.ExitCode(decision) != 2 {
		t.Errorf("daemon non raggiungibile: atteso exit code 2, got %d", mcphook.ExitCode(decision))
	}
}
