#!/usr/bin/env bash
# After every tool call: renew this session's own card claims that have used half their lease, silently, so a
# long step is never offered to another session while its owner lives. Whatever happens (no project, no command,
# an old core), it exits 0: a hook that fails would report an error into the session.
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "${CLAUDE_PROJECT_DIR:-$PWD}" 2>/dev/null || exit 0

if [ -n "${KANBANZAI_BIN:-}" ]; then
  [ -x "$KANBANZAI_BIN" ] || exit 0
  kz="$KANBANZAI_BIN"
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
