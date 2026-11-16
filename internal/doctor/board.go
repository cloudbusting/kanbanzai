package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/boardbranch"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
)

// board reports on the board branch (`[board] home = "branch"`): where it stands, and what in this repository will
// stand in the way of the commits and pushes the core makes there. With preview set (`doctor --board`) the project's
// board is still on main and only the second half runs: it answers "would a board branch work here?". Everything
// is read from local files and local git configuration. Nothing is run, and the network is never touched: whether
// the remote lets the branch be created needs the remote, and the findings say so.
func (r *Report) board(p *config.Project, preview bool, bases []string) {
	if !preview {
		r.boardBranch(p)
	}
	r.gitHooks(p)
	r.agentRules(p, bases)
}

func (r *Report) boardBranch(p *config.Project) {
	b, err := boardbranch.New(p)
	if err != nil {
		return
	}
	local := gitutil.Git
	_, localErr := local(p.Root, "rev-parse", "--verify", "--quiet", "refs/heads/"+p.BoardBranch)
	_, trackErr := local(p.Root, "rev-parse", "--verify", "--quiet", "refs/remotes/"+p.BoardRemote+"/"+p.BoardBranch)
	switch {
	case localErr == nil:
		r.add("board.branch", "pass", "Board branch "+p.BoardBranch+" exists in this clone.", "")
	case trackErr == nil:
		r.add("board.branch", "warning", "Board branch "+p.BoardBranch+" exists on "+p.BoardRemote+" but not in this clone yet.", "Run `kanbanzai sync --attach`.")
	default:
		r.add("board.branch", "warning", "Board branch "+p.BoardBranch+" does not exist yet.", "Run `kanbanzai sync --attach` to create it, then `kanbanzai sync` to publish it.")
	}
	if b.Attached() {
		r.add("board.worktree", "pass", "The board branch is checked out at "+p.Work+".", "")
	} else {
		r.add("board.worktree", "warning", "The board branch is not checked out at "+p.Work+"; the board is empty here.", "Run `kanbanzai sync --attach`.")
	}
	if b.RemoteConfigured() {
		r.add("board.remote", "pass", "Remote "+p.BoardRemote+" is configured (not contacted: whether it accepts the branch name needs the remote; ask its administrator that "+p.BoardBranch+" may be created and pushed to).", "")
	} else {
		r.add("board.remote", "warning", "Remote "+p.BoardRemote+" is not configured, so the board cannot be shared.", "git remote add "+p.BoardRemote+" <address>, or set board.remote.")
	}
	rel, err := filepath.Rel(p.Root, p.Work)
	if err == nil {
		if _, ignoreErr := gitutil.Git(p.Root, "check-ignore", "--no-index", "-q", "--", filepath.ToSlash(rel)+"/"); ignoreErr == nil {
			r.add("board.ignore", "pass", "The work folder is ignored in the main checkout, so it never shows as a change there.", "")
		} else {
			r.add("board.ignore", "warning", "The work folder "+filepath.ToSlash(rel)+"/ is not ignored in the main checkout, so its files show as untracked there.", "`kanbanzai sync --attach` adds it to .git/info/exclude.")
		}
	}
}

// hookNames are the git hooks that run on a commit, a merge commit, a ref update or a push.
var hookNames = []string{"pre-commit", "prepare-commit-msg", "commit-msg", "pre-merge-commit", "reference-transaction", "pre-push"}

func (r *Report) gitHooks(p *config.Project) {
	dir, err := gitutil.Git(p.Root, "rev-parse", "--git-path", "hooks")
	if err != nil {
		r.add("board.hooks.git", "skipped", "Requires a readable git checkout.", "")
		return
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(p.Root, dir)
	}
	var found []string
	for _, name := range hookNames {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.IsDir() {
			continue
		}
		if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
			continue // git ignores a hook that is not executable
		}
		found = append(found, name)
	}
	if len(found) == 0 {
		r.add("board.hooks.git", "pass", "No git hooks run on commits or pushes in "+dir+".", "")
		return
	}
	r.add("board.hooks.git", "warning", fmt.Sprintf("Git hooks that will run on the board branch's commits and pushes (the core never skips them): %s, in %s. "+
		"The board's commit messages begin with the card id, `<ID>: <what changed>`; a commit-msg hook that wants another form will reject them.",
		strings.Join(found, ", "), dir),
		"Check that each hook accepts a commit that touches only the board folder and a push of the board branch; adjust the hook or ask its owner. doctor never runs a hook.")
}

// boardCommands are what a session runs that commit or push the board, in the form a permission rule is written.
var boardCommands = []string{"git commit", "git push", "kanbanzai sync"}

