# Night Agent — Website Brief

Documento di istruzioni per il web developer.  
Contiene struttura, copy, requisiti tecnici e note di design per ogni sezione della landing page.

---

## Tono e riferimenti visivi

**Tono:** enterprise sobrio, soft nerd. Niente sensazionalismo, niente fear marketing. Il prodotto si presenta come infrastruttura seria — il riferimento è Linear, Tailscale, Fly.io.

**Palette:** scura (dark mode first). Sfondo near-black, testo bianco/grigio chiaro, accento monocromatico o verde terminale tenue. Niente rossi allarmistici.

**Typography:** monospace per codice e UI chrome, sans-serif pulito per il copy.

**Tono del copy:** frasi corte. Niente esclamativi. Niente aggettivi superlativi. Il prodotto parla con i fatti.

---

## Struttura della pagina

```
1. Nav
2. Hero
3. How it works
4. Features (3 colonne)
5. Policy example (codice)
6. Audit log (codice)
7. Install
8. Footer
```

---

## 1. Nav

**Logo:** `Night Agent` — wordmark testo, monospace, niente icone elaborate.

**Link nav (desktop):**
- How it works
- Docs
- GitHub ↗

**CTA nav:** nessuna. Il nav è minimale, niente pulsanti colorati.

**Comportamento:** sticky, sfondo leggermente più scuro dell'hero al scroll (blur/frosted glass opzionale).

---

## 2. Hero

**Layout:** centrato, full-width, padding generoso sopra e sotto.

**Eyebrow (piccolo, monospace, opacità ridotta):**
```
macOS · CLI · Open Source
```

**Headline (H1, grande, peso bold):**
```
Runtime policy enforcement
for AI agents.
```

**Subheadline (corpo, max ~60 caratteri per riga):**
```
Night Agent sits between your system and your AI agents.
Define what they can do. Audit what they did.
```

**CTA block — due elementi in riga:**

1. Input copiabile con syntax highlight:
```bash
brew install nightagent
```
Accompagnato da icona copy-to-clipboard.

2. Link testo:
```
View on GitHub →
```

**Nota design:** nessuna hero image, nessun mockup. Solo tipo e codice. Eventuale elemento decorativo: una riga sottile animata o terminale statico come sfondo a bassa opacità.

---

## 3. How it works

**Titolo sezione (H2):**
```
How it works
```

**Layout:** schema orizzontale a tre step con frecce/connettori. Su mobile diventa verticale.

**Step 1**
```
Label:   AI Agent
Detail:  Claude Code, Codex, or any shell-based agent
```

**Connettore → freccia con label:**
```
intercepts before execution
```

**Step 2**
```
Label:   Night Agent
Detail:  Evaluates every command against your policy rules
```

**Connettore → freccia con tre uscite:**
```
allow  ·  block  ·  sandbox
```

**Step 3**
```
Label:   Your system
Detail:  Only permitted actions reach execution
```

**Nota design:** lo schema deve essere grafico ma asciutto — niente illustrazioni, niente icone decorative. Linee, box, label. Stile quasi wireframe ma rifinito.

---

## 4. Features — tre colonne

**Titolo sezione:** nessuno. Le card parlano da sole.

**Layout:** griglia 3 colonne desktop, 1 colonna mobile. Niente bordi spessi, niente shadow elaborate — separatori sottili o spazio bianco.

---

**Card 1**

```
Titolo:  Policy as code
Body:    Define allow, block, and sandbox rules in YAML.
         Versioned in git, readable by humans.
```

---

**Card 2**

```
Titolo:  Execution control
Body:    Commands are evaluated before they run. Risky
         operations are routed to an isolated Docker
         environment automatically.
```

---

**Card 3**

```
Titolo:  Audit log
Body:    Every agent action is logged as structured JSONL.
         Filterable by decision, command type, or outcome.
         Risk score and anomaly signals included.
```

---

**Card 4**

```
Titolo:  Framework-agnostic
Body:    Works with Claude Code, Codex, and any agent
         that executes shell commands.
```

---

**Card 5**

```
Titolo:  macOS native
Body:    Runs as a LaunchAgent. Starts at login,
         no manual setup after init.
```

---

**Card 6**

```
Titolo:  Low-level interception
Body:    PATH shims and DYLD injection. No agent
         modification required.
```

**Card 7**

```
Titolo:  Risk scoring
Body:    Every action gets a contextual risk score based
         on heuristics — sudo, pipes, sensitive paths,
         action bursts. No ML. No black box.
```

**Card 8**

```
Titolo:  Policy suggestions
Body:    Night Agent surfaces patterns it notices:
         repeated overrides, anomalous sequences, risky
         commands without a rule. You decide what to do.
```

