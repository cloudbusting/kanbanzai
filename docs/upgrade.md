# Upgrading

Kanbanzai has four parts that can each be a different release: the **command** (`kanbanzai`), the **Claude Code
plugin**, the **OpenCode plugin** and the **editor extension**. Keep them on one release. Nothing upgrades itself and
nothing is committed for you: an upgrade is a few commands, and each one that writes to your project shows a plan and
waits for you.

## 1. Upgrade the parts

| Part | How |
|---|---|
| The command | Download the new archive from [GitHub Releases](https://github.com/cloudbusting/kanbanzai/releases), verify it against `SHA256SUMS` and replace the file on your `PATH`; or `go install github.com/cloudbusting/kanbanzai/cmd/kanbanzai@latest`. Then `kanbanzai install` copies it to the per-user folder the plugins' hooks search, and `kanbanzai --version` says which release you have. |
| The editor extension | Install the new `.vsix` (**Extensions: Install from VSIX…**). It brings the matching command and refreshes the per-user copy (`kanbanzai install --refresh`) every time it starts. |
| The Claude Code plugin | Update it through Claude Code's plugin manager (`/plugin`). |
| The OpenCode plugin | Update the checkout or package your `.opencode/opencode.json` points at. |

If the plugin and the command are different releases, the first line of a session's context says so and names the fix.
`kanbanzai doctor` reports the running command's version and where `PATH` finds it ([troubleshooting.md](troubleshooting.md)).

## 2. Let the project catch up

A release that needs your project's files changed (today the config's `config_version`, tomorrow perhaps a card or a
folder) says so itself, and never changes them silently:

- the editor shows **Kanbanzai: Migrate** and offers Preview and Apply;
- a Claude Code or OpenCode session starts with a line telling it to preview, ask you and apply only on your approval;
- by hand, `kanbanzai migrate` previews the whole migration as one plan and prints its hash, and
  `kanbanzai migrate --apply --plan HASH` applies exactly that plan (journalled, backed up, checked and rolled back on
  any failure). `kanbanzai migrate --rollback` restores the files after a crash.

[migrate.md](migrate.md) has the contract. Nothing runs without your approval and nothing is committed. Review the
diff and commit it yourself.

## 3. The config version

New configs start with the root key `config_version = 1`. A project with no file uses the embedded version-1
defaults. An existing unversioned file is version 0: normal commands refuse it with an upgrade instruction, rather
than silently changing it. From either the main checkout or a worktree:

```bash
kanbanzai config upgrade           # preview; writes nothing
kanbanzai config upgrade --apply   # validate, back up config.toml.v0.bak, then write atomically
```

`config upgrade` is the config-only form of the migration in step 2.

The v0→v1 upgrade only adds or replaces the root version value; it preserves comments, formatting and settings.
It first validates the prospective config, including references. Unknown or invalid settings require a correction
before upgrading: none are dropped or renamed automatically. Apply refuses a symlink or a pre-existing backup,
and a repeated apply on version 1 changes nothing. Keep the backup until you have verified the installation; it
can contain private project settings. Do not edit the source concurrently with an upgrade. A newer version is
always refused: install a compatible newer binary, never lower the file's version to bypass validation.

Every table has an allowlist: root, `[project]`, `[claims]`, `[flow]`, each `[[columns]]`, each `[[types]]`, each `[schemas.<name>]`, and each
`[steps.<name>]` (step names remain user-defined). Errors identify the source file and key and suggest a repair or
the nearest valid key. Values are not coerced: strings, booleans, integers and lists of strings must have their
declared types. `digits` is an integer from 1 through 18; `wip_limit` and `stale_days` are positive integers;
`lease_minutes` and `move_grace_minutes` are positive finite numbers at most 153722867 minutes (the duration
limit). Zero is not an alias for a default. `started` accepts only `true`, `false` or `"claimed"`; `kind` accepts
`"open"`, `"parked"` or `"done"`. Existing custom-column behavior is retained: inherited steps with no destination
on that board are inactive and absent Doing columns are dropped. Referenced steps and pull-order columns must exist.
See `defaults/config.toml` for every key, optional value and merge rule.

To intentionally retain a setting from a future version, opt in by **exact unknown dotted path** at the root:

```toml
config_version = 1
config_allow_unknown = ["project.future_key", "extensions"]

[project]
future_key = "not used by this binary"
```

Unknown table paths may be allowlisted as a whole. Column keys use `columns.<setting>` (applies to all entries);
steps use `steps.<step-name>.<setting>`. No wildcards are accepted. An exception never bypasses a known key's type,
range, version or reference checks; it does not make the setting work. Doctor warns for every actually ignored
path. `config --json` adds `config_version` and `config_allow_unknown` alongside the existing resolved fields;
consumers should keep treating fields defensively and tolerate additive fields.
