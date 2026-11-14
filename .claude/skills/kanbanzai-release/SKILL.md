---
name: kanbanzai-release
description: Publish a packaged, owner-approved Kanbanzai card as a versioned public release — settle the semver and licence, update the changelog and metadata, tag the plugin and repository, push to the public CloudBusting remote, publish the GitHub release and verify an install from the published artifacts. Main session only; never runs before the package step.
argument-hint: "<the card id or version>"
---

# Release

This is the card's shipping step (`ship` in the shipped flow). It takes a **QA-approved** card whose package
artifacts are built and proven, publishes them from one commit, and records the release. It never changes code
except the release metadata. Run `kanbanzai step ship` first; this skill is the step's skill in this repository.

Read the card, its **Comments**, `CHANGELOG.md` if it exists, and the package report. A package run may have
packaged several cards at one commit: the report and each card's Progress name the companions. Each card is still
released on its own approval, and releasing companions as one version is the owner's decision. The main session runs this
step; a subagent never pushes, tags or publishes.

## Board first

The card must be **QA-approved** with the owner's approval recorded for this version and this publication. If the
approval is not in the conversation, ask with the question tool, recommended option first. QA-approved is an owner
gate: record the approval with `kanbanzai gate pass <ID> --note "<the owner's words>"` (`kanbanzai gate status <ID>`
shows where it stands); `kanbanzai move` will not leave the column without it. Then move the card into **Shipping**
(`kanbanzai move <ID> start`) before the first push, and to **Done** only after the published artifact has been
verified.

A card whose brief makes it skip the deploy steps (`kanbanzai step --for <ID>` says `skip: … and closes`, in the shipped
flow a **Deploy** line of `None`) never reaches this step; it closes from Checked.

## 1. The public remote

The owner has allowed a public repository in the `CloudBusting` GitHub organisation. The expected remote is
`https://github.com/cloudbusting/kanbanzai`.

```bash
git remote -v
gh repo view cloudbusting/kanbanzai
```

- If `origin` already exists, it must be that repository. If it points anywhere else, stop and ask; never
  force-push a different history into it.
- If the repository does not exist and the owner has approved this release, create it once:

```bash
gh repo create cloudbusting/kanbanzai --public --source . --remote origin \
  --description "A kanban of work-item cards built for coding agents."
```

Do not create a second remote or a second repository. Do not push while `kanbanzai tree` says `blocked`
(a change outside the board, a path it cannot verify, a merge or rebase under way, the wrong branch). Card briefs that
other sessions are editing in the main checkout (`board-only`) do not stop a push, which sends commits and not the
working tree; they are never staged or committed with the release.

## 2. Settle the version and the licence

Read the current versions:

```bash
node -e 'const fs=require("fs"); console.log(JSON.parse(fs.readFileSync(".claude-plugin/plugin.json","utf8")).version)'
node -e 'const fs=require("fs"); console.log(JSON.parse(fs.readFileSync("board-view/package.json","utf8")).version)'
```

Use semantic versioning. A breaking public-contract change is major, a new capability is minor, a fix is patch.
The plugin and the extension are versioned separately; a release that publishes both records both. If the card
does not settle the bump, ask the owner with the recommended option first.

A public marketplace listing needs a real licence, author, repository, homepage and publisher. If the metadata
still says `UNLICENSED`, `private: true` or a placeholder publisher, stop and ask the owner before publishing. A
public source repository may be created without a marketplace listing, but it still carries the owner's choice of
licence; do not choose one for them.

## 3. Prepare the release commit

Update only the release's own files:

- `.claude-plugin/plugin.json` — version, author, repository, homepage, keywords, licence.
- `.claude-plugin/marketplace.json` — the owner's and plugin's metadata; the plugin entry's version must agree with
  `plugin.json` when it carries one.
- `board-view/package.json` — version, publisher, licence, repository, icon, categories; remove `private` only
  when the owner has chosen to publish it.
- `CHANGELOG.md` — create it if absent. One section per release:

```markdown
## [<version>] — YYYY-MM-DD

- <ID>: <what changed for a user>.
```

- `README.md` — the install line for the published marketplace, if it is not there already.
- The extension's icon or README only when the owner has supplied them.

Then validate before committing:

```bash
claude plugin validate .
go run ./tools/release -check
```

`-check` fails when the manifests and `CHANGELOG.md` disagree on the version; the workflow runs it again against the
tag (`-check -version v<version>`), so a mismatch you push is a failed release.

A warning about missing author or icon is a release finding, not noise. Fix what the card owns; ask the owner for
an identity or an icon they must supply.

