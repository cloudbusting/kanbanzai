#!/usr/bin/env bash
# At session start: print the board's standing context (where the cards and claims live, the columns and their
# steps, the pull rule, the project's files to read first). Claude Code adds what this prints to the session.
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "${CLAUDE_PROJECT_DIR:-$PWD}" 2>/dev/null || exit 0
"$root/bin/kanbanzai" context 2>/dev/null
exit 0
