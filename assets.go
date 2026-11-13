// Package kanbanzai holds the assets embedded in the command.
//
// The shipped defaults and the card template travel inside the binary, so `kanbanzai init`, `kanbanzai check` and
// a project with no configuration work without a source tree beside them. A project overrides the config key by
// key and may name its own template with [project] template. The skills are not embedded: each harness loads them
// as files, and the command only names them.
package kanbanzai

import "embed"

// Files holds defaults/config.toml, templates/card.md and the opt-in per-type examples: templates/task.md, bug.md,
// change.md and schemas.example.toml. The examples are never applied to a project unless it copies and names them.
//
//go:embed defaults/config.toml templates/card.md templates/task.md templates/bug.md templates/change.md templates/schemas.example.toml
var Files embed.FS
