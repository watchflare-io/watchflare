#!/bin/bash
set -e

# Watchflare Agent - Linux Installation Script
#
# Usage:
#   curl -fsSL https://get.watchflare.io | sudo bash -s -- [options]
#   sudo ./install-agent.sh [options]
#
# Options:
#   --uninstall           Remove the agent and its files
#   --local               Use local binary from ./dist/ (dev mode)
#   --token=TOKEN or --token TOKEN   Registration token
#   --host=HOST   or --host HOST     Backend hostname (default: localhost)
#   --port=PORT   or --port PORT     Backend port (default: 50051)
#   --containers          Enable container metrics collection (Docker/Podman/Colima)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

GITHUB_REPO="watchflare-io/watchflare"
BINARY_NAME="watchflare-agent"
SERVICE_NAME="watchflare-agent"
AGENT_USER="watchflare"
AGENT_GROUP="watchflare"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/watchflare"
DATA_DIR="/var/lib/watchflare"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
LOG_FILE="/var/log/watchflare-agent.log"

UNINSTALL=false
LOCAL_MODE=false
TOKEN=""
HOST=""
PORT=""
CONTAINERS=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --uninstall) UNINSTALL=true; shift ;;
        --local) LOCAL_MODE=true; shift ;;
        --token=*) TOKEN="${1#*=}"; shift ;;
        --token) TOKEN="$2"; shift 2 ;;
        --host=*) HOST="${1#*=}"; shift ;;
        --host) HOST="$2"; shift 2 ;;
        --port=*) PORT="${1#*=}"; shift ;;
        --port) PORT="$2"; shift 2 ;;
        --containers) CONTAINERS=true; shift ;;
        -h|--help)
            echo "Usage: sudo $0 [--uninstall] [--local] [--token=TOKEN] [--host=HOST] [--port=PORT] [--containers]"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown argument: $1${NC}"
            echo "Usage: sudo $0 [--uninstall] [--local] [--token=TOKEN] [--host=HOST] [--port=PORT] [--containers]"
            exit 1
            ;;
    esac
done

if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}Error: This script must be run as root (use sudo)${NC}"
    exit 1
fi

# ─── Uninstall ────────────────────────────────────────────────────────────────

if [ "$UNINSTALL" = true ]; then
    echo -e "${YELLOW}=== Watchflare Agent Uninstallation ===${NC}"
    echo ""
    read -p "This will remove the Watchflare agent and all its data. Continue? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Uninstallation cancelled."
        exit 0
    fi
    echo ""

    echo -e "${YELLOW}[1/5]${NC} Stopping service..."
    if command -v systemctl >/dev/null 2>&1; then
        if systemctl is-active --quiet ${SERVICE_NAME}; then
            systemctl stop ${SERVICE_NAME}
            echo "  → Service stopped"
        else
            echo "  → Service not running"
        fi
        if systemctl is-enabled --quiet ${SERVICE_NAME} 2>/dev/null; then
            systemctl disable ${SERVICE_NAME}
            echo "  → Service disabled"
        fi
        if [ -f "$SERVICE_FILE" ]; then
            rm -f "$SERVICE_FILE"
            systemctl daemon-reload
            echo "  → Service file removed"
        fi
    else
        echo "  → Systemd not available"
    fi

    echo -e "${YELLOW}[2/5]${NC} Removing binary..."
    if [ -f "${INSTALL_DIR}/${BINARY_NAME}" ]; then
        rm -f "${INSTALL_DIR}/${BINARY_NAME}"
        echo "  → Removed ${INSTALL_DIR}/${BINARY_NAME}"
    else
        echo "  → Binary not found"
    fi

    echo -e "${YELLOW}[3/5]${NC} Removing data directory..."
    if [ -d "$DATA_DIR" ]; then
        read -p "Remove data directory ${DATA_DIR}? (WAL, package state) [y/N] " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "$DATA_DIR"
            echo "  → Removed $DATA_DIR"
        else
            echo "  → Kept $DATA_DIR"
        fi
    else
        echo "  → Data directory not found"
    fi

    echo -e "${YELLOW}[4/5]${NC} Removing configuration..."
    if [ -d "$CONFIG_DIR" ]; then
        read -p "Remove configuration directory ${CONFIG_DIR}? (agent.conf, ca.pem) [y/N] " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "$CONFIG_DIR"
            echo "  → Removed $CONFIG_DIR"
        else
            echo "  → Kept $CONFIG_DIR"
        fi
    else
        echo "  → Configuration directory not found"
    fi

    echo -e "${YELLOW}[5/5]${NC} Removing system user..."
    if id -u ${AGENT_USER} >/dev/null 2>&1; then
        read -p "Remove system user '${AGENT_USER}'? [y/N] " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            userdel ${AGENT_USER} 2>/dev/null || true
            groupdel ${AGENT_GROUP} 2>/dev/null || true
            echo "  → Removed user '${AGENT_USER}'"
        else
            echo "  → Kept user '${AGENT_USER}'"
        fi
    else
        echo "  → User '${AGENT_USER}' not found"
    fi

    echo ""
    echo -e "${GREEN}=== Uninstallation Complete ===${NC}"
    exit 0
fi

# ─── Install ──────────────────────────────────────────────────────────────────

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  PLATFORM="linux_amd64" ;;
    aarch64|arm64) PLATFORM="linux_arm64" ;;
    *)
        echo -e "${RED}Error: Unsupported architecture: $ARCH${NC}"
        exit 1
        ;;
esac

