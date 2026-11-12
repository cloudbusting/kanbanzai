#!/usr/bin/env bash
# After every tool call: renew this session's own card claims that have used half their lease, silently, so a
# long step is never offered to another session while its owner lives. Whatever happens (no project, no command,
# an old core), it exits 0: a hook that fails would report an error into the session.
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "${CLAUDE_PROJECT_DIR:-$PWD}" 2>/dev/null || exit 0

# The per-user folder `kanbanzai install` writes to: $XDG_DATA_HOME/kanbanzai/bin when XDG_DATA_HOME is an absolute
# path, else $HOME/.local/share/kanbanzai/bin. Empty when neither is known, so the checks below never match.
case "${XDG_DATA_HOME:-}" in
  /*) data="$XDG_DATA_HOME" ;;
  *) data="${HOME:+$HOME/.local/share}" ;;
esac

if [ -n "${KANBANZAI_BIN:-}" ]; then
  [ -x "$KANBANZAI_BIN" ] || exit 0
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
  exit 0
fi

"$kz" claim heartbeat --hook >/dev/null 2>&1
exit 0
