---
name: kanbanzai-package
description: Build and prove the distributable Kanbanzai artifacts for every checked card that is ready, in one run with one claim per card — the editor extension .vsix and the Claude Code and OpenCode plugin bundles — record their versions, checksums and the set of cards on each card, and install-test them in throwaway projects. A card it is unsure about is left out, never asked about. This is the QA step; it never publishes or tags.
argument-hint: "[card ids to restrict the run to]"
---

# Package

This is the cards' QA step (`deploy-qa` in the shipped flow). It takes **every Checked card that is ready**, builds
the files the release step publishes once from the main commit, proves them, and records on each included card what
was built and with which other cards. It never pushes, tags, creates a remote or publishes to a marketplace.

Run `kanbanzai step deploy-qa` first. When its `skill` is not `kanbanzai-package`, follow that skill instead.
`README.md` and each included card's brief and **Comments** are the background.

Artifacts are built from the main commit, so one build already contains every card merged there. Packaging a set is
one build and one proof run, with a claim, a move and a record for each card.

## Select the set

From the main checkout, on its main branch:

```bash
go run ./tools/packageset
```

It is read-only and prints JSON: `head`, `branch`, `clean`, `tree` (the core's `kanbanzai tree --json` verdict and
paths), `include` (card ids) and `exclude` (each with `id`,
`reason` and `in_head`). It includes a card only when the board and `kanbanzai step --for <ID> --json` say the card
is Checked, its next step is `deploy-qa`, it is not held, blocked or claimed, its Deploy is not None, and its branch
is merged into `HEAD`. It exits non-zero, including nothing, when the board cannot be read or the checkout is not on
the main branch: stop and report that.

**When unsure, leave the card out.** Never ask the owner whether to include a card and never include one the tool
excluded. If the invocation names card ids, package only those of them that `include` lists; the rest are left out.
A card the tool excludes is not claimed and not touched. If `include` is empty, report "nothing to package" with the
excluded cards and their reasons, and stop before building anything.

## Claim and start each card

For each id in `include`, one at a time:

1. `kanbanzai claim claim <ID>`. Exit 3 (another session holds it) or exit 4 (an expired claim) leaves the card out.
   Never use `--takeover` here: taking over needs the workspace inspection that "leave it out when unsure" does not
   allow.
2. Re-read `kanbanzai step --for <ID>`, because the claim may already have moved the card. If it is still Checked,
   run the start move that `kanbanzai workspace <ID>` prints in its `then:` line (`kanbanzai move <ID> start`, with
   `--where "<the card's workspace record>"` when the column needs a workspace). A refusal, such as the
   work-in-progress cap or a missing workspace, leaves the card out: release its claim and never force the move.

The included set is the cards that now hold this session's claim and sit in the step's Doing column. If none are
left, report "nothing to package" and stop. Every tool call renews a claim. If a later `move` is refused, report it and leave
that card truthful; never force it.

## Before building

- Each included card's checks are green and the main session has reviewed its diff.
- The check step merged each card's branch into the main branch, because this project has a deploy step. Package
  from the main checkout, not from a card's worktree: the release tags the main branch and the artifact must match it.
- **The main-checkout requirement lives in this bullet only.** The main checkout is on its main branch and need not
  be clean: other sessions keep editing card briefs there, and board files are not an input to any artifact. Read the
  selector's `tree.verdict` (it is `kanbanzai tree`'s, so there is one definition of "board-only"). Go ahead on
  `clean` or `board-only`. On `blocked` (a change outside the board, a path that cannot be verified, a merge or
  rebase under way, or the wrong branch) stop and report its `reasons`; never stash, stage, commit, reset or edit
  another session's files to clear it. Read it when the set is selected and again just before the build. Note the
  commit: `SHA=$(git rev-parse HEAD)`. Every artifact is built from an export of exactly that commit
  (`git archive "$SHA"`, below), never from the working tree, so what the main checkout holds cannot reach one.
  This session's own claims and moves edit only the included cards' briefs; commit them, if the project's rules
  allow, by path (`git commit --only -- <those briefs>`), never `-A`, leaving every other session's change alone.
