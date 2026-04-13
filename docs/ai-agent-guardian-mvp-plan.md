# AI Agent Guardian
Piano operativo prodotto + MVP in 3 cicli

## 1. Scopo del documento

Questo documento serve ad allineare team prodotto, team tecnico e stakeholder su:

- problema da risolvere
- tesi di mercato
- proposta di valore
- perimetro iniziale
- architettura MVP
- roadmap in 3 cicli in logica Lean Startup
- rischi, metriche e decisioni tecniche

Obiettivo pratico: costruire una prima versione installabile su Mac che protegga l'esecuzione di agenti come Codex, Claude Code e strumenti simili, senza rompere il workflow dell'utente.

---

## 2. Visione del prodotto

AI Agent Guardian è un layer di controllo che si posiziona tra un agente AI e gli strumenti che può usare davvero.

Non protegge "il modello" in astratto.
Protegge le azioni eseguite dall'agente su:

- shell
- file system
- git
- processi
- rete
- API esterne
- ambienti di esecuzione locali o isolati

Idea guida:

Un firewall + policy engine + audit layer per agenti AI.

Nel tempo il prodotto può evolvere da tool locale per sviluppatori a piattaforma di runtime governance per agenti enterprise.

---

## 3. Problema che vogliamo risolvere

Gli agenti AI stanno iniziando a fare azioni concrete:

- leggere e modificare file
- lanciare comandi shell
- installare pacchetti
- fare commit e push
- aprire browser
- usare API e credenziali
- interagire con dati sensibili

Questo abilita produttività ma introduce rischi nuovi:

- cancellazione accidentale di file
- modifica di codice su path sbagliati
- esecuzione di comandi distruttivi
- esfiltrazione di dati
- uso improprio di token e segreti
- push involontari su branch protetti
- chain di azioni innocue che insieme diventano pericolose

Oggi l'utente ha due opzioni deboli:

1. fidarsi dell'agente
2. controllare manualmente ogni step

Manca un layer intermedio che dica:

"questo l'agente lo può fare"
"questo lo deve chiedere"
"questo va bloccato"
"questo può farlo solo in ambiente isolato"

---

## 4. Tesi di prodotto

La tesi è:

Se gli agenti avranno sempre più capacità operative, allora emergerà una categoria software dedicata a controllare il loro runtime, allo stesso modo in cui firewall, antivirus, IAM e osservabilità hanno controllato i sistemi tradizionali.

Il wedge iniziale non deve essere "sicurezza AI" generica.
Deve essere un caso d'uso concreto e ad alta percezione di rischio:

protezione locale di agenti che eseguono codice e comandi su macchina sviluppatore.

Perché questo wedge è forte:

- problema immediatamente comprensibile
- valore dimostrabile in demo
- possibilità di uso diretto su Mac
- feedback rapido
- integrazione iniziale più semplice
- spazio per espansione verso team engineering e poi aziende

---

## 5. Utente iniziale

### 5.1 Utente primario
Sviluppatore, founder tecnico, AI power user che usa agenti come:

- Claude Code
- Codex / agent CLI
- tool agentici con shell access
- wrapper interni su LLM + tool use

### 5.2 Pain principali
- paura di fare danni locali
- poca visibilità su cosa stia realmente facendo l'agente
- mancanza di confini chiari
- approvazioni troppo manuali o troppo assenti
- bisogno di audit minimo

### 5.3 Job to be done
"Voglio usare agenti in modo molto più spinto, senza rischiare di compromettere file, repo, macchina o workflow."

---

## 6. Proposta di valore

### Valore utente
- più fiducia nell'uso quotidiano degli agenti
- riduzione dei danni accidentali
- visibilità su cosa è successo
- controllo granulare senza dover verificare tutto a mano

### Valore tecnico
- point of control unico
- policy semplici e trasparenti
- installazione locale
- possibilità di enforcement reale, non solo monitoraggio

### Valore business
- primo step verso categoria "runtime security for agents"
- possibile espansione verso team e compliance
- differenziazione rispetto a semplici guardrails applicativi

---

