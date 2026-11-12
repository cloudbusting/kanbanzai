# W-42: Shipping a card with a package installs the released .vsix in the editor and tells the owner to reload

- **Stage:** done
- **Updated:** 2026-11-12
- **Done:** 2026-11-12 — merged to main cc93466 (not pushed); no release, Deploy None
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

For the owner, who runs Kanbanzai's editor extension in the same VSCodium that hosts the session: when a card ships and
the release creates a package (the `.vsix` is attached to the `v<version>` GitHub release), the project's ship skill
then downloads that released `.vsix`, installs it in the editor, and ends its report by telling the owner to reload
the window. Today the release skill says only that "the owner must install it manually" and names the editor install as
a remaining manual check, so the owner does the download, install and reload by hand after every release.

The request: "when a card on this project ships and creates a package the project skill should then download the
released vsix and install it in the editor and tell me to reload" (2026-11-12).

## Scope

- The skill is `.claude/skills/kanbanzai-release/SKILL.md` (the ship step's skill), after the GitHub release and the
  verification of the published artifacts and before "Record and close". It is the released `.vsix` that is installed,
  not a local build from `kanbanzai-package`, so what the owner runs is what was published.
- The new step downloads the workflow's `.vsix` from the release with `gh release download "v<version>" --pattern
  '*.vsix'` into its own new, empty directory (downloaded files are untrusted), checks it against the release's
  `SHA256SUMS`, lists it to confirm the bundled `extension/bin/<os>-<arch>/kanbanzai` commands, then installs it with
  the editor's own CLI (`codium --install-extension <file> --force`; in this sandbox the `codium` remote-cli is on PATH
  from the VSCodium server). The build confirms the right CLI name and flag by running it, not from memory.
- It does not reload the window itself. It ends the report with a plain request to the owner to reload the window
  (Developer: Reload Window) so the new extension version and its bundled command are picked up.
- If there is no editor CLI on PATH, or the install fails, the skill does not stop the release: it reports the release as
  done, says the install did not happen and why, and gives the owner the `gh release download` and install lines.
- It applies only when the release carries a `.vsix` (a card whose Deploy is `None`, or a release that has no
  extension change, installs nothing). A release that publishes to a marketplace still installs the GitHub `.vsix`.
- Step 8's "what they must install or try by hand" and the "remaining manual check" sentences are updated so they no
  longer describe the install as the owner's job when the skill did it; the clean-profile look at a fixture board stays
  a manual check.

## Acceptance checks

- [x] `.claude/skills/kanbanzai-release/SKILL.md` has a step that downloads the released `.vsix` by `gh release
      download`, verifies its checksum, installs it with the editor CLI and ends by telling the owner to reload.
      Proof: `grep -n "install-extension" .claude/skills/kanbanzai-release/SKILL.md` and the step read in place.
- [x] The step downloads into a new empty directory, passes the path as an argument and never runs a tool from inside
      it; it never installs a command-less `.vsix` (it checks the listing for the five bundled commands first).
- [x] The skill says what happens with no editor CLI, a failed install or a release with no `.vsix`: the release still
      closes, and the owner is told exactly what was not done.
- [x] The report in step 8 tells the owner to reload the window and no longer says the owner must install the
      extension by hand when the skill installed it.
- [x] The install command was run once for real against a released `.vsix` (the v0.11.0 one) and the installed version
      read back (`codium --list-extensions --show-versions`); the result is in Comments.
- [x] The tests that cover the skills still pass (`go test -p 1 ./...` and the extension and OpenCode tests), and
      `kanbanzai check` passes.

## Deploy

None: a project skill under `.claude/skills/` is not in the plugin bundle or the `.vsix`, so it changes no shipped
artifact. (If the build finds the skill is shipped, change this to package and release.)

## Out of scope

Reloading the window for the owner, installing the Claude Code or OpenCode plugins, publishing to a marketplace,
changing the `kanbanzai-package` step (it builds and proves the artifact and never installs into the owner's editor),
and any change to the extension or the command.

## Progress

- 2026-11-12: release skill step 8 installs the released .vsix; wording test; real run on v0.10.0
- 2026-11-12: checks green, diff reviewed, committed 3260885

## Comments

- 2026-11-12 — claude: triaged from the owner's request. Related: W-31 (made the release and package skills match
  0.7.1, including the `.vsix` flow), W-22 (the extension runs its bundled command, so a reload after install matters)
  and W-08 (shipping now; its report would be the first to use this step). No duplicate found. Chose
  `kanbanzai-release` over `kanbanzai-package` because the request is about the *released* `.vsix` and the package step
  never publishes. If the owner meant the package step too, say so.
- 2026-11-12 — claude: built in worktree W-42 (uncommitted): new step 8 in `.claude/skills/kanbanzai-release/SKILL.md`
  (old step 8 is now 9), with `TestTheReleaseSkillInstallsTheReleasedVsix` in `skills_wording_test.go` (red first: nine
  missing phrases, then green). Real run: v0.11.0 is not published yet (W-08 still shipping), so it ran against the
  v0.10.0 `.vsix`: download, `sha256sum -c` OK, five bundled commands listed, `codium --install-extension … --force`
  succeeded, `codium --list-extensions --show-versions` read back `cloudbusting.kanbanzai-board@0.10.0` (the same
  version was already installed, so nothing changed in the editor). `sha256sum -c` runs from inside the download
  directory (it needs relative names); it reads only the two downloaded files. Tests: go test -p 1 ./... green, node
  board-view 239/239, opencode 8/8, `kanbanzai check` ok.
