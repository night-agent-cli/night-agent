# Night Agent CLI

![License: MIT](https://img.shields.io/badge/license-MIT-blue)
![Platform: macOS](https://img.shields.io/badge/platform-macOS-lightgrey)
![Arch: arm64](https://img.shields.io/badge/arch-arm64-lightgrey)
![Release](https://img.shields.io/github/v/release/night-agent-cli/night-agent)

**Intercetta e governa ogni comando eseguito dagli agenti AI sul tuo Mac.**

Gli agenti AI come Claude Code, Codex o Cursor possono eseguire comandi shell, modificare file, fare push su Git — spesso senza che tu li veda. Night Agent si mette in mezzo: ogni azione passa per un policy engine che decide se permetterla, bloccarla o eseguirla in un container Docker isolato, secondo regole che definisci tu in YAML.

---

## Installazione rapida

```bash
curl -sSL https://raw.githubusercontent.com/night-agent-cli/night-agent/main/install.sh | bash
```

Poi inizializza e verifica:

```bash
nightagent init
nightagent doctor
```

`init` avvia un wizard interattivo per configurare le prime regole di policy e registra il daemon come LaunchAgent (si avvia automaticamente al login). `doctor` conferma che tutto funziona.

> **macOS Gatekeeper**: se vedi "cannot be opened because Apple cannot verify the developer", esegui:
> ```bash
> sudo xattr -d com.apple.quarantine /usr/local/bin/nightagent
> sudo xattr -d com.apple.quarantine /usr/local/lib/night-agent/guardian-shim
> sudo xattr -d com.apple.quarantine /usr/local/lib/night-agent/guardian-intercept.dylib
> ```

**Requisiti**: macOS arm64 (Apple Silicon). Docker Desktop opzionale, necessario solo per la modalita sandbox.

---

## Agenti supportati

| Agente | Intercettazione |
|--------|----------------|
| Claude Code | PATH shims + MCP hooks (PreToolUse) |
| Codex CLI | PATH shims |
| GitHub Copilot Workspace | PATH shims |
| Cursor | PATH shims |
| Qualsiasi agente CLI | PATH shims |

I **PATH shims** intercettano i comandi shell (`git`, `curl`, `rm`, `python3`...) per tutti gli agenti.
I **MCP hooks** intercettano le tool call interne di Claude Code (`Bash`, `Edit`, `Write`, `WebFetch`...) che non passano per la shell.

---

## Come funziona

```
Agente AI (Claude Code, python3, bash...)
        |
  [PATH shims]         ogni comando passa per guardian-shim
        |
  [Policy engine]      valuta le regole YAML
        |
  allow    -->  esegue il comando sull'host
  block    -->  blocca con messaggio
  sandbox  -->  esegue in container Docker isolato
        |
  [Audit log]          ogni evento registrato in ~/.night-agent/audit.jsonl
```

Il daemon gira in background, avviato automaticamente al login tramite LaunchAgent macOS.

---

## Integrazione Claude Code

Per intercettare anche le tool call MCP (non solo i comandi shell), aggiungi questo hook in `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "nightagent mcp-hook --tool $TOOL_NAME --input-file $TOOL_INPUT_FILE"
      }]
    }]
  }
}
```

Se `~/.claude/settings.json` non esiste, crealo. Se esiste gia, aggiungi solo la chiave `hooks` preservando il resto. Riavvia Claude Code dopo la modifica.

Da questo momento ogni tool call (`Bash`, `Edit`, `Write`, `Read`, `WebFetch`...) viene valutata dalla stessa policy YAML che governa i comandi shell. Nessuna configurazione doppia.

**Tool intercettati:**

| Tool | Cosa viene valutato |
|------|-------------------|
| `Bash` | comando shell completo + workdir |
| `Edit` | path del file modificato |
| `Write` | path del file scritto |
| `Read` | path del file letto |
| `WebFetch` | URL della richiesta |
| `WebSearch` | query di ricerca |
| tool custom | nome del tool come identificatore |

Se il daemon non e in ascolto, `mcp-hook` consente l'esecuzione senza bloccare (fail-open). Avvia il daemon prima di usare Claude Code:

```bash
nightagent start
```

---

## Policy

Le regole sono in `~/.night-agent/policy.yaml`. Ogni regola ha un'azione (`allow`, `block`, `ask`, `sandbox`) e un pattern di matching.

### Esempio

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
    decision: ask
    reason: "cancellazione ricorsiva richiede conferma"

  - id: ask_git_push_main
    when:
      action_type: git
      command_matches: ["git push * main", "git push * master", "git push --force *"]
    match_type: glob
    decision: ask
    reason: "push su branch protetto richiede conferma"

  - id: sandbox_python_scripts
    when:
      action_type: shell
      command_matches: ["python *.py", "python3 *.py"]
    match_type: glob
    decision: sandbox
    sandbox:
      image: "python:3.12-alpine"
      network: "none"
    reason: "script Python eseguito in ambiente isolato"
```

**Decisioni**: `allow` esegue, `block` blocca, `ask` blocca con messaggio (trattato come block a runtime), `sandbox` esegue in Docker.

### Configurazione sandbox

Il campo `sandbox` si applica solo alle regole con `decision: sandbox`. Il workspace corrente viene montato automaticamente come `/workspace` nel container.

| Campo | Default | Valori |
|-------|---------|--------|
| `image` | `alpine:3.20` | qualsiasi immagine Docker |
| `network` | `none` | `none`, `bridge` |

### Gestione regole via CLI

```bash
nightagent policy list              # mostra tutte le regole
nightagent policy toggle block_sudo # attiva/disattiva una regola
nightagent policy add               # aggiungi una regola interattivamente
nightagent policy remove block_sudo # rimuovi una regola
```

---

## Risk scoring

Night Agent assegna a ogni azione un punteggio di rischio euristico, indipendente dalla policy. Il punteggio e visibile nei log come segnale aggiuntivo — non modifica mai la decisione della policy.

| Segnale | Peso |
|---------|------|
| `sudo` nel comando | +0.50 |
| `curl`/`wget` piped a `bash`/`sh` | +0.70 |
| `rm` ricorsivo (`-r`, `-rf`) | +0.30 |
| `chmod 777` | +0.30 |
| `git push --force` | +0.50 |
| `git push` su `main`/`master` | +0.20 |
| Accesso a path sensibili (`.env`, `.ssh`, `.aws`) | +0.30 |
| Installazione pacchetti (`pip`, `npm`, `brew`) | +0.15 |
| Script shell (`bash *.sh`) | +0.20 |
| Burst anomalo (>10 azioni in 30s) | +0.25 |
| 3 o piu blocchi nelle azioni recenti | +0.25 |

Livelli: `low` (<0.3) · `medium` (0.3-0.7) · `high` (>=0.7). Il `!` nel log segnala un'anomalia contestuale.

---

## Log e audit

```bash
nightagent logs                      # tutti gli eventi
nightagent logs --limit 20           # ultimi 20
nightagent logs --decision block     # solo eventi bloccati
nightagent logs --decision sandbox   # solo eventi sandbox
nightagent logs --json               # output strutturato JSONL
```

Output:

```
TIMESTAMP            DECISIONE  RISCHIO        TIPO   COMANDO                   MOTIVO
2026-04-12 10:01:02  allow      low(0.00)      git    git status
2026-04-12 10:01:15  block      high(0.80)     shell  sudo rm -rf /var/log      sudo disabilitato
2026-04-12 10:01:33  sandbox    medium(0.35)!  shell  python3 deploy.py         script Python in sandbox
```

Ogni evento e firmato con HMAC-SHA256 e collegato al precedente tramite una catena hash (tamper-evident log). Per verificare l'integrita:

```bash
nightagent verify
```

La chiave di firma e in `~/.night-agent/signing.key` (permessi `0600`), generata durante `init` e mai trasmessa.

---

## Comandi

```
nightagent init                      avvia il wizard di configurazione
nightagent init --yes                configura con tutti i default, senza wizard
nightagent start                     avvia il daemon in foreground
nightagent run <agente> [args...]    avvia un agente AI sotto protezione
nightagent sandbox run <cmd>         esegui un comando esplicitamente in sandbox Docker
  --image <img>                      immagine Docker da usare (default: alpine:3.20)
  --network <net>                    modalita rete: none (default) o bridge
nightagent policy list               mostra tutte le regole
nightagent policy toggle <id>        attiva/disattiva una regola
nightagent policy add                aggiungi una regola interattivamente
nightagent policy remove <id>        rimuovi una regola
nightagent logs                      mostra l'audit trail
nightagent logs --decision <d>       filtra per decisione (allow/block/sandbox)
nightagent logs --type <t>           filtra per tipo azione
nightagent logs --limit <n>          limita il numero di righe
nightagent logs --json               output JSONL strutturato
nightagent verify                    verifica integrita firme nell'audit log
nightagent doctor                    diagnostica installazione e stato Docker
nightagent uninstall                 rimuove Night Agent dal sistema
nightagent help                      mostra l'help
```

---

## Build da sorgente

**Prerequisiti**: macOS arm64, Go 1.21+, Xcode Command Line Tools, Docker Desktop (opzionale).

```bash
xcode-select --install
git clone https://github.com/night-agent-cli/night-agent
cd night-agent
make all
./nightagent init
```

`make all` produce tre artifact: `nightagent` (CLI Go), `guardian-shim` (C, intercettazione PATH), `guardian-intercept.dylib` (C, DYLD injection per agenti senza Hardened Runtime).

---

## Limitazioni

- Claude Code e altri agenti con Hardened Runtime non sono intercettabili via `DYLD_INSERT_LIBRARIES`. Night Agent usa PATH shims come approccio principale, compatibile con qualsiasi agente.
- Intercetta comandi eseguiti via shell. Syscall dirette o chiamate native non passano per il layer di interception.
- La sandbox richiede Docker Desktop installato e in esecuzione. Se Docker non e disponibile, le regole `sandbox` fanno fail-safe su `block`.
- Richiede macOS arm64. Linux e Windows al momento non sono ancora supportati.

---

## Licenza

MIT
