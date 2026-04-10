#!/usr/bin/env bash
set -euo pipefail

APP_NAME="dntproxy"
REPO="${DNTPROXY_REPO:-dungnt/dntproxy}"
VERSION="${DNTPROXY_VERSION:-latest}"
INSTALL_DIR="${DNTPROXY_INSTALL_DIR:-$HOME/.local/bin}"
CONFIG_DIR="${DNTPROXY_CONFIG_DIR:-$HOME/.dntproxy}"
TMP_DIR=""

usage() {
    cat <<'EOF'
Install dntproxy from GitHub Releases.

Usage:
  ./install.sh [--version <tag|latest>] [--install-dir <path>] [--repo <owner/repo>]

Examples:
  ./install.sh
  ./install.sh --version v0.1.0
  DNTPROXY_INSTALL_DIR=/usr/local/bin ./install.sh
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

ensure_downloader() {
    if command -v curl >/dev/null 2>&1; then
        echo "curl"
        return
    fi
    if command -v wget >/dev/null 2>&1; then
        echo "wget"
        return
    fi
    log_err "Missing downloader. Install curl or wget."
    exit 1
}

detect_target() {
    local os arch
    os="$(uname -s)"
    arch="$(uname -m)"

    case "$os" in
        Linux) os="linux" ;;
        Darwin) os="darwin" ;;
        *)
            log_err "Unsupported OS: $os (supported: Linux, Darwin)"
            exit 1
            ;;
    esac

    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *)
            log_err "Unsupported architecture: $arch (supported: amd64, arm64)"
            exit 1
            ;;
    esac

    printf '%s-%s' "$os" "$arch"
}

asset_url() {
    local target="$1"
    local file="${APP_NAME}-${target}.tar.gz"
    if [ "$VERSION" = "latest" ]; then
        printf 'https://github.com/%s/releases/latest/download/%s' "$REPO" "$file"
        return
    fi

    local tag="$VERSION"
    case "$tag" in
        v*) ;;
        *) tag="v${tag}" ;;
    esac
    printf 'https://github.com/%s/releases/download/%s/%s' "$REPO" "$tag" "$file"
}

download_file() {
    local downloader="$1"
    local url="$2"
    local out="$3"
    if [ "$downloader" = "curl" ]; then
        curl -fsSL "$url" -o "$out"
    else
        wget -qO "$out" "$url"
    fi
}

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --version)
                VERSION="$2"
                shift 2
                ;;
            --install-dir)
                INSTALL_DIR="$2"
                shift 2
                ;;
            --repo)
                REPO="$2"
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

    local downloader target url archive extracted_bin bin_path config_example
    downloader="$(ensure_downloader)"
    target="$(detect_target)"
    url="$(asset_url "$target")"

    TMP_DIR="$(mktemp -d)"
    trap cleanup EXIT
    archive="${TMP_DIR}/${APP_NAME}.tar.gz"

    log_info "Downloading ${APP_NAME} (${target}) from ${url}"
    if ! download_file "$downloader" "$url" "$archive"; then
        log_err "Download failed. Ensure release asset exists for target ${target}."
        exit 1
    fi

    tar -xzf "$archive" -C "$TMP_DIR"
    extracted_bin="$(find "$TMP_DIR" -type f -name "$APP_NAME" | head -n 1)"
    if [ -z "$extracted_bin" ]; then
        log_err "Binary not found in release archive."
        exit 1
    fi

    mkdir -p "$INSTALL_DIR"
    bin_path="${INSTALL_DIR}/${APP_NAME}"
    install -m 0755 "$extracted_bin" "$bin_path"

    config_example="$(find "$TMP_DIR" -type f -name "config.example.json" | head -n 1)"
    if [ -n "$config_example" ] && [ ! -f "${CONFIG_DIR}/db.json" ]; then
        mkdir -p "$CONFIG_DIR"
        cp "$config_example" "${CONFIG_DIR}/db.json"
        log_ok "Created default config at ${CONFIG_DIR}/db.json"
    fi

    log_ok "Installed to ${bin_path}"
    if ! printf '%s' "$PATH" | tr ':' '\n' | grep -Fxq "$INSTALL_DIR"; then
        log_info "Add this path to your shell profile if needed:"
        printf '  export PATH="%s:$PATH"\n' "$INSTALL_DIR"
    fi
    log_info "Try: ${APP_NAME} --help"
}

main "$@"
