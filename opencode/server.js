// Kanbanzai for OpenCode (V2 server plugin). Load it from a project's `.opencode/opencode.json`:
//
//   { "plugins": ["<path to kanbanzai>/opencode"] }
//
// It does for OpenCode what `.claude-plugin/` and `hooks/` do for Claude Code, from the same files:
//   - registers Kanbanzai's skills (../skills/*/SKILL.md) under their own names;
//   - puts `kanbanzai` (../bin) on the PATH of every shell OpenCode starts, and labels the shell with its
//     session (KANBANZAI_OWNER=opencode:<session id>) so claims are this session's;
//   - renews this session's claims after every tool call (the heartbeat);
//   - adds the board's standing context (`kanbanzai context`) to every model request.
//
// A plain object is exported (no import of @opencode/plugin), so the folder loads with nothing to install.
// Nothing here may throw into OpenCode: every step is guarded and fails quietly.

import fs from "node:fs"
import path from "node:path"
import { execFile } from "node:child_process"
import { fileURLToPath } from "node:url"

const HERE = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.resolve(HERE, "..")
const BIN = path.join(ROOT, "bin")
const KANBANZAI = path.join(BIN, "kanbanzai")
const CONTEXT_TTL_MS = 10 * 60 * 1000
const SHELL_TOOLS = new Set(["bash", "shell"])
const debug = (what) => {
  if (process.env.KANBANZAI_DEBUG_LOG) try { fs.appendFileSync(process.env.KANBANZAI_DEBUG_LOG, JSON.stringify(what) + "\n") } catch {}
}

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
        if (!current.split(path.delimiter).includes(BIN)) event.env.PATH = BIN + path.delimiter + current
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
      if (Date.now() - cached.at > CONTEXT_TTL_MS) cached = { text: (await run(["context"], directory, {})).trim(), at: Date.now() }
      return cached.text
    }
    try {
      await ctx.session.hook("context", async (event) => {
        const text = await standing()
        if (text && event && Array.isArray(event.system)) event.system.push({ type: "text", text })
      })
    } catch {}
  },
}