## 7. Principi di design

1. Il prodotto controlla le azioni, non le intenzioni
2. Determinismo prima di intelligenza
3. Spiegabilità prima di sofisticazione
4. Sicurezza by default, frizione minima compatibile con il valore
5. L'utente deve capire sempre perché qualcosa è stato bloccato
6. Il sistema deve poter fallire in modo sicuro
7. Isolamento e policy sono complementari
8. L'MVP deve essere framework-agnostic quanto possibile

---

## 8. Cosa NON è il prodotto all'inizio

- non è un antivirus tradizionale
- non è un prodotto di EDR completo
- non è un SIEM
- non è un sandbox cloud enterprise
- non è un plugin dedicato a un singolo vendor
- non è un sistema di classificazione general purpose dei prompt

L'MVP è un runtime guardian locale per agenti con capacità operative.

---

## 9. Esperienza d'uso ideale

### Scenario base
1. L'utente installa AI Agent Guardian sul Mac
2. Configura la modalità protetta per uno o più progetti
3. Usa Claude Code / Codex quasi normalmente
4. Il Guardian intercetta comandi e operazioni rilevanti
5. In base alla policy:
   - consente
   - blocca
   - chiede approvazione
   - forza esecuzione in sandbox
6. Tutto viene loggato
7. L'utente può vedere cosa è successo e modificare le policy

### Esempio concreto
L'agente prova a:
- fare `rm -rf ./build`
- modificare file sotto `.env`
- fare `git push origin main`

Il Guardian:
- può consentire il primo solo in path temporanei
- blocca la modifica di file sensibili
- richiede approvazione per il push

---

## 10. Architettura funzionale di alto livello

Il prodotto avrà cinque blocchi logici.

### 10.1 Interception layer
Punto in cui passano le azioni dell'agente.

Responsabilità:
- ricevere richiesta di esecuzione
- normalizzare il comando o l'operazione
- raccogliere contesto minimo

Esempi:
- comando shell
- file write
- git push
- process spawn

### 10.2 Policy engine
Motore che applica regole deterministiche.

Output possibili:
- allow
- block
- ask
- sandbox

### 10.3 Execution layer
Esegue l'azione approvata:
- local execution
- sandboxed execution
- denied execution

### 10.4 Audit layer
Salva:
- timestamp
- agente/sessione
- tool
- payload normalizzato
- decisione
- motivo
- eventuale override utente

### 10.5 Config and UX layer
Permette all'utente di:
- inizializzare il progetto
- vedere log
- cambiare policy
- approvare una singola azione
- definire path consentiti e vietati

---

## 11. Architettura tecnica MVP

### 11.1 Forma del prodotto iniziale
CLI locale installabile su Mac.

Componente principale:
- `guardian` CLI

Comandi indicativi:
- `guardian init`
- `guardian run`
- `guardian logs`
- `guardian policy edit`
- `guardian doctor`
- `guardian sandbox run`

### 11.2 Modalità di integrazione iniziali
Per MVP, prevedere due strade.

#### Modalità A: wrapper esplicito
L'agente o l'utente invoca i tool passando dal Guardian.

Esempio:
- invece di `bash`, usa `guardian-shell`
- invece di `git push`, passa da wrapper Guardian

Vantaggi:
- semplice da costruire
- esplicito
- controllabile

Svantaggi:
- richiede configurazione
- meno invisibile

#### Modalità B: profilo di progetto protetto
Il Guardian genera script/alias per far passare alcuni comandi dal layer di controllo.

Vantaggi:
- esperienza più fluida
- meno sforzo manuale

Svantaggi:
- più complessità
- rischio edge case

Per il primo ciclo si parte da modalità A.

---

## 12. Oggetti da controllare nel MVP

### 12.1 Shell commands
Controllo su:
- comando raw
- argomenti
- working directory
- utente
- path target individuati

### 12.2 File operations
Minimo:
- scritture su file
- cancellazioni
- rename/move
- accessi a path sensibili

### 12.3 Git operations
Minimo:
- add
- commit
- checkout
- push
- force push