The card's branch was already merged into the main branch by the check step. Make the release prep in the card's
worktree on a short release branch from main, commit it with the card id, and merge it to main as the check step
does: run `kanbanzai tree`, merge only when it does not say `blocked` (board-only brief changes by other sessions stay
as they are, and git keeps them), and stop on a refused merge or a conflict as `/check` does. Stage only the release's
files by path. The release worktree must be clean before tagging (`git -C <worktree> status --porcelain` is empty), and
the tag goes on that release commit; the main checkout need only not be `blocked`.

## 4. Push main alone and wait for CI

Push the main branch **before** any tag exists on the remote, and wait for CI to pass on all three systems:

```bash
git push origin main
gh run list --repo cloudbusting/kanbanzai --workflow CI --commit "$(git rev-parse HEAD)"
gh run watch <run-id> --repo cloudbusting/kanbanzai --exit-status
```

Every job of the `CI` run must be green for this commit: `Go (ubuntu-latest)`, `Go (macos-latest)` and
`Go (windows-latest)`, plus the adapters and plugin jobs. The reason is 0.7.0: its first attempt tagged a commit whose
CI had never run (42 commits were unpushed), the first CI run failed, and the tags could not be moved. A first CI run
of long-unpushed work is the risk this step removes. Run it even when everything is green locally; the local machine
is one operating system.

If CI is red, fix forward: a new commit on main, push main alone again, and wait again. Do not tag until the commit
you will tag has a green run on all three. If the fix changed the release's files, the tag goes on the fixed commit.

If the push is rejected, stop. Never force-push `main`.

## 5. Tag and push the tags

Only now, with the tagged commit green on Ubuntu, macOS and Windows, tag the plugin through its validator, so the tag
and the metadata cannot disagree:

```bash
claude plugin tag . --dry-run
claude plugin tag . --message "Kanbanzai <version>"
```

The tag is `kanbanzai--v<version>` and points at the release commit. The Go module needs the standard
`v<version>` tag on the same commit, so `go install github.com/cloudbusting/kanbanzai/cmd/kanbanzai@latest`
resolves the release:

```bash
git tag "v<version>" "$(git rev-parse "kanbanzai--v<version>^{commit}")"
```

Push the tags only after the tests, the package report and the green CI of step 4 are in hand:

```bash
git push origin "v<version>" "kanbanzai--v<version>"
```

The `v<version>` tag starts `.github/workflows/release.yml`: it runs the Go tests on Ubuntu, macOS and Windows,
validates the plugin, checks the manifests against the tag, builds the five archives and `SHA256SUMS`, builds the
`.vsix` with the command for all five platforms, and attaches them to the `v<version>` release. The
`kanbanzai--v<version>` tag is the marketplace tag and starts nothing. Never tag a different commit.

Expect one Release run and one CI run, not four (W-29). The `ci.yml` workflow runs on a push of `main` and on pull
requests, never on a tag; the `release.yml` workflow runs on a `v*` tag, never on `main`. A single
`git push origin main "v<version>" "kanbanzai--v<version>"` would start the Release workflow once and CI once; with
the order above they start from two pushes, still once each.

**A pushed tag is never moved or deleted.** If the Release workflow fails, or the release turns out to be wrong,
fix forward: a new commit, a new patch version and new tags. Do not force-push a tag, and do not re-run the workflow
against a moved one.

If a push is rejected, stop. Never force-push `main` or move a published tag.

## 6. Publish

### Collect the final artifacts from the tag

The package step proved the procedure at the pre-release commit. The version bump and changelog changed the tree
after that, so do not reuse the earlier directory. Wait for the Release workflow to finish
(`gh run watch --exit-status` on its run), then collect what it built and add the source bundle:

```bash
OUT="${TMPDIR:-/tmp}/kanbanzai-release-<ID>-<version>"
mkdir -p "$OUT"
gh release download "v<version>" --repo cloudbusting/kanbanzai --dir "$OUT" \
  --pattern '*.tar.gz' --pattern '*.zip' --pattern '*.vsix'
git archive --format=tar.gz --prefix="kanbanzai-<version>/" \
  -o "$OUT/kanbanzai-<version>.tar.gz" "v<version>" \
  .claude-plugin assets.go cmd internal go.mod defaults presets templates skills hooks opencode board-view tools docs README.md LICENSE CHANGELOG.md
(cd "$OUT" && sha256sum *.tar.gz *.zip *.vsix > SHA256SUMS && wc -l SHA256SUMS)
```

The `.vsix` is the workflow's, built with the command for five platforms. **Never upload a command-less `.vsix`**: a
`go run ./tools/vsix -o …` build without `-bin` is a dev build with no command, and uploading one with `--clobber`
would replace the real one. If you need your own copy of the `.vsix` (to compare it, or because the workflow did not
attach one), build it the way the workflow does, from the tagged commit and into a directory other than `$OUT`:

```bash
BINS="$OUT-bins" CHECK="$OUT-check"
go run ./tools/release -version "v<version>" -out "$CHECK" -bins "$BINS"
go run ./tools/vsix -bin "$BINS" -o "$CHECK/kanbanzai-board-<extension-version>.vsix"
```

`SHA256SUMS` must list seven files: the five archives, the `.vsix` and the source bundle. The workflow's
`SHA256SUMS` covers only the five archives, so this one replaces it. `git archive` fails on a path that is not in
the tag, so keep the path list to files that are present (`docs` is there because the README links `docs/setup.md`).
Run this from a clean checkout at the tagged commit. Verify the archive listing, and the `.vsix` listing and manifest
version (`extension/bin/<os>-<arch>/kanbanzai[.exe]` for all five platforms, no `extension/bin.json`), before
publishing; the package step's checks apply again to these files.

### GitHub release

The tag workflow creates the `v<version>` release with the five binary archives, the `.vsix` and a `SHA256SUMS` over
the archives only. Attach the source bundle, re-upload the regenerated `SHA256SUMS` over the workflow's, and write
the notes. The `.vsix` is already there; do not upload it again unless it is the one the workflow built:

```bash
gh release upload "v<version>" "$OUT/kanbanzai-<version>.tar.gz" "$OUT/SHA256SUMS" --clobber
gh release edit "v<version>" --notes-file <the release notes>
```

Write the notes from the changelog: what changed for a user, the artifacts and their versions, the commit, the
`go install` line, and anything that waits for a manual editor check.

### Claude Code marketplace

The repository is its own marketplace, so the published repository is the listing. Verify from a throwaway
configuration, not this machine's plugins:

```bash
C="$(mktemp -d)"
CLAUDE_CONFIG_DIR="$C" claude plugin marketplace add cloudbusting/kanbanzai
CLAUDE_CONFIG_DIR="$C" claude plugin install kanbanzai@kanbanzai
CLAUDE_CONFIG_DIR="$C" claude plugin list
CLAUDE_CONFIG_DIR="$C" claude plugin details kanbanzai@kanbanzai
```

Check that the installed plugin contains the skills, hooks, `cmd/kanbanzai`, `internal/`, `defaults/`, `templates/`
and `tools/`, and that no path points at this checkout. The `kanbanzai` command is installed separately on PATH.

### The editor extension

The `.vsix` carries the `kanbanzai` command for five platforms and no machine path, so it is a portable artifact;
the `kanbanzai.command` setting still overrides the bundled command. Say in the notes which platforms it covers.

Publish to a marketplace only when `board-view/package.json` has a real publisher and licence and the owner's
publishing token is in the environment. Publish the workflow's `.vsix` (the one with the command), never a build
without `-bin`:

```bash
npx @vscode/vsce publish --packagePath "$OUT/kanbanzai-board-<extension-version>.vsix"
```

Never put a token in the repository or a command that is logged. If there is no publisher or token, leave
the workflow's `.vsix` on the GitHub release; step 8 installs it in the owner's editor. Do not describe an unpublished extension as available in a marketplace.

### OpenCode

When `opencode/package.json` exists, publish the package the OpenCode project documents. Until then, the source
bundle on the GitHub release is the distribution: a user loads `opencode/` from the extracted bundle. Say which
one was published; never claim an npm package that does not exist.

## 7. Verify

From a clean machine state, not the build tree:

```bash
V="$(mktemp -d)" && cd "$V"
gh release download "v<version>" --repo cloudbusting/kanbanzai
sha256sum -c SHA256SUMS
unzip -l kanbanzai-board-<extension-version>.vsix | grep "extension/bin/"
tar -xzf "kanbanzai_<version>_linux_amd64.tar.gz"
./kanbanzai --version
./kanbanzai init
./kanbanzai check
./kanbanzai board --json | node -e 'JSON.parse(require("fs").readFileSync(0, "utf8"))'
GOBIN="$(mktemp -d)" go install github.com/cloudbusting/kanbanzai/cmd/kanbanzai@"v<version>"
"$GOBIN/kanbanzai" --version
```

For the plugin, the throwaway `CLAUDE_CONFIG_DIR` install above is the proof. For the extension, step 8 installs the
released `.vsix` in the owner's editor; a look at a fixture board in a clean editor profile stays the owner's manual
check, so name it.

## 8. Install the released `.vsix` in the editor

