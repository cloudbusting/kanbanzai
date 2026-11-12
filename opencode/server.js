// Kanbanzai for OpenCode (V2 server plugin). Load it from a project's `.opencode/opencode.json`:
//
//   { "plugins": ["<path to kanbanzai>/opencode"] }
//
// It does for OpenCode what `.claude-plugin/` and `hooks/` do for Claude Code, from the same files:
//   - registers Kanbanzai's skills (../skills/*/SKILL.md) under their own names, and the commands that run
//     them under their bare name and `kanbanzai:<name>`;
//   - puts `kanbanzai` (KANBANZAI_BIN, the per-user folder, PATH, or a local build) on the PATH of every shell OpenCode starts, and
//     labels the shell with its session (KANBANZAI_OWNER=opencode:<session id>) so claims are this session's;
//   - renews this session's claims after every tool call (the heartbeat);
//   - adds the board's standing context (`kanbanzai context`) to every model request;
//   - writes each session's state (busy, idle, or asking you) under the project's state folder, so the board view
//     shows an OpenCode card as it shows a Claude Code one (W-101).
//
// A plain object is exported (no import of @opencode/plugin), so the folder loads with nothing to install.
// Nothing here may throw into OpenCode: every step is guarded and fails quietly.

import fs from "node:fs"
import path from "node:path"
import { execFile } from "node:child_process"
import { fileURLToPath } from "node:url"

const HERE = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.resolve(HERE, "..")
const DEV_BIN = path.join(ROOT, "bin", process.platform === "win32" ? "kanbanzai.exe" : "kanbanzai")
const CONTEXT_TTL_MS = 10 * 60 * 1000
const SHELL_TOOLS = new Set(["bash", "shell"])
const MISSING = "Kanbanzai: the `kanbanzai` command is not installed. Install it from " +
  "https://github.com/cloudbusting/kanbanzai/releases and put it on PATH, or set KANBANZAI_BIN."
const debug = (what) => {
  if (process.env.KANBANZAI_DEBUG_LOG) try { fs.appendFileSync(process.env.KANBANZAI_DEBUG_LOG, JSON.stringify(what) + "\n") } catch {}
}

function isFile(candidate) {
  try {
    return fs.statSync(candidate).isFile()
  } catch {
    return false
  }
}

// The well-known per-user folder `kanbanzai install` puts the command in (the same rule as hooks/*.sh), or null when
// the user's home is unknown. Linux and macOS: $XDG_DATA_HOME/kanbanzai/bin when XDG_DATA_HOME is an absolute path,
// else <home>/.local/share/kanbanzai/bin. Windows: %LOCALAPPDATA%\kanbanzai\bin, else <profile>\AppData\Local\kanbanzai\bin.
// `home` is the user's home (HOME, or USERPROFILE on Windows); on Windows USERPROFILE in `env` fills in for it.
export function wellKnownDir(env, platform, home) {
  try {
    const e = env || {}
    if (platform === "win32") {
      const p = path.win32
      if (typeof e.LOCALAPPDATA === "string" && e.LOCALAPPDATA) return p.join(e.LOCALAPPDATA, "kanbanzai", "bin")
      const profile = home || e.USERPROFILE
      return profile ? p.join(profile, "AppData", "Local", "kanbanzai", "bin") : null
    }
    const p = path.posix
    if (typeof e.XDG_DATA_HOME === "string" && p.isAbsolute(e.XDG_DATA_HOME)) return p.join(e.XDG_DATA_HOME, "kanbanzai", "bin")
    return home ? p.join(home, ".local", "share", "kanbanzai", "bin") : null
  } catch {
    return null
  }
}

