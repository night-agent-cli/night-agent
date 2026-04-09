package shim

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ShimmedCommands è la lista dei comandi intercettati via PATH shim.
// guardian-shim viene installato come symlink con ognuno di questi nomi.
var ShimmedCommands = []string{
	"sudo", "rm", "git", "curl", "wget",
	"chmod", "chown", "mv", "cp",
	"bash", "sh", "pip", "pip3",
	"npm", "brew",
}

const ShimBinaryName = "guardian-shim"

// ShimDir restituisce il path della directory degli shim dato guardianDir.
func ShimDir(guardianDir string) string {
	return filepath.Join(guardianDir, "shims")
}

// PrependPath prepende shimDir alla variabile PATH nell'env dato.
// Se shimDir è già il primo elemento, non duplica.
// Se PATH non esiste, la crea con solo shimDir.
func PrependPath(env []string, shimDir string) []string {
	result := make([]string, 0, len(env)+1)
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			existing := e[5:]
			// evita duplicazione se shimDir è già in prima posizione
			if strings.HasPrefix(existing, shimDir+":") || existing == shimDir {
				result = append(result, e)
			} else {
				result = append(result, "PATH="+shimDir+":"+existing)
			}
			found = true
		} else {
			result = append(result, e)
		}
	}
	if !found {
		result = append(result, "PATH="+shimDir)
	}
	return result
}

// CreateSymlinks crea symlink per ogni ShimmedCommand che punta a shimBinaryPath.
// Sovrascrive symlink esistenti. Non tocca file regolari (non-symlink).
func CreateSymlinks(shimDir, shimBinaryPath string) error {
	for _, cmd := range ShimmedCommands {
		linkPath := filepath.Join(shimDir, cmd)
		if info, err := os.Lstat(linkPath); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				if err := os.Remove(linkPath); err != nil {
					return fmt.Errorf("impossibile rimuovere symlink esistente %s: %w", linkPath, err)
				}
			}
			// se è un file regolare, lo lasciamo stare (non sovrascriviamo binari reali)
		}
		if err := os.Symlink(shimBinaryPath, linkPath); err != nil {
			return fmt.Errorf("impossibile creare symlink per %s: %w", cmd, err)
		}
	}
	return nil
}

// Install crea la directory degli shim e installa le symlink.
// Il binario guardian-shim deve essere già compilato e disponibile a shimBinaryPath.
func Install(guardianDir, shimBinaryPath string) error {
	shimDirPath := ShimDir(guardianDir)
	if err := os.MkdirAll(shimDirPath, 0755); err != nil {
		return fmt.Errorf("impossibile creare %s: %w", shimDirPath, err)
	}
	return CreateSymlinks(shimDirPath, shimBinaryPath)
}
