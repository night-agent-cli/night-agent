package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/google/uuid"
	"github.com/pietroperona/agent-guardian/internal/audit"
	"github.com/pietroperona/agent-guardian/internal/interception"
	"github.com/pietroperona/agent-guardian/internal/policy"
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
}

// Server è il daemon che ascolta su Unix socket e valuta le richieste.
type Server struct {
	socketPath string
	policy     *policy.Policy
	logger     *audit.Logger
	listener   net.Listener
	quit       chan struct{}
}

// NewServer crea il daemon e apre il Unix socket.
func NewServer(socketPath string, p *policy.Policy, logger *audit.Logger) (*Server, error) {
	// rimuovi socket residuo
	_ = os.Remove(socketPath)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("impossibile creare il socket: %w", err)
	}

	return &Server{
		socketPath: socketPath,
		policy:     p,
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

	event := audit.Event{
		ID:        uuid.New().String(),
		AgentName: req.AgentName,
		WorkDir:   req.WorkDir,
		Command:   req.Command,
		Decision:  string(result.Decision),
		RuleID:    result.RuleID,
		Reason:    result.Reason,
	}
	_ = s.logger.Write(event)

	logDecision(result.Decision, req.Command, result.Reason)

	resp := Response{
		Decision: string(result.Decision),
		Reason:   result.Reason,
		RuleID:   result.RuleID,
	}
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