### 12.4 Process / install
Minimo:
- esecuzione di installer
- script remoti
- comandi con pipe pericolose
- sudo

---

## 13. Policy model iniziale

### 13.1 Decisioni supportate
- allow
- block
- ask
- sandbox

### 13.2 Regole per tipo di azione
Esempi:

- `rm -rf /` -> block
- `rm -rf` su path fuori workspace -> block
- `git push origin main` -> ask
- `sudo *` -> block
- `curl * | bash` -> block
- `npm install` -> allow
- scrittura su `.env` -> ask o block
- accesso a `~/.ssh` -> block

### 13.3 Scope delle policy
- globali
- per progetto
- per tool
- per path

### 13.4 Override utente
L'utente può:
- approvare una volta
- approvare sempre in quel progetto
- lasciare bloccato

---

## 14. Perché all'inizio niente ML forte

Nel primo ciclo le decisioni vanno fatte con regole deterministiche.

Motivi:
- prevedibilità
- facilità di debug
- bassa ambiguità
- velocità di sviluppo
- più fiducia da parte del team e degli utenti
- migliore auditabilità

L'uso di ML o LLM lightweight entra dopo come supporto, non come fondazione del controllo.

Ruoli possibili del ML nei cicli successivi:
- classificazione del rischio più contestuale
- detection di pattern anomali
- spiegazioni più umane
- suggerimento di policy

Mai demandare a un modello non deterministico l'ultima decisione critica nell'MVP.

---

## 15. Perché la sandbox è prioritaria subito dopo

La policy riduce il rischio.
La sandbox riduce l'impatto anche quando la policy sbaglia.

È il secondo pilastro del prodotto.

### 15.1 Definizione operativa
Sandbox = esecuzione in ambiente isolato rispetto al Mac host.

Per l'MVP:
- Docker locale
- workspace montato in modo controllato
- rete limitabile
- privilegi minimi
- filesystem confinato

### 15.2 Beneficio
Se un agente lancia un comando distruttivo o errato:
- il danno resta confinato
- l'ambiente si può ricreare
- il Mac resta intatto o molto più protetto

### 15.3 Trade-off
- maggiore complessità tecnica
- possibile rallentamento
- compatibilità da gestire
- UX più articolata

Per questo entra nel ciclo 2, non nel ciclo 1.

---

## 16. Roadmap

L'approccio è:
build -> measure -> learn

Ogni ciclo deve produrre:
- incremento usabile
- ipotesi validata o invalidata
- decisione chiara sul ciclo successivo

---

# CICLO 1 ✅ COMPLETATO
Guardian locale rule-based per shell e git

## 16.1 Obiettivo
Validare che gli utenti percepiscano valore immediato nel controllo delle azioni più rischiose, prima ancora della sandbox.

## 16.2 Ipotesi da validare
1. Gli utenti vogliono un layer di controllo locale per agenti
2. Il caso d'uso shell + git è sufficiente a mostrare valore
3. Una CLI con policy deterministiche è usabile
4. L'utente accetta una frizione moderata se il valore è evidente

## 16.3 Output del ciclo
Prodotto installabile in locale che:
- intercetta comandi shell passati tramite wrapper
- classifica il rischio con regole
- blocca o chiede conferma
- logga tutto
- controlla almeno alcune operazioni git sensibili

## 16.4 Funzioni incluse
### Core
- CLI `guardian`
- wrapper `guardian-shell`
- file config YAML o JSON
- risk engine rule-based
- audit log locale

### Controlli iniziali
- rm pericolosi
- sudo
- curl | bash
- chmod molto permissivi
- accesso a path sensibili
- git push
- git push --force
- modifica di file protetti

### UX minima
- output chiaro a terminale
- messaggio con decisione e motivo
- prompt di conferma quando serve
- comando per vedere ultimi eventi

## 16.5 Esclusioni
- niente UI grafica
- niente Docker
- niente detection comportamentale
- niente supporto enterprise/team
- niente rete policy avanzata

## 16.6 Architettura tecnica del ciclo 1
### Componenti
- CLI app
- parser comando
- policy evaluator
- executor locale
- event logger

