# Security

Kanbanzai is a local tool. The command, the Claude Code and OpenCode plugins and the board extension make no network
request, collect no telemetry and run only the `kanbanzai` command they are configured to run. The security surface
is the project's own files: the cards, the settings, the leases and the board's JSON.

## Report a vulnerability

Do not open a public issue for a vulnerability. Report it privately through GitHub Security Advisories:

<https://github.com/cloudbusting/kanbanzai/security/advisories/new>

Include:

- the version (`kanbanzai --version`), the harness or editor and its version, and the OS;
- what an attacker can do and what they need (a card, a settings file, a brief path, a session id);
- the smallest reproduction you have, and whether the issue is in the command, a plugin or the extension.

We aim to acknowledge a report within seven days, and to ship a fix or a mitigation in the next release. We will
credit you in the release notes if you want.

## Supported versions

Security fixes ship in a new release; install the latest release. There is no long-term-support line. If you run an
older version, upgrade before reporting if you can.
