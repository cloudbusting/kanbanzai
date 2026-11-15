'use strict';
// TASK-70: an operating-system notification for the editor, kept free of the vscode module so node can test it.
// The editor has no API for one, so the extension runs the platform's own notifier: a fixed program, an argument array,
// never a shell. The text is data (an argument, or an environment variable on Windows), never part of a script.
// `createNotifier` is the seam the workspace calls and the tests stub: it never throws and says whether it delivered.

const TITLE_MAX = 120;
const BODY_MAX = 240;
const TIMEOUT_MS = 10000;

// The Windows script is fixed. It reads the text from the environment, escapes it as XML and shows a toast under
// PowerShell's own app id (a toast needs a registered one).
const WINDOWS_SCRIPT = [
  '$t = [System.Security.SecurityElement]::Escape($env:KANBANZAI_NOTIFY_TITLE)',
  '$b = [System.Security.SecurityElement]::Escape($env:KANBANZAI_NOTIFY_BODY)',
  '[void][Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime]',
  '[void][Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime]',
  '$x = New-Object Windows.Data.Xml.Dom.XmlDocument',
  '$x.LoadXml("<toast><visual><binding template=\'ToastGeneric\'><text>$t</text><text>$b</text></binding></visual></toast>")',
  '$n = New-Object Windows.UI.Notifications.ToastNotification($x)',
  '[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier(\'{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\\WindowsPowerShell\\v1.0\\powershell.exe\').Show($n)',
].join('; ');

// One line of plain text: no control character or newline, spaces collapsed, cut to `max` characters.
function clean(text, max) {
  const s = text === undefined || text === null ? '' : String(text);
  return s.replace(/[\u0000-\u001f\u007f-\u009f]+/g, ' ').replace(/\s+/g, ' ').trim().slice(0, max);
}

// { file, args, env?, windowsHide? } for the platform, or null for one with no notifier.
function build(platform, title, body) {
  const t = clean(title, TITLE_MAX);
  const b = clean(body, BODY_MAX);
  if (platform === 'linux') return { file: 'notify-send', args: ['--app-name=Kanbanzai', '--', t, b] };
  if (platform === 'darwin') {
    return {
      file: 'osascript',
      args: ['-e', 'on run argv', '-e', 'display notification (item 2 of argv) with title (item 1 of argv)',
        '-e', 'end run', '--', t, b],
    };
  }
  if (platform === 'win32') {
    return {
      file: 'powershell.exe',
      args: ['-NoProfile', '-NonInteractive', '-Command', WINDOWS_SCRIPT],
      env: { KANBANZAI_NOTIFY_TITLE: t, KANBANZAI_NOTIFY_BODY: b },
      windowsHide: true,
    };
  }
  return null;
}

const MAX_LINES = 3;

// The notification lines for the blocked, waiting-on-the-owner and interrupted events of one board read (the
// `level: 'blocked'` events of toasts.diffBoards): one line each up to three, then one summary per project.
function blockedLines(events) {
  const list = (Array.isArray(events) ? events : []).filter((e) => e && e.level === 'blocked' && e.headline);
  if (list.length <= MAX_LINES) return list.map((e) => (e.held ? `${e.headline} — ${e.held}` : String(e.headline)));
  const groups = new Map();
  for (const e of list) {
    const label = typeof e.projectLabel === 'string' ? e.projectLabel : '';
    groups.set(label, (groups.get(label) || 0) + 1);
  }
  return [...groups].map(([label, count]) => `${label ? `${label}: ` : ''}${count} cards need a look`);
}

// { send(title, body) -> Promise<boolean> }. `remote`: the extension host is on another machine, so a notice would
// appear there, not in front of the owner. The first failure of each kind is a warning in the Output channel, later
// ones are debug lines; a failure is never an error box.
function createNotifier({ execFile, platform, remote, log, env }) {
  const told = new Set();
  const say = (kind, text) => {
    const first = !told.has(kind);
    told.add(kind);
    const level = first ? (kind === 'remote' ? 'info' : 'warn') : 'debug';
    try { if (log && typeof log[level] === 'function') log[level](text); } catch (e) { /* the log never fails a notice */ }
  };
  async function send(title, body) {
    if (remote) { say('remote', 'no OS notification: this window is remote, so a notice would appear on the remote machine'); return false; }
    const command = build(platform, title, body);
    if (!command) { say('platform', `no OS notification: no notifier for ${platform}`); return false; }
    const options = { timeout: TIMEOUT_MS };
    if (command.env) options.env = { ...(env || process.env), ...command.env };
    if (command.windowsHide) options.windowsHide = true;
    return new Promise((resolve) => {
      let done = false;
      const finish = (ok) => { if (!done) { done = true; resolve(ok); } };
      try {
        execFile(command.file, command.args, options, (error, _stdout, stderr) => {
          if (!error) { finish(true); return; }
          const why = error.code === 'ENOENT' ? `${command.file} was not found`
            : `${command.file} failed: ${clean(stderr || error.message, 200)}`;
          say(error.code === 'ENOENT' ? 'missing' : 'failed', `no OS notification: ${why}`);
          finish(false);
        });
      } catch (e) {
        say('threw', `no OS notification: ${clean(e && e.message, 200)}`);
        finish(false);
      }
    });
  }
  return { send };
}

module.exports = { TITLE_MAX, BODY_MAX, TIMEOUT_MS, clean, build, blockedLines, createNotifier };
