#!/usr/bin/env bash
set -euo pipefail

REPO="pietroperona/night-agent"
INSTALL_BIN="/usr/local/bin"
INSTALL_LIB="/usr/local/lib/night-agent"
CONFIG_DIR="$HOME/.night-agent"

# Colori
RED='\033[0;31m'
GREEN='\033[0;32m'
BOLD='\033[1m'
RESET='\033[0m'

die() { echo -e "${RED}Errore: $1${RESET}" >&2; exit 1; }
ok()  { echo -e "${GREEN}✓${RESET} $1"; }

# Verifica prerequisiti
command -v curl >/dev/null 2>&1 || die "curl non trovato"
command -v tar  >/dev/null 2>&1 || die "tar non trovato"
command -v shasum >/dev/null 2>&1 || die "shasum non trovato"

# Verifica architettura
ARCH="$(uname -m)"
[ "$ARCH" = "arm64" ] || die "Night Agent supporta solo Apple Silicon (arm64). Rilevato: $ARCH"

# Trova ultima versione
echo "Ricerca ultima versione..."
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
[ -n "$VERSION" ] || die "impossibile rilevare l'ultima versione"

ARCHIVE="night-agent-${VERSION}-darwin-arm64.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

echo "Installo Night Agent ${VERSION}..."

# Download in directory temporanea
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "${BASE_URL}/${ARCHIVE}"        -o "${TMP}/${ARCHIVE}"
curl -fsSL "${BASE_URL}/${ARCHIVE}.sha256" -o "${TMP}/${ARCHIVE}.sha256"

# Verifica checksum
EXPECTED=$(cat "${TMP}/${ARCHIVE}.sha256")
ACTUAL=$(shasum -a 256 "${TMP}/${ARCHIVE}" | awk '{print $1}')
[ "$EXPECTED" = "$ACTUAL" ] || die "checksum non valido — download corrotto"
ok "Checksum verificato"

# Estrai
tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP"

# Installa binario principale
sudo install -m 755 "${TMP}/nightagent" "${INSTALL_BIN}/nightagent"
ok "nightagent → ${INSTALL_BIN}/nightagent"

# Installa shim e dylib
sudo mkdir -p "${INSTALL_LIB}"
sudo install -m 755 "${TMP}/guardian-shim"           "${INSTALL_LIB}/guardian-shim"
sudo install -m 644 "${TMP}/guardian-intercept.dylib" "${INSTALL_LIB}/guardian-intercept.dylib"
ok "guardian-shim + dylib → ${INSTALL_LIB}/"

# Copia policy di default se non esiste
if [ ! -f "${CONFIG_DIR}/policy.yaml" ]; then
  mkdir -p "${CONFIG_DIR}"
  install -m 600 "${TMP}/configs/default_policy.yaml" "${CONFIG_DIR}/policy.yaml"
  ok "Policy di default → ${CONFIG_DIR}/policy.yaml"
fi

echo ""
echo -e "${BOLD}Night Agent ${VERSION} installato.${RESET}"
echo ""
echo "Se macOS blocca il binario con un avviso Gatekeeper, esegui:"
echo "  sudo xattr -d com.apple.quarantine ${INSTALL_BIN}/nightagent"
echo "  sudo xattr -d com.apple.quarantine ${INSTALL_LIB}/guardian-shim"
echo "  sudo xattr -d com.apple.quarantine ${INSTALL_LIB}/guardian-intercept.dylib"
echo ""
echo "Prossimo step:"
echo "  nightagent init"
