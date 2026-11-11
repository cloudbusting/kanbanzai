# W-24: Design the Kanbanzai icon and export the plugin and extension PNGs

- **Stage:** qa-approved
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** release: v0.6.0 after the owner approves, with the plugin icon as its first submission
- **Where:** worktree W-24
- **Held:** waiting: the owner's installed-.vsix look and release approval

## Goal

The owner asked: "Triage a work item to design one with SVG." The owner approved the mark (A, 2026-11-11): a K of
three cards on an indigo tile — a white stem and upper arm, and an amber lower arm, the card being pulled. Give the
Claude Code plugin and the editor extension the icon they lack: one SVG source, a dependency-free export, and the
square PNGs (1024 for the plugin, 512 for the extension), so `claude plugin validate` loses its `ICON_MISSING`
warning and the extension package carries the mark with the next release.

## The mark

An indigo `#4F46E5` rounded tile (`rx` 232 in a 1024 viewBox) and a K of three cards. The stem and upper arm are
white; the lower arm is the amber `#F59E0B` card being pulled. The arms are drawn before the stem, so the stem stays
continuous and the amber arm reads as a card clipped behind it. Nothing is text, a gradient or a thin stroke: the
stroke is 156 of 1024 (2.4 px at 16 px), and the tile carries the mark on light and dark themes. The glyph is
496 × 560 inside the tile, centred, with 264 px of horizontal and 232 px of vertical padding.

The exact source, `icon/kanbanzai.svg`:

```svg
<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024" viewBox="0 0 1024 1024">
  <!-- The Kanbanzai mark: a K of three cards; the amber lower arm is the card being pulled. -->
  <rect width="1024" height="1024" rx="232" fill="#4F46E5"/>
  <line x1="342" y1="512" x2="682" y2="310" stroke="#FFFFFF" stroke-width="156" stroke-linecap="round"/>
  <line x1="342" y1="512" x2="682" y2="714" stroke="#F59E0B" stroke-width="156" stroke-linecap="round"/>
  <line x1="342" y1="300" x2="342" y2="724" stroke="#FFFFFF" stroke-width="156" stroke-linecap="round"/>
</svg>
```

The preview the owner approved is `bin/icon-looks.png` (A at 512/128/32/16 px on light and dark; untracked and safe
to delete).

## The icon's surfaces

Every extension and plugin surface that can show a mark gets it; the one that cannot is named so nobody looks for it.

- **Claude Code plugin** — `.claude-plugin/plugin.json` gets `"icon": "./.claude-plugin/icon.png"`, and the 1024 PNG
  sits at `.claude-plugin/icon.png`. The validator uses it as the plugin's directory-listing icon. Claude Code does
  not read the field at load time, and the marketplace entry (`.claude-plugin/marketplace.json`) cannot set it: the
  directory-listing fields belong only in `plugin.json`, and the validator rejects them in an entry.
- **Editor extension** — `board-view/package.json` gets `"icon": "icon.png"`, and the 512 PNG sits at
  `board-view/icon.png`. `showBoard` sets the board panel's `iconPath` to the same file, so the Board tab shows the
  mark as well as the Extensions view and the marketplace. `tools/vsix` packs it as `extension/icon.png`, declares
  the `png` content type and adds the `Microsoft.VisualStudio.Services.Icons.Default` asset.
- **OpenCode plugin** — OpenCode V2's plugin package metadata has no icon field, and this server plugin has no TUI
  part, so there is no surface to set. It stays unchanged.

## Acceptance checks

- [ ] `claude plugin validate .` reports no icon warning; `.claude-plugin/plugin.json` carries
      `"icon": "./.claude-plugin/icon.png"`; `.claude-plugin/icon.png` is a square 1024×1024 PNG under 2 MB. Proof:
      the command, the file, and `go test ./tools/icon`, which checks the committed PNG's dimensions and size.
- [ ] `go run ./tools/icon` regenerates `.claude-plugin/icon.png` and `board-view/icon.png` from
      `icon/kanbanzai.svg`; `go run ./tools/icon -check` exits 0; `go test ./tools/icon` fails before the source and
      tool exist and passes after. The renderer is Go's standard library only, with no module dependency.
