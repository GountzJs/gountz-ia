#!/bin/sh
# Script de instalación rápida de gz-ia
# Uso: curl -fsSL https://raw.githubusercontent.com/GountzJs/gountz-ia/main/install.sh | bash

set -e

REPO="${GITHUB_REPO:-"GountzJs/gountz-ia"}"
BINARY_NAME="gz-ia"

# Colores para salida formateada
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

printf "${BLUE}${BOLD}› Gountz IA (gz-ia) — Instalador Universal${NC}\n"

# 1. Detectar Sistema Operativo
OS="$(uname -s)"
case "${OS}" in
    Linux*)     PLATFORM="linux" ;;
    *)          printf "${RED}Error: El instalador por script soporta Linux (amd64). Para Windows descarga el archivo .zip desde GitHub Releases: https://github.com/${REPO}/releases${NC}\n" >&2; exit 1 ;;
esac

# 2. Detectar Arquitectura
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64|amd64)   TARGET_ARCH="amd64" ;;
    *)              printf "${RED}Error: La arquitectura testeada y soportada es amd64/x86_64 (detectado: ${ARCH}).${NC}\n" >&2; exit 1 ;;
esac

printf "  Plataforma detectada: ${BOLD}${PLATFORM}/${TARGET_ARCH}${NC}\n"

# 3. Determinar Versión
REQUESTED_VERSION="${1:-}"
if [ -z "${REQUESTED_VERSION}" ]; then
    printf "  Consultando última versión en GitHub Releases...\n"
    # Consultar API de GitHub o seguir redirección
    LATEST_TAG=$(curl -sSL -H "Accept: application/vnd.github.v3+json" "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || true)
    
    if [ -z "${LATEST_TAG}" ]; then
        # Fallback a cabeceras HTTP si la API anónima alcanza rate limit
        LATEST_TAG=$(curl -sSI "https://github.com/${REPO}/releases/latest" 2>/dev/null | grep -i '^location:' | sed -E 's|.*/tag/(.*)|\1|' | tr -d '\r\n' || true)
    fi

    if [ -z "${LATEST_TAG}" ]; then
        LATEST_TAG="v0.4.0"
        printf "  ${BLUE}Nota:${NC} No se pudo consultar la API de GitHub, utilizando tag por defecto: ${LATEST_TAG}\n"
    fi
    VERSION="${LATEST_TAG#v}"
else
    LATEST_TAG="${REQUESTED_VERSION}"
    VERSION="${REQUESTED_VERSION#v}"
fi

printf "  Versión seleccionada: ${BOLD}${VERSION} (${LATEST_TAG})${NC}\n"

# 4. Construir URL de Descarga
ARCHIVE_NAME="gz-ia_${VERSION}_${PLATFORM}_${TARGET_ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/${ARCHIVE_NAME}"

# 5. Descargar y Extraer
TMP_DIR="$(mktemp -d)"
cleanup() {
    rm -rf "${TMP_DIR}"
}
trap cleanup EXIT INT TERM

printf "  Descargando ${ARCHIVE_NAME}...\n"
if ! curl -fSL --progress-bar "${DOWNLOAD_URL}" -o "${TMP_DIR}/${ARCHIVE_NAME}"; then
    # Intento de reintento invirtiendo prefijo 'v' si el release usa convención alternativa
    if [ "${LATEST_TAG#v}" = "${LATEST_TAG}" ]; then
        ALT_TAG="v${LATEST_TAG}"
    else
        ALT_TAG="${LATEST_TAG#v}"
    fi
    ALT_URL="https://github.com/${REPO}/releases/download/${ALT_TAG}/${ARCHIVE_NAME}"
    printf "  Reintentando descarga desde tag alternativo (${ALT_TAG})...\n"
    if ! curl -fSL --progress-bar "${ALT_URL}" -o "${TMP_DIR}/${ARCHIVE_NAME}"; then
        printf "${RED}Error: No se pudo descargar el binario desde:${NC}\n  ${DOWNLOAD_URL}\n  ${ALT_URL}\n" >&2
        printf "Verifica que el release contenga los binarios compilados y que el repositorio sea público.\n" >&2
        exit 1
    fi
fi

printf "  Extrayendo archivos...\n"
tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "${TMP_DIR}"

if [ ! -f "${TMP_DIR}/${BINARY_NAME}" ]; then
    printf "${RED}Error: El archivo binario no fue encontrado en el paquete descargado.${NC}\n" >&2
    exit 1
fi

# 6. Determinar Directorio de Instalación
if [ -n "${INSTALL_DIR:-}" ]; then
    DEST_DIR="${INSTALL_DIR}"
elif [ "$(id -u)" -eq 0 ]; then
    DEST_DIR="/usr/local/bin"
elif [ -d "${HOME}/.local/bin" ]; then
    DEST_DIR="${HOME}/.local/bin"
else
    mkdir -p "${HOME}/.local/bin"
    DEST_DIR="${HOME}/.local/bin"
fi

printf "  Instalando en ${BOLD}${DEST_DIR}/${BINARY_NAME}${NC}...\n"
chmod +x "${TMP_DIR}/${BINARY_NAME}"

if [ -w "${DEST_DIR}" ]; then
    mv "${TMP_DIR}/${BINARY_NAME}" "${DEST_DIR}/${BINARY_NAME}"
else
    printf "  Se requieren permisos de administrador (sudo) para escribir en ${DEST_DIR}:\n"
    sudo mv "${TMP_DIR}/${BINARY_NAME}" "${DEST_DIR}/${BINARY_NAME}"
fi

# 7. Verificación Final
printf "\n${GREEN}${BOLD}✓ gz-ia instalado exitosamente.${NC}\n"

# Comprobar si el directorio está en el PATH
case ":${PATH}:" in
    *:"${DEST_DIR}":*) ;;
    *)
        printf "${BLUE}Sugerencia:${NC} Asegúrate de tener '${DEST_DIR}' en tu variable PATH:\n"
        printf "  export PATH=\"\$PATH:${DEST_DIR}\"\n\n"
        ;;
esac

if command -v "${DEST_DIR}/${BINARY_NAME}" >/dev/null 2>&1; then
    "${DEST_DIR}/${BINARY_NAME}" version || true
fi