When the release carries a `.vsix` (it does whenever the card's Deploy is a package; a release with no extension
change attaches none, so install nothing and say so), install **the released file** in the editor that hosts this
session, so what the owner runs is what was published. Do it after step 7 and before closing. This step never stops
the release: whatever happens here, the release still closes (step 9) and the report says exactly what was done.

```bash
VSIX="$(mktemp -d)"
gh release download "v<version>" --repo cloudbusting/kanbanzai --dir "$VSIX" --pattern '*.vsix' --pattern SHA256SUMS
(cd "$VSIX" && sha256sum -c --ignore-missing SHA256SUMS)
unzip -l "$VSIX/kanbanzai-board-<extension-version>.vsix" | grep "extension/bin/"
codium --install-extension "$VSIX/kanbanzai-board-<extension-version>.vsix" --force
codium --list-extensions --show-versions | grep kanbanzai
```

- The download is untrusted data: it goes into its own new, empty directory, and the path is passed to each command
  as an argument. Do not run a tool or interpreter from inside it (`sha256sum -c` is the one place the directory is
  entered, and it reads only the two files just downloaded).
- Check the listing before installing: it must show the five bundled commands (`extension/bin/<os>-<arch>/kanbanzai`
  for linux and darwin on amd64 and arm64, and `windows-amd64/kanbanzai.exe`). Never install a command-less `.vsix`.
  Stop the install, not the release, if the checksum or the listing is wrong.
- `codium` is the editor's own CLI. In this sandbox it is the VSCodium server's remote-cli, on PATH inside an editor
  terminal; it installs into the host editor. If the CLI differs on this machine (`code`, `code-insiders`), use the
  editor's own and say which. Read the installed version back with `--list-extensions --show-versions`; it must be the
  released `<extension-version>`.
- No editor CLI on PATH, a failed checksum or listing, or a failed install: do not retry in a loop and do not stop
  the release. The release still closes, and the report says the install did not happen, why, and gives the owner
  the two lines to run: the `gh release download` above and `codium --install-extension <file> --force`.
- A release that publishes to a marketplace still installs the GitHub `.vsix`.
- Do not reload the window yourself. The report ends by asking the owner to run **Developer: Reload Window** so the new extension version and its bundled command are picked up.

## 9. Record and close

- The release notes and `CHANGELOG.md` name the version, the card, the artifacts and the commit.
- `README.md` has the published install command if the listing is live.
- Both tags point at the release commit; the `v<version>` release carries the five binaries, the `.vsix` with its
  command, the source bundle and a `SHA256SUMS` over all seven (`sha256sum -c` passes on a fresh download).
- The card moves to Done:
  `kanbanzai move <ID> done --done "YYYY-MM-DD — v<version>, <sha>, <release URL>"`.
- Release the claim and merge the card's branch. Then remove the card's workspace: its worktree and every branch it
  used, including the `release/<ID>` branch of the release prep. The owner's approval of this release is the approval
  for it, but prove it first, for each worktree: the card is Done and not an early close, `git -C <path> status
  --porcelain` is empty, and `git branch --merged <integration-branch>` lists the exact branch it is on. Remove the
  worktree (`git worktree remove <path>`) and each merged branch with `git branch -d`, never `-D` or `--force`. A
  failed proof stops at that worktree: keep it and say why in the report; `kanbanzai wip --audit` shows it as a
  warning or a hard finding. Never remove the main checkout or a path outside the worktrees folder. Report to the
  owner: what was published, where, the checksums, whether step 8 installed the `.vsix` (and which version `--list-extensions`
  read back), and what they must still try by hand (the clean-profile look at a fixture board). If the install did
  not happen, say so and why, with the lines to run. When the `.vsix` was installed, do not tell the owner to
  install it; end the report by asking them to run **Developer: Reload Window**.

## Rollback

A published tag and a marketplace version are effectively permanent. Never force-push or delete a published tag.
If the release is broken:

- before the tags are pushed: delete the local tags and fix the commit (a red CI run on main is fixed with a new
  commit, and main is pushed alone again);
- after the tags are pushed: fix forward with a new commit, a new patch version and new tags; the pushed tag stays
  where it is;
- after the GitHub release: mark it as a pre-release or edit its notes to say withdrawn, and keep the artifacts
  for diagnosis;
- after a marketplace listing: publish the next patch; do not try to rewrite the old version;
- keep the previous tag, commit and artifacts as the rollback reference.

## You never

Publish without the package report and the owner's approval, tag a commit whose CI has not passed on Ubuntu,
macOS and Windows, move a pushed tag, upload a command-less `.vsix`, tag from a dirty release worktree or while `kanbanzai tree` says `blocked`, push a different commit than the
one that was packaged, put a token or private path in the repository, or describe an unpublished artifact as
available.
