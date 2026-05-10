package configdir

import (
	"os"
	"path/filepath"
)

const (
	LocalDirName  = ".nightagent"  // config locale nel progetto
	GlobalDirName = ".night-agent" // config globale utente
)

// Resolve restituisce la config dir da usare.
// Se .nightagent/ esiste nella cwd E contiene policy.yaml → usa quella (config locale).
// Controlla policy.yaml per non scambiare la dir del socket daemon con una config locale.
// Altrimenti → fallback su ~/.night-agent/ (config globale).
func Resolve(cwd string) (string, error) {
	local := filepath.Join(cwd, LocalDirName)
	if info, err := os.Stat(local); err == nil && info.IsDir() {
		if _, err := os.Stat(filepath.Join(local, "policy.yaml")); err == nil {
			return local, nil
		}
	}
	return Global()
}

// Global restituisce sempre ~/.night-agent/ indipendentemente dalla cwd.
func Global() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, GlobalDirName), nil
}

// CreateLocal crea .nightagent/ nella cwd e restituisce il path.
// Idempotente: se già esiste non fa nulla.
func CreateLocal(cwd string) (string, error) {
	local := filepath.Join(cwd, LocalDirName)
	if err := os.MkdirAll(local, 0700); err != nil {
		return "", err
	}
	return local, nil
}

// IsLocal controlla se il path è una config dir locale (.nightagent/).
func IsLocal(dir string) bool {
	return filepath.Base(dir) == LocalDirName
}