- This package is the rehearsal and the proof of the procedure. If the release commit later differs because the
  version metadata changed, the release step rebuilds the final artifacts from its tag; the package step does not
  publish.
- The version metadata is what the release will publish:
  - `.claude-plugin/plugin.json` → `version`
  - `.claude-plugin/marketplace.json` → the plugin entry (a version here must agree with `plugin.json`)
  - `board-view/package.json` → `version`
- If a version is `0.0.0`, `UNLICENSED` where a licence is needed, or a publisher is a placeholder, build locally
  and record it as a blocker for the release. Do not invent a licence, publisher or marketplace identity.

## Build the artifacts

Use a fresh directory outside the repository, named for the first included card (the lead) and the version:

```bash
OUT="${TMPDIR:-/tmp}/kanbanzai-package-<lead ID>-<version>"
mkdir -p "$OUT"
```

### 1. The source/plugin bundle

The Claude Code marketplace, the OpenCode plugin and the command all live in this one tree. Archive the shipped
paths from the committed tree, never the working directory:

```bash
git archive --format=tar.gz --prefix="kanbanzai-<version>/" -o "$OUT/kanbanzai-<version>.tar.gz" "$SHA" \
  .claude-plugin assets.go cmd internal go.mod defaults presets templates skills hooks opencode board-view tools docs README.md LICENSE CHANGELOG.md
```

`docs` is in the list because the README links `docs/setup.md` and the other guides; a bundle without it has dead
links. `git archive` fails on a path that is not in the commit, so keep the list to paths that exist. Do not include `.kanbanzai/`, `.claude/`, `.git/`, pycache or state. The extension's own test files travel in the
source bundle and that is fine for a source release. Verify the listing with `tar -tzf`; every runtime file must be
there, and no state or secret may be.

### 2. The binary archives

The binaries are built from the commit too. Export it into a new, empty directory outside the repository and build
there; the export has no `.git`, and `tools/release` and `tools/vsix` read nothing from it:

```bash
SRC="${TMPDIR:-/tmp}/kanbanzai-package-<lead ID>-<version>-src"
mkdir -p "$SRC" && git archive "$SHA" | tar -x -C "$SRC"
cd "$SRC"
```

Rehearse the release builder from `$SRC`, and keep the raw binaries for the extension:

```bash
BINS="${TMPDIR:-/tmp}/kanbanzai-package-<lead ID>-<version>-bins"
go run ./tools/release -version "v<version>" -out "$OUT" -bins "$BINS"
```

It cross-compiles `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` and `windows/amd64` with
`CGO_ENABLED=0 -trimpath`, writes the five archives and a `SHA256SUMS` to `$OUT`, and leaves the raw commands in
`$BINS/<os>-<arch>/kanbanzai[.exe]` for the next step. The release workflow rebuilds the same set from the tag; this
run proves the procedure.

### 3. The editor extension

The `.vsix` carries the `kanbanzai` command for the five platforms, so the editor works with nothing installed.
Build it from the binaries of the step above, still from `$SRC`, never without `-bin`:

```bash
go run ./tools/vsix -bin "$BINS" -o "$OUT/kanbanzai-board-<extension-version>.vsix"
```

`go run ./tools/vsix -o …` with no `-bin` packs a dev build with no command. It is for working on the extension
only: do not package it, record it as an artifact or hand it to the release step. The release workflow builds the
real one the same way (`-bins`, then `-bin`) and attaches it to the release. The `.vsix` carries no machine path, so
it is a portable artifact; the `kanbanzai.command` setting still overrides the bundled command.

Verify the archive:

```bash
unzip -l "$OUT/kanbanzai-board-<extension-version>.vsix"
```

`extension/package.json`, `extension/README.md`, every runtime `.js` module and `extension.vsixmanifest` must be
present, with `extension/bin/<os>-<arch>/kanbanzai[.exe]` for all five platforms (`linux-amd64`, `linux-arm64`,
`darwin-amd64`, `darwin-arm64`, `windows-amd64`). There must be no `extension/bin.json`. The manifest version must
match `package.json`.

### 4. Checksums

```bash
cd "$OUT" && sha256sum *.tar.gz *.zip *.vsix > SHA256SUMS
```

