# Old card ids and their new ones

The board moved from the `kanbanzai-board` branch to GitHub issues (CHANGE-97). A commit message, the changelog or a release note that names an old id means the card in the first column; the same text on the board now means the card in the second.

| Old id | New id | Title |
|---|---|---|
| W-01 | TASK-03 | Marketplace release foundation: licence, metadata, versioning and CI |
| W-02 | TASK-04 | Portable, packaged core: one installable kanbanzai command |
| W-03 | TASK-05 | Strict, versioned config schema with actionable validation and a doctor |
| W-04 | TASK-06 | Harden init: detect, plan, dry-run and leave a sound board |
| W-05 | TASK-07 | Agent-driven setup skill: /kanbanzai:setup |
| W-06 | TASK-08 | Workflow command: show, validate, plan, apply and export a step flow |
| W-07 | TASK-09 | Agent workflow skill: configure and reconfigure the step mapping and state flow |
| W-08 | TASK-10 | Make the shipped skills flow-agnostic and remove hardcoded workflow words |
| W-09 | TASK-11 | Issue types and global numbering: the model and migration policy |
| W-10 | TASK-12 | Typed IDs and one global ledger across the core |
| W-11 | TASK-13 | Type-aware brief templates and a configurable brief schema |
| W-12 | TASK-14 | Isolation modes: worktree, branch-only, shared checkout and external |
| W-13 | TASK-15 | Configurable delivery and gates: deploy-less close, approvals and release trains |
| W-14 | TASK-16 | Workflow presets and a fixture matrix for different delivery styles |
| W-15 | TASK-17 | Bundle both agent plugins with the editor extension |
| W-16 | TASK-18 | Board view: multi-project workspace selector, combined and separate boards |
| W-17 | TASK-19 | Board view marketplace packaging and install path |
| W-18 | TASK-20 | Marketplace documentation and host integration guide |
| W-19 | TASK-21 | Migrate a host project to typed IDs, the new schema and the configurable flow |
| W-20 | TASK-22 | Port the core to Go: one binary `kanbanzai` command and its tooling |
| W-21 | TASK-23 | Project skills for triage, design, build, QA, package and release |
| W-22 | TASK-24 | The installed command follows the release (slice 1 of automatic upgrade) |
| W-23 | TASK-25 | The OpenCode plugin registers commands |
| W-24 | TASK-26 | Design the Kanbanzai icon and export the plugin and extension PNGs |
| W-25 | TASK-27 | Pluggable backing stores for work items, with the in-repo store, JIRA and GitHub Issues as plugins and per-plugin filters |
| W-26 | TASK-28 | The claim test is a wall-clock time bomb, failing CI on every platform |
| W-27 | TASK-29 | Remove the /obs skill now that triage is the front door |
| W-28 | TASK-30 | Default worktree path `../worktrees/<project-dir-name>/{id}` |
| W-29 | TASK-31 | CI runs on branch pushes and pull requests only, not on every tag |
| W-30 | TASK-32 | Breakdown command and skill: turn a big requirement into triaged, sequenced cards by asking questions |
| W-31 | TASK-33 | Release and package skills describe the release as it was really done in 0.7.1 |
| W-32 | TASK-34 | /kanbanzai:setup knows about init --plugins and the Init/Apply pending plan |
| W-33 | TASK-35 | Closed batch and card records carry local paths and session ids into the public plugin |
| W-34 | TASK-36 | The command and the extension migrate a host's config and cards on a release (slice 2 of automatic upgrade) |
| W-35 | TASK-37 | Close all board toasts |
| W-36 | TASK-38 | The 🔒 on a gated card shows a table of the needs that fail the gate |
| W-37 | TASK-39 | Make workflows without Doing columns progress safely |
| W-38 | TASK-40 | Refuse starting a step the card skips |
| W-39 | TASK-41 | Detangle blocked cards |
| W-40 | TASK-42 | Pause a batch card on its question and answer it later |
| W-41 | TASK-43 | Typed IDs by default, with legacy W cards and a seeded global counter |
| W-42 | TASK-44 | Shipping a card with a package installs the released .vsix in the editor and tells the owner to reload |
| BUG-44 | BUG-45 | Allow board-only main checkout changes during batch audit |
| TASK-45 | TASK-46 | Keep /do work in context before pulling a new card |
| BUG-46 | BUG-47 | Typed cards missing from the board view |
| CHANGE-47 | CHANGE-48 | Early close with a transition reason |
| BUG-48 | BUG-49 | Report the release version after go install |
| BUG-49 | BUG-50 | Interrupted work audit blocks its own recovery |
| BUG-50 | BUG-51 | Keep /do progressing beyond each completed step |
| CHANGE-51 | CHANGE-52 | Allow safe integration with concurrent board changes |
| BUG-52 | BUG-53 | Avoid unnecessary question handoff in the main session |
| CHANGE-53 | CHANGE-54 | Package ready cards together |
| BUG-54 | BUG-55 | Resume safely after an audit stop without creating a claim deadlock |
| BUG-55 | BUG-56 | Investigate closed-card worktree audit findings |
| TASK-56 | TASK-57 | Browser board for terminal users |
| BUG-57 | BUG-58 | A worktree's card is taken from its branch name, so a multi-card release branch is misattributed |
| BUG-58 | BUG-59 | /do stops on a card the owner named because another card is first in the pull order |
| BUG-59 | BUG-60 | Do's carried claim excludes its card from package selection |
| TASK-60 | TASK-61 | Terminal board view |
| BUG-61 | BUG-62 | OpenCode migration test rejects a preceding version warning |
| BUG-62 | BUG-63 | Prevent /do stopping at unfinished build checkpoints |
| BUG-64 | BUG-64 | Board shows blocked, not asking, while the question tool is open |
| CHANGE-66 | CHANGE-65 | Install and setup Apply put the command on the user's PATH |
| BUG-67 | BUG-66 | Audit hold blocks recovery of the card's own work |
| BUG-68 | BUG-67 | The board shows a red box for a workspace folder that is not a project |
| CHANGE-69 | CHANGE-68 | Every command and skill that takes a card id also takes a bare number |
| TASK-70 | TASK-69 | OS notifications for an asking or blocked card, only when the window is not active |
| TASK-71 | TASK-70 | Scan everything packaged and published for personal paths, personal config and sensitive data |
| TASK-72 | TASK-71 | Publish the editor extension to an extension marketplace |
| TASK-73 | TASK-72 | A `team-pr` delivery preset for a team that ships through reviewed pull requests, with remote integration |
| TASK-74 | TASK-73 | Trusted publishing on Open VSX, and retire the marketplace PATs for kanbanzai and teamclaude-status |
| TASK-75 | TASK-74 | Rewrite the marketplace documentation |
| BUG-76 | BUG-75 | Board toasts and status changes steal the cursor focus while typing |
| CHANGE-77 | CHANGE-76 | Create and track work items and their state without committing to the protected main branch |
| BUG-78 | BUG-77 | A `work` folder outside the main checkout makes `move` (and three other commands) fail on a mangled path |
| BUG-79 | BUG-78 | The OpenCode plugin does not load in card worktrees: its reference reads the git-ignored `.kanbanzai/state/` |
| TASK-80 | TASK-79 | Board lane headers (the column group names) should be prominent |
| TASK-81 | TASK-80 | Board updates still steal focus from the active cursor (the prompt input, a terminal) |
| BUG-82 | BUG-81 | `plugins link` refuses an empty pointer file, which `doctor` says to fix with `plugins link` |
| TASK-83 | TASK-82 | A /kanbanzai:help cheat sheet and a help popup over the extension board |
| TASK-84 | TASK-83 | An animated GIF for the extension's marketplace listing, showing the board with an agent session at work in VSCodium |
| BUG-85 | BUG-84 | The editor's board does not live-update when the board lives on a board branch |
| EPIC-86 | EPIC-85 | Pluggable backing stores: plugin model, JIRA and GitHub plugins |
| CHANGE-87 | CHANGE-86 | Backing-store plugin model: the contract, the trust model and the in-repo store as the standard plugin |
| CHANGE-88 | CHANGE-87 | JIRA over twg backing-store plugin, with JIRA filters |
| CHANGE-89 | CHANGE-88 | JIRA over MCP backing-store plugin, with JIRA filters |
| CHANGE-90 | CHANGE-89 | GitHub (public) issues backing-store plugin, with GitHub filters |
| CHANGE-91 | CHANGE-90 | GitHub Enterprise Server issues backing-store plugin, with GitHub filters |
| BUG-92 | BUG-91 | workflow apply does not commit the briefs it moves on a board branch |
| BUG-93 | BUG-92 | a test extension in one editor profile replaces every profile's extension and the shared command |
| CHANGE-94 | CHANGE-93 | The board branch becomes this project's board home and the default for new projects on the files store |
| TASK-96 | TASK-94 | A skill and a command to remake the board demo GIF, so each new version of the tool can refresh the film |
| CHANGE-97 | CHANGE-95 | Migrate this project's board from the board branch to GitHub issues |
| CHORE-98 | CHORE-96 | Cut this project's board over from the board branch to GitHub issues |
