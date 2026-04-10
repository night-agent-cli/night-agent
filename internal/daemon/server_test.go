package daemon_test

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pietroperona/agent-guardian/internal/audit"
	"github.com/pietroperona/agent-guardian/internal/daemon"
	"github.com/pietroperona/agent-guardian/internal/policy"
)

func buildTestPolicy() *policy.Policy {
	return &policy.Policy{
		Version: 1,
		Rules: []policy.Rule{
			{
				ID:        "block_sudo",
				MatchType: policy.MatchGlob,
				When:      policy.Condition{ActionType: "shell", CommandMatches: []string{"sudo *"}},
				Decision:  policy.DecisionBlock,
				Reason:    "sudo disabilitato",
			},
			{
				ID:        "ask_push_main",
				MatchType: policy.MatchGlob,
				When:      policy.Condition{ActionType: "git", CommandMatches: []string{"git push * main"}},
				Decision:  policy.DecisionAsk,
				Reason:    "push su main richiede conferma",
			},
		},
	}
}

func startTestServer(t *testing.T) (socketPath string, logPath string) {
	t.Helper()
	// Su macOS i path Unix socket hanno un limite di 104 caratteri: usiamo /tmp con nome breve.
	socketPath = fmt.Sprintf("/tmp/grd-%d.sock", os.Getpid())
	logDir := t.TempDir()
	logPath = filepath.Join(logDir, "audit.jsonl")
	t.Cleanup(func() { os.Remove(socketPath) })

	p := buildTestPolicy()
	logger, err := audit.NewLogger(logPath)
	if err != nil {
		t.Fatalf("errore creazione logger: %v", err)
	}

	srv, err := daemon.NewServer(socketPath, p, logger)
	if err != nil {
		t.Fatalf("errore creazione server: %v", err)
	}

	go srv.Serve()
	t.Cleanup(func() {
		srv.Stop()
		logger.Close()
	})

	// attendi che il socket sia disponibile
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return socketPath, logPath
}

func sendRequest(t *testing.T, socketPath string, req daemon.Request) daemon.Response {
	t.Helper()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("errore connessione al socket: %v", err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("errore invio richiesta: %v", err)
	}

	var resp daemon.Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("errore lettura risposta: %v", err)
	}
	return resp
}

func TestDaemon_BlocksMatchingRule(t *testing.T) {
	socketPath, _ := startTestServer(t)

	resp := sendRequest(t, socketPath, daemon.Request{
		Command:   "sudo rm -rf /",
		WorkDir:   "/home/user",
		AgentName: "claude-code",
	})

	if resp.Decision != string(policy.DecisionBlock) {
		t.Errorf("atteso block, ottenuto %s", resp.Decision)
	}
	if resp.Reason == "" {
		t.Error("atteso reason non vuoto")
	}
}

func TestDaemon_AllowsNonMatchingCommand(t *testing.T) {
	socketPath, _ := startTestServer(t)

	resp := sendRequest(t, socketPath, daemon.Request{
		Command:   "ls -la",
		WorkDir:   "/home/user",
		AgentName: "claude-code",
	})

	if resp.Decision != string(policy.DecisionAllow) {
		t.Errorf("atteso allow, ottenuto %s", resp.Decision)
	}
}

// TestDaemon_AskDecision verifica che un comando con policy "ask" venga gestito
// dal prompt interattivo. In ambiente di test senza display (osascript fallisce)
// il safe failure produce "block". Il comando NON deve restituire "ask" raw.
func TestDaemon_AskDecision(t *testing.T) {
	socketPath, _ := startTestServer(t)

	resp := sendRequest(t, socketPath, daemon.Request{
		Command:   "git push origin main",
		WorkDir:   "/home/user/project",
		AgentName: "claude-code",
	})

	// in test senza display osascript fallisce → safe failure = block
	// l'importante è che non sia restituito "ask" grezzo al client
	if resp.Decision == string(policy.DecisionAsk) {
		t.Error("il daemon non deve restituire 'ask' al client: deve risolvere internamente")
	}
	validDecisions := map[string]bool{
		string(policy.DecisionAllow): true,
		string(policy.DecisionBlock): true,
	}
	if !validDecisions[resp.Decision] {
		t.Errorf("atteso allow o block, ottenuto %s", resp.Decision)
	}
}

func TestDaemon_WritesAuditLog(t *testing.T) {
	socketPath, logPath := startTestServer(t)

	sendRequest(t, socketPath, daemon.Request{
		Command:   "sudo rm -rf /",
		WorkDir:   "/home/user",
		AgentName: "claude-code",
	})

	// piccola attesa per flush asincrono
	time.Sleep(20 * time.Millisecond)

	events, err := audit.ReadAll(logPath)
	if err != nil {
		t.Fatalf("errore lettura log: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("atteso almeno 1 evento nel log")
	}
	if events[0].Decision != string(policy.DecisionBlock) {
		t.Errorf("atteso decision=block nel log, ottenuto %s", events[0].Decision)
	}
}

func TestDaemon_MultipleClients(t *testing.T) {
	socketPath, _ := startTestServer(t)

	results := make(chan string, 3)
	for range 3 {
		go func() {
			resp := sendRequest(t, socketPath, daemon.Request{
				Command: "ls -la",
				WorkDir: "/home/user",
			})
			results <- resp.Decision
		}()
	}

	for range 3 {
		select {
		case decision := <-results:
			if decision != string(policy.DecisionAllow) {
				t.Errorf("atteso allow, ottenuto %s", decision)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout in attesa risposta")
		}
	}
}