### Tecnologie possibili
- linguaggio: Go o Rust o Node.js
- config: YAML
- log: SQLite locale o JSONL
- packaging Mac: Homebrew tap o installer semplice

Scelta consigliata:
Go per rapidità, binario singolo, buona portabilità, ottima gestione processi.

## 16.7 Data model minimo
Evento:
- id
- timestamp
- session_id
- agent_name opzionale
- project_path
- action_type
- raw_payload
- normalized_payload
- risk_level
- decision
- reason
- user_override
- exit_code

## 16.8 Metriche del ciclo 1
Prodotto:
- numero installazioni attive
- numero sessioni
- numero eventi intercettati
- percentuale allow / ask / block

Valore:
- quanti blocchi evitano operazioni realmente dannose
- quanti prompt di conferma vengono considerati utili
- tasso di disinstallazione o bypass

UX:
- tempo medio introdotto per azione
- numero di false positive
- numero di override permanenti

## 16.9 Criterio di successo
Il ciclo 1 è riuscito se:
- almeno il 60% dei tester continua a usarlo dopo 1 settimana
- il feedback qualitativo dice che "fa sentire più sicuri"
- i false positive restano tollerabili
- la frizione è accettata in cambio del controllo

## 16.10 Criterio di kill o pivot
Se gli utenti lo percepiscono come troppo invasivo e poco utile, bisogna:
- restringere il perimetro a pochi comandi ad alto rischio
oppure
- cambiare integrazione e renderlo più invisibile

---

# CICLO 2 ✅ COMPLETATO
Sandbox Docker + isolamento operativo

## 17.1 Obiettivo
Validare che l'isolamento sia il vero moltiplicatore di fiducia e che l'utente accetti l'idea di far eseguire certe azioni in ambiente protetto.

## 17.2 Ipotesi da validare
1. Gli utenti capiscono e apprezzano la differenza tra block e sandbox
2. Per molte azioni rischiose l'esecuzione in sandbox è preferibile al blocco puro
3. Docker locale è un compromesso accettabile per l'MVP
4. È possibile mantenere una UX semplice

## 17.3 Output del ciclo
Estensione del prodotto che:
- può eseguire alcune operazioni dentro container
- separa workspace host e workspace sandbox
- permette reset rapido dell'ambiente
- mantiene log delle esecuzioni isolate

## 17.4 Funzioni incluse
- `guardian sandbox run`
- profili sandbox per progetto
- mount controllati
- allowlist di path
- variabile per abilitare/disabilitare rete
- opzione read-only per mount sensibili
- reset o rebuild sandbox

## 17.5 Decision logic aggiornata
Una regola può produrre:
- allow on host
- block
- ask
- force sandbox

Esempi:
- installazioni pacchetti -> sandbox
- script sconosciuti -> sandbox
- test batch su repo -> sandbox
- comandi distruttivi su host -> block

## 17.6 Architettura tecnica del ciclo 2
Nuovi componenti:
- sandbox manager
- image builder
- mount policy resolver
- network policy adapter

### Esempio flusso
1. l'agente chiede di eseguire comando
2. il policy engine decide sandbox
3. il sandbox manager prepara container
4. monta workspace consentito
5. esegue comando
6. raccoglie stdout/stderr/exit code
7. salva tutto nei log

## 17.7 Considerazioni tecniche
- Docker Desktop su Mac come prerequisito iniziale
- immagini base curate dal prodotto
- attenzione a performance e compatibilità dei path
- evitare privilegi inutili
- distinguere chiaramente output host vs output sandbox

## 17.8 Metriche del ciclo 2
- quante azioni vengono deviate in sandbox
- tempo medio di startup sandbox
- tasso di successo esecuzioni isolate
- riduzione percepita del rischio
- numero di incidenti sul host evitati

## 17.9 Criterio di successo
Il ciclo 2 è riuscito se:
- gli utenti dichiarano maggiore fiducia
- il costo in latenza resta accettabile
- la sandbox non rompe i casi principali
- si osserva preferenza per "sandbox" rispetto a "block" in diversi casi