// The command: KANBANZAI_BIN (an explicit path, even a wrong one), then the well-known per-user folder (the copy the
// editor extension keeps at this plugin's release), then kanbanzai on PATH, then a local build.
export function resolveCommand() {
  const explicit = (process.env.KANBANZAI_BIN || "").trim()
  if (explicit) return { command: explicit, found: isFile(explicit) }
  const names = process.platform === "win32" ? ["kanbanzai.exe", "kanbanzai"] : ["kanbanzai"]
  const home = process.platform === "win32" ? process.env.USERPROFILE : process.env.HOME
  const dirs = []
  const known = wellKnownDir(process.env, process.platform, home || "")
  if (known) dirs.push(known)
  dirs.push(...(process.env.PATH || "").split(path.delimiter))
  for (const dir of dirs) {
    if (!dir) continue
    for (const name of names) {
      const candidate = path.join(dir, name)
      if (isFile(candidate)) return { command: candidate, found: true }
    }
  }
  if (isFile(DEV_BIN)) return { command: DEV_BIN, found: true }
  return { command: "kanbanzai", found: false }
}

// This plugin's release, from the manifest every release keeps in step (tools/release checks it), or "" when unreadable.
export function pluginVersion() {
  try {
    const found = JSON.parse(fs.readFileSync(path.join(ROOT, ".claude-plugin", "plugin.json"), "utf8")).version
    return typeof found === "string" ? found : ""
  } catch {
    return ""
  }
}

const resolved = resolveCommand()
const KANBANZAI = resolved.command
const COMMAND_FOUND = resolved.found
const COMMAND_DIR = path.isAbsolute(KANBANZAI) ? path.dirname(KANBANZAI) : null

