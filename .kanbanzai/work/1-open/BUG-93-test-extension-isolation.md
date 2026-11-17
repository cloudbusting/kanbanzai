# BUG-93: a test extension in one editor profile replaces every profile's extension and the shared command

- **Stage:** checked
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** none
- **Next action:** package: kanbanzai-package builds the artifacts; then the owner's VSCodium check of a labelled package (acceptance checks 3 and 4)
- **Where:** worktree BUG-93

## Goal

Installing a test build of the board extension in a separate editor profile should leave every other profile, and the
command that the agents and the shell run, exactly as they were.

The owner asked "should the shared tools sit under a dir per profile?" and then "triage that isolation card"
(2026-11-17). That was after CHANGE-87's editor check, in a separate VSCodium profile, upgraded the main profile's
extension and the per-user `kanbanzai` used by every hook. This card covers the board extension and the packaging of
test builds (`tools/vsix` and the package step). It also touches the per-user install (`kanbanzai install --refresh`)
that the Claude Code and OpenCode adapters run.

## Reproduction

What happened on 2026-11-17 during CHANGE-87's check:

1. A test `.vsix` of the board extension was built from a card's branch with `go run ./tools/vsix -bin <dir>`, which
   bundles that branch's command. It carries the version in `board-view/package.json`, which is the current
   release's (0.27.0).
2. It was installed with **Install from VSIX…** in a separate VSCodium profile made for the test.
3. The main profile also ran the test build. VSCodium keeps one folder per extension id and version
   (`~/.vscodium-server/extensions/cloudbusting.kanbanzai-board-0.27.0`), shared by every profile, and the test
   package overwrote the release's folder.
4. On start-up the extension ran its refresh (`install --refresh`, `docs/editor.md` "Updates"). This replaced the
   per-user `~/.local/share/kanbanzai/bin/kanbanzai` (v0.27.0 → `v0.27.1-…e4444e7`) and `plugins/current` with the
   test build. The shell (through `~/.local/bin/kanbanzai`), the Claude Code and OpenCode hooks, and every editor
   profile then ran the card's unreleased code.
5. Reinstalling the release `.vsix` (rebuilt from the `v0.27.0` tag) and reloading restored all three. The per-user
   command and the extension's bundle were then byte-identical to the tag's build.

Possibly related, not yet confirmed: after the plugin bundle had been swapped twice, the resumed Claude Code session
in that work printed start-up context from the old v0.1.0 plugin in `~/.claude/plugins/cache/kanbanzai/kanbanzai/0.1.0`.

## Design

Two separate faults made one accident, so there are two fixes, and neither adds a setting or a prompt.

1. **The test package had the release's version**, so the editor wrote it into the release's folder
   (`cloudbusting.kanbanzai-board-<version>`, shared by every profile). `tools/vsix` gets `-label <name>`, which packs
   the extension as `<version>-<name>` (for example `0.27.0-change87`): its own folder, and the release's is never
   touched. The label goes into `extension.vsixmanifest` and the packed `package.json`; `board-view/package.json` in the
   tree is not edited.
2. **The start-up refresh wrote the shared per-user command and `plugins/current` from whatever bundle was running.**
   `refreshInstalled` (`board-view/init.js`) now runs the bundled command's `--version` first and refreshes only when it
   equals the extension's own version (`context.extension.packageJSON.version`, leading `v` ignored). Every release
   build satisfies that: `tools/release` stamps the tag, and its check keeps the manifests on the same number. A test
   build from a branch carries a pseudo-version or a labelled extension, so it skips, and writes one line to the Output
   channel saying why and how to refresh by hand (`kanbanzai install --refresh`, or **Kanbanzai: Apply**). A command
   that does not answer `--version` is treated as a mismatch.

Only the extension runs the refresh; the Claude Code and OpenCode adapters never call `install --refresh`, so they need
no change. **Kanbanzai: Apply** installs with the bundled command on purpose, and stays as it is: it is "the owner asked".

Rules check: no dependency, no network, no change to `board --json`, `config --json` or `install`'s exit codes and
messages (the command is untouched); the extension stays read-only and runs the same command with a different argument
list. Parity: nothing for the adapters to change, for the reason above.

## Acceptance checks

- [ ] `go run ./tools/vsix -label change87 -bin <bins> -o <file>` writes a `.vsix` whose `extension.vsixmanifest` identity
  and `extension/package.json` both say `<version>-change87`, and `board-view/package.json` is unchanged. Without
  `-label` the output is byte-for-byte what it was. A label that is not semver pre-release text (`a b`, `-x`, empty after
  trimming, `x..y`) is refused with exit 2 and writes nothing. (Go test in `tools/vsix/vsix_test.go`, red without the flag.)