## 17.10 Rischi
- complessità alta
- UX meno trasparente
- edge case su file e toolchain
- differenze tra ambiente host e container

---

# CICLO 3 ✅ COMPLETATO
Minimo layer intelligente + policy suggestions

## 18.1 Obiettivo
Aggiungere un primo livello di intelligenza utile senza compromettere controllo e spiegabilità.

## 18.2 Ipotesi da validare
1. Un supporto intelligente riduce false positive
2. Le policy possono diventare più contestuali
3. L'utente accetta suggerimenti automatici se la decisione finale resta governata
4. La qualità del prodotto migliora senza perdere trasparenza

## 18.3 Definizione corretta di minimo ML
Non un sistema autonomo che decide da solo.
Piuttosto uno o più moduli che aiutano il motore deterministico.

### Possibili usi
- risk scoring contestuale
- classificazione delle sequenze di azioni
- suggerimento di policy
- spiegazione comprensibile del rischio
- anomaly hinting

## 18.4 Funzioni incluse
- score aggiuntivo oltre alle regole
- suggerimento: questa azione somiglia a pattern ad alto rischio
- suggerimento: vuoi rendere questa approvazione permanente nel progetto X?
- raggruppamento di eventi per pattern
- primi alert su chain sospette

## 18.5 Esempi
- `rm -rf ./dist` può diventare low risk in progetto front-end noto
- 20 comandi shell in rapida sequenza con modifiche a credenziali può alzare il rischio
- script offuscato o pipe complesse può generare warning forte

## 18.6 Architettura tecnica del ciclo 3
Nuovi componenti:
- feature extractor leggero
- scorer locale o servizio opzionale
- policy suggestion engine
- anomaly heuristic engine

Importante:
la decisione finale resta deterministica e auditabile

Formula consigliata:
decisione finale = max(policy hard rules, contextual score thresholds)

Dove:
- le regole hard prevalgono sempre
- il score può alzare o abbassare livello entro limiti definiti

## 18.7 Metriche del ciclo 3
- riduzione false positive
- qualità percepita delle spiegazioni
- tasso di accettazione dei suggerimenti
- incremento nell'uso continuativo
- numero di casi in cui il layer intelligente ha evitato errore o frizione

## 18.8 Criterio di successo
Il ciclo 3 è riuscito se:
- il sistema è percepito come più utile e meno rumoroso
- i suggerimenti non sembrano arbitrari
- il team tecnico riesce a mantenere osservabilità completa della logica

---

## 19. Pila tecnologica consigliata

### Core agent runtime
- Go per CLI e orchestration
- YAML per policy
- SQLite o JSONL per log locali
- gRPC/Unix socket opzionale per processi futuri

### Sandbox
- Docker locale
- immagini base minimali
- mount policy configurabile
- possibilità futura di Podman / Lima

### Eventing / logs
- SQLite locale per query veloci
- export JSON per debug
- firma/hash eventi in roadmap futura

### ML minimo ciclo 3
Opzioni:
- heuristics engine classico
- modello lightweight locale
- supporto LLM opzionale solo per labeling/suggerimenti, mai per enforcement hard

---

## 20. Esempio di policy file MVP

```yaml
version: 1

global:
  default_decision: allow

paths:
  protected:
    - "~/.ssh"
    - "~/.aws"
    - ".env"
    - ".env.local"
  sandbox_preferred:
    - "./scripts"
    - "./tmp"

rules:
  - id: block_sudo
    when:
      action_type: shell
      command_matches: ["sudo *"]
    decision: block
    reason: "sudo disabilitato nel profilo MVP"

  - id: block_dangerous_rm
    when:
      action_type: shell
      command_matches:
        - "rm -rf /"
        - "rm -rf ~"
        - "rm -rf ../*"
    decision: block
    reason: "comando distruttivo"

  - id: ask_git_push_main
    when:
      action_type: git
      command_matches: ["git push origin main", "git push --force*"]
    decision: ask
    reason: "push sensibile"

  - id: protect_env_files
    when:
      action_type: file_write
      path_matches: [".env", ".env.local"]
    decision: ask
    reason: "file sensibile"

  - id: sandbox_unknown_script
    when:
      action_type: shell
      command_matches: ["bash *.sh", "sh *.sh", "python *.py"]
    decision: sandbox
    reason: "script da eseguire in ambiente isolato"
```