func (r *Report) agentRules(p *config.Project, bases []string) {
	var items []string
	seen := map[string]bool{}
	for _, base := range bases {
		for _, name := range []string{"settings.json", "settings.local.json"} {
			items = append(items, claudeItems(filepath.Join(base, ".claude", name), seen)...)
		}
		for _, path := range []string{filepath.Join(base, "opencode.json"), filepath.Join(base, ".opencode", "opencode.json")} {
			items = append(items, openCodeItems(path, seen)...)
		}
	}
	if len(items) == 0 {
		r.add("board.hooks.agent", "pass", "No agent permission rule or hook in this repository's Claude Code or OpenCode settings touches git commit, git push or kanbanzai sync.", "")
		return
	}
	sort.Strings(items)
	r.add("board.hooks.agent", "warning", "Agent settings that touch the board branch's commands: "+strings.Join(items, "; ")+".",
		"Allow `kanbanzai sync` and the kanbanzai commands (`Bash(kanbanzai:*)`) so a session can publish the board; the core's own git calls run inside them. User-level and managed settings are not inspected.")
}

func claudeItems(path string, seen map[string]bool) []string {
	doc, err := readObject(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return []string{filepath.Base(path) + " could not be read (" + err.Error() + ")"}
	}
	var out []string
	var permissions struct {
		Deny, Ask []string
	}
	if raw, ok := doc["permissions"]; ok {
		_ = json.Unmarshal(raw, &permissions)
	}
	for verb, rules := range map[string][]string{"denies": permissions.Deny, "asks before": permissions.Ask} {
		for _, rule := range rules {
			for _, command := range boardCommands {
				if bashRuleCovers(rule, command) {
					out = appendOnce(out, seen, fmt.Sprintf("%s %s `%s` (rule %s)", filepath.Base(path), verb, command, rule))
				}
			}
		}
	}
	var hooks map[string][]struct {
		Matcher string
		Hooks   []json.RawMessage
	}
	if raw, ok := doc["hooks"]; ok {
		_ = json.Unmarshal(raw, &hooks)
	}
	for _, entry := range hooks["PreToolUse"] {
		if len(entry.Hooks) > 0 && (entry.Matcher == "" || entry.Matcher == "*" || strings.Contains(entry.Matcher, "Bash")) {
			out = appendOnce(out, seen, filepath.Base(path)+" has a PreToolUse hook for Bash: it sees the `kanbanzai` and `git` commands a skill runs, but the git commits and pushes made inside `kanbanzai sync` and `kanbanzai move` do not pass through it")
		}
	}
	return out
}

func openCodeItems(path string, seen map[string]bool) []string {
	doc, err := readObject(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return []string{filepath.Base(path) + " could not be read (" + err.Error() + ")"}
	}
	var permission struct {
		Bash json.RawMessage `json:"bash"`
	}
	if raw, ok := doc["permission"]; ok {
		_ = json.Unmarshal(raw, &permission)
	}
	rules := map[string]string{}
	var whole string
	if json.Unmarshal(permission.Bash, &whole) == nil && whole != "" {
		rules["*"] = whole
	} else {
		_ = json.Unmarshal(permission.Bash, &rules)
	}
	var out []string
	for pattern, action := range rules {
		verb := map[string]string{"deny": "denies", "ask": "asks before"}[action]
		if verb == "" {
			continue
		}
		for _, command := range boardCommands {
			if wildcard(pattern, command) || wildcard(pattern, command+" x") {
				out = appendOnce(out, seen, fmt.Sprintf("%s %s `%s` (bash rule %q)", filepath.Base(path), verb, command, pattern))
			}
		}
	}
	return out
}

func appendOnce(list []string, seen map[string]bool, item string) []string {
	if seen[item] {
		return list
	}
	seen[item] = true
	return append(list, item)
}

// bashRuleCovers reports whether a Claude Code permission rule applies to a command: `Bash` is every command,
// `Bash(git push:*)` is the prefix form, and `*` inside a pattern matches anything.
func bashRuleCovers(rule, command string) bool {
	if rule == "Bash" {
		return true
	}
	if !strings.HasPrefix(rule, "Bash(") || !strings.HasSuffix(rule, ")") {
		return false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(rule, "Bash("), ")")
	if prefix, ok := strings.CutSuffix(inner, ":*"); ok {
		return command == prefix || strings.HasPrefix(command, prefix+" ")
	}
	return wildcard(inner, command) || wildcard(inner, command+" x")
}

// wildcard matches text against a pattern in which `*` stands for any run of characters.
func wildcard(pattern, text string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == text
	}
	if !strings.HasPrefix(text, parts[0]) {
		return false
	}
	text = text[len(parts[0]):]
	for _, part := range parts[1 : len(parts)-1] {
		i := strings.Index(text, part)
		if i < 0 {
			return false
		}
		text = text[i+len(part):]
	}
	return strings.HasSuffix(text, parts[len(parts)-1])
}