- [ ] `board-view/package.json` carries `"icon": "icon.png"` and the board panel's `iconPath` names `icon.png`;
      `go run ./tools/vsix -o /tmp/kanbanzai-board.vsix` writes a `.vsix` containing `extension/icon.png`, the
      `Microsoft.VisualStudio.Services.Icons.Default` asset and a `png` content type; `go test ./tools/vsix` and
      `node --test board-view/*.test.mjs` pass; the packed `package.json` names the icon; the marketplace entry
      still validates with no icon field of its own.
- [ ] The mark reads at 16 px on light and dark themes; the owner has approved the look. The owner's manual check:
      install the built `.vsix`, open the editor's Extensions view, and see the Kanbanzai Board mark at 16–32 px;
      open the Board tab and see the same mark in the tab. The plugin's mark is its directory-listing icon in
      `plugin.json`; Claude Code does not show it in the local plugin list, so the validator and the portal's
      submission are its proof. The build cannot open the editor and reports the extension check as not run.
- [ ] The plugin's PNG is 1024 (within 512–2048) and the extension's is 512 (at least the marketplace's 128); both
      are square PNGs, and the source is in git. Proof: `go test ./tools/icon` and the committed files.

## Deploy

`package and release`: the icon ships in the Claude Code plugin (the repo's `.claude-plugin/icon.png`, named by
`.claude-plugin/plugin.json`) and in the editor `.vsix` (`tools/vsix` packs `board-view/icon.png`). The release
step attaches the `.vsix` and records the icon under the next version. The plugin's icon is the first submission, so
the approved mark is final before the portal's first save.

## Tasks

- [x] T1 — `icon/kanbanzai.svg`, `tools/icon/main.go`, `tools/icon/icon_test.go`: write the exact source above;
      write a strict stdlib SVG renderer for the `rect`/`line`/`circle` subset (hex fills, round caps, 4×4
      supersampling, `-in`, `-out`, `-size`, and a `-check` that re-renders and compares), which finds the repo
      root and writes `.claude-plugin/icon.png` at 1024 and `board-view/icon.png` at 512 by default. Write the test
      first: the renderer on the source, an unsupported `<path>` rejected, the two sizes. Check: `go test
      ./tools/icon` and `go run ./tools/icon`.
- [x] T2 — `.claude-plugin/icon.png`, `.claude-plugin/plugin.json`, `board-view/icon.png`,
      `tools/icon/committed_test.go`: generate the two PNGs with T1's command; name the plugin's icon in
      `.claude-plugin/plugin.json` as `"./.claude-plugin/icon.png"`; add the test that renders the source and
      compares the committed PNGs (pixels, not bytes), checks the plugin's 1024/512–2048/<2 MB and the extension's
      512, and runs the `-check`; run `claude plugin validate .`. Depends on T1. Check: `go test ./tools/icon`, `go
      run ./tools/icon -check`, `claude plugin validate .` (the icon warning is gone and the marketplace entry
      stays free of the field).
- [x] T3 — `board-view/package.json`, `board-view/extension.js`, `board-view/extension_output.test.mjs`,
      `board-view/README.md`, `tools/vsix/main.go`, `tools/vsix/vsix_test.go`: name the icon in the package; set
      the board panel's `iconPath` to it and prove it in the extension test; pack `icon.png`, declare the `png`
      content type and add the `Icons.Default` asset; test the packed file, the content type and the asset. Depends
      on T2's PNG for its end-to-end check; the code can be written in parallel with T2. Check: `node --test
      board-view/*.test.mjs`; `go test ./tools/vsix`; `go run ./tools/vsix -o /tmp/kanbanzai-board.vsix` then list
      the zip; the owner's install check (the Board tab and the Extensions view).
- [x] T4 — `README.md`: add `icon/` and `tools/icon/` to "What it ships" and `go run ./tools/icon` to
      "Development", with the source path and the two canonical outputs. Parallel with T2/T3. Check: the command in
      the README runs from the repo root and regenerates both PNGs.

T1 first. T2, T3 and T4 then run in parallel: T3's code and T4's docs do not share files with T2, though T3's
end-to-end check needs T2's PNG.

## Docs

- `README.md`: "What it ships" gains `icon/` and `tools/icon/`; "Development" gains `go run ./tools/icon` and the
  source path; the command regenerates both PNGs.
- `board-view/README.md`: one line in Install names the mark's source and the export command, and says the Board
  tab and the Extensions view show it.
- `board-view/package.json`: the `icon` field, `icon.png`; `board-view/extension.js` sets the board panel's
  `iconPath` to the same file.
- `.claude-plugin/plugin.json`: the `icon` field, `./.claude-plugin/icon.png`; this is the plugin's
  directory-listing icon.
- `.claude-plugin/marketplace.json`: unchanged; its plugin entry cannot carry the directory-listing icon.
- `CHANGELOG.md`: unchanged in this change; the release step records the icon under the next version, and the 0.6.0
  line that says there is no icon stays as history.

## Decisions

- Owner, 2026-11-11: approved A — the K of three cards, indigo `#4F46E5` tile, white K, amber `#F59E0B` lower arm
  (the card being pulled). The owner saw the 512/128/32/16 px preview on light and dark. The plugin's icon is the
  first submission and becomes the listing icon only once: the approved mark is final before the portal's first
  save.
- Engineer: the source is `icon/kanbanzai.svg` (1024 viewBox); the plugin's PNG is 1024 and its listing names it in
  `.claude-plugin/plugin.json`; the extension's PNG is 512, named by `board-view/package.json`, used as the board
  panel's `iconPath`, and packed by `tools/vsix` with the `Icons.Default` asset; the export is a new `tools/icon` in
  Go's standard library, with no
  module dependency and no network call; the PNGs are committed and the SVG is the truth. The marketplace entry
  cannot carry the icon; the OpenCode plugin has no icon surface and is unchanged.

## Progress

- 2026-11-11: captured from the owner's request during the W-20 release; W-20 ships without an icon and names this
  card.
- 2026-11-11: design: the owner approved A — the K of three cards on the indigo tile with the amber card; the source is icon/kanbanzai.svg, the export is a stdlib tools/icon, and the plugin 1024 / extension 512 PNGs are next.
- 2026-11-11: design: every icon surface is named — `plugin.json` `icon` for the plugin's directory listing; for the extension, `board-view/package.json` `icon`, the board panel's `iconPath` (the Board tab) and the `.vsix` `Icons.Default` asset; the marketplace entry cannot carry it and the OpenCode plugin has no icon surface.
- 2026-11-11: build: recovered the crashed session's T1 in worktree W-24 — the exact source and the stdlib renderer are green at 32/512/1024 and reject an unsupported <path>; both PNGs are written; T2 next.
- 2026-11-11: build: T1–T4 done — icon/kanbanzai.svg renders the plugin 1024 and extension 512 PNGs through the stdlib tools/icon; the board panel and .vsix carry the icon; docs name the export; all Go and Node suites plus claude plugin validate pass. The owner's installed-.vsix look check is not run.
- 2026-11-11: check: kanbanzai-qa pass — go test ./... and go vet clean, node 130 pass, hooks and JS syntax clean, claude plugin validate passes, the .vsix carries extension/icon.png with the Icons.Default asset and no bin.json, go run ./tools/icon -check says icon ok, audit and kanbanzai check pass; the owner's installed-.vsix look check remains.
- 2026-11-11: deploy-qa: packaged 0.6.0 at 53079ab — source bundle, five binaries and the .vsix; proofs pass (fresh project, claude validate/install, opencode, vsix icon); installed cloudbusting.kanbanzai-board@0.6.0 in the editor; reload and the owner's release approval remain.

## Out of scope

- A UI redesign or a change to the board's layout and colours.
- The W-20 Go port.
- The OpenCode plugin's UI: it has no icon surface; the mark is the same source, and that adapter is unchanged.
- The marketplace listing copy, the release version and the portal submission; the release step owns those.

## Risk

- The plugin's icon is the first submission and cannot change after the portal's first save (the validator's note).
  The owner's approval above is the gate; do not submit a different mark. The `plugin.json` field is the only place
  the directory listing reads it, and the marketplace entry cannot set it.
- Claude Code does not display the icon at load time; the plugin mark's first public view is the directory listing,
  so the validator and the portal's submission are its proof, not the local plugin list.
- A mark that stops reading at 16 px; the 2.4 px stroke and the 16 px preview are the evidence; the owner's
  installed-extension check is the final proof.
- A PNG outside the plugin's 512–2048 or over 2 MB, or an extension icon below 128; the 1024/512 export and the
  tests guard both.
- A new renderer that drifts from the SVG: the tool is strict (an unsupported element exits non-zero), and the
  committed PNGs are compared to a fresh render; the SVG is the only input.
- The `.vsix` packaging: the icon must be in the zip, the manifest and the packed `package.json`; the `tools/vsix`
  test checks all three, so a broken package fails before release.

## Comments

- 2026-11-11 — triage: the owner asked for an SVG; the plugin validator needs a square PNG, so design covers the SVG
  source and its PNG export.
- 2026-11-11 — opencode:ses_e36b7b9d0ffeXdaaSBSUdeCrq4: design: the owner approved A; the plugin's icon is the first
  submission and cannot change after the portal's first save. The source is `icon/kanbanzai.svg`; `go run
  ./tools/icon` (stdlib) writes the plugin 1024 and extension 512 PNGs.
- 2026-11-11 — opencode:ses_e36b7b9d0ffeXdaaSBSUdeCrq4: build handed over after this session's process crash. The
  new session (`opencode:ses_e36a55c45ffeCvsaDnPbJ1vdBr`) holds the claim and finished the build in
  `../kanbanzai-W-24` (branch W-24): `icon/kanbanzai.svg`, `tools/icon`, both PNGs, the plugin's `icon`, the
  extension's `iconPath`, `tools/vsix` and both READMEs. Verified green: `go test ./...`, `go vet ./...`,
  `node --test` 130 pass, the `.vsix` carries `extension/icon.png`, `claude plugin validate .` passes,
  `go run ./tools/icon -check` says `icon ok`, `kanbanzai check` passes. Nothing committed. Next: the claim owner
  reviews the diff, runs the owner's install check (Extensions view and Board tab; the plugin icon is its directory
  listing and first submission), then `kanbanzai move W-24 finish --next "check: …"` and the check step.
- 2026-11-11 — opencode:ses_e36a55c45ffeCvsaDnPbJ1vdBr: recovered after OpenChamber crashed
  ses_e36b7b9d0ffeXdaaSBSUdeCrq4 mid-T1. Its three T1 files were intact on disk; renamed `tools/icon`'s `readPNG`
  to `loadPNG` (it collided with the test helper) and moved the tile pixel check to 4,4 (the arms meet at the 32 px
  centre). T1 is green; T2 next.
- 2026-11-11 — opencode:ses_e36a55c45ffeCvsaDnPbJ1vdBr: the lost session was still live but paused on a question
  about concurrent edits; this recovery session finished T1–T4 and committed `16fea2a`. The renderer is
  `*image.NRGBA` and `-check` compares straight colours — do not go back to premultiplied `image.RGBA`. The owner's
  installed-`.vsix` look check is the only step not run.
- 2026-11-11 — opencode:ses_e36a55c45ffeCvsaDnPbJ1vdBr: deploy-qa: packaged 0.6.0 at main `53079ab` in
  `/tmp/kanbanzai-package-W-24-0.6.0` — source bundle with `icon/`, five binary archives and
  `kanbanzai-board-0.6.0.vsix`, all in `SHA256SUMS`. Fresh-project build/init/check/board JSON, `claude plugin
  validate` and a throwaway marketplace install, `node --check opencode`, and the vsix icon/manifest all pass.
  Installed `cloudbusting.kanbanzai-board@0.6.0` in the editor; its `icon.png` sha256 matches the artifact. Reload
  the window to see the Board tab and Extensions view. Release waits on the owner: version 0.6.0, MIT, publisher
  cloudbusting, and the plugin icon is the first submission (final).
