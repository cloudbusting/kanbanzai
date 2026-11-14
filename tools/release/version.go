package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A release version is a semver, with an optional pre-release and build part, written without the tag's leading v.
var releaseVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

// Check reports the version the release manifests and CHANGELOG.md agree on. When tag is not empty, the tag must
// name that same version: a release is built from a tag that matches what the repository says.
//
// It reads the four files a release keeps in step: the plugin manifest, the marketplace manifest (its version and
// its entry's), the editor extension's package.json and the top changelog heading.
func Check(root, tag string) (string, error) {
	plugin, err := readVersion(filepath.Join(root, ".claude-plugin", "plugin.json"), ".claude-plugin/plugin.json")
	if err != nil {
		return "", err
	}
	marketplace, entry, err := readMarketplaceVersion(filepath.Join(root, ".claude-plugin", "marketplace.json"))
	if err != nil {
		return "", err
	}
	extension, err := readVersion(filepath.Join(root, "board-view", "package.json"), "board-view/package.json")
	if err != nil {
		return "", err
	}
	for _, version := range []struct{ file, value string }{
		{".claude-plugin/plugin.json", plugin},
		{".claude-plugin/marketplace.json", marketplace},
		{"the marketplace entry", entry},
		{"board-view/package.json", extension},
	} {
		if !releaseVersion.MatchString(version.value) {
			return "", fmt.Errorf("%s version %q is not semver", version.file, version.value)
		}
		if version.value != plugin {
			return "", fmt.Errorf("%s says %q but .claude-plugin/plugin.json says %q: the release manifests must agree",
				version.file, version.value, plugin)
		}
	}
	changelog, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return "", fmt.Errorf("CHANGELOG.md: %w", err)
	}
	heading := regexp.MustCompile(`(?m)^## \[` + regexp.QuoteMeta(plugin) + `\]`)
	if !heading.Match(changelog) {
		return "", fmt.Errorf("CHANGELOG.md has no `## [%s]` entry for the version the manifests carry", plugin)
	}
	if tag != "" {
		named := strings.TrimPrefix(tag, "v")
		if named != plugin {
			return "", fmt.Errorf("the tag %q names %q but the manifests carry %q: tag the version the repository says",
				tag, named, plugin)
		}
	}
	if err := checkBoardRecords(root); err != nil {
		return "", err
	}
	return plugin, nil
}

func readVersion(path, label string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	if manifest.Version == "" {
		return "", fmt.Errorf("%s has no version", label)
	}
	return manifest.Version, nil
}

func readMarketplaceVersion(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf(".claude-plugin/marketplace.json: %w", err)
	}
	var manifest struct {
		Version string `json:"version"`
		Plugins []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", "", fmt.Errorf(".claude-plugin/marketplace.json: %w", err)
	}
	if manifest.Version == "" {
		return "", "", fmt.Errorf(".claude-plugin/marketplace.json has no version")
	}
	if len(manifest.Plugins) != 1 {
		return "", "", fmt.Errorf(".claude-plugin/marketplace.json lists %d plugins, want 1", len(manifest.Plugins))
	}
	if manifest.Plugins[0].Version == "" {
		return "", "", fmt.Errorf("the marketplace entry for %q has no version", manifest.Plugins[0].Name)
	}
	return manifest.Version, manifest.Plugins[0].Version, nil
}