---

## 21. Esempio di flusso utente nel ciclo 1

### Installazione
```bash
brew install ai-guardian
guardian init
```

### Uso
```bash
guardian-shell "git push origin main"
```

Output:
```text
Decisione: ASK
Motivo: push verso branch sensibile
Vuoi procedere?
[y] una volta
[a] sempre per questo progetto
[n] blocca
```

### Logs
```bash
guardian logs --last 20
```

---

## 22. Esempio di flusso utente nel ciclo 2

```bash
guardian sandbox run "python migration_script.py"
```

Output:
```text
Decisione: SANDBOX
Container: guardian-python-base
Workspace montato: ./project
Rete: disabilitata
Esecuzione completata con exit code 0
```

---

## 23. Esempio di flusso utente nel ciclo 3

Output:
```text
Decisione: ASK
Motivo regola: modifica file sensibile
Segnale contestuale: sequenza anomala di 12 operazioni in 45 secondi
Suggerimento: vuoi rendere questo path read-only di default?
```

---

## 24. Backlog tecnico prioritizzato

### Must have ciclo 1
- CLI
- config loader
- command normalization
- rules engine
- prompt approval
- executor
- event logger
- git command matcher
- protected path matcher

### Must have ciclo 2
- Docker detection e doctor
- image management
- sandbox launcher
- mount policy
- stdout/stderr capture
- reset sandbox

### Must have ciclo 3
- feature extraction
- risk scoring lightweight
- suggestion engine
- grouped event analysis

### Nice to have post ciclo 3
- UI desktop
- dashboard team
- policy packs per linguaggio o stack
- secret scanner
- network egress control serio
- signed audit trail
- agent profile by vendor
- remote management

---

## 25. Rischi principali

### 25.1 Rischi prodotto
- troppo attrito
- valore poco percepito se i blocchi non sono chiari
- scope troppo ampio
- difficile convincere utenti a cambiare workflow

### 25.2 Rischi tecnici
- impossibilità di intercettare tutto in modo pulito
- bypass facili se integrazione debole
- parsing comandi ambiguo
- differenze host vs Docker
- compatibilità Mac

### 25.3 Rischi strategici
- vendor incorporano feature simili
- il mercato si frammenta per framework
- la categoria tarda a maturare

---

## 26. Come ridurre il rischio

1. partire con shell + git
2. fare pilot con utenti avanzati
3. misurare false positive in modo brutale
4. investire presto nella sandbox
5. tenere il positioning framework-agnostic
6. evitare UI pesante troppo presto
7. comunicare il valore con demo concrete

---

## 27. Test plan iniziale

### Test funzionali
- blocco comandi vietati
- conferma comandi sensibili
- logging corretto
- override persistente
- fallback sicuro su errori

### Test di sicurezza
- tentativi di bypass via quoting
- subshell
- script wrapper
- comandi concatenati con `&&`
- pipe
- env vars
- path traversal

### Test UX
- tempo di risposta
- chiarezza dei messaggi
- numero di prompt accettabili
- comprensione delle decisioni

### Test sandbox
- mount read-only
- reset container
- rete off
- comportamento pacchetti/installazioni

---

## 28. Domande aperte per il team

### Prodotto
- il primo wedge è solo developer local o anche team engineering?
- il nome Guardian comunica sicurezza o frizione?
- serve da subito differenziare modalità strict e relaxed?

### Tecnico
- Go o Rust per il core?
- YAML o JSON per policy?
- SQLite o JSONL per i log?
- Docker Desktop è prerequisito accettabile?
- wrapper esplicito o integrazione più trasparente già dal primo ciclo?

### Go-to-market
- open source core + premium?
- freeware locale + team version?
- plugin per community agent tool oppure prodotto standalone?

---

