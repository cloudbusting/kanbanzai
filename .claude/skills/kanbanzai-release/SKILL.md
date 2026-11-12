---
name: kanbanzai-release
description: Publish a packaged, owner-approved Kanbanzai card as a versioned public release — settle the semver and licence, update the changelog and metadata, tag the plugin and repository, push to the public CloudBusting remote, publish the GitHub release and verify an install from the published artifacts. Main session only; never runs before the package step.
argument-hint: "<the card id or version>"
---

# Release

This is the card's shipping step (`ship` in the shipped flow). It takes a **QA-approved** card whose package
artifacts are built and proven, publishes them from one commit, and records the release. It never changes code
except the release metadata. Run `kanbanzai step ship` first; this skill is the step's skill in this repository.

Read the card, its **Comments**, `CHANGELOG.md` if it exists, and the package report. The main session runs this
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

Do not create a second remote or a second repository. Do not push from a dirty tree.

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
does. Stage only the release's files by path. The tree must be clean before tagging, and the tag goes on that
release commit.

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
  .claude-plugin assets.go cmd internal go.mod defaults templates skills hooks opencode board-view tools docs README.md LICENSE CHANGELOG.md
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
the workflow's `.vsix` on the GitHub release, and record that the owner must install it manually. Do not describe an unpublished extension as available in a marketplace.

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

For the plugin, the throwaway `CLAUDE_CONFIG_DIR` install above is the proof. For the extension, the owner's
install into a clean editor profile and a look at a fixture board is the remaining manual check; name it.

## 8. Record and close

- The release notes and `CHANGELOG.md` name the version, the card, the artifacts and the commit.
- `README.md` has the published install command if the listing is live.
- Both tags point at the release commit; the `v<version>` release carries the five binaries, the `.vsix` with its
  command, the source bundle and a `SHA256SUMS` over all seven (`sha256sum -c` passes on a fresh download).
- The card moves to Done:
  `kanbanzai move <ID> done --done "YYYY-MM-DD — v<version>, <sha>, <release URL>"`.
- Release the claim, merge the card's branch, remove the worktree and branch, and report to the owner: what was
  published, where, the checksums, and what they must install or try by hand.

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
macOS and Windows, move a pushed tag, upload a command-less `.vsix`, tag a dirty tree, push a different commit than the
one that was packaged, put a token or private path in the repository, or describe an unpublished artifact as
available.