function frontmatter(text) {
  const found = /^---\n([\s\S]*?)\n---\n?/.exec(text)
  const fields = {}
  if (!found) return fields
  for (const line of found[1].split("\n")) {
    const m = /^([A-Za-z-]+):\s*(.*)$/.exec(line)
    if (m) fields[m[1]] = m[2].replace(/^["']|["']$/g, "")
  }
  return fields
}

function stripFrontmatter(text) {
  const found = /^---\n[\s\S]*?\n---\n?/.exec(text)
  return found ? text.slice(found[0].length) : text
}

export function readSkills(dir = path.join(ROOT, "skills")) {
  const skills = []
  let names = []
  try {
    names = fs.readdirSync(dir).sort()
  } catch {
    return skills
  }
  for (const name of names) {
    const file = path.join(dir, name, "SKILL.md")
    let content
    try {
      content = fs.readFileSync(file, "utf8")
    } catch {
      continue
    }
    const front = frontmatter(content)
    skills.push({ id: front.name || name, name: front.name || name, description: front.description || "", path: file, content })
  }
  return skills
}

function run(args, cwd, env) {
  return new Promise((resolve) => {
    execFile(KANBANZAI, args, { cwd, env: { ...process.env, ...env }, timeout: 10_000 }, (err, stdout) =>
      resolve(err ? "" : String(stdout || "")),
    )
  })
}

// The standing context. The plugin's release goes with the request, so a command of another release says so in
// the first line; a command too old to know the flag fails on it, and the plain context is asked for instead.
function standingContext(cwd) {
  const version = pluginVersion()
  if (!version) return run(["context"], cwd, {})
  return new Promise((resolve) => {
    execFile(KANBANZAI, ["context", "--adapter-version", version], { cwd, env: process.env, timeout: 10_000 },
      (err, stdout) => (err ? resolve(run(["context"], cwd, {})) : resolve(String(stdout || ""))))
  })
}

export const owner = (sessionID) => (sessionID ? `opencode:${sessionID}` : undefined)

export default {
  id: "kanbanzai",
  async setup(ctx) {
    const directory = (ctx.location && ctx.location.directory) || process.cwd()

    try {
      await ctx.skill.transform((editor) => {
        for (const skill of readSkills()) {
          try {
            editor.add(skill)
          } catch (e) {
            debug({ skill: skill.id, error: String(e) })
          }
        }
        debug({ registered: editor.list().map((s) => s.id) })
      })
    } catch {}

    try {
      await ctx.command.transform((editor) => {
        const commands = []
        for (const skill of readSkills()) {
          for (const name of [skill.id, `kanbanzai:${skill.id}`]) {
            try {
              editor.add({
                name,
                description: skill.description,
                execute: async ({ sessionID, prompt, delivery }) => {
                  try {
                    const text = [stripFrontmatter(skill.content), prompt && prompt.text].filter(Boolean).join("\n\n")
                    await ctx.session.prompt({ ...prompt, sessionID, text, delivery })
                  } catch (e) {
                    debug({ command: name, error: String(e) })
                  }
                },
              })
              commands.push(name)
            } catch (e) {
              debug({ command: name, error: String(e) })
            }
          }
        }
        debug({ commands })
      })
    } catch {}

    // A shell's create event does not say which session started it, but the tool call before it does, with
    // the same command: remember the session by command, and label the shell when it is created.
    const pending = new Map()
    try {
      await ctx.tool.hook("execute.before", (event) => {
        const command = event && event.input && typeof event.input.command === "string" ? event.input.command : null
        if (command && SHELL_TOOLS.has(event.tool) && event.sessionID) pending.set(command, event.sessionID)
      })
    } catch {}
    try {
      await ctx.shell.hook("create.before", (event) => {
        if (!event || !event.env) return
        const current = event.env.PATH || process.env.PATH || ""
        if (COMMAND_DIR && !current.split(path.delimiter).includes(COMMAND_DIR)) {
          event.env.PATH = COMMAND_DIR + path.delimiter + current
        }
        const sessionID = pending.get(event.command)
        if (sessionID) {
          pending.delete(event.command)
          event.env.KANBANZAI_OWNER = owner(sessionID)
        }
      })
    } catch {}

    try {
      await ctx.tool.hook("execute.after", (event) => {
        if (event && event.input && typeof event.input.command === "string") pending.delete(event.input.command)
        if (event && event.sessionID) {
          run(["claim", "heartbeat"], directory, { KANBANZAI_OWNER: owner(event.sessionID) })
        }
      })
    } catch {}

    let cached = { text: "", at: 0 }
    const standing = async () => {
      if (Date.now() - cached.at > CONTEXT_TTL_MS) cached = { text: (await standingContext(directory)).trim(), at: Date.now() }
      return cached.text
    }
    try {
      await ctx.session.hook("context", async (event) => {
        const text = await standing()
        if (text && event && Array.isArray(event.system)) event.system.push({ type: "text", text })
        if (!COMMAND_FOUND && event && Array.isArray(event.system)) {
          event.system.push({ type: "text", text: MISSING })
        }
      })
    } catch {}

    // W-101: the board shows what this session is doing. OpenCode keeps a session's state in its server, not in a
    // file, and the board's core also runs where no OpenCode does, so this plugin writes one small record per session
    // into the project's state folder (`.kanbanzai/state/sessions/opencode/<session id>.json`) and the core reads it for
    // an `opencode:` claim -- never Claude Code's files, and Claude Code's reader never reads these.
    //
    // The record is W-99's {state, since, started, name} plus the server's pid and start time, so a dead server leaves
    // no stale mark; `state` is the board's vocabulary. A question is a form (`form.created`), a permission prompt is
    // `permission.asked`; both make the card ask. Nothing here may throw into OpenCode.
    const stop = new AbortController()
    try {
      const here = path.resolve(directory)
      const canonical = path.resolve(
        (ctx.location && ctx.location.project && typeof ctx.location.project.canonical === "string")
          ? ctx.location.project.canonical
          : directory,
      )
      const folder = await stateFolder(directory, canonical)
      const written = new Map() // session id -> the record last written
      const known = new Map() // session id -> {name, started}, looked up once

      const startTime = () => {
        try {
          const stat = fs.readFileSync("/proc/self/stat", "utf8")
          return stat.slice(stat.lastIndexOf(")") + 1).trim().split(/\s+/)[19]
        } catch {
          return null // no /proc (not Linux): the core asks only whether the pid exists
        }
      }
      const fileName = (sessionID) => `${String(sessionID).replace(/[^A-Za-z0-9_-]/g, "_")}.json`
      const target = (sessionID) => path.join(folder, fileName(sessionID))

      const remember = async (sessionID, title) => {
        const found = known.get(sessionID)
        if (found) {
          if (title) found.name = title
          return found
        }
        const detail = { name: title || "", started: null }
        known.set(sessionID, detail)
        try {
          const session = await ctx.session.get({ sessionID })
          if (session) {
            if (typeof session.title === "string" && session.title) detail.name = session.title
            if (session.time && typeof session.time.created === "number") detail.started = session.time.created
          }
        } catch {}
        return detail
      }

      const write = (sessionID, state, at) => {
        const detail = known.get(sessionID) || { name: "", started: null }
        const previous = written.get(sessionID)
        const name = detail.name || (previous ? previous.name : "")
        if (previous && previous.state === state && previous.name === name) return
        const record = {
          sessionId: sessionID,
          state,
          since: previous && previous.state === state ? previous.since : at,
          started: detail.started !== null && detail.started !== undefined
            ? detail.started
            : (previous ? previous.started : null),
          name,
          pid: process.pid,
          procStart: startTime(),
          updated: Date.now(),
          directory: canonical,
        }
        try {
          fs.mkdirSync(folder, { recursive: true })
          const tmp = `${target(sessionID)}.${process.pid}.tmp`
          fs.writeFileSync(tmp, JSON.stringify(record) + "\n")
          fs.renameSync(tmp, target(sessionID))
          written.set(sessionID, record)
        } catch {}
      }

      const forget = (sessionID) => {
        written.delete(sessionID)
        known.delete(sessionID)
        try {
          fs.unlinkSync(target(sessionID))
        } catch {}
      }

      const STATES = new Map([
        ["session.execution.started", "busy"],
        ["session.idle", "idle"],
        ["session.execution.succeeded", "idle"],
        ["session.execution.failed", "idle"],
        ["session.execution.interrupted", "idle"],
        ["form.created", "waiting"],
        ["permission.asked", "waiting"],
        ["form.replied", "busy"],
        ["permission.replied", "busy"],
        ["form.cancelled", "busy"],
      ])
      const sessionOf = (event) => {
        const data = event.data || {}
        if (event.type === "form.created") return data.form && data.form.sessionID
        return data.sessionID
      }
      const stateOf = (event) => {
        if (event.type === "session.status") {
          const kind = (event.data && event.data.status && event.data.status.type) || ""
          return kind === "busy" || kind === "retry" ? "busy" : kind === "idle" ? "idle" : null
        }
        return STATES.get(event.type) || null
      }

      const handle = async (event) => {
        if (!event || typeof event.type !== "string") return
        const where = event.location && event.location.directory
        if (typeof where === "string" && path.resolve(where) !== here) return // another project on the same server
        const sessionID = sessionOf(event)
        if (typeof sessionID !== "string" || !sessionID) return
        if (event.type === "session.deleted" || event.type === "session.archived") return forget(sessionID)
        const at = typeof event.created === "number" ? event.created : Date.now()
        if (event.type === "session.created" || event.type === "session.renamed") {
          const title = event.data && event.data.title
          await remember(sessionID, typeof title === "string" ? title : "")
          const previous = written.get(sessionID)
          if (previous) write(sessionID, previous.state, at)
          return
        }
        const state = stateOf(event)
        if (!state) return
        await remember(sessionID, "")
        write(sessionID, state, at)
      }

      void (async () => {
        try {
          for await (const event of ctx.event.subscribe({ signal: stop.signal })) {
            try {
              await handle(event)
            } catch {}
          }
        } catch {}
      })()
    } catch {}
    return () => stop.abort()
  },
}

/** The OpenCode session state folder: the board's own `state/sessions/opencode`, asked of the core, else the default. */
async function stateFolder(directory, canonical) {
  const fallback = path.join(canonical, ".kanbanzai", "state", "sessions", "opencode")
  try {
    const answer = await run(["config", "--json"], directory, {})
    const data = JSON.parse(answer)
    const state = data && data.project && data.project.state
    return typeof state === "string" && state ? path.join(state, "sessions", "opencode") : fallback
  } catch {
    return fallback
  }
}
