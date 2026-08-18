#!/usr/bin/env bash
set -euo pipefail

APP_NAME="dntproxy"
INSTALL_DIR="${DNTPROXY_INSTALL_DIR:-$HOME/.local/bin}"
TMP_DIR=""

usage() {
    cat <<'EOF'
Build dntproxy from the local source tree and install it to ~/.local/bin.

Usage:
  ./install-local.sh [--install-dir <path>] [--skip-ui-install]

Environment:
  DNTPROXY_INSTALL_DIR  Override install directory (default: ~/.local/bin)

Examples:
  ./install-local.sh
  ./install-local.sh --install-dir /usr/local/bin
  DNTPROXY_INSTALL_DIR="$HOME/bin" ./install-local.sh
EOF
}

log_info() { printf '[INFO] %s\n' "$*"; }
log_ok() { printf '[OK] %s\n' "$*"; }
log_err() { printf '[ERR] %s\n' "$*" >&2; }

cleanup() {
    if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
        rm -rf "$TMP_DIR"
    fi
}

require_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        log_err "Missing required command: $1"
        exit 1
    fi
}

parse_args() {
    SKIP_UI_INSTALL=0
    RESTART_PM2=0
    PM2_PROCESS_NAME="dntproxy"

    while [ $# -gt 0 ]; do
        case "$1" in
            --install-dir)
                if [ $# -lt 2 ]; then
                    log_err "Missing value for --install-dir"
                    exit 1
                fi
                INSTALL_DIR="$2"
                shift 2
                ;;
            --skip-ui-install)
                SKIP_UI_INSTALL=1
                shift
                ;;
            --restart)
                RESTART_PM2=1
                shift
                ;;
            --pm2-name)
                if [ $# -lt 2 ]; then
                    log_err "Missing value for --pm2-name"
                    exit 1
                fi
                PM2_PROCESS_NAME="$2"
                shift 2
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                log_err "Unknown argument: $1"
                usage
                exit 1
                ;;
        esac
    done
}

main() {
    parse_args "$@"

    require_command go
    require_command bun


    local repo_dir bin_name built_bin installed_bin
    repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    TMP_DIR="$(mktemp -d)"
    trap cleanup EXIT

    case "$(uname -s)" in
        MINGW*|MSYS*|CYGWIN*) bin_name="${APP_NAME}.exe" ;;
        *) bin_name="$APP_NAME" ;;
    esac

    log_info "Building UI with Bun"
    if [ "$SKIP_UI_INSTALL" -eq 1 ]; then
        (cd "$repo_dir/ui" && bun run build)
    else
        (cd "$repo_dir/ui" && bun i && bun run build)
    fi

    built_bin="${TMP_DIR}/${bin_name}"
    log_info "Building Go binary"
    (cd "$repo_dir" && go build -o "$built_bin" ./cmd/dntproxy/)

    mkdir -p "$INSTALL_DIR"
    installed_bin="${INSTALL_DIR}/${bin_name}"
    install -m 0755 "$built_bin" "$installed_bin"

    log_ok "Installed to ${installed_bin}"
    if ! printf '%s' "$PATH" | tr ':' '\n' | grep -Fxq "$INSTALL_DIR"; then
        log_info "Add this path to your shell profile if needed:"
        printf '  export PATH="%s:$PATH"\n' "$INSTALL_DIR"
    fi

    if [ "$RESTART_PM2" -eq 1 ]; then
        if command -v pm2 >/dev/null 2>&1; then
            log_info "Restarting pm2 process: ${PM2_PROCESS_NAME}"
            pm2 restart "$PM2_PROCESS_NAME" --update-env
            log_ok "pm2 restarted"
        else
            log_err "pm2 not found in PATH, cannot restart"
            exit 1
        fi
    fi

    log_info "Try: ${APP_NAME} --help"
}

main "$@"
