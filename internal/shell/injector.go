package shell

import (
	"fmt"
	"os"
	"strings"
)

const (
	beginMarker = "# BEGIN nightagent"
	endMarker   = "# END nightagent"
)

// hookTemplate è la funzione zsh iniettata nel profilo shell.
// Usa preexec (eseguita prima di ogni comando) per intercettare i comandi.
// Un singolo processo Python gestisce encoding JSON, comunicazione Unix socket
// e parsing della risposta — 4x più efficiente del template precedente.
const hookTemplate = `
# BEGIN nightagent
# Night Agent — hook di intercettazione comandi (non modificare manualmente)
_nightagent_socket="%s"
_nightagent_preexec() {
  local cmd="$1"
  local workdir="$(pwd)"
  if [[ -S "$_nightagent_socket" ]]; then
    local result
    result=$(python3 - "$cmd" "$workdir" "$_nightagent_socket" 2>/dev/null <<'__NIGHTAGENT_PY__'
import json, socket as _s, sys
cmd, workdir, sock_path = sys.argv[1], sys.argv[2], sys.argv[3]
try:
    payload = json.dumps({"command": cmd, "work_dir": workdir, "agent_name": ""})
    s = _s.socket(_s.AF_UNIX, _s.SOCK_STREAM)
    s.settimeout(2)
    s.connect(sock_path)
    s.sendall(payload.encode())
    s.shutdown(_s.SHUT_WR)
    data = b""
    while True:
        chunk = s.recv(4096)
        if not chunk:
            break
        data += chunk
    s.close()
    resp = json.loads(data)
    print(resp.get("decision", "allow"))
    print(resp.get("reason", ""))
    print(resp.get("output", ""))
except Exception:
    print("allow")
    print("")
    print("")
__NIGHTAGENT_PY__
)
    local decision reason output
    decision=$(printf '%%s' "$result" | head -1)
    reason=$(printf '%%s' "$result" | sed -n '2p')
    output=$(printf '%%s' "$result" | tail -n +3)
    if [[ "$decision" == "block" ]]; then
      echo "nightagent: comando bloccato — $reason" >&2
      return 1
    fi
    if [[ "$decision" == "sandbox" ]]; then
      echo "nightagent: esecuzione in sandbox — $reason" >&2
      [[ -n "$output" ]] && echo "$output"
      return 1
    fi
  fi
}
autoload -Uz add-zsh-hook
add-zsh-hook preexec _nightagent_preexec
# END nightagent
`

// Inject aggiunge l'hook nightagent al file di profilo shell specificato.
// L'operazione è idempotente: se l'hook è già presente non viene duplicato.
// Restituisce (true, nil) se l'hook è stato iniettato ora,
// (false, nil) se era già presente, (false, err) in caso di errore.
func Inject(rcPath, socketPath string) (bool, error) {
	content, err := os.ReadFile(rcPath)
	if err != nil {
		return false, fmt.Errorf("impossibile leggere %s: %w", rcPath, err)
	}

	if strings.Contains(string(content), beginMarker) {
		return false, nil // già iniettato
	}

	hook := fmt.Sprintf(hookTemplate, socketPath)
	updated := string(content) + hook

	return true, os.WriteFile(rcPath, []byte(updated), 0600)
}

// Remove elimina il blocco nightagent dal file di profilo shell.
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

// IsInjected verifica se l'hook nightagent è già presente nel file.
func IsInjected(rcPath string) bool {
	content, err := os.ReadFile(rcPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), beginMarker)
}
