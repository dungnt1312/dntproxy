#!/usr/bin/env bash
set -euo pipefail

# ─── dntproxy installer ───────────────────────────────────────────────────────
# Builds from source and installs to ~/.local/bin/dntproxy
# Usage:
#   curl -sSL <raw-url>/install.sh | bash
#   or: ./install.sh
# ──────────────────────────────────────────────────────────────────────────────

APP_NAME="dntproxy"
INSTALL_DIR="$HOME/.local/bin"
BINARY="$INSTALL_DIR/$APP_NAME"
CONFIG_DIR="$HOME/.dntproxy"
REPO_URL="https://github.com/dungnt/dntproxy.git"

# ── Colors ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

info()    { echo -e "${CYAN}ℹ${NC}  $*"; }
success() { echo -e "${GREEN}✔${NC}  $*"; }
warn()    { echo -e "${YELLOW}⚠${NC}  $*"; }
error()   { echo -e "${RED}✖${NC}  $*" >&2; }

# ── Check prerequisites ──────────────────────────────────────────────────────
check_deps() {
    local missing=()

    if ! command -v go &>/dev/null; then
        missing+=("go (https://go.dev/dl/)")
    fi

    if ! command -v git &>/dev/null; then
        missing+=("git")
    fi

    if [ ${#missing[@]} -gt 0 ]; then
        error "Missing required tools:"
        for dep in "${missing[@]}"; do
            echo -e "   ${RED}→${NC} $dep"
        done
        exit 1
    fi
}

# ── Main ──────────────────────────────────────────────────────────────────────
main() {
    echo ""
    echo -e "${BOLD}${CYAN}╔══════════════════════════════════════╗${NC}"
    echo -e "${BOLD}${CYAN}║       dntproxy — installer           ║${NC}"
    echo -e "${BOLD}${CYAN}╚══════════════════════════════════════╝${NC}"
    echo ""

    # 1. Check deps
    info "Checking prerequisites..."
    check_deps
    success "go $(go version | awk '{print $3}' | sed 's/go//') found"

    # 2. Determine source directory
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

    # If running from the repo (go.mod exists), build in-place
    # Otherwise, clone to a temp directory
    if [ -f "$SCRIPT_DIR/go.mod" ]; then
        SRC_DIR="$SCRIPT_DIR"
        info "Building from local source: ${BOLD}$SRC_DIR${NC}"
    else
        SRC_DIR="$(mktemp -d)"
        trap "rm -rf '$SRC_DIR'" EXIT
        info "Cloning repository..."
        git clone --depth 1 "$REPO_URL" "$SRC_DIR"
        success "Cloned to temp directory"
    fi

    # 3. Build
    info "Building ${BOLD}$APP_NAME${NC}..."
    (
        cd "$SRC_DIR"
        CGO_ENABLED=0 go build \
            -ldflags="-s -w" \
            -trimpath \
            -o "$APP_NAME" \
            ./cmd/dntproxy/
    )
    success "Build complete"

    # 4. Install
    mkdir -p "$INSTALL_DIR"
    mv "$SRC_DIR/$APP_NAME" "$BINARY"
    chmod +x "$BINARY"
    success "Installed to ${BOLD}$BINARY${NC}"

    # 5. Config directory
    if [ ! -d "$CONFIG_DIR" ]; then
        mkdir -p "$CONFIG_DIR"
        # Copy example config if available
        if [ -f "$SRC_DIR/config.example.json" ]; then
            cp "$SRC_DIR/config.example.json" "$CONFIG_DIR/db.json"
            success "Created default config at ${BOLD}$CONFIG_DIR/db.json${NC}"
        fi
    else
        info "Config directory already exists: $CONFIG_DIR"
    fi

    # 6. Check PATH
    if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
        echo ""
        warn "${BOLD}~/.local/bin${NC} is not in your PATH."
        echo ""
        echo -e "   Add this to your ${BOLD}~/.bashrc${NC} or ${BOLD}~/.zshrc${NC}:"
        echo ""
        echo -e "   ${CYAN}export PATH=\"\$HOME/.local/bin:\$PATH\"${NC}"
        echo ""
        echo -e "   Then run: ${CYAN}source ~/.bashrc${NC}"
    fi

    # 7. Done
    echo ""
    echo -e "${GREEN}${BOLD}══════════════════════════════════════${NC}"
    echo -e "${GREEN}${BOLD}  ✔  Installation complete!${NC}"
    echo -e "${GREEN}${BOLD}══════════════════════════════════════${NC}"
    echo ""
    echo -e "  Run:   ${CYAN}$APP_NAME${NC}"
    echo -e "  Port:  ${CYAN}$APP_NAME --port 8080${NC}"
    echo -e "  Help:  ${CYAN}$APP_NAME --help${NC}"
    echo ""
}

main "$@"
