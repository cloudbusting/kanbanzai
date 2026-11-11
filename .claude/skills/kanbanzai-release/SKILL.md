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
approval is not in the conversation, ask with the question tool, recommended option first. Move the card into
**Shipping** (`kanbanzai move <ID> start`) before the first push, and to **Done** only after the published
artifact has been verified.

A card whose **Deploy** line is `None` never reaches this step; it closes from Checked.

## 1. The public remote

The owner has allowed a public repository in the `CloudBusting` GitHub organisation. The expected remote is
`https://github.com/CloudBusting/kanbanzai`.

```bash
git remote -v
gh repo view CloudBusting/kanbanzai
```

- If `origin` already exists, it must be that repository. If it points anywhere else, stop and ask; never
  force-push a different history into it.
- If the repository does not exist and the owner has approved this release, create it once:

```bash
gh repo create CloudBusting/kanbanzai --public --source . --remote origin \
  --description "A kanban of work-item cards built for coding agents."
```

Do not create a second remote or a second repository. Do not push from a dirty tree.

## 2. Settle the version and the licence

Read the current versions:

```bash
python3 -m json.tool .claude-plugin/plugin.json | grep '"version"'
python3 -m json.tool board-view/package.json | grep '"version"'
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
```

A warning about missing author or icon is a release finding, not noise. Fix what the card owns; ask the owner for
an identity or an icon they must supply.

Commit the release prep on the card's branch with the card id, then merge it to the main branch as the check step
does. Stage only the release's files by path. The tree must be clean before tagging.

## 4. Tag and push

Tag the plugin through its validator, so the tag and the metadata cannot disagree:

```bash
claude plugin tag . --dry-run
claude plugin tag . --message "Kanbanzai <version>"
```

The tag is `kanbanzai--v<version>` and points at the release commit. If the owner wants one repository-wide tag
as well, create it on the same commit; never tag a different commit.

Push the main branch and the tag only after the tests and the package report are green:

```bash
git push origin main
git push origin kanbanzai--v<version>
```

If the push is rejected, stop. Never force-push `main` or move a published tag.

## 5. Publish

### GitHub release

Attach the package artifacts and the checksums:

```bash
gh release create "kanbanzai--v<version>" \
  "$OUT"/*.tar.gz "$OUT"/*.vsix "$OUT"/SHA256SUMS \
  --title "Kanbanzai <version>" \
  --notes-file <the release notes> \
  --verify-tag
```

Write the notes from the changelog: what changed for a user, the artifacts and their versions, the commit, and
anything that waits for a manual editor check.

### Claude Code marketplace

The repository is its own marketplace, so the published repository is the listing. Verify from a throwaway
configuration, not this machine's plugins:

```bash
C="$(mktemp -d)"
CLAUDE_CONFIG_DIR="$C" claude plugin marketplace add CloudBusting/kanbanzai
CLAUDE_CONFIG_DIR="$C" claude plugin install kanbanzai@kanbanzai
CLAUDE_CONFIG_DIR="$C" claude plugin list
CLAUDE_CONFIG_DIR="$C" claude plugin details kanbanzai@kanbanzai
```

Check that the installed plugin contains the skills, hooks, `bin/kanbanzai`, `kanbanzai/`, `defaults/` and
`templates/`, and that no path points at this checkout.

### The editor extension

Publish to a marketplace only when `board-view/package.json` has a real publisher, licence and the owner's
publishing token in the environment:

```bash
npx @vscode/vsce publish --packagePath "$OUT/kanbanzai-board-<extension-version>.vsix"
```

Never put a token in the repository or a command that is logged. If there is no publisher or token, attach the
`.vsix` to the GitHub release and record that the owner must install it manually. Do not describe an unpublished
extension as available in a marketplace.

### OpenCode

When `opencode/package.json` exists, publish the package the OpenCode project documents. Until then, the source
bundle on the GitHub release is the distribution: a user loads `opencode/` from the extracted bundle. Say which
one was published; never claim an npm package that does not exist.

## 6. Verify

From a clean machine state, not the build tree:

```bash
V="$(mktemp -d)" && cd "$V"
gh release download "kanbanzai--v<version>" --repo CloudBusting/kanbanzai
sha256sum -c SHA256SUMS
tar -xzf kanbanzai-<version>.tar.gz
./kanbanzai-<version>/bin/kanbanzai init
./kanbanzai-<version>/bin/kanbanzai check
./kanbanzai-<version>/bin/kanbanzai board --json | python3 -m json.tool >/dev/null
```

For the plugin, the throwaway `CLAUDE_CONFIG_DIR` install above is the proof. For the extension, the owner's
install into a clean editor profile and a look at a fixture board is the remaining manual check; name it.

## 7. Record and close

- The release notes and `CHANGELOG.md` name the version, the card, the artifacts and the commit.
- `README.md` has the published install command if the listing is live.
- The card moves to Done:
  `kanbanzai move <ID> done --done "YYYY-MM-DD — v<version>, <sha>, <release URL>"`.
- Release the claim, merge the card's branch, remove the worktree and branch, and report to the owner: what was
  published, where, the checksums, and what they must install or try by hand.

## Rollback

A published tag and a marketplace version are effectively permanent. Never force-push or delete a published tag.
If the release is broken:

- before any push: delete the local tag and fix the commit;
- after the GitHub release: mark it as a pre-release or edit its notes to say withdrawn, and keep the artifacts
  for diagnosis;
- after a marketplace listing: publish the next patch; do not try to rewrite the old version;
- keep the previous tag, commit and artifacts as the rollback reference.

## You never

Publish without the package report and the owner's approval, tag a dirty tree, push a different commit than the
one that was packaged, put a token or private path in the repository, or describe an unpublished artifact as
available.
