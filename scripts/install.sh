#!/bin/sh
set -eu

REPO="InkyQuill/x-skills"
BIN_NAME="x-skills"
INSTALL_DIR="${X_SKILLS_INSTALL_DIR:-$HOME/.local/bin}"

fail() {
  printf '%s\n' "x-skills install: $*" >&2
  exit 1
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
    curl -fsSL "$url" -o "$dest"
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
    printf '%s\n' "xs already exists; leaving it unchanged"
    return 0
  fi

  if [ -e "$link" ]; then
    printf '%s\n' "$link already exists; leaving it unchanged"
    return 0
  fi

  ln -s "$target" "$link" 2>/dev/null || printf '%s\n' "could not create xs shortcut; x-skills is installed"
}

main() {
  need uname
  need mktemp
  need tar

  os="$(detect_os)"
  arch="$(detect_arch)"
  version="$(latest_version)"
  asset="${BIN_NAME}_${os}_${arch}.tar.gz"

  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT INT TERM

  mkdir -p "$INSTALL_DIR"
  if [ "$version" = "latest" ]; then
    url="https://github.com/${REPO}/releases/latest/download/${asset}"
  else
    url="https://github.com/${REPO}/releases/download/${version}/${asset}"
  fi

  archive="$tmp/$asset"
  download "$url" "$archive"
  tar -xzf "$archive" -C "$tmp"
  install -m 0755 "$tmp/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
  install_xs_link "$INSTALL_DIR/$BIN_NAME"

  printf '%s\n' "installed $BIN_NAME to $INSTALL_DIR/$BIN_NAME"
}

main "$@"
