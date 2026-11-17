#!/bin/sh
# The command behind /kanbanzai-demo-gif: the steps of remaking the board demo GIF that need no judgement.
#
#   demo.sh prepare   download and check the pinned editor, build the .vsix from this checkout's HEAD, seed the
#                     demo project, write a scratch Claude Code config, install the extension and start the editor
#   demo.sh stop      stop the editor
#   demo.sh clean     stop it and remove every scratch folder prepare made, and the demo project's worktrees
#   demo.sh assemble FRAMES_DIR OUT_DIR   pick, mask and caption the recorded frames (assemble.py)
#   demo.sh verify    check a take and the candidate GIF against the budget (verify.py)
#   demo.sh step NAME FRAMES_DIR EDITOR_URL   print a Playwright step script with its placeholders filled in (step.py)
#
# Everything lives under DEMO_WORK (default /tmp/kanbanzai-demo-gif) and DEMO_PROJECT (default /tmp/demo-proj);
# DEMO_PORT (default 4001) is the loopback port. DEMO_LOCK, DEMO_EDITOR_TARBALL (use this file instead of downloading
# it) and DEMO_SKIP_BUILD exist for the tests. Nothing here is shipped, and nothing leaves the machine except the one
# download of the pinned editor.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
work=${DEMO_WORK:-/tmp/kanbanzai-demo-gif}
project=${DEMO_PROJECT:-/tmp/demo-proj}
port=${DEMO_PORT:-4001}
lock=${DEMO_LOCK:-$here/editor.lock}

usage() {
	echo "usage: demo.sh prepare | stop | clean | assemble FRAMES_DIR OUT_DIR | verify [options] | step NAME FRAMES_DIR EDITOR_URL" >&2
	exit 2
}

lockvalue() {
	sed -n "s/^$1=//p" "$lock" | head -n 1
}

stop() {
	if [ -f "$work/editor.pid" ]; then
		pid=$(cat "$work/editor.pid")
		# the pid file holds the editor's own process, started by prepare
		kill "$pid" 2>/dev/null || true
		rm -f "$work/editor.pid"
		echo "editor stopped"
	fi
}

