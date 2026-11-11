# W-24: Design the Kanbanzai icon and export the plugin and extension PNGs

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design it: choose the mark and palette, the small-size treatment and the export, and get the owner's look before building
- **Where:** —

## Goal

The owner asked: "Triage a work item to design one with SVG." Kanbanzai has no icon: the Claude plugin validator warns
`ICON_MISSING` and the extension package has no icon. Design one mark as an SVG source, export the square PNGs the
plugin and extension require (512–2048 px, under 2 MB), and ship them with the next release.

## Acceptance checks

- [ ] `claude plugin validate .` no longer reports the icon warning; `.claude-plugin/icon.png` is a square PNG 512–2048
      px and under 2 MB.
- [ ] `board-view/package.json` names the icon and the packaged `.vsix` contains it.
- [ ] The SVG source is in git with a reproducible export command that regenerates the PNG(s).
- [ ] The mark reads at 16 px and on light and dark backgrounds; the owner has approved the look.

## Deploy

`package and release`: the icon ships in the plugin and extension artifacts.

## Tasks

- [ ] Design writes these once the owner approves the mark and the export.

## Docs

README and `board-view/README.md` if the identity is described; the plugin and extension metadata; the export command
in the repository.

## Decisions

- Owner, 2026-11-11: "Triage a work item to design one with SVG." The source is an SVG; the plugin/extension need square
  PNGs, so this card includes the export.

## Progress

- 2026-11-11: captured from the owner's request during the W-20 release; W-20 ships without an icon and names this
  card.

## Out of scope

- A UI redesign or a change to the board's layout and colours.
- The W-20 Go port.

## Risk

- A mark that does not read at 16 px or on both themes; a PNG outside the marketplace's size and format; the owner's
  identity and approval.

## Comments

- 2026-11-11 — triage: the owner asked for an SVG; the plugin validator needs a square PNG, so design covers the SVG
  source and its PNG export.
