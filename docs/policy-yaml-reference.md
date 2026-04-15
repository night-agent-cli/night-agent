# Policy YAML — Riferimento completo

Night Agent usa file YAML per definire le regole di controllo degli agenti AI.
Ogni regola specifica **quando** applicarsi e **cosa fare**.

---

## Struttura base

```yaml
version: 1

rules:
  - id: nome_regola
    when:
      action_type: shell
      command_matches:
        - "sudo *"
    match_type: glob
    decision: block
    reason: "spiegazione mostrata all'utente"
```

Il campo `version` è obbligatorio e deve essere `1`.

---

## Campi di una regola

### `id` — string, obbligatorio

Identificatore univoco della regola. Usato da `nightagent policy toggle <id>` e nei log.

```yaml
id: block_sudo
```

Usare snake_case. Non usare spazi o caratteri speciali.

---

### `when` — oggetto, obbligatorio

Condizione che attiva la regola. Contiene `action_type` e uno tra `command_matches` o `path_matches`.

---

### `when.action_type` — string, obbligatorio

Tipo di azione da intercettare.

| Valore | Intercetta |
|--------|-----------|
| `shell` | Comandi shell (bash, python, npm, git, ecc.) |
| `git` | Operazioni git |
| `file` | Operazioni su file (lettura/scrittura/cancellazione) |

```yaml
when:
  action_type: shell
```

---

### `when.command_matches` — lista di pattern

Lista di pattern da confrontare con il comando completo (inclusi argomenti).
Usato con `action_type: shell` o `action_type: git`.

```yaml
when:
  action_type: shell
  command_matches:
    - "sudo *"
    - "rm -rf *"
```

La regola scatta se **almeno uno** dei pattern matcha.

---

### `when.path_matches` — lista di pattern

Lista di pattern da confrontare con il path del file.
Usato con `action_type: file`.

```yaml
when:
  action_type: file
  path_matches:
    - "~/.ssh/*"
    - "**/.env"
    - "**/.env.*"
```

---

### `match_type` — string, opzionale

Modalità di confronto dei pattern. Default: `glob`.

| Valore | Comportamento |
|--------|--------------|
| `glob` | Pattern con wildcard `*` e `**` (default) |
| `regex` | Espressione regolare completa |

```yaml
match_type: glob     # default
match_type: regex    # per pattern avanzati
```

---

### `decision` — string, obbligatorio

Cosa fare quando la regola matcha.

| Valore | Comportamento |
|--------|--------------|
| `allow` | Consenti l'azione |
| `block` | Blocca l'azione, mostra `reason` |
| `ask` | Trattato come `block` — richiede conferma manuale |
| `sandbox` | Esegui in container Docker isolato |

```yaml
decision: block
```

---

### `reason` — string, obbligatorio

Messaggio mostrato all'utente quando la regola scatta (block/ask/sandbox).
Comparirà nel log e nel terminale.

```yaml
reason: "sudo è disabilitato per gli agenti AI"
```

---

### `sandbox` — oggetto, opzionale

Obbligatorio se `decision: sandbox`. Configura il container Docker.

```yaml
decision: sandbox
sandbox:
  image: "python:3.12-alpine"
  network: "none"
```

| Campo | Tipo | Default | Valori |
|-------|------|---------|--------|
| `image` | string | `alpine:3.20` | Qualsiasi immagine Docker valida |
| `network` | string | `none` | `none` (isolato), `bridge` (internet) |

Il workspace corrente viene montato come `/workspace` nel container.
`/tmp` è montato read-only.

---

## Glob — sintassi

Quando `match_type: glob` (default):

| Pattern | Matcha |
|---------|--------|
| `*` | Qualsiasi sequenza di caratteri (inclusi spazi e `/` nei comandi) |
| `**` | Qualsiasi sequenza incluso separatori di path (nei path_matches) |
| `?` | Un singolo carattere |
| `[abc]` | Uno tra i caratteri elencati |

Esempi pratici:

```yaml
command_matches:
  - "sudo *"            # sudo qualsiasi-cosa
  - "rm -rf *"          # rm -rf qualsiasi-cosa
  - "curl * | *"        # curl con pipe
  - "git push * main"   # push su main da qualsiasi remote
  - "python3 *.py"      # qualsiasi script .py
  - "bash *.sh"         # qualsiasi script .sh

path_matches:
  - "~/.ssh/*"          # qualsiasi file in .ssh
  - "**/.env"           # .env in qualsiasi subdirectory
  - "**/.env.*"         # .env.local, .env.production, ecc.
  - "~/.aws/*"          # credenziali AWS
```