prepare() {
	url=$(lockvalue url)
	want=$(lockvalue sha256)
	if [ -z "$url" ] || [ -z "$want" ]; then
		echo "demo.sh: $lock lacks a url or a sha256" >&2
		exit 2
	fi
	mkdir -p "$work/dl" "$work/editor"
	tarball=${DEMO_EDITOR_TARBALL:-}
	if [ -z "$tarball" ]; then
		tarball=$work/dl/editor.tar.gz
		[ -f "$tarball" ] || curl -fsSL -o "$tarball" "$url"
	fi
	got=$(sha256sum "$tarball" | cut -d ' ' -f 1)
	if [ "$got" != "$want" ]; then
		echo "demo.sh: the editor download does not match editor.lock" >&2
		echo "  expected sha256 $want" >&2
		echo "  got      sha256 $got" >&2
		exit 2
	fi
	tar -xzf "$tarball" -C "$work/editor" --strip-components=1
	editor=$work/editor/bin/openvscode-server

	if [ -z "${DEMO_SKIP_BUILD:-}" ]; then
		root=$(git -C "$here" rev-parse --show-toplevel)
		sha=$(git -C "$root" rev-parse HEAD)
		version=$(sed -n 's/^  "version": "\(.*\)",/\1/p' "$root/board-view/package.json" | head -n 1)
		mkdir -p "$work/src"
		# built from the committed tree, as the package step does, so the film shows what is committed
		git -C "$root" archive "$sha" | tar -x -C "$work/src"
		(cd "$work/src" && go run ./tools/release -version "v$version" -out "$work/release" -bins "$work/bins" >/dev/null &&
			go run ./tools/vsix -bin "$work/bins" -o "$work/board.vsix")
		"$editor" --user-data-dir "$work/data" --extensions-dir "$work/ext" --install-extension "$work/board.vsix"
	fi

	sh "$here/seed.sh" "$project"

	mkdir -p "$work/claude" "$work/data/Machine"
	cat > "$work/claude/settings.json" <<EOF
{
  "model": "sonnet",
  "permissions": {
    "defaultMode": "acceptEdits",
    "allow": ["Bash(kanbanzai:*)", "Bash(git:*)", "Bash(node:*)", "Bash(npm:*)", "Bash(ls:*)", "Bash(cat:*)", "Bash(mv:*)", "Bash(cd:*)", "Bash(mkdir:*)"]
  },
  "autoUpdates": false
}
EOF
	cat > "$work/claude/.claude.json" <<EOF
{
  "hasCompletedOnboarding": true,
  "theme": "dark",
  "autoUpdates": false,
  "autoInstallIdeExtension": false,
  "projects": { "$project": { "hasTrustDialogAccepted": true, "hasCompletedProjectOnboarding": true } }
}
EOF
	cat > "$work/data/Machine/settings.json" <<EOF
{
  "workbench.startupEditor": "none",
  "workbench.tips.enabled": false,
  "security.workspace.trust.enabled": false,
  "chat.disableAIFeatures": true,
  "workbench.secondarySideBar.defaultVisibility": "hidden",
  "workbench.colorTheme": "Default Dark Modern",
  "window.commandCenter": false,
  "telemetry.telemetryLevel": "off",
  "update.mode": "none",
  "extensions.autoUpdate": false,
  "workbench.activityBar.location": "hidden",
  "breadcrumbs.enabled": false,
  "editor.minimap.enabled": false,
  "editor.fontSize": 14,
  "terminal.integrated.fontSize": 14,
  "terminal.integrated.defaultProfile.linux": "demo",
  "terminal.integrated.profiles.linux": {
    "demo": { "path": "bash", "args": ["--noprofile", "--norc"], "env": { "PS1": "\$ " } }
  },
  "git.openRepositoryInParentFolders": "never",
  "git.autofetch": false
}
EOF
	# a normal session, not a child of this one: the CLAUDECODE* markers are dropped; the editor is on loopback only
	env -u CLAUDECODE -u CLAUDE_CODE_CHILD_SESSION -u CLAUDE_CODE_ENTRYPOINT -u CLAUDE_CODE_EXECPATH \
		-u CLAUDE_CODE_SESSION_ID -u CLAUDE_CODE_MESSAGING_SOCKET -u CLAUDE_CODE_MESSAGING_TOKEN -u CLAUDE_PID \
		DISABLE_AUTOUPDATER=1 CLAUDE_CONFIG_DIR="$work/claude" \
		nohup "$editor" --host 127.0.0.1 --port "$port" --without-connection-token \
		--user-data-dir "$work/data" --extensions-dir "$work/ext" > "$work/editor.log" 2>&1 &
	echo $! > "$work/editor.pid"
	i=0
	until curl -fs -o /dev/null "http://127.0.0.1:$port/"; do
		i=$((i + 1))
		if [ "$i" -gt 40 ]; then
			echo "demo.sh: the editor did not answer on 127.0.0.1:$port (see $work/editor.log)" >&2
			exit 1
		fi
		sleep 0.5
	done
	echo "editor: http://127.0.0.1:$port/?folder=$project"
	echo "project: $project"
	echo "claude config: $work/claude"
}

clean() {
	stop
	# the demo project's worktrees live outside it: list them, then remove each literal path
	if [ -d "$project/.git" ]; then
		git -C "$project" worktree list --porcelain | sed -n 's/^worktree //p' | while read -r path; do
			if [ -n "$path" ] && [ "$path" != "$project" ]; then
				rm -rf "$path"
			fi
		done
	fi
	if [ -n "$project" ] && [ -d "$project" ]; then
		rm -rf "$project"
	fi
	if [ -n "$work" ] && [ -d "$work" ]; then
		rm -rf "$work"
	fi
	echo "clean"
}

[ $# -ge 1 ] || usage
cmd=$1
shift
case "$cmd" in
prepare) prepare ;;
stop) stop ;;
clean) clean ;;
assemble) exec python3 -I "$here/assemble.py" "$@" ;;
verify) exec python3 -I "$here/verify.py" "$@" ;;
step) exec python3 -I "$here/step.py" "$@" ;;
*) usage ;;
esac