**Nota design:** le card 7 e 8 estendono la griglia a 3×3 (o 4×2 a scelta). Stessa uniformità delle precedenti. Nessun accento cromatico diverso. Titolo in monospace, corpo in sans-serif. Nessun colore di accento per singola card — uniformità totale.

---

## 5. Policy example

**Titolo sezione (H2):**
```
Your rules. Your system.
```

**Subhead:**
```
Write policy in YAML. Night Agent enforces it at runtime.
```

**Blocco codice (syntax highlighted, dark theme, monospace):**

```yaml
version: 1
rules:
  - id: block_rm_rf
    when:
      action_type: shell
      command_matches: ["rm -rf *"]
    decision: block
    reason: "destructive operation not permitted"

  - id: sandbox_python_scripts
    when:
      action_type: shell
      command_matches: ["python3 *.py"]
    decision: sandbox
    sandbox:
      image: "python:3.12-alpine"
      network: "none"
    reason: "execute in isolated environment"

  - id: allow_git_status
    when:
      action_type: git
      command_matches: ["git status", "git log *"]
    decision: allow
```

**Sotto il codice, tre label inline (piccole, monospace):**
```
allow  ·  block  ·  sandbox
```

**Nota design:** il blocco codice è il protagonista visivo della sezione. Occupa 60-70% della larghezza su desktop. Sfondo leggermente più chiaro del background pagina, bordo sottile, niente decorazioni.

---

## 6. Audit log

**Titolo sezione (H2):**
```
Full visibility. Always.
```

**Subhead:**
```
Every command evaluated by Night Agent is logged
as structured JSONL. Queryable, storable, yours.
```

**Blocco codice (come sopra, stesso stile):**

```jsonl
{"timestamp":"2026-04-11T02:14:33Z","agent":"claude-code","action_type":"shell","command":"rm -rf ./dist","decision":"block","reason":"destructive operation not permitted","risk_score":0.30,"risk_level":"medium","risk_signals":["rm ricorsivo"]}
{"timestamp":"2026-04-11T02:14:41Z","agent":"claude-code","action_type":"shell","command":"python3 deploy.py","decision":"sandbox","sandboxed":true,"sandbox_image":"python:3.12-alpine","sandbox_exit_code":0,"risk_score":0.35,"risk_level":"medium","anomaly_detected":true,"suggestions":["burst anomalo rilevato — considera sandbox per questo pattern"]}
{"timestamp":"2026-04-11T02:14:55Z","agent":"claude-code","action_type":"git","command":"git status","decision":"allow","risk_score":0.00,"risk_level":"low"}
```

**Nota design:** il codice può scrollare orizzontalmente su mobile. Non troncarlo. I campi chiave (`decision`, `sandboxed`) possono avere highlight di colore tenue differenziato per valore: verde per allow, neutro per block, blu per sandbox.

---

## 7. Install

**Layout:** sezione centrata, sfondo leggermente diverso (un tono più chiaro o più scuro del body) per creare separazione visiva.

**Titolo (H2):**
```
Get started in 30 seconds.
```

**Tre comandi in sequenza, numerati, monospace:**

```
1.  brew tap pietroperona/nightagent
2.  brew install nightagent
3.  nightagent init
```

**Sotto i comandi, copy piccolo:**
```
Requires macOS and Docker Desktop for sandbox mode.
```

**Due link testo:**
```
Read the docs →     View on GitHub →
```

**Nota design:** nessun pulsante colorato. I link sono testo con freccia. La semplicità della sezione è il messaggio.

---

## 8. Footer

**Layout:** una riga orizzontale, minimal.

**Sinistra:**
```
Night Agent — Open Source
```

**Destra:**
```
GitHub  ·  Docs  ·  MIT License
```

**Nota:** nessun copyright year, nessuna newsletter, nessun social. Pulito.

---

## Note tecniche generali

- **Dark mode only.** Non implementare light mode in questa versione.
- **Nessuna animazione pesante.** Al massimo fade-in leggero sullo scroll (Intersection Observer). Il prodotto è serio, non un portfolio creativo.
- **Performance first.** Nessun framework JS pesante se la pagina è statica. HTML/CSS puro o framework leggero (Astro, 11ty).
- **Codice sempre copiabile.** Tutti i blocchi codice hanno icona copy-to-clipboard.
- **Mobile responsive** ma il target primario è desktop (developer tool).
- **Nessuna immagine raster.** Solo testo, codice, SVG se necessario.
- **Meta tag:** title `Night Agent — Runtime policy enforcement for AI agents`, description `Night Agent sits between your system and your AI agents. Define what they can do. Audit what they did.`
- **OG image:** wordmark `Night Agent` su sfondo scuro, niente altro.