The workflow's release carries a `SHA256SUMS` for the five binary archives only; this one covers the source bundle
and the `.vsix` as well. The release step regenerates the release's `SHA256SUMS` the same way, over every asset.

## Prove it

Run each proof from the artifact, not from the source tree where possible. The five binary archives and their
`SHA256SUMS` are in `$OUT`; the release workflow proves the same build from the tag.

### The command and a fresh project

```bash
T="$(mktemp -d)" && cd "$T" && git init -q -b main
tar -xzf "$OUT/kanbanzai-<version>.tar.gz"
(cd "$T/kanbanzai-<version>" && go build -trimpath -o "$T/kanbanzai" ./cmd/kanbanzai)
"$T/kanbanzai" init
"$T/kanbanzai" check
"$T/kanbanzai" board --json | node -e 'JSON.parse(require("fs").readFileSync(0, "utf8"))'
```

The bundle must work with no checkout beside it and no runtime prerequisite.

### The Claude Code plugin

```bash
claude plugin validate "$T/kanbanzai-<version>"
```

Then use a throwaway configuration so this machine's plugins are untouched:

```bash
C="$(mktemp -d)"
CLAUDE_CONFIG_DIR="$C" claude plugin marketplace add "$T/kanbanzai-<version>"
CLAUDE_CONFIG_DIR="$C" claude plugin install kanbanzai@kanbanzai
CLAUDE_CONFIG_DIR="$C" claude plugin list
```

The install must find `kanbanzai` and its skills from the extracted bundle. If the CLI cannot run non-interactively
in this environment, say which step is manual and leave it to the owner.

### The OpenCode plugin

The fake-harness test ran on the source in the check step. In the bundle, verify the entry point and its files:

```bash
node --check "$T/kanbanzai-<version>/opencode/server.js"
ls "$T/kanbanzai-<version>"/skills/*/SKILL.md
```

The bundle is archived from the same commit as the tested tree; say that, and do not hand-edit the extracted copy.

### The extension

The unit and render tests ran in the check step. The archive listing is the packaging proof. A real editor
install and a look at the Board tab is the owner's check; name the screen and the step in the report.

## Security and cleanliness

- `tar -tzf`, `unzip -l` and the five binary archives contain no `.git`, `.kanbanzai/state`, `.env`, key, token or
  local absolute path.
- No network call was added to build the artifacts. The commands above use only local tools.
- The artifacts were built from an export of the committed SHA, not from uncommitted work, and not from a working
  tree that other sessions are editing.

## Record and finish

### One report

```
Package: <lead ID> — <version> at <sha>
Cards: <included IDs, each with its claim and move result>
Left out: <ID — reason, one line each; "in the artifact, not packaged" where in_head is true>
Artifacts: <names, sizes, sha256 each, directory>
Proof: command <pass> · claude validate <pass/warnings> · opencode <pass> · extension listing <pass>
Known limits: <a manual editor check, a missing publisher/licence>
```

A card in `Left out` with `in_head: true` has its code in the artifact because its branch is merged into `<sha>`, but
it was not claimed, moved or recorded as packaged. Say that, and never say or imply the artifact was packaged for it.

### Each included card, when every proof passes

Append to the card's **Progress**, then move it on, then release its claim; the card waits for the owner:

```bash
kanbanzai move <ID> finish --next "release: <version> after the owner approves; package artifacts at <dir> from main <sha>" \
  --progress "Package QA passed at <sha> with <the other included IDs, or: alone>: artifacts <dir>, sha256 <each>"
kanbanzai claim release <ID>
```

Say what the owner must approve: the version, the licence and publisher, the public release, and any manual
editor check. The release step stays per card; releasing several of these as one version is the owner's decision.
Never publish, tag, push, create the public remote or attach an artifact to a release from this step.

### When the build or a proof fails

No card finishes: the artifact is one thing at one commit, and the flow has no In QA to Checked move. Each included
card stays In QA. For each one, edit the brief's **Next action** line and add a dated **Progress** line (never the
Stage line) to say `package failed at <sha>: <why>; rerun package`, then release its claim:

```bash
kanbanzai claim release <ID>
```

Releasing the claims leaves the cards truthfully interrupted, and the board pulls them first on the next run, which
resumes them. Report the failing command and its shortest output.
