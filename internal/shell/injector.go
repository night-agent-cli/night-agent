package shell

import (
	"fmt"
	"os"
	"strings"
)

const (
	beginMarker = "# BEGIN guardian"
	endMarker   = "# END guardian"
)

// hookTemplate è la funzione zsh iniettata nel profilo shell.
// Usa preexec (eseguita prima di ogni comando) per intercettare i comandi
// e inviarli al daemon via socat/nc sul Unix socket.
const hookTemplate = `
# BEGIN guardian
# AI Guardian — hook di intercettazione comandi (non modificare manualmente)
_guardian_socket="%s"
_guardian_preexec() {
  local cmd="$1"
  local workdir="$(pwd)"
  if [[ -S "$_guardian_socket" ]]; then
    local payload="{\"command\":$(printf '%%s' "$cmd" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))'),\"work_dir\":\"$workdir\",\"agent_name\":\"\"}"
    local response
    response=$(echo "$payload" | nc -U "$_guardian_socket" 2>/dev/null)
    local decision
    decision=$(echo "$response" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("decision","allow"))' 2>/dev/null)
    if [[ "$decision" == "block" ]]; then
      local reason
      reason=$(echo "$response" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("reason",""))' 2>/dev/null)
      echo "guardian: comando bloccato — $reason" >&2
      return 1
    fi
  fi
}
autoload -Uz add-zsh-hook
add-zsh-hook preexec _guardian_preexec
# END guardian
`

// Inject aggiunge l'hook guardian al file di profilo shell specificato.
// L'operazione è idempotente: se l'hook è già presente non viene duplicato.
func Inject(rcPath, socketPath string) error {
	content, err := os.ReadFile(rcPath)
	if err != nil {
		return fmt.Errorf("impossibile leggere %s: %w", rcPath, err)
	}

	if strings.Contains(string(content), beginMarker) {
		return nil // già iniettato
	}

	hook := fmt.Sprintf(hookTemplate, socketPath)
	updated := string(content) + hook

	return os.WriteFile(rcPath, []byte(updated), 0600)
}

// Remove elimina il blocco guardian dal file di profilo shell.
func Remove(rcPath string) error {
	content, err := os.ReadFile(rcPath)
	if err != nil {
		return fmt.Errorf("impossibile leggere %s: %w", rcPath, err)
	}

	s := string(content)
	start := strings.Index(s, beginMarker)
	end := strings.Index(s, endMarker)

	if start == -1 || end == -1 {
		return nil // nessun hook da rimuovere
	}

	end += len(endMarker)
	// rimuovi anche l'eventuale newline dopo il marker di chiusura
	if end < len(s) && s[end] == '\n' {
		end++
	}

	updated := s[:start] + s[end:]
	return os.WriteFile(rcPath, []byte(updated), 0600)
}

// IsInjected verifica se l'hook guardian è già presente nel file.
func IsInjected(rcPath string) bool {
	content, err := os.ReadFile(rcPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), beginMarker)
}
