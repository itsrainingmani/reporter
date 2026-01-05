#!/usr/bin/env bash
set -euo pipefail

# Reporter installer
# Usage: curl -sSL https://raw.githubusercontent.com/itsrainingmani/reporter/main/install.sh | bash

REPO="itsrainingmani/reporter"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
SHARE_DIR="${SHARE_DIR:-$HOME/.local/share/reporter}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info() { echo -e "${CYAN}$*${NC}"; }
success() { echo -e "${GREEN}$*${NC}"; }
warn() { echo -e "${YELLOW}$*${NC}"; }
error() { echo -e "${RED}$*${NC}" >&2; exit 1; }

detect_platform() {
  local os arch

  case "$(uname -s)" in
    Darwin) os="darwin" ;;
    Linux)  os="linux" ;;
    *)      error "Unsupported OS: $(uname -s)" ;;
  esac

  case "$(uname -m)" in
    x86_64|amd64)   arch="amd64" ;;
    arm64|aarch64)  arch="arm64" ;;
    *)              error "Unsupported architecture: $(uname -m)" ;;
  esac

  echo "${os}_${arch}"
}

get_latest_version() {
  local response http_code body
  
  # Fetch with HTTP status code appended at the end
  response=$(curl -sSL -w "\n%{http_code}" "https://api.github.com/repos/${REPO}/releases/latest" 2>&1) || {
    warn "Network error: could not reach GitHub API"
    return 1
  }
  
  # Split response into body and status code
  http_code=$(echo "$response" | tail -n1)
  body=$(echo "$response" | sed '$d')
  
  case "$http_code" in
    200)
      echo "$body" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
      ;;
    403)
      warn "GitHub API rate limit exceeded. Try again later or set GITHUB_TOKEN."
      return 1
      ;;
    404)
      warn "No releases found for ${REPO}"
      return 1
      ;;
    *)
      warn "GitHub API returned HTTP ${http_code}"
      return 1
      ;;
  esac
}

main() {
  info "Installing reporter..."
  echo ""

  local platform version download_url tmp_dir

  platform="$(detect_platform)"
  info "Detected platform: ${platform}"

  version="$(get_latest_version)"
  if [[ -z "$version" ]]; then
    error "Could not determine latest version. Check https://github.com/${REPO}/releases"
  fi
  info "Latest version: ${version}"

  download_url="https://github.com/${REPO}/releases/download/${version}/reporter_${version#v}_${platform}.tar.gz"
  info "Downloading from: ${download_url}"

  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT

  if ! curl -sSL "$download_url" | tar -xz -C "$tmp_dir"; then
    error "Download failed. Check if the release exists: https://github.com/${REPO}/releases"
  fi

  mkdir -p "$INSTALL_DIR" "$SHARE_DIR"

  mv "$tmp_dir/reporter" "$INSTALL_DIR/"
  chmod +x "$INSTALL_DIR/reporter"

  if [[ -f "$tmp_dir/shell/reporter-auto.sh" ]]; then
    cp "$tmp_dir/shell/reporter-auto.sh" "$SHARE_DIR/"
  else
    # Fallback: download shell script directly
    curl -sSL "https://raw.githubusercontent.com/${REPO}/main/shell/reporter-auto.sh" \
      -o "$SHARE_DIR/reporter-auto.sh"
  fi

  echo ""
  success "✓ Installed reporter to ${INSTALL_DIR}/reporter"
  success "✓ Installed shell hook to ${SHARE_DIR}/reporter-auto.sh"
  echo ""

  # Check if INSTALL_DIR is in PATH
  if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    warn "Note: ${INSTALL_DIR} is not in your PATH."
    echo "Add this to your shell rc file:"
    echo ""
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo ""
  fi

  info "To enable automatic notifications, add this to your shell rc file:"
  echo ""
  echo "  source ${SHARE_DIR}/reporter-auto.sh"
  echo ""
  info "Optional environment variables:"
  echo "  REPORTER_THRESHOLD=10s     # minimum duration before notifying"
  echo "  REPORTER_ALWAYS=1          # always notify regardless of duration"
  echo "  REPORTER_PUSH_URL=...      # HTTP endpoint for phone notifications"
  echo "  REPORTER_EXCLUDE=ls,cd,... # comma-separated commands to skip"
  echo ""
  success "Done! Open a new shell to start using reporter."
}

main "$@"
