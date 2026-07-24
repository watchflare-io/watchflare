#!/bin/bash
set -e

# Watchflare Agent - macOS Homebrew Installation Script
#
# Usage:
#   ./install-agent-brew.sh [options]
#
# Options:
#   --uninstall           Remove the agent via Homebrew
#   --token=TOKEN or --token TOKEN   Registration token
#   --host=HOST   or --host HOST     Backend hostname (default: localhost)
#   --port=PORT   or --port PORT     Backend port (default: 50051)
#   --containers          Enable container metrics collection (Docker/Podman/Colima)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

UNINSTALL=false
TOKEN=""
HOST=""
PORT=""
CONTAINERS=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --uninstall) UNINSTALL=true; shift ;;
        --token=*) TOKEN="${1#*=}"; shift ;;
        --token) TOKEN="$2"; shift 2 ;;
        --host=*) HOST="${1#*=}"; shift ;;
        --host) HOST="$2"; shift 2 ;;
        --port=*) PORT="${1#*=}"; shift ;;
        --port) PORT="$2"; shift 2 ;;
        --containers) CONTAINERS=true; shift ;;
        -h|--help)
            echo "Usage: $0 [--uninstall] [--token=TOKEN] [--host=HOST] [--port=PORT] [--containers]"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown argument: $1${NC}"
            echo "Usage: $0 [--uninstall] [--token=TOKEN] [--host=HOST] [--port=PORT] [--containers]"
            exit 1
            ;;
    esac
done

# ─── Uninstall ────────────────────────────────────────────────────────────────

if [ "$UNINSTALL" = true ]; then
    echo -e "${YELLOW}=== Watchflare Agent Uninstallation ===${NC}"
    echo ""
    read -p "This will remove the Watchflare agent via Homebrew. Continue? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Uninstallation cancelled."
        exit 0
    fi

    brew services stop watchflare-agent 2>/dev/null || true
    brew uninstall watchflare-agent 2>/dev/null || true
    brew untap watchflare-io/watchflare 2>/dev/null || true

    echo ""
    echo -e "${GREEN}✓ Uninstalled${NC}"
    echo ""
    echo "Configuration and data may still exist at:"
    echo "  $(brew --prefix)/etc/watchflare/"
    echo "  $(brew --prefix)/var/watchflare/"
    echo ""
    echo "Remove manually if needed:"
    echo "  rm -rf \$(brew --prefix)/etc/watchflare \$(brew --prefix)/var/watchflare"
    exit 0
fi

# ─── Install ──────────────────────────────────────────────────────────────────

echo -e "${GREEN}=== Watchflare Agent Installation (macOS) ===${NC}"
echo ""

# Check/install Homebrew
if ! command -v brew >/dev/null 2>&1; then
    read -p "Homebrew is not installed. Install it now? (y/n): " install_brew
    if [[ $install_brew =~ ^[Yy]$ ]]; then
        echo "Installing Homebrew..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        if ! command -v brew >/dev/null 2>&1; then
            echo -e "${RED}Homebrew installation failed. Install manually and retry.${NC}"
            exit 1
        fi
        echo -e "${GREEN}✓ Homebrew installed${NC}"
    else
        echo "Homebrew is required. Install it from https://brew.sh and retry."
        exit 1
    fi
fi

echo "Installing watchflare-agent via Homebrew..."
brew tap watchflare-io/watchflare
brew install watchflare-agent
echo -e "${GREEN}✓ Installed${NC}"
echo ""

# Registration
if [ -n "$TOKEN" ]; then
    HOST="${HOST:-localhost}"
    PORT="${PORT:-50051}"
    REGISTER_ARGS=(--token="$TOKEN" --host="$HOST" --port="$PORT")
    [ "$CONTAINERS" = true ] && REGISTER_ARGS+=(--containers)
    echo "Registering agent..."
    if watchflare-agent-launcher register "${REGISTER_ARGS[@]}"; then
        echo -e "${GREEN}✓ Registration successful${NC}"
        echo ""
        echo "Starting service..."
        brew services start watchflare-agent
        echo -e "${GREEN}✓ Service started${NC}"
    else
        echo -e "${RED}Registration failed — start the service manually after fixing the configuration.${NC}"
    fi
else
    echo -e "${YELLOW}No token provided. Register manually before starting:${NC}"
    echo "  watchflare-agent-launcher register --token=YOUR_TOKEN --host=YOUR_HOST [--containers]"
    echo "  brew services start watchflare-agent"
fi

echo ""
echo -e "${GREEN}=== Installation Complete ===${NC}"
echo ""
echo "Service management:"
echo "  Status:  brew services info watchflare-agent"
echo "  Stop:    brew services stop watchflare-agent"
echo "  Start:   brew services start watchflare-agent"
echo "  Restart: brew services restart watchflare-agent"
echo "  Logs:    tail -f \$(brew --prefix)/var/log/watchflare-agent.log"
echo ""
echo "To update:"
echo "  brew upgrade watchflare-agent && brew services restart watchflare-agent"
echo ""
echo "To uninstall:"
echo "  $0 --uninstall"
