# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Night Agent** è un CLI tool per macOS che funge da runtime security layer per agenti AI (Claude Code, Codex, ecc.). Intercetta e governa le azioni degli agenti prima che vengano eseguite.

Documento di specifica completo: [docs/ai-night-agent-mvp-plan.md](docs/ai-night-agent-mvp-plan.md)

**Stato attuale: Cycle 1 ✅ + Cycle 2 ✅ + Cycle 3 ✅**

## Tech Stack

- **Linguaggio**: Go (single binary) + C (shim + dylib)
- **Config**: YAML
- **Logging**: JSONL
- **Sandbox**: Docker Desktop (Cycle 2)
- **Packaging**: Homebrew tap (futuro)

## Struttura del Progetto

```
cmd/guardian/          CLI principale (Cobra) — init, run, start, logs, policy, doctor, sandbox, uninstall
internal/
  policy/              Policy engine — load/evaluate YAML, glob/regex matching, SandboxConfig
  audit/               Audit logger — JSONL append-only, filtri decision/action_type, campi sandbox + risk
  daemon/              Unix socket server — valuta policy, gestisce sandbox Docker, routing decisioni
  sandbox/             Sandbox manager — docker run wrapper, resolveDockerBinary, BuildDockerArgs
  scorer/              Risk scorer — heuristics pesate, anomaly burst detection (Cycle 3)
  suggestions/         Policy suggestion engine — hints contestuali su path, override, anomalie (Cycle 3)
  shim/                PATH shim — CreateSymlinks, ShimmedCommands (include python/python3)
  intercept/           DYLD injection — per agenti senza Hardened Runtime
  interception/        Normalizer — classifica comandi in shell/git/file
  shell/               Hook injector — preexec in .zshrc, gestisce block + sandbox response
  wizard/              Setup wizard — UI ASCII art + progress bar
  policyeditor/        Policy CRUD — toggle/add/remove rules
  launchagent/         LaunchAgent macOS — autostart al login
configs/
  default_policy.yaml  Policy di default con regole Cycle 1 + Cycle 2 (sandbox)
```

## Architettura — 5 Layer Logici

```
Agente AI
    ↓
[Interception Layer]  ← PATH shims + DYLD injection + shell hook preexec
    ↓
[Policy Engine]       ← valuta regole YAML → allow / block / ask / sandbox
    ↓
[Execution Layer]     ← locale (allow) · bloccato (block) · Docker isolato (sandbox)
    ↓
[Audit Layer]         ← JSONL con campi sandbox (sandboxed, sandbox_image, sandbox_exit_code)
    ↑
[Config & UX Layer]   ← CLI completo
```

## CLI Commands Implementati

```bash
nightagent init [--yes]         # Setup + wizard policy
nightagent start                # Avvia daemon (Unix socket)
nightagent run <agente>         # Avvia agente con interception attiva
nightagent sandbox run <cmd>    # Esegui comando in Docker sandbox (Cycle 2)
  --image <img>                 # Immagine Docker (default: alpine:3.20)
  --network <net>               # Rete: none (default) o bridge
nightagent logs [--decision] [--type] [--limit] [--json]
nightagent policy list|toggle|add|remove
nightagent doctor               # Check installazione + Docker status
nightagent uninstall
nightagent help
```

## Policy Model

```yaml
version: 1
rules:
  - id: block_sudo
    when: {action_type: shell, command_matches: ["sudo *"]}
    match_type: glob
    decision: block
    reason: "sudo disabilitato"

  - id: sandbox_python_scripts
    when: {action_type: shell, command_matches: ["python3 *.py"]}
    match_type: glob
    decision: sandbox
    sandbox:
      image: "python:3.12-alpine"
      network: "none"
    reason: "script Python in ambiente isolato"
```

Decisioni: `allow`, `block`, `ask` (= block a runtime), `sandbox`

## Comportamento Sandbox (Cycle 2)

Quando il policy engine restituisce `sandbox`:

1. Il daemon verifica che Docker sia disponibile (`resolveDockerBinary` — cerca anche in path fissi macOS)
2. Avvia `docker run --rm --network <net> -v <workdir>:/workspace:rw -w /workspace <image> sh -c <cmd>`
3. I path host nel comando vengono riscritti (`workdir` → `/workspace`)
4. Cattura stdout/stderr/exit code
5. Restituisce risposta `{"decision":"sandbox","exit_code":N,"output":"..."}` al shim
6. Il shim stampa `[⬡ sandbox] <cmd> — <reason>` su stderr e propaga l'exit code
7. L'evento viene loggato con `sandboxed: true`, `sandbox_image`, `sandbox_exit_code`

Fail-safe: se Docker non è disponibile → blocca con messaggio esplicito.

## Roadmap

- **Cycle 1** ✅ — Policy engine, PATH shims, DYLD, shell hook, audit log, LaunchAgent
- **Cycle 2** ✅ — Docker sandbox, `nightagent sandbox run`, routing automatico, path rewriting
- **Cycle 3** ✅ — Risk scorer (heuristics), anomaly detection, policy suggestions, risk score in logs

## Repository & Git Workflow

- **Remote**: [github.com/pietroperona/night-agent](https://github.com/pietroperona/night-agent)
- **Branch principali**: `main` (produzione), `develop`
- **Commit**: non citare mai il nome di strumenti AI nei messaggi di commit

## Approccio di Sviluppo — TDD

Tutto lo sviluppo segue **Test-Driven Development**:

1. Scrivi il test (red)
2. Scrivi il codice minimo per farlo passare (green)
3. Refactoring se necessario

```bash
go test ./...                        # tutti i test
go test ./internal/sandbox/...       # solo sandbox
go test -run TestNomeFunzione ./...  # singolo test
make                                 # build completo (dylib + shim + nightagent)
```

## Principi di Design Fondamentali

1. Controlla **azioni**, non intenzioni
2. **Determinismo** prima di intelligenza — no ML per decisioni hard
3. **Explainability** — l'utente deve sempre capire perché qualcosa è bloccato
4. **Safe failure mode** — in caso di errore, blocca (non permette)
5. Security by default, friction minimale
6. Framework-agnostic — funziona con qualsiasi agente AI