## 29. Decisioni consigliate adesso

1. fissare il wedge su agenti che eseguono comandi e modificano codice in locale
2. costruire ciclo 1 solo con CLI + regole + log
3. progettare già da subito il punto di estensione sandbox
4. tenere il sistema framework-agnostic
5. rimandare ML vero e UI desktop
6. validare con 5-10 utenti reali ad alta intensità d'uso

---

## 30. Executive summary finale

AI Agent Guardian nasce come un runtime control layer per agenti AI operativi.

Il primo prodotto non deve cercare di proteggere qualsiasi agente in qualsiasi contesto.
Deve risolvere molto bene un problema iniziale chiaro:

rendere sicuro e osservabile l'uso di agenti locali che possono lanciare comandi, toccare file e usare git su Mac.

Roadmap completata:

- ciclo 1 ✅ — controllo deterministico locale su shell e git
- ciclo 2 ✅ — sandbox Docker per confinare il rischio
- ciclo 3 ✅ — layer intelligente: risk scoring, anomaly detection, policy suggestions
- ciclo 4 ✅ (parziale) — signed audit trail + MCP hook Claude Code
- ciclo 5 → cloud dashboard, sync agent, AI analysis layer

Il principio fondante resta uno:

non fidarsi delle intenzioni dell'agente
controllare le sue azioni
e, quando serve, isolarne l'esecuzione

---

# CICLO 4 ✅ COMPLETATO
Trust layer + protezione MCP

## Obiettivo
Rendere il log probatorio e estendere l'interception alle MCP tool calls di Claude Code.

## Funzioni implementate

### Signed audit trail
- Ogni evento firmato con HMAC-SHA256 + catena hash (prev_hash)
- Struttura blockchain-like: cancellare o modificare qualsiasi evento rompe la catena
- `nightagent verify` controlla l'integrità retroattiva dell'intero log
- Chiave locale 32 byte in `~/.night-agent/signing.key` (generata durante init)
- Prerequisito tecnico per la cloud dashboard: eventi verificabili anche server-side

### MCP hook (Claude Code)
- `nightagent mcp-hook` intercetta le tool call MCP via PreToolUse hook
- Tool intercettati: Bash, Edit, Write, Read, Glob, Grep, WebFetch, WebSearch
- Stessa policy YAML dei comandi shell — nessuna configurazione doppia
- Fail-open se daemon non disponibile
- Configurazione in `~/.claude/settings.json`

## Ipotesi validate
- Il log firmato aumenta la fiducia nel prodotto come strumento di audit serio
- Le MCP tool call sono intercettabili senza modificare Claude Code
- Un hook leggero è sufficiente per coprire i casi d'uso principali

---

# CICLO 5 — Cloud dashboard + sync agent

## Obiettivo
Portare Night Agent da strumento locale a piattaforma osservabile via web. L'agente locale resta OSS. Il cloud è opt-in, premium.

## Ipotesi da validare
1. Gli utenti vogliono vedere i dati degli agenti da browser, non solo da terminale
2. Il modello OSS core + cloud premium è accettato dagli utenti developer
3. La connessione macchina → workspace tramite token è sufficientemente semplice
4. Un AI layer sopra i dati di audit aggiunge valore percepito concreto

## Output del ciclo
Un sistema connesso composto da tre parti:

### Parte 1 — Sync agent locale
Nuovo processo leggero che legge `audit.jsonl` e invia gli eventi firmati alla cloud API.

- `nightagent cloud connect <TOKEN>` — attiva la sincronizzazione
- Batchizza eventi ogni N secondi (configurabile)
- Non tocca il daemon esistente — zero impatto sull'uso offline
- Verifica le firme prima dell'invio: non invia eventi corrotti
- Riprende dall'ultimo evento sincronizzato in caso di interruzione

### Parte 2 — Cloud API
Backend multi-tenant che riceve, archivia e serve i dati delle macchine connesse.

- Autenticazione per workspace token
- Verifica server-side della catena hash: rileva se il client ha omesso eventi
- API REST per la dashboard
- Webhook configurabili per alert su eventi ad alto rischio

