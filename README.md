# Fortify

**Runtime security layer per agenti AI su macOS.**

Fortify si mette tra te e gli agenti AI (Claude Code, Codex, ecc.) e intercetta ogni comando pericoloso prima che venga eseguito — `sudo`, `rm -rf`, `git push --force`, accesso a file sensibili. Tu decidi le regole. L'agente le rispetta.

---

## Come funziona

```
Agente AI (Claude Code, python3, node...)
        ↓
  [PATH shims]  ←  ogni comando passa per guardian-shim
        ↓
  [Policy engine]  ←  valuta le regole YAML
        ↓
  allow → esegue il comando
  block → blocca con messaggio
        ↓
  [Audit log]  ←  ogni evento viene registrato in ~/.guardian/audit.jsonl
```

Il daemon gira in background e viene avviato automaticamente al login tramite LaunchAgent macOS.

---

## Installazione

### Prerequisiti

- macOS (arm64 o x86_64)
- Go 1.21+
- Clang (incluso in Xcode Command Line Tools)

```bash
xcode-select --install
```

### Build

```bash
git clone https://github.com/pietroperona/agent-guardian
cd agent-guardian
make all
```

Produce tre binari nella root del progetto:
- `guardian` — CLI principale
- `guardian-shim` — binario C per l'interception PATH
- `guardian-intercept.dylib` — libreria per DYLD injection (agenti senza Hardened Runtime)

### Setup

```bash
./guardian init
```

Il wizard ti guida nella configurazione delle regole di policy. Al termine:
- La policy viene salvata in `~/.guardian/policy.yaml`
- Il daemon viene registrato come LaunchAgent (avvio automatico al login)
- Gli shims vengono installati in `~/.guardian/shims/`

---

## Utilizzo

### Avvia un agente sotto protezione

```bash
./guardian run claude
./guardian run python3 my_agent.py
./guardian run node agent.js
```

### Verifica che tutto funzioni

```bash
./guardian doctor
```

### Vedi cosa sta succedendo

```bash
./guardian logs
./guardian logs --limit 20
./guardian logs --decision block
```

---

## Gestione policy

```bash
# Mostra tutte le regole
guardian policy list

# Attiva/disattiva una regola
guardian policy toggle block_sudo

# Aggiungi una regola custom
guardian policy add

# Rimuovi una regola
guardian policy remove block_sudo
```

### Esempio policy.yaml

```yaml
version: 1
rules:
  - id: block_sudo
    when:
      action_type: shell
      command_matches: ["sudo *", "*/sudo *"]
    match_type: glob
    decision: block
    reason: "sudo disabilitato per gli agenti AI"

  - id: block_rm_rf
    when:
      action_type: shell
      command_matches: ["rm -rf *", "rm -fr *"]
    match_type: glob
    decision: block
    reason: "cancellazione ricorsiva bloccata"

  - id: ask_git_push_main
    when:
      action_type: git
      command_matches: ["git push * main", "git push * master", "git push --force *"]
    match_type: glob
    decision: block
    reason: "push su branch protetto bloccato"
```

Decisioni disponibili: `allow`, `block`, `ask` (trattato come `block` a runtime).

---

## Comandi disponibili

```
guardian init                     Installa Guardian, esegui il wizard di policy
guardian init --yes               Installa con tutti i default senza wizard
guardian start                    Avvia il daemon in foreground
guardian run <agente> [args...]   Avvia un agente AI sotto protezione
guardian policy list              Mostra tutte le regole
guardian policy toggle <id>       Attiva/disattiva una regola
guardian policy add               Aggiungi una regola interattivamente
guardian policy remove <id>       Rimuovi una regola
guardian logs                     Mostra l'audit trail
guardian doctor                   Diagnostica installazione
guardian uninstall                Rimuovi Guardian dal sistema
guardian help                     Mostra questo help
```

---

## Limitazioni note

- **Claude Code** (e altri agenti con Hardened Runtime) non sono intercettabili via `DYLD_INSERT_LIBRARIES`. Fortify usa PATH shims come approccio principale, che funziona con qualsiasi agente.
- Fortify intercetta comandi eseguiti via shell (`bash`, `sh`). Chiamate syscall dirette in Go o chiamate native non passano per il layer di interception.
- Richiede macOS. Linux e Windows non sono supportati nel Cycle 1.

---

## Roadmap

- **Cycle 1 (attuale)** — Policy engine rule-based, PATH shims, audit log, LaunchAgent
- **Cycle 2** — Docker sandbox integration, isolamento di rete
- **Cycle 3** — Risk scoring, anomaly detection, suggerimenti policy automatici

---

## Licenza

MIT
