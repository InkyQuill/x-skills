#!/bin/sh
set -eu

REPO_URL="https://github.com/InkyQuill/x-skills.git"
missing=""

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    missing="$missing $1"
  fi
}

need git
need uv

if [ -n "$missing" ]; then
  printf '%s\n' "x-skills install: missing required command(s):$missing" >&2
  printf '%s\n' "Install the missing command(s), then re-run this script." >&2
  exit 1
fi

uv tool install --upgrade "git+$REPO_URL"

if command -v x-skills >/dev/null 2>&1; then
  x-skills doctor
else
  printf '%s\n' "x-skills installed. Ensure uv's tool bin directory is on PATH." >&2
fi
