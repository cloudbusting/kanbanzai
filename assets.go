// Package kanbanzai holds the assets embedded in the command.
//
// The shipped defaults and the card template travel inside the binary, so `kanbanzai init`, `kanbanzai check` and
// a project with no configuration work without a source tree beside them. A project overrides the config key by
// key and may name its own template with [project] template. The agent plugin assets are embedded for local
// installation; each harness still loads them as files.
package kanbanzai

import "embed"

// Files holds defaults/config.toml, templates/card.md and the opt-in per-type examples: templates/task.md, bug.md,
// change.md and schemas.example.toml. Epic has its own shipped template/schema; other examples are opt-in.
//
//go:embed defaults/config.toml presets templates/card.md templates/task.md templates/bug.md templates/change.md templates/epic.md templates/schemas.example.toml
//go:embed README.md .claude-plugin/plugin.json .claude-plugin/marketplace.json .claude-plugin/icon.png hooks/hooks.json hooks/heartbeat.sh hooks/session-start.sh opencode/server.js skills/*/SKILL.md
var Files embed.FS

// BoardFiles travels with the command; the HTTP host serves only its explicit
// browser asset allowlist, never arbitrary embedded files or project paths.
//
//go:embed board-view/render.js board-view/sessions.js board-view/toasts.js board-view/browser.js
var BoardFiles embed.FS