# Locate or download binary
if [ "$LOCAL_MODE" = true ]; then
    echo "Running in LOCAL MODE (development)"
    if [ -f "./dist/${PLATFORM}/${BINARY_NAME}" ]; then
        TMP_BINARY="/tmp/${BINARY_NAME}"
        cp "./dist/${PLATFORM}/${BINARY_NAME}" "$TMP_BINARY"
        chmod +x "$TMP_BINARY"
        echo "  → Using local binary: ./dist/${PLATFORM}/${BINARY_NAME}"
    elif [ -f "./${BINARY_NAME}" ]; then
        TMP_BINARY="/tmp/${BINARY_NAME}"
        cp "./${BINARY_NAME}" "$TMP_BINARY"
        chmod +x "$TMP_BINARY"
        echo "  → Using local binary: ./${BINARY_NAME}"
    else
        echo -e "${RED}Error: Binary not found. Build first: cd agent && go build -o ${BINARY_NAME}${NC}"
        exit 1
    fi
else
    echo "Downloading latest release from GitHub..."
    VERSION=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4 | sed 's/^v//')
    if [ -z "$VERSION" ]; then
        echo -e "${RED}Error: Failed to detect latest version${NC}"
        exit 1
    fi
    echo "  → Version: ${VERSION}"

    ARCHIVE_NAME="${BINARY_NAME}_${VERSION}_${PLATFORM}.tar.gz"
    ARCHIVE_URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/${ARCHIVE_NAME}"
    TMP_ARCHIVE="/tmp/${ARCHIVE_NAME}"
    TMP_BINARY="/tmp/${BINARY_NAME}"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$ARCHIVE_URL" -o "$TMP_ARCHIVE"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$ARCHIVE_URL" -O "$TMP_ARCHIVE"
    else
        echo -e "${RED}Error: curl or wget required${NC}"
        exit 1
    fi

    tar xzf "$TMP_ARCHIVE" -C /tmp "${BINARY_NAME}"
    rm -f "$TMP_ARCHIVE"
    chmod +x "$TMP_BINARY"
    echo "  → Downloaded and extracted"
fi

echo -e "${YELLOW}[1/5]${NC} Checking for existing installation..."
if command -v systemctl >/dev/null 2>&1; then
    SYSTEMD_STATE=$(systemctl is-system-running 2>/dev/null || true)
    if [ "$SYSTEMD_STATE" = "running" ] || [ "$SYSTEMD_STATE" = "degraded" ]; then
        echo "  → Systemd detected (state: ${SYSTEMD_STATE})"
        if systemctl is-active --quiet ${SERVICE_NAME}; then
            echo "  → Stopping existing service..."
            systemctl stop ${SERVICE_NAME}
            sleep 1
        fi
    else
        echo "  → Systemd not available (state: ${SYSTEMD_STATE:-none})"
    fi
else
    echo "  → Systemd not available (systemctl not found)"
fi

echo -e "${YELLOW}[2/5]${NC} Creating system user '${AGENT_USER}'..."
if ! getent group ${AGENT_GROUP} >/dev/null 2>&1; then
    groupadd --system ${AGENT_GROUP}
    echo "  → Created group '${AGENT_GROUP}'"
else
    echo "  → Group '${AGENT_GROUP}' already exists"
fi
if ! id -u ${AGENT_USER} >/dev/null 2>&1; then
    useradd --system \
        --gid ${AGENT_GROUP} \
        --home-dir /var/empty \
        --shell /usr/sbin/nologin \
        --comment "Watchflare Agent" \
        ${AGENT_USER}
    echo "  → Created user '${AGENT_USER}'"
else
    echo "  → User '${AGENT_USER}' already exists"
fi

echo -e "${YELLOW}[3/5]${NC} Creating directories..."
for dir in "$CONFIG_DIR" "$DATA_DIR" "${DATA_DIR}/wal"; do
    mkdir -p "$dir"
done
chown root:${AGENT_GROUP} "$CONFIG_DIR" && chmod 750 "$CONFIG_DIR"
chown ${AGENT_USER}:${AGENT_GROUP} "$DATA_DIR" "${DATA_DIR}/wal" && chmod 750 "$DATA_DIR" "${DATA_DIR}/wal"
echo "  → Created $CONFIG_DIR, $DATA_DIR"

echo -e "${YELLOW}[4/5]${NC} Installing binary..."
cp "$TMP_BINARY" "${INSTALL_DIR}/${BINARY_NAME}"
chown root:root "${INSTALL_DIR}/${BINARY_NAME}"
chmod 755 "${INSTALL_DIR}/${BINARY_NAME}"
touch "$LOG_FILE"
chown ${AGENT_USER}:${AGENT_GROUP} "$LOG_FILE" && chmod 644 "$LOG_FILE"
echo "  → Installed to ${INSTALL_DIR}/${BINARY_NAME}"

echo -e "${YELLOW}[5/5]${NC} Installing service and registering..."
INSTALL_ARGS=()
[ -n "$TOKEN" ] && INSTALL_ARGS+=(--token="$TOKEN")
[ -n "$HOST" ]  && INSTALL_ARGS+=(--host="$HOST")
[ -n "$PORT" ]  && INSTALL_ARGS+=(--port="$PORT")
[ "$CONTAINERS" = true ] && INSTALL_ARGS+=(--containers)
if ! "${INSTALL_DIR}/${BINARY_NAME}" install "${INSTALL_ARGS[@]}"; then
    echo -e "  ${RED}→ Installation failed${NC}"
    exit 1
fi
