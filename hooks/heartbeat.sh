#!/usr/bin/env bash
# After every tool call: renew this session's own card claims that have used half their lease, silently, so a
# long step is never offered to another session while its owner lives. Whatever happens (no project, no python,
# an old core), it exits 0: a hook that fails would report an error into the session.
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "${CLAUDE_PROJECT_DIR:-$PWD}" 2>/dev/null || exit 0
"$root/bin/kanbanzai" claim heartbeat >/dev/null 2>&1
exit 0
