package initcmd

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// StoreInfo is the store a new project's cards are kept by, when it is not the project's own files: what `--store`,
// `--store-setting` and `--store-query` chose, and what the owner does next. It is in the plan, so a plan file carries
// the choice and its digest covers the `[store]` text it wrote.
type StoreInfo struct {
	Plugin   string            `json:"plugin"`
	Settings map[string]string `json:"settings,omitempty"`
	Query    string            `json:"query,omitempty"`
	Source   string            `json:"source"`
	// NextSteps are the owner's: init runs no plugin, trusts none and makes no network request.
	NextSteps []string `json:"next_steps"`
}

// filesStore is the default store, the project's own files: no [store] table.
const filesStore = "files"

var (
	storeName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	storeKey  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]*$`)
	repoName  = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
)

// credentialWords are what a setting's key must not say: `[store.settings]` is committed text, and a plugin signs in
// with its own tool (gh), so no secret belongs in it.
var credentialWords = []string{"token", "password", "passwd", "secret", "credential", "apikey", "api_key"}

// checkStore validates --store, --store-setting and --store-query before any plan is made. It returns nil for the
// files store, which writes nothing. A refusal is exit 2 with nothing written.
func checkStore(opts Options) (*StoreInfo, error) {
	given := len(opts.StoreSettings) > 0 || opts.StoreQuery != ""
	switch opts.Store {
	case "":
		if given {
			return nil, refuse("--store-setting and --store-query need --store NAME")
		}
		return nil, nil
	case filesStore:
		if given {
			return nil, refuse("--store-setting and --store-query are for a store plugin: the files store has no settings")
		}
		return nil, nil
	}
	if !storeName.MatchString(opts.Store) {
		return nil, refuse("--store %q: a plugin name is lower-case letters, digits, '-' and '_' (it runs "+
			"kanbanzai-store-NAME from your PATH), never a path or an argument", opts.Store)
	}
	if opts.BoardBranch {
		return nil, refuse("--board-branch cannot be combined with --store: a store plugin keeps the cards elsewhere, " +
			"so there is no board branch")
	}
	info := &StoreInfo{Plugin: opts.Store, Settings: map[string]string{}, Source: "--store"}
	for _, pair := range opts.StoreSettings {
		key, value, found := strings.Cut(pair, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !found || value == "" || !storeKey.MatchString(key) {
			return nil, refuse("--store-setting %q: write KEY=VALUE, a key of letters, digits, '.', '_' or '-' and a "+
				"value that is not empty", pair)
		}
		lower := strings.ToLower(key)
		for _, word := range credentialWords {
			if strings.Contains(lower, word) {
				return nil, refuse("--store-setting %s: a setting is text committed with the config, never a credential; "+
					"a plugin signs in with its own tool (for GitHub, `gh auth login`)", key)
			}
		}
		if err := plainText("--store-setting "+key, value); err != nil {
			return nil, err
		}
		info.Settings[key] = value
	}
	if err := plainText("--store-query", opts.StoreQuery); err != nil {
		return nil, err
	}
	info.Query = strings.TrimSpace(opts.StoreQuery)
	if opts.Store == "github" && !repoName.MatchString(info.Settings["repo"]) {
		return nil, refuse("--store github needs --store-setting repo=owner/name, the one repository that keeps the cards")
	}
	info.NextSteps = storeNextSteps(info)
	return info, nil
}

// plainText refuses a value that would not survive as one line of a TOML string.
func plainText(what, value string) error {
	for _, r := range value {
		if unicode.IsControl(r) {
			return refuse("%s: a control character is not allowed", what)
		}
	}
	return nil
}

// storeNextSteps is what the owner does after init: the plugin is theirs to sign in to and to trust.
func storeNextSteps(info *StoreInfo) []string {
	steps := []string{}
	if info.Plugin == "github" {
		steps = append(steps, "gh auth login (the plugin signs in with your own gh; Kanbanzai holds no token)")
	}
	return append(steps,
		fmt.Sprintf("kanbanzai store trust (once per clone: it lets kanbanzai-store-%s run as you)", info.Plugin),
		"kanbanzai sync (fetch the cards into the working copy)")
}

// StoreText is the [store] tables written into the config of a project whose cards a plugin keeps.
func StoreText(info *StoreInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# The cards are kept by the %s store plugin (docs/store-plugins.md); Kanbanzai keeps a local working\n", info.Plugin)
	b.WriteString("# copy of them as Markdown briefs. `kanbanzai store trust` lets the plugin run, then `kanbanzai sync` fetches them.\n")
	b.WriteString("[store]\n")
	fmt.Fprintf(&b, "plugin = %s\n", tomlString(info.Plugin))
	if info.Query != "" {
		fmt.Fprintf(&b, "query = %s\n", tomlString(info.Query))
	}
	if len(info.Settings) > 0 {
		keys := make([]string, 0, len(info.Settings))
		for key := range info.Settings {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		b.WriteString("\n[store.settings]\n")
		for _, key := range keys {
			fmt.Fprintf(&b, "%s = %s\n", key, tomlString(info.Settings[key]))
		}
	}
	return b.String()
}
