package launchagent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const label = "com.guardian.daemon"

// PlistPath restituisce il path del file plist dato la home directory.
func PlistPath(homeDir string) string {
	return filepath.Join(homeDir, "Library", "LaunchAgents", label+".plist")
}

// IsInstalled verifica se il LaunchAgent è già installato.
func IsInstalled(homeDir string) bool {
	_, err := os.Stat(PlistPath(homeDir))
	return err == nil
}

// GeneratePlist genera il contenuto XML del plist per il LaunchAgent.
// binaryPath è il path assoluto del binario guardian.
// guardianDir è ~/.guardian (usato per stdout/stderr log).
func GeneratePlist(binaryPath, guardianDir string) string {
	stdoutLog := filepath.Join(guardianDir, "daemon.log")
	stderrLog := filepath.Join(guardianDir, "daemon-error.log")

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>

    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>start</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>StandardOutPath</key>
    <string>%s</string>

    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>
`, label, binaryPath, stdoutLog, stderrLog)
}

// Install scrive il plist e carica il LaunchAgent con launchctl.
// Se già installato, aggiorna il file e ricarica.
func Install(homeDir, binaryPath, guardianDir string) error {
	plistPath := PlistPath(homeDir)
	launchAgentsDir := filepath.Dir(plistPath)

	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("impossibile creare LaunchAgents dir: %w", err)
	}

	plist := GeneratePlist(binaryPath, guardianDir)
	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("impossibile scrivere plist: %w", err)
	}

	// se già caricato, fai unload prima
	_ = exec.Command("launchctl", "unload", plistPath).Run()

	if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
		return fmt.Errorf("launchctl load fallito: %w", err)
	}
	return nil
}

// Uninstall ferma il LaunchAgent e rimuove il plist.
func Uninstall(homeDir string) error {
	plistPath := PlistPath(homeDir)

	_ = exec.Command("launchctl", "unload", plistPath).Run()

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("impossibile rimuovere plist: %w", err)
	}
	return nil
}
