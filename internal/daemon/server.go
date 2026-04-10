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
	"github.com/pietroperona/agent-guardian/internal/prompt"
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
	policyPath string // path per scrivere regole allow_always
	logger     *audit.Logger
	listener   net.Listener
	quit       chan struct{}
	session    *prompt.SessionAllowlist
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
	// rimuovi socket residuo
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
		session:    prompt.NewSessionAllowlist(),
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

	finalDecision := result.Decision
	finalReason := result.Reason

	// gestione decisione ask: mostra prompt interattivo
	if result.Decision == policy.DecisionAsk {
		finalDecision, finalReason = s.handleAsk(req, result)
	}

	event := audit.Event{
		ID:        uuid.New().String(),
		AgentName: req.AgentName,
		WorkDir:   req.WorkDir,
		Command:   req.Command,
		Decision:  string(finalDecision),
		RuleID:    result.RuleID,
		Reason:    finalReason,
	}
	_ = s.logger.Write(event)

	logDecision(finalDecision, req.Command, finalReason)

	resp := Response{
		Decision: string(finalDecision),
		Reason:   finalReason,
		RuleID:   result.RuleID,
	}
	_ = json.NewEncoder(conn).Encode(resp)
}

// handleAsk gestisce la decisione "ask": controlla la session allowlist,
// mostra il dialog macOS, e gestisce la risposta dell'utente.
func (s *Server) handleAsk(req Request, result policy.EvalResult) (policy.Decision, string) {
	agentName := req.AgentName
	if agentName == "" {
		agentName = "agente sconosciuto"
	}

	// controlla session allowlist
	if s.session.IsAllowed(agentName, req.Command) {
		return policy.DecisionAllow, "consentito per questa sessione"
	}

	// mostra dialog macOS
	response := prompt.Ask(agentName, req.Command, result.Reason)

	switch response {
	case prompt.ResponseAllowOnce:
		return policy.DecisionAllow, "consentito dall'utente (una volta)"

	case prompt.ResponseAllowSession:
		s.session.Add(agentName, req.Command)
		return policy.DecisionAllow, "consentito per questa sessione"

	case prompt.ResponseAllowAlways:
		if s.policyPath != "" {
			if err := policy.AppendAllowRule(s.policyPath, agentName, req.Command); err != nil {
				fmt.Printf("[!] errore scrittura regola allow_always: %v\n", err)
			} else {
				// ricarica policy per applicare immediatamente
				if p, err := policy.Load(s.policyPath); err == nil {
					s.policy = p
				}
			}
		}
		return policy.DecisionAllow, "consentito sempre per questo agente"

	default: // ResponseBlock
		return policy.DecisionBlock, result.Reason + " (bloccato dall'utente)"
	}
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
