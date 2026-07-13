#!/bin/sh
set -eu

REPO="InkyQuill/x-skills"
BIN_NAME="x-skills"
INSTALL_DIR="${X_SKILLS_INSTALL_DIR:-$HOME/.local/bin}"
# Release binaries embed github.com/InkyQuill/x-skills/internal/buildinfo.version at build time.

fail() {
  printf '%s\n' "x-skills install: $*" >&2
  exit 1
}

log() {
  printf '%s\n' "x-skills install: $*" >&2
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

detect_os() {
  case "$(uname -s)" in
    Darwin) printf 'darwin' ;;
    Linux) printf 'linux' ;;
    *) fail "unsupported operating system: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) printf 'amd64' ;;
    arm64 | aarch64) printf 'arm64' ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
  esac
}

latest_version() {
  if [ "${X_SKILLS_VERSION:-}" ]; then
    printf '%s' "$X_SKILLS_VERSION"
  else
    printf 'latest'
  fi
}

download() {
  url="$1"
  dest="$2"

  if command -v curl >/dev/null 2>&1; then
    if [ -t 2 ]; then
      curl -fL --progress-bar "$url" -o "$dest"
    else
      curl -fsSL "$url" -o "$dest"
    fi
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
  else
    fail "missing required command: curl or wget"
  fi
}

install_xs_link() {
  target="$1"
  link="$INSTALL_DIR/xs"

  if command -v xs >/dev/null 2>&1; then
    log "xs already exists; leaving it unchanged"
    return 0
  fi

  if [ -e "$link" ] || [ -L "$link" ]; then
    log "$link already exists; leaving it unchanged"
    return 0
  fi

  log "Creating xs shortcut at $link"
  ln -s "$target" "$link" 2>/dev/null || log "could not create xs shortcut; x-skills is installed"
}

main() {
  log "Starting installer"
  need uname
  need mktemp
  need tar
  need install
  need mv

  os="$(detect_os)"
  arch="$(detect_arch)"
  version="$(latest_version)"
  asset="${BIN_NAME}_${os}_${arch}.tar.gz"
  log "Detected $os/$arch"

  tmp="$(mktemp -d)"
  staged=""
  trap '[ -z "$staged" ] || rm -f "$staged"; rm -rf "$tmp"' EXIT INT TERM

  log "Using install directory $INSTALL_DIR"
  mkdir -p "$INSTALL_DIR"
  if [ "${X_SKILLS_DOWNLOAD_URL:-}" ]; then
    case "$X_SKILLS_DOWNLOAD_URL" in
      http://* | https://*) url="$X_SKILLS_DOWNLOAD_URL" ;;
      *) fail "X_SKILLS_DOWNLOAD_URL must be an absolute http:// or https:// URL" ;;
    esac
  elif [ "$version" = "latest" ]; then
    url="https://github.com/${REPO}/releases/latest/download/${asset}"
  else
    url="https://github.com/${REPO}/releases/download/${version}/${asset}"
  fi

  archive="$tmp/$asset"
  log "Downloading $asset from $url"
  download "$url" "$archive"
  log "Extracting $asset"
  tar -xzf "$archive" -C "$tmp"
  target="$INSTALL_DIR/$BIN_NAME"
  staged="$INSTALL_DIR/.${BIN_NAME}.install.$$"
  if [ -e "$target" ] || [ -L "$target" ]; then
    log "existing $BIN_NAME found at $target; replacing it"
  fi
  log "Installing $BIN_NAME to $target"
  install -m 0755 "$tmp/$BIN_NAME" "$staged"
  mv -f "$staged" "$target"
  staged=""
  install_xs_link "$target"

  printf '%s\n' "installed $BIN_NAME to $target"
}

main "$@"
