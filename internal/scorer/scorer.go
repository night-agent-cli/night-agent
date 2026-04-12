// Package scorer implementa il risk scoring contestuale (Cycle 3).
// Le decisioni deterministiche del policy engine hanno sempre priorità;
// il score è un segnale aggiuntivo per alzare o abbassare il livello di
// attenzione su azioni che le regole hard non coprono esattamente.
package scorer

import (
	"strings"
	"time"

	"github.com/pietroperona/night-agent/internal/audit"
)

// RiskLevel rappresenta il livello di rischio calcolato dallo scorer.
type RiskLevel string

const (
	LevelLow    RiskLevel = "low"
	LevelMedium RiskLevel = "medium"
	LevelHigh   RiskLevel = "high"
)

// Action descrive l'azione da valutare.
type Action struct {
	Type    string
	Command string
	Path    string
	WorkDir string
}

// Result è l'output dello scorer per una singola azione.
type Result struct {
	Score           float64   // [0.0 – 1.0]
	Level           RiskLevel
	Signals         []string // segnali che hanno contribuito al punteggio
	AnomalyDetected bool     // true se rilevato burst anomalo di azioni
}

// Scorer valuta il rischio contestuale di un'azione.
type Scorer struct{}

// New crea uno Scorer.
func New() *Scorer { return &Scorer{} }

// Score calcola il rischio di action considerando la storia recente events.
// Formula: score = sum(pesi segnali) clamped a [0.0, 1.0]
// La decisione finale del daemon resta deterministica (policy hard rules);
// il score alimenta segnali aggiuntivi nel log e le suggestions.
func (s *Scorer) Score(action Action, events []audit.Event) Result {
	var total float64
	var signals []string

	// --- Segnali sul comando ---

	cmd := strings.ToLower(action.Command)

	// sudo: alta pericolosità
	if strings.Contains(cmd, "sudo ") {
		total += 0.5
		signals = append(signals, "comando con sudo")
	}

	// curl/wget piped a shell: molto pericoloso
	if (strings.Contains(cmd, "curl ") || strings.Contains(cmd, "wget ")) &&
		(strings.Contains(cmd, "| bash") || strings.Contains(cmd, "| sh") || strings.Contains(cmd, "|bash") || strings.Contains(cmd, "|sh")) {
		total += 0.7
		signals = append(signals, "script remoto eseguito via pipe")
	}

	// rm con flag ricorsivo
	if strings.Contains(cmd, "rm ") && strings.Contains(cmd, "-r") {
		total += 0.3
		signals = append(signals, "rm ricorsivo")
	}

	// chmod molto permissivo (777, 755 su path non di progetto)
	if strings.Contains(cmd, "chmod ") && strings.Contains(cmd, "777") {
		total += 0.3
		signals = append(signals, "chmod 777")
	}

	// git force push
	if action.Type == "git" &&
		(strings.Contains(cmd, "--force") || strings.Contains(cmd, " -f ")) {
		total += 0.5
		signals = append(signals, "git force push")
	}

	// git push su branch protetto
	if action.Type == "git" &&
		(strings.Contains(cmd, "push origin main") || strings.Contains(cmd, "push origin master")) {
		total += 0.2
		signals = append(signals, "push su branch principale")
	}

	// accesso a path sensibili
	sensitivePaths := []string{".env", ".aws", ".ssh", "id_rsa", "credentials", "secrets", "token"}
	targetStr := strings.ToLower(action.Command + " " + action.Path)
	for _, sp := range sensitivePaths {
		if strings.Contains(targetStr, sp) {
			total += 0.3
			signals = append(signals, "accesso path sensibile: "+sp)
			break
		}
	}

	// install di pacchetti: medio rischio
	pkgManagers := []string{"pip install", "pip3 install", "npm install", "yarn add", "brew install", "apt install", "apt-get install"}
	for _, pm := range pkgManagers {
		if strings.Contains(cmd, pm) {
			total += 0.15
			signals = append(signals, "installazione pacchetto: "+pm)
			break
		}
	}

	// script shell generici
	if (strings.HasPrefix(cmd, "bash ") || strings.HasPrefix(cmd, "sh ")) && strings.HasSuffix(strings.Fields(cmd)[len(strings.Fields(cmd))-1], ".sh") {
		total += 0.2
		signals = append(signals, "esecuzione script shell")
	}

	// --- Segnali contestuali (storia eventi) ---

	anomaly, anomalySignals := detectAnomalies(events)
	if anomaly {
		total += 0.25
		signals = append(signals, anomalySignals...)
	}

	// --- Clamp e livello ---

	if total > 1.0 {
		total = 1.0
	}

	return Result{
		Score:           total,
		Level:           LevelFromScore(total),
		Signals:         signals,
		AnomalyDetected: anomaly,
	}
}

// LevelFromScore converte un punteggio numerico in RiskLevel.
// < 0.3 → low, 0.3–0.7 → medium, ≥ 0.7 → high
func LevelFromScore(score float64) RiskLevel {
	switch {
	case score >= 0.7:
		return LevelHigh
	case score >= 0.3:
		return LevelMedium
	default:
		return LevelLow
	}
}

// detectAnomalies analizza la storia recente degli eventi per rilevare
// pattern anomali: burst di azioni, sequenze di blocchi ravvicinati, ecc.
func detectAnomalies(events []audit.Event) (bool, []string) {
	if len(events) == 0 {
		return false, nil
	}

	var signals []string
	anomaly := false

	now := time.Now()
	window := 30 * time.Second

	// Burst: >10 azioni nei 30 secondi precedenti
	var recentCount int
	var recentBlocks int
	for _, e := range events {
		if now.Sub(e.Timestamp) <= window {
			recentCount++
			if e.Decision == "block" {
				recentBlocks++
			}
		}
	}

	if recentCount > 10 {
		anomaly = true
		signals = append(signals, "burst anomalo: "+itoa(recentCount)+" azioni in 30s")
	}

	// Sequenza di blocchi: ≥3 blocchi recenti
	if recentBlocks >= 3 {
		anomaly = true
		signals = append(signals, itoa(recentBlocks)+" blocchi nelle ultime azioni")
	}

	return anomaly, signals
}

// itoa converte int in stringa senza import "strconv" per non appesantire le dipendenze.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
