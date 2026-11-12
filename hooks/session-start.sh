#!/usr/bin/env bash
# At session start: print the board's standing context (where the cards and claims live, the columns and their
# steps, the pull rule, the project's files to read first). Claude Code adds what this prints to the session.
# The command is KANBANZAI_BIN, then the per-user folder (the copy the editor extension keeps at this plugin's release),
# then `kanbanzai` on PATH, then a local build beside this plugin.
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
elif [ -n "$data" ] && [ -x "$data/kanbanzai/bin/kanbanzai" ] && [ -f "$data/kanbanzai/bin/kanbanzai" ]; then
  kz="$data/kanbanzai/bin/kanbanzai"
elif [ -n "$data" ] && [ -x "$data/kanbanzai/bin/kanbanzai.exe" ] && [ -f "$data/kanbanzai/bin/kanbanzai.exe" ]; then
  kz="$data/kanbanzai/bin/kanbanzai.exe"
elif kz="$(command -v kanbanzai 2>/dev/null)"; then
  :
elif [ -x "$root/bin/kanbanzai" ]; then
  kz="$root/bin/kanbanzai"
elif [ -x "$root/bin/kanbanzai.exe" ]; then
  kz="$root/bin/kanbanzai.exe"
else
  missing
fi

# This plugin's release, so a command from another release says so (`context --adapter-version`). A command too old to
# know the flag fails on it, and the plain context is printed instead.
version=""
if [ -r "$root/.claude-plugin/plugin.json" ]; then
  while IFS= read -r line; do
    if [[ $line =~ \"version\"[[:space:]]*:[[:space:]]*\"([^\"]*)\" ]]; then
      version="${BASH_REMATCH[1]}"
      break
    fi
  done < "$root/.claude-plugin/plugin.json"
fi
if [ -n "$version" ] && out="$("$kz" context --adapter-version "$version" 2>/dev/null)"; then
  [ -z "$out" ] || printf '%s\n' "$out"
  exit 0
fi
"$kz" context 2>/dev/null
exit 0