---

## Regex — sintassi

Quando `match_type: regex`, il pattern è una regexp Go standard.

```yaml
- id: sandbox_python
  when:
    action_type: shell
    command_matches:
      - "python3?\\s+.*\\.py"
  match_type: regex
  decision: sandbox
  sandbox:
    image: "python:3.12-alpine"
    network: "none"
  reason: "script Python in sandbox"
```

Nota: usare `\\s` invece di `\s` (il backslash va escapato in YAML).

---

## Ordine di valutazione

Le regole vengono valutate **in ordine dall'alto verso il basso**.
La **prima regola che matcha** vince. Le successive vengono ignorate.

```yaml
rules:
  - id: allow_safe_rm        # prima regola: allow per rm specifico
    when:
      action_type: shell
      command_matches: ["rm ./build/*"]
    decision: allow

  - id: block_rm_rf          # seconda regola: block rm -rf generico
    when:
      action_type: shell
      command_matches: ["rm -rf *"]
    decision: block
    reason: "cancellazione ricorsiva bloccata"
```

In questo esempio `rm ./build/*` è consentito, `rm -rf qualsiasi` è bloccato.

Se nessuna regola matcha → `allow` (fail-open).

---

## Priorità file policy

Night Agent carica la policy con questa priorità (la prima trovata vince):

1. **Cloud** — policy sincronizzata dal server (se connesso)
2. **Locale progetto** — `nightagent-policy.yaml` nella directory corrente o in un parent
3. **Locale config dir** — `.nightagent/policy.yaml` nella directory corrente
4. **Globale** — `~/.night-agent/policy.yaml`
5. **Permissiva** — tutto allow (se nessun file trovato)

Per forzare la policy globale ignorando quella del progetto:

```bash
nightagent start --local-policy-only
```

---

## Esempi completi

### Policy minimale

```yaml
version: 1

rules:
  - id: block_sudo
    when:
      action_type: shell
      command_matches: ["sudo *"]
    decision: block
    reason: "sudo disabilitato"
```

---

### Policy per progetto Python

```yaml
version: 1

rules:
  - id: sandbox_python
    when:
      action_type: shell
      command_matches: ["python3 *.py", "python *.py"]
    match_type: glob
    decision: sandbox
    sandbox:
      image: "python:3.12-alpine"
      network: "none"
    reason: "script Python eseguito in ambiente isolato"

  - id: sandbox_pip
    when:
      action_type: shell
      command_matches: ["pip install *", "pip3 install *"]
    match_type: glob
    decision: sandbox
    sandbox:
      image: "python:3.12-alpine"
      network: "bridge"
    reason: "installazione pacchetti in sandbox (rete abilitata)"

  - id: block_sensitive
    when:
      action_type: file
      path_matches: ["**/.env", "**/.env.*", "~/.ssh/*"]
    decision: block
    reason: "accesso a file sensibili non consentito"
```

---

### Policy con override allow esplicito

```yaml
version: 1

rules:
  # allow esplicito prima del block generico
  - id: allow_rm_dist
    when:
      action_type: shell
      command_matches: ["rm -rf ./dist", "rm -rf ./build"]
    decision: allow

  - id: block_rm_rf
    when:
      action_type: shell
      command_matches: ["rm -rf *", "rm -fr *"]
    decision: block
    reason: "cancellazione ricorsiva bloccata"

  - id: ask_git_push
    when:
      action_type: git
      command_matches: ["git push * main", "git push --force *"]
    decision: ask
    reason: "push su branch protetto — conferma richiesta"

  - id: block_remote_scripts
    when:
      action_type: shell
      command_matches: ["curl * | *", "wget * | *", "bash <(*"]
    decision: block
    reason: "esecuzione di script remoti non consentita"

  - id: block_sudo
    when:
      action_type: shell
      command_matches: ["sudo *"]
    decision: block
    reason: "sudo disabilitato per agenti AI"
```

---

## Comandi utili per gestire la policy

```bash
nightagent policy list              # mostra tutte le regole attive
nightagent policy toggle <id>       # attiva/disattiva una regola (block ↔ allow)
nightagent policy add               # aggiunge una regola in modo interattivo
nightagent policy remove <id>       # rimuove una regola

nightagent start                    # hot-reload: le modifiche al file sono applicate senza restart
```
