#!/usr/bin/env bash
# At session start: print the board's standing context (where the cards and claims live, the columns and their
# steps, the pull rule, the project's files to read first). Claude Code adds what this prints to the session.
# The command is KANBANZAI_BIN, then `kanbanzai` on PATH, then the per-user folder, then a local build beside this plugin.
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "${CLAUDE_PROJECT_DIR:-$PWD}" 2>/dev/null || exit 0

# The per-user folder `kanbanzai install` writes to: $XDG_DATA_HOME/kanbanzai/bin when XDG_DATA_HOME is an absolute
# path, else $HOME/.local/share/kanbanzai/bin. Empty when neither is known, so the checks below never match.
case "${XDG_DATA_HOME:-}" in
  /*) data="$XDG_DATA_HOME" ;;
  *) data="${HOME:+$HOME/.local/share}" ;;
esac

missing() {
  printf '%s\n' 'Kanbanzai: the `kanbanzai` command is not installed. Install it from https://github.com/cloudbusting/kanbanzai/releases and put it on PATH, or set KANBANZAI_BIN.'
  exit 0
}

if [ -n "${KANBANZAI_BIN:-}" ]; then
  [ -x "$KANBANZAI_BIN" ] || missing
  kz="$KANBANZAI_BIN"
elif kz="$(command -v kanbanzai 2>/dev/null)"; then
  :
elif [ -n "$data" ] && [ -x "$data/kanbanzai/bin/kanbanzai" ] && [ -f "$data/kanbanzai/bin/kanbanzai" ]; then
  kz="$data/kanbanzai/bin/kanbanzai"
elif [ -n "$data" ] && [ -x "$data/kanbanzai/bin/kanbanzai.exe" ] && [ -f "$data/kanbanzai/bin/kanbanzai.exe" ]; then
  kz="$data/kanbanzai/bin/kanbanzai.exe"
elif [ -x "$root/bin/kanbanzai" ]; then
  kz="$root/bin/kanbanzai"
elif [ -x "$root/bin/kanbanzai.exe" ]; then
  kz="$root/bin/kanbanzai.exe"
else
  missing
fi

"$kz" context 2>/dev/null
exit 0
