package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/pietroperona/agent-guardian/internal/audit"
	"github.com/pietroperona/agent-guardian/internal/interception"
	"github.com/pietroperona/agent-guardian/internal/policy"
	"github.com/pietroperona/agent-guardian/internal/sandbox"
)

// Request è il messaggio inviato dalla shell hook al daemon.
type Request struct {
	Command   string `json:"command"`
	WorkDir   string `json:"work_dir"`
	AgentName string `json:"agent_name"`
}

// Response è la risposta del daemon alla shell hook.
type Response struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
	RuleID   string `json:"rule_id"`
	// Campi sandbox: presenti solo quando il daemon ha eseguito il comando in Docker.
	ExitCode *int   `json:"exit_code,omitempty"`
	Output   string `json:"output,omitempty"`
}

// Server è il daemon che ascolta su Unix socket e valuta le richieste.
type Server struct {
	socketPath string
	policy     *policy.Policy
	policyPath string
	logger     *audit.Logger
	listener   net.Listener
	quit       chan struct{}
}

// NewServer crea il daemon e apre il Unix socket.
func NewServer(socketPath string, p *policy.Policy, logger *audit.Logger) (*Server, error) {
	return newServer(socketPath, "", p, logger)
}

// NewServerWithPolicyPath crea il daemon con il path della policy per allow_always.
func NewServerWithPolicyPath(socketPath, policyPath string, p *policy.Policy, logger *audit.Logger) (*Server, error) {
	return newServer(socketPath, policyPath, p, logger)
}

func newServer(socketPath, policyPath string, p *policy.Policy, logger *audit.Logger) (*Server, error) {
	_ = os.Remove(socketPath)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("impossibile creare il socket: %w", err)
	}

	return &Server{
		socketPath: socketPath,
		policy:     p,
		policyPath: policyPath,
		logger:     logger,
		listener:   ln,
		quit:       make(chan struct{}),
	}, nil
}

// Serve avvia il loop di accettazione delle connessioni.
func (s *Server) Serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				continue
			}
		}
		go s.handle(conn)
	}
}

// Stop ferma il daemon.
func (s *Server) Stop() {
	close(s.quit)
	s.listener.Close()
	os.Remove(s.socketPath)
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()

	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		writeError(conn, "richiesta non valida")
		return
	}

	action, err := interception.Normalize(req.Command, req.WorkDir, req.AgentName)
	if err != nil {
		writeError(conn, err.Error())
		return
	}

	result := s.policy.Evaluate(action.ToPolicyAction())

	// "ask" a runtime si comporta come "block" — la configurazione avviene durante init
	decision := result.Decision
	if decision == policy.DecisionAsk {
		decision = policy.DecisionBlock
	}

	event := audit.Event{
		ID:        uuid.New().String(),
		AgentName: req.AgentName,
		WorkDir:   req.WorkDir,
		Command:   req.Command,
		Decision:  string(decision),
		RuleID:    result.RuleID,
		Reason:    result.Reason,
	}

	resp := Response{
		Decision: string(decision),
		Reason:   result.Reason,
		RuleID:   result.RuleID,
	}

	// Gestione sandbox: esegui il comando in Docker e restituisci il risultato.
	if decision == policy.DecisionSandbox {
		mgr := sandbox.New()
		if !mgr.IsAvailable() {
			// Docker non disponibile: fail safe — blocca e notifica.
			event.Decision = string(policy.DecisionBlock)
			event.Reason = "Docker non disponibile — sandbox non attivabile"
			_ = s.logger.Write(event)
			logDecision(policy.DecisionBlock, req.Command, event.Reason)
			resp.Decision = string(policy.DecisionBlock)
			resp.Reason = event.Reason
			_ = json.NewEncoder(conn).Encode(resp)
			return
		}

		cfg := sandbox.Config{
			Image:   sandboxImage(result),
			Network: sandboxNetwork(result),
			WorkDir: req.WorkDir,
		}

		// Carica il profilo sandbox del progetto (.guardian.yaml nella workdir)
		// e lo fonde con la config della regola (la regola ha priorità).
		if req.WorkDir != "" {
			profile, profileErr := sandbox.LoadProfile(req.WorkDir)
			if profileErr != nil {
				fmt.Printf("  avviso profilo sandbox: %v\n", profileErr)
			} else {
				cfg = sandbox.MergeConfig(cfg, profile)
			}
		}

		// Riscrive i path host nel comando con i path container equivalenti.
		// Il workspace è montato come /workspace nel container.
		command := rewriteHostPaths(req.Command, req.WorkDir)

		sandboxResult, execErr := mgr.Execute(context.Background(), command, cfg)
		if execErr != nil {
			event.Decision = string(policy.DecisionBlock)
			event.Reason = fmt.Sprintf("errore sandbox: %v", execErr)
			_ = s.logger.Write(event)
			logDecision(policy.DecisionBlock, req.Command, event.Reason)
			resp.Decision = string(policy.DecisionBlock)
			resp.Reason = event.Reason
			_ = json.NewEncoder(conn).Encode(resp)
			return
		}

		// Aggiorna evento con dettagli sandbox
		event.Sandboxed = true
		event.SandboxImage = cfg.Image
		event.SandboxExitCode = &sandboxResult.ExitCode
		_ = s.logger.Write(event)

		logDecision(policy.DecisionSandbox, req.Command, result.Reason)
		fmt.Printf("  immagine: %s  rete: %s  exit: %d\n", cfg.Image, cfg.Network, sandboxResult.ExitCode)

		resp.ExitCode = &sandboxResult.ExitCode
		resp.Output = sandboxResult.Stdout
		_ = json.NewEncoder(conn).Encode(resp)
		return
	}

	_ = s.logger.Write(event)
	logDecision(decision, req.Command, result.Reason)
	_ = json.NewEncoder(conn).Encode(resp)
}

func writeError(conn net.Conn, msg string) {
	resp := Response{Decision: string(policy.DecisionBlock), Reason: msg}
	_ = json.NewEncoder(conn).Encode(resp)
}

func logDecision(decision policy.Decision, command, reason string) {
	icon := map[policy.Decision]string{
		policy.DecisionAllow:   "✓",
		policy.DecisionBlock:   "✗",
		policy.DecisionAsk:     "?",
		policy.DecisionSandbox: "⬡",
	}[decision]

	cmd := command
	if len(cmd) > 60 {
		cmd = cmd[:57] + "..."
	}

	if reason != "" {
		fmt.Printf("[%s] %s  →  %s\n", icon, cmd, reason)
	} else {
		fmt.Printf("[%s] %s\n", icon, cmd)
	}
}

// sandboxImage restituisce l'immagine Docker dalla SandboxConfig della regola,
// o il default se non specificata.
func sandboxImage(result policy.EvalResult) string {
	if result.Sandbox != nil && result.Sandbox.Image != "" {
		return result.Sandbox.Image
	}
	return sandbox.DefaultImage
}

// sandboxNetwork restituisce la modalità rete dalla SandboxConfig della regola,
// o il default (none) se non specificata.
func sandboxNetwork(result policy.EvalResult) string {
	if result.Sandbox != nil && result.Sandbox.Network != "" {
		return result.Sandbox.Network
	}
	return sandbox.DefaultNetwork
}

// rewriteHostPaths sostituisce i riferimenti al workDir host con /workspace
// nel comando, in modo che i path assoluti funzionino dentro il container.
// Es: "python3 /Users/foo/project/test.py" → "python3 /workspace/test.py"
func rewriteHostPaths(command, workDir string) string {
	if workDir == "" {
		return command
	}
	return strings.ReplaceAll(command, workDir, "/workspace")
}