- [ ] `refreshInstalled` with a bundled command whose `--version` differs from the extension's version does not run
  `install --refresh`, logs one line naming both versions and the manual command, and does not call `error`
  (no toast). With equal versions (`kanbanzai v0.27.0` and `0.27.0`) it runs it exactly as before; a failed `--version` skips
  and logs. (`board-view/init.test.mjs`, red before the change; `extension_init.test.mjs` proves the extension passes its
  own version through.)
- [ ] A test package built from a card's branch installs into its own extension folder and leaves the release's folder
  byte-for-byte unchanged. Proof by the owner, in VSCodium: install the release `.vsix`, `sha256sum` its folder, install
  the `-label` package in a second profile, and the release's folder hash is the same and a second folder
  `cloudbusting.kanbanzai-board-<version>-<label>` exists. **If VSCodium refuses the suffix, the build uses a fourth
  numeric part instead (`0.27.0.<n>`) and the owner is told before it ships.**
- [ ] Starting the editor with that package leaves `~/.local/share/kanbanzai/bin/kanbanzai` and `plugins/current`
  byte-identical (owner's check, same steps; the Output channel says the refresh was skipped). Starting the release
  `.vsix` after a new release still refreshes them, as `docs/editor.md` says.
- [ ] `docs/editor.md` "Updates", `board-view/README.md` and the package skill say the rule, and `go test ./tools/vsix/...`,
  `node --test board-view/` and `kanbanzai check` pass.

## Deploy

`package and release`: the board extension's refresh and the packaging tool ship in a release.

## Tasks

- [x] **1. The label** (parallel with 2). Owns `tools/vsix/main.go`, `tools/vsix/vsix_test.go`. Add `-label`; validate it
  as semver pre-release identifiers (`[0-9A-Za-z-]+` joined by single dots, no empty part, not starting with `-`);
  join with `-`, or with `.` when the version already has a pre-release part; rewrite the `version` field of the packed
  `package.json` (a targeted replacement, so the other fields and their order stay) and use the same string in the
  manifest and the printed line. Proof: the first check.
- [x] **2. The refresh rule** (parallel with 1). Owns `board-view/init.js`, `board-view/extension.js`,
  `board-view/init.test.mjs`, `board-view/extension_init.test.mjs`. Pass `version` into `refreshInstalled`; add the
  `--version` run (`runFile`, a short timeout) and the comparison; log the skip. Proof: the second check.
- [x] **3. Documents** (after 1 and 2). Owns `docs/editor.md`, `docs/upgrade.md`, `board-view/README.md`,
  `.claude/skills/kanbanzai-package/SKILL.md`, `CHANGELOG.md`. State the rule and that a test package is always built with
  `-label <card id>`; the package step's artifact `.vsix` stays unlabelled. Proof: the last check, plus the owner's
  VSCodium steps above, which the build cannot do.

## Docs

`docs/editor.md` ("Updates"), `docs/upgrade.md`, `board-view/README.md` (the `install --refresh` line),
the package skill's extension step, and `CHANGELOG.md` under Unreleased.

## Decisions

Settled with the owner, 2026-11-17:
- A test package gets its own version through `tools/vsix -label <name>` (`<release>-<name>`).
- The start-up refresh skips, with a line in the Output channel, when the bundled command is not the release the
  extension says it is. No prompt and no setting: a setting is machine-wide, so every profile would share it.
- One per-user command and one plugin bundle stay; they are not made per editor profile (the hooks, shell and OpenCode
  know nothing about profiles). The fix is that a test build never writes them.

Settled by the engineer: the comparison is made at runtime against `--version`, not by a marker file in the bundle, so no
packaging step can forget it. The stale v0.1.0 plugin context on resume is **not** here: it is Claude Code's own plugin
cache, and nothing in this change touches it. It should be triaged on its own card once reproduced.

## Progress

- 2026-11-17: triaged at the owner's request, from what CHANGE-87's editor check did to the main profile.
- 2026-11-17: designed; the owner chose the skip rule, the `-label` version and one shared command.
- 2026-11-17: built in worktree BUG-93 (wip commit on branch BUG-93): tasks 1-3, vsix Go tests and 425 extension tests green; the owner's VSCodium steps (checks 3 and 4) are not run.
- 2026-11-17: checked: go test ./... (after merging main for CHANGE-87's 9884d60, which fixed the store boundary test that was red on main, not on this card), go vet, 425 extension and 13 OpenCode tests, hook and JS syntax, plugin validate, config and the five steps resolve, vsix dry run (plain 0.27.0; `-label bug93` 0.27.0-bug93 in package.json and manifest); review of the diff found nothing. The owner's VSCodium steps (acceptance checks 3 and 4) are the one manual item.

## Out of scope

- Store plugins (CHANGE-87).
- How Claude Code itself caches plugins, beyond what Kanbanzai's install writes.

## Risk

A refresh that never runs leaves agents on an old command after a real update. The fix must still bring a release
update to the hooks and OpenCode, which is what the refresh exists for.

## Comments
