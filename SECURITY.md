# Security

Kanbanzai is a local tool. The command makes no outbound network request; the plugins and editor extension make
no network request. There is no telemetry. The security surface is the project's own files and, only when
explicitly started with `kanbanzai board --web`, the read-only loopback browser host.

## Local browser host

The optional foreground HTTP server binds only to `127.0.0.1`, on a random port unless chosen explicitly. Its
cryptographically random per-run capability URL grants read access to board/session information and card briefs:
keep it private, including terminal logs and screenshots. No access URL is stored in the project, no request log
is written, and all responses use no-store and no-referrer headers. Ctrl-C stops the server. Closing a tab does not.

Exact listener Host and same-origin checks, a restrictive CSP, no CORS and GET/HEAD-only routes limit hostile
websites, rebinding and injection. Only allowlisted embedded assets and current board-listed Markdown briefs are
served; filesystem reads are confined to the configured cards folder, including symlinks. Briefs are literal text,
not active Markdown or HTML. There are no write, command-execution, arbitrary-file or directory-listing endpoints.
The browser makes only same-origin local requests, with no external script, font, image or service.

This is not authentication against another process running as your user. Do not publish the URL or expose/forward
the listener to an untrusted network. SSH forwarding is an explicit user action; use the same local/remote port
to preserve exact Host checks. Public hosting, reverse proxies and LAN listeners are unsupported.

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
