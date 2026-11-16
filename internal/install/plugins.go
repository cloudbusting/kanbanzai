package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	kanbanzai "github.com/cloudbusting/kanbanzai"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/version"
)

type pluginManifest struct {
	Version string            `json:"version"`
	Files   map[string]string `json:"files"`
}

// pluginAssets names only the embedded files that belong in the local plugin.
func pluginAssets() ([]string, error) {
	names := []string{"README.md"}
	for _, root := range []string{".claude-plugin", "hooks", "opencode", "skills"} {
		err := fs.WalkDir(kanbanzai.Files, root, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				names = append(names, name)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return names, nil
}

// realParents rejects a plugins folder that is a symlink or not a directory. Only that folder is checked: the
// user's own home, data folder and kanbanzai folder may be reached through links (macOS's /var, a /home that is a
// link, a dotfiles repository), and the install never writes through a link below them.
func realParents(dir string) error {
	info, err := os.Lstat(dir)
	if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return fmt.Errorf("plugin path %s is not a real directory", dir)
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// inspectPlugins refuses unknown or modified files, and reports whether this
// complete, managed bundle matches the embedded assets.
func inspectPlugins(root string, names []string) (bool, error) {
	manifestPath := filepath.Join(root, ".kanbanzai-plugin.json")
	info, err := os.Lstat(manifestPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return false, fmt.Errorf("%s has no valid ownership manifest", root)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return false, err
	}
	var manifest pluginManifest
	if json.Unmarshal(data, &manifest) != nil || manifest.Version == "" || len(manifest.Files) == 0 || len(manifest.Files) > 1000 {
		return false, fmt.Errorf("%s has an invalid ownership manifest", root)
	}
	same := len(manifest.Files) == len(names)
	wanted := make(map[string]bool, len(names))
	directories := map[string]bool{".": true}
	for name := range manifest.Files {
		if !fs.ValidPath(name) || name == ".kanbanzai-plugin.json" {
			return false, fmt.Errorf("invalid path in plugin ownership manifest")
		}
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			directories[dir] = true
		}
	}
	for _, name := range names {
		wanted[name] = true
	}
	err = filepath.WalkDir(root, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("plugin contains a symlink: %s", file)
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if entry.IsDir() {
			if !directories[name] {
				return fmt.Errorf("unmanaged plugin directory %s; left unchanged", file)
			}
			return nil
		}
		if name == ".kanbanzai-plugin.json" {
			return nil
		}
		expected, ok := manifest.Files[name]
		if !ok || !fs.ValidPath(name) {
			return fmt.Errorf("unmanaged plugin file %s; left unchanged", file)
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 4<<20 {
			return fmt.Errorf("invalid plugin file %s", file)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != expected {
			return fmt.Errorf("modified plugin file %s; left unchanged", file)
		}
		delete(manifest.Files, name)
		current, err := kanbanzai.Files.ReadFile(name)
		if !wanted[name] || err != nil || sha256.Sum256(current) != hash {
			same = false
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	if len(manifest.Files) != 0 {
		return false, fmt.Errorf("%s has missing plugin files; left unchanged", root)
	}
	return same, nil
}

func pluginLock(parent string) (*fsutil.Lock, error) {
	lockPath := filepath.Join(parent, ".install.lock")
	if info, err := os.Lstat(lockPath); err == nil {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("plugin lock %s is not a regular file", lockPath)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return fsutil.OpenLock(lockPath)
}

// recoverPluginSwap restores a prior complete bundle if publication was
// interrupted. The journal names only a managed backup under the plugin home.
func recoverPluginSwap(parent string, names []string, dry bool) error {
	journal := filepath.Join(parent, ".install-journal")
	info, err := os.Lstat(journal)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 255 {
		return fmt.Errorf("plugin installation journal is invalid")
	}
	data, err := os.ReadFile(journal)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(string(data))
	if !strings.HasPrefix(name, ".previous-") || filepath.Base(name) != name || name == ".previous-" {
		return fmt.Errorf("plugin installation journal names an invalid backup")
	}
	current, previous := filepath.Join(parent, "current"), filepath.Join(parent, name)
	if _, err := os.Lstat(current); os.IsNotExist(err) {
		if _, err := inspectPlugins(previous, names); err != nil {
			return fmt.Errorf("cannot recover previous plugin: %w", err)
		}
	} else if err != nil {
		return err
	} else if _, err := inspectPlugins(current, names); err != nil {
		return err
	}
	if dry {
		return nil
	}
	lock, err := pluginLock(parent)
	if err != nil {
		return err
	}
	defer lock.Close()
	if _, err := os.Lstat(current); os.IsNotExist(err) {
		if _, err := inspectPlugins(previous, names); err != nil {
			return err
		}
		if err := os.Rename(previous, current); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if _, err := inspectPlugins(current, names); err != nil {
		return err
	}
	return os.Remove(journal)
}

// installPlugins publishes a complete bundle and preserves replaced versions.
func installPlugins(base string, dry, refresh bool, out io.Writer) error {
	parent := filepath.Join(base, "plugins")
	target := filepath.Join(parent, "current")
	if err := realParents(parent); err != nil {
		return err
	}
	if info, err := os.Lstat(parent); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("plugin directory %s is not a real directory", parent)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	names, err := pluginAssets()
	if err != nil {
		return err
	}
	if err := recoverPluginSwap(parent, names, dry); err != nil {
		return err
	}
	info, err := os.Lstat(target)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	existing := err == nil
	if existing {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is not a managed plugin directory; left unchanged", target)
		}
		same, err := inspectPlugins(target, names)
		if err != nil {
			return err
		}
		if same {
			fmt.Fprintf(out, "plugins already installed: %s\n", target)
			return nil
		}
	}
	if refresh && !existing {
		return nil
	}
	if dry {
		fmt.Fprintf(out, "would install plugins: %s\n", target)
		return nil
	}
	if info, err := os.Stat(base); err != nil || !info.IsDir() {
		return fmt.Errorf("plugin parent %s is not a directory", base)
	}
	if err := os.Mkdir(parent, 0o755); err != nil && !os.IsExist(err) {
		return err
	}
	if info, err := os.Lstat(parent); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("plugin directory %s is not a real directory", parent)
	}
	lock, err := pluginLock(parent)
	if err != nil {
		return err
	}
	defer lock.Close()
	if info, err := os.Lstat(target); err == nil {
		if !existing || !info.IsDir() {
			return fmt.Errorf("plugin directory changed during installation; retry")
		}
		if _, err := inspectPlugins(target, names); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) || existing {
		return fmt.Errorf("plugin directory changed during installation; retry")
	}
	tmp, err := os.MkdirTemp(parent, ".install-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp) // this invocation created the temporary directory
	manifest := pluginManifest{Version: version.Effective(), Files: make(map[string]string, len(names))}
	for _, name := range names {
		data, err := kanbanzai.Files.ReadFile(name)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		manifest.Files[name] = hex.EncodeToString(hash[:])
		dest := filepath.Join(tmp, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if strings.HasPrefix(name, "hooks/") && path.Ext(name) == ".sh" {
			mode = 0o755
		}
		if err := os.WriteFile(dest, data, mode); err != nil {
			return err
		}
	}
	metadata, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmp, ".kanbanzai-plugin.json"), append(metadata, '\n'), 0o644); err != nil {
		return err
	}
	backup := ""
	if existing {
		backup, err = os.MkdirTemp(parent, ".previous-")
		if err != nil {
			return err
		}
		if err := os.Remove(backup); err != nil {
			return err
		}
		journal := filepath.Join(parent, ".install-journal")
		handle, err := os.OpenFile(journal, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		_, writeErr := fmt.Fprintln(handle, filepath.Base(backup))
		if writeErr == nil {
			writeErr = handle.Sync()
		}
		if closeErr := handle.Close(); writeErr == nil {
			writeErr = closeErr
		}
		if writeErr != nil {
			return fmt.Errorf("write plugin installation journal: %w", writeErr)
		}
		if err := os.Rename(target, backup); err != nil {
			_ = os.Remove(journal)
			return err
		}
	}
	if err := os.Rename(tmp, target); err != nil {
		if backup != "" {
			if restoreErr := os.Rename(backup, target); restoreErr != nil {
				return fmt.Errorf("publishing plugins: %v; previous bundle remains at %s: %v", err, backup, restoreErr)
			}
			_ = os.Remove(filepath.Join(parent, ".install-journal"))
		}
		return err
	}
	if backup != "" {
		if err := os.Remove(filepath.Join(parent, ".install-journal")); err != nil {
			return fmt.Errorf("plugins installed but journal remains: %w", err)
		}
	}
	prunePluginLeftovers(parent, backup)
	fmt.Fprintf(out, "installed plugins: %s\n", target)
	return nil
}

// prunePluginLeftovers removes backups of earlier updates and temporary directories of interrupted installs, keeping
// only the newest backup. The caller holds the lock with no journal pending, so nothing here is needed for recovery.
// Only real directories with this installer's own prefixes are touched; a symlink is never followed.
func prunePluginLeftovers(parent, keep string) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".previous-") && !strings.HasPrefix(name, ".install-") || name == filepath.Base(keep) {
			continue
		}
		file := filepath.Join(parent, name)
		if info, err := os.Lstat(file); err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			_ = os.RemoveAll(file)
		}
	}
}