### Parte 3 — Dashboard web
Interfaccia web per osservare e governare gli agenti in real-time.

Funzioni prioritarie:
1. Feed eventi real-time con filtri (decisione, rischio, agente, macchina)
2. Heatmap rischio per ora/giorno
3. Top comandi bloccati e pattern anomali ricorrenti
4. Policy editor web — modifica YAML e sincronizza sul client
5. Alert email/Slack su eventi ad alto rischio
6. Gestione multi-macchina — connessione tramite token alfanumerico

## Architettura

```
~/.night-agent/audit.jsonl
        ↓
sync agent (Go, nuovo processo)
        ↓  HTTPS + token
Cloud API (Go)  →  Postgres
        ↓
Dashboard (Next.js)
```

## Connessione macchina

```bash
# sul web: "Aggiungi macchina" → genera TOKEN
nightagent cloud connect ABC123XYZ
# sync agent si avvia in background e inizia a inviare eventi
```

## Componenti nuovi da costruire

```
cmd/guardian/cloud.go         — comandi: cloud connect, cloud status, cloud disconnect
internal/sync/agent.go        — sync agent: legge JSONL, batchizza, invia
cloud/api/                    — backend Go: auth, ingest, query API
cloud/dashboard/              — Next.js frontend
```

## Stack tecnologico
- Sync agent: Go (stdlib `net/http`, nessuna dipendenza esterna)
- Backend API: Go o Node.js
- Database: Supabase (Postgres + realtime) o Postgres standalone
- Frontend: Next.js + Tailwind
- Auth: token workspace (alpha), OAuth per team (beta)

## Esclusioni di questo ciclo
- nessun AI layer (ciclo 6)
- nessun policy editor completo (solo YAML upload/download)
- nessun multi-utente per team (un account = una o più macchine personali)

## Metriche del ciclo
- numero di macchine connesse
- eventi sincronizzati per sessione
- tempo medio di latenza sync
- tasso di adozione cloud vs solo locale
- retention a 2 settimane

## Criterio di successo
Il ciclo 5 è riuscito se:
- gli utenti connettono almeno una macchina e tornano sulla dashboard
- la latenza di sync è percepita come accettabile (< 5 secondi)
- almeno un utente usa la dashboard come strumento primario di osservazione

---

# CICLO 6 — AI analysis layer

## Obiettivo
Aggiungere un layer di intelligenza sopra i dati di audit cloud. Claude API come motore di analisi, non come decisore.

## Ipotesi da validare
1. Una AI che legge i log e risponde in linguaggio naturale è utile in pratica
2. I pattern identificati dalla AI sono diversi e complementari rispetto all'heuristic scorer locale
3. Gli utenti accettano di pagare per questo layer

## Funzioni incluse

### AI chat contestuale
- "cosa ha fatto il mio agente nelle ultime 2 ore?"
- "ci sono pattern anomali questa settimana?"
- "quali comandi sono stati bloccati più spesso e perché?"
- Risponde con citazioni dirette agli eventi nel log

### Report automatici
- Riepilogo giornaliero via email: eventi ad alto rischio, anomalie, suggerimenti policy
- Report settimanale: trend, comandi nuovi non coperti da policy, pattern ricorrenti

### Policy suggestions avanzate
- Identifica comandi ripetuti senza regola esplicita e suggerisce di aggiungerla
- Confronta la policy corrente con i pattern reali d'uso e segnala regole inutili o mancanti

## Architettura
```
Cloud API → Claude API (analisi batch + query real-time)
                ↓
         risposta in linguaggio naturale
                ↓
         Dashboard (chat UI + report)
```

La decisione finale resta sempre nel policy engine locale deterministico.
Claude API è advisory — non ha accesso al daemon e non può modificare policy senza conferma esplicita dell'utente.

## Esclusioni
- Claude API non ha mai accesso diretto al daemon o alla policy locale
- Nessuna decisione di enforcement delegata alla AI
- I dati inviati a Claude API sono gli stessi già sincronizzati in cloud (nessun dato aggiuntivo)
