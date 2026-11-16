# Contributing and developing

For people who build Kanbanzai from a checkout. A user installing it needs none of this: see the [README](../README.md).

## What the repository holds

```
cmd/kanbanzai              the command's entry point
internal/                  the core: config, brief, board, claim, move, number, step, context, check, sessions, wip, flow, cli
go.mod                     the Go module (Go 1.24+, standard library only, no require)
assets.go                  the embedded shipped defaults and card template
defaults/config.toml       the shipped workflow (embedded in the binary)
templates/card.md          the brief template (embedded in the binary)
templates/{task,bug,change}.md, schemas.example.toml   opt-in per-type examples (embedded; see [types.md](types.md))
skills/                    setup, workflow, breakdown, triage, design, do, next, answer, wip, audit, cleanup, build, check
.claude-plugin/            Claude Code: plugin.json, and marketplace.json (this repo is its own marketplace)
hooks/                     Claude Code: the heartbeat after every tool call, the board's context at session start
opencode/                  OpenCode (V2): the same skills, heartbeat, context, and the command on the shell's PATH
board-view/                a read-only board in a VSCodium/VS Code tab, with each claimed card's session state
docs/                      the guides the README links; tests keep their links, keys and commands true
icon/                      the Kanbanzai mark's SVG source
tools/icon/                the stdlib renderer for the plugin and extension PNGs
tools/vsix/                the .vsix packer
tools/release/             the per-OS/arch release builder and version check
LICENSE                    MIT
CHANGELOG.md               what each release changed
SECURITY.md                how to report a vulnerability
SUPPORT.md                 where to ask a question
```

## Build, test and package

```bash
go build -o bin/kanbanzai ./cmd/kanbanzai   # a local build, ignored by git
go test ./...
go vet ./...
node --test board-view/*.test.mjs opencode/*.test.mjs
go run ./tools/icon                        # icon/kanbanzai.svg → the plugin and extension PNGs
go run ./tools/icon -check                 # prove the committed PNGs are the source's render
go run ./tools/vsix -o /tmp/kanbanzai-board.vsix
go run ./tools/packageset                  # which Checked cards one package run includes, and why not the rest
go run ./tools/release -check              # the manifests and CHANGELOG.md agree
go run ./tools/release -version v0.0.0-test -out /tmp/kanbanzai-dist
KANBANZAI_PLUGIN_SMOKE=1 go test ./tools/release -run TestThePackagedPluginInstallsInAFreshProject -v
KANBANZAI_PLUGIN_SMOKE=1 go test ./tools/vsix -run TestHostsLoadTheBundledPluginsFromTheVSIX -v   # needs claude and opencode
```

The command is `cmd/kanbanzai` over `internal/`; the Go tests sit beside the code. The release workflow builds the
five archives from a tag, and `tools/vsix` builds the editor package. CI validates the plugin and marketplace with
`claude plugin validate --strict`, installs a packaged copy into a throwaway project, and checks that the manifests,
`CHANGELOG.md` and the version agree; a tag runs the same version check before it builds. The release check also
rejects apparent personal home paths and session IDs (an OpenCode `ses_…` or a Claude Code `claude:<uuid>`) in
`.kanbanzai/work/` when that directory is present, and in every other shipped text file (`.go`, `.mjs`, `.js`,
`.json`, `.md`, `.toml`, `.yml`, `.sh`): marketplace installs include repository records, and the source bundle
includes the tests. It also rejects a tracked settings file from one machine (`settings.local.json`,
`scheduled_tasks.lock`, a `*.openchamber.backup`) and an absolute plugin path in `.opencode/opencode.json`. In a git
checkout it reads the tracked files, so your own ignored files do not fail it; in an export with no `.git` it walks
the tree. Fixtures use the fictional forms: a user called `example` (`/home/example/`, `/Users/example/`,
`C:\Users\example\`, and `example2` when a test needs two), the all-zero Claude Code session id
(`00000000-0000-4000-8000-000000000001`) and `ses_example…` for OpenCode. A test that must show a bad value builds
it from parts, as `tools/release/privacy_check_test.go` does. `/tmp/kanbanzai-…` scratch paths are fine. The package
step also runs `go run ./tools/release -scan <folder>` on each unpacked artifact, binaries included. The guard is a
targeted check, not a general secret scanner.

This repository's package skill (`.claude/skills/kanbanzai-package/`) packages every Checked card that is ready in
one run: one claim per card, one build from the main commit, and a record on each card naming the cards packaged with
it. `tools/packageset` chooses the set from the board and git, read-only. A card it is unsure about is left out and
reported, never asked about, and a left-out card whose code is already merged is reported as in the artifact but not
packaged. Eligible cards may be free or carry the caller's live claim; the selector checks claim status through the
core and excludes expired or uncertain claims. The package step reclaims or renews each selected card before starting,
so a selection does not override a competing claim. Releasing the packaged cards stays one card and one owner approval
at a time.
