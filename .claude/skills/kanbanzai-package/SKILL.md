---
name: kanbanzai-package
description: Build and prove the distributable Kanbanzai artifacts for a checked card — the editor extension .vsix and the Claude Code and OpenCode plugin bundles — record their versions and checksums, and install-test them in throwaway projects. This is the QA step; it never publishes or tags.
argument-hint: "<the card id>"
---

# Package

This is the card's QA step (`deploy-qa` in the shipped flow). It turns a Checked card's commit into the files the
release step publishes, proves they are complete and installable, and records what was built. It never pushes,
tags, creates a remote or publishes to a marketplace.

Run `kanbanzai step deploy-qa` first. When its `skill` is not `kanbanzai-package`, follow that skill instead.
Read the card's brief, its **Comments** and `README.md`.

## Board first

The card must be **Checked**, with this session's claim. Move it into the step's Doing column
(`kanbanzai move <ID> start`) before the first artifact, and to its Done column (`kanbanzai move <ID> finish`) when
every artifact and smoke test passes. A card whose **Deploy** line is `None` is not packaged; it closes from
Checked.

If the card says `package and release`, package it. If the change is docs, the board, tests or the project skills
only, stop and say the card's Deploy line is None.

## Before building

- The card's checks are green and the main session has reviewed the diff.
- The check step merged the card's branch into the main branch, because this project has a deploy step. Package
  from the main checkout at that merge commit, not from the card's worktree: the release tags the main branch and
  the artifact must match it.
- The main checkout is clean and on its main branch. Note the commit: `git rev-parse HEAD`.
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

Use a fresh directory outside the repository, named for the card and version:

```bash
OUT="${TMPDIR:-/tmp}/kanbanzai-package-<ID>-<version>"
mkdir -p "$OUT"
```

### 1. The source/plugin bundle

The Claude Code marketplace, the OpenCode plugin and the command all live in this one tree. Archive the shipped
paths from the committed tree, never the working directory:

```bash
git archive --format=tar.gz --prefix="kanbanzai-<version>/" -o "$OUT/kanbanzai-<version>.tar.gz" HEAD \
  .claude-plugin bin kanbanzai defaults templates skills hooks opencode board-view README.md
```

Do not include `.kanbanzai/`, `.claude/`, `.git/`, pycache or state. The extension's own test files travel in the
source bundle and that is fine for a source release. Verify the listing with `tar -tzf`; every runtime file must be
there, and no state or secret may be.

### 2. The editor extension

```bash
python3 -I board-view/install.py --vsix-only "$OUT/kanbanzai-board-<extension-version>.vsix"
```

The `.vsix` records this checkout's `bin/kanbanzai` in `extension/bin.json`. That is useful for a local install
and is not a portable marketplace artifact. Say so in the report; the release step must not attach a `.vsix` that
bakes in this machine unless the owner accepts it.

Verify the archive:

```bash
python3 -m zipfile -l "$OUT/kanbanzai-board-<extension-version>.vsix"
```

`extension/package.json`, `extension/README.md`, every runtime `.js` module, `extension.vsixmanifest` and
`extension/bin.json` must be present. The manifest version must match `package.json`.

### 3. Checksums

```bash
cd "$OUT" && sha256sum * > SHA256SUMS
```

## Prove it

Run each proof from the artifact, not from the source tree where possible.

### The command and a fresh project

```bash
T="$(mktemp -d)" && cd "$T" && git init -q -b main
tar -xzf "$OUT/kanbanzai-<version>.tar.gz"
"$T/kanbanzai-<version>/bin/kanbanzai" init
"$T/kanbanzai-<version>/bin/kanbanzai" check
"$T/kanbanzai-<version>/bin/kanbanzai" board --json | python3 -m json.tool >/dev/null
```

The bundle must work with no checkout beside it and no Python package installed.

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

- `tar -tzf` and the `.vsix` listing contain no `.git`, `.kanbanzai/state`, `__pycache__`, `.env`, key, token or
  local absolute path other than the known `bin.json` record.
- No network call was added to build the artifacts. The commands above use only local tools.
- The artifacts were built from the committed SHA, not from uncommitted work.

## Record and finish

Report:

```
Package: <ID> — <version> at <sha>
Artifacts: <names, sizes, sha256 each, directory>
Proof: command <pass> · claude validate <pass/warnings> · opencode <pass> · extension listing <pass>
Known limits: <the .vsix command path, a manual editor check, a missing publisher/licence>
```

Then move the card to the step's Done column with the release as the next action, and release the claim because
the card waits for the owner:

```bash
kanbanzai move <ID> finish --next "release: <version> after the owner approves"
kanbanzai claim release <ID>
```

Say what the owner must approve: the version, the licence and publisher, the public release, and any manual
editor check. Never publish, tag, push, create the public remote or attach an artifact to a release from this
step.
