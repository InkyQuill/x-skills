#!/bin/sh
set -eu

BIN_NAME="x-skills"
INSTALL_DIR="${X_SKILLS_INSTALL_DIR:-$HOME/.local/bin}"

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
  log "Starting development installer"
  need go
  need mktemp
  need install
  need mv

  SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
  REPO_ROOT=$(dirname "$SCRIPT_DIR")
  cd "$REPO_ROOT"

  tmp="$(mktemp -d)"
  staged=""
  trap '[ -z "$staged" ] || rm -f "$staged"; rm -rf "$tmp"' EXIT INT TERM

  log "Building development $BIN_NAME"
  go build \
    -ldflags '-X github.com/InkyQuill/x-skills/internal/buildinfo.version=dev' \
    -o "$tmp/$BIN_NAME" \
    ./cmd/x-skills

  log "Using install directory $INSTALL_DIR"
  mkdir -p "$INSTALL_DIR"
  target="$INSTALL_DIR/$BIN_NAME"
  staged="$INSTALL_DIR/.${BIN_NAME}.install.$$"
  if [ -e "$target" ] || [ -L "$target" ]; then
    log "existing $BIN_NAME found at $target; replacing it"
  fi
  install -m 0755 "$tmp/$BIN_NAME" "$staged"
  mv -f "$staged" "$target"
  staged=""
  install_xs_link "$target"

  printf '%s\n' "installed $BIN_NAME to $target"
}

main "$@"
