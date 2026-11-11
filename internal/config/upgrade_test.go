package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestUpgradePreservesSourceAndBackup(t *testing.T) {
	for _, legacy := range []string{
		"# old\r\n[project]\r\nhome='custom'\r\nstate='custom/state'\r\n",
		"# old\n'config_version' = 0x0 # version comment\n[project]\nowner='Sam'\n",
		"# old\r\n'config_version' = 0x0 # version comment\r\n[project]\r\nowner='Sam'\r\n",
		"config_version=0\nconfig_allow_unknown=['project.future']\n[project]\nfuture={a=1}\n",
	} {
		root := testutil.Repo(t, "")
		path := BootstrapPath(root)
		testutil.Write(t, path, legacy)
		if _, err := Upgrade(root, false); err != nil {
			t.Fatal(err)
		}
		if data, _ := os.ReadFile(path); string(data) != legacy {
			t.Fatal("preview wrote source")
		}
		if _, err := os.Stat(path + ".v0.bak"); !os.IsNotExist(err) {
			t.Fatal("preview wrote backup")
		}
		if _, err := Upgrade(root, true); err != nil {
			t.Fatal(err)
		}
		if data, _ := os.ReadFile(path + ".v0.bak"); string(data) != legacy {
			t.Fatal("backup differs from source")
		}
		if _, err := Find(root); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), "# old") && !strings.Contains(legacy, "config_allow_unknown") {
			t.Fatal("lost comment")
		}
		if _, err := Upgrade(root, true); err != nil {
			t.Fatal(err)
		}
		again, _ := os.ReadFile(path)
		if !bytes.Equal(data, again) {
			t.Fatal("repeat changed config")
		}
	}
}

func TestUpgradeInvalidOrConflictChangesNothing(t *testing.T) {
	for _, text := range []string{"[project\n", "config_version=2\n", "[project]\npreifx='X'\n", "[project]\ndigits='2'\n", "[flow]\npull_order=['missing']\n", "config_version=1\n[flow]\npull_order=['missing']\n"} {
		root := testutil.Repo(t, "")
		path := BootstrapPath(root)
		testutil.Write(t, path, text)
		if _, err := Upgrade(root, true); err == nil {
			t.Fatalf("invalid source upgraded: %q", text)
		}
		if data, _ := os.ReadFile(path); string(data) != text {
			t.Fatal("invalid source was changed")
		}
		if _, err := os.Stat(path + ".v0.bak"); !os.IsNotExist(err) {
			t.Fatal("backup made before validation")
		}
	}
	root := testutil.Repo(t, "")
	path := BootstrapPath(root)
	testutil.Write(t, path, "[project]\nowner='Sam'\n")
	testutil.Write(t, path+".v0.bak", "original backup")
	if _, err := Upgrade(root, true); err == nil {
		t.Fatal("overwrote a backup")
	}
	if data, _ := os.ReadFile(path + ".v0.bak"); string(data) != "original backup" {
		t.Fatal("backup overwritten")
	}
}

func TestUpgradeFromWorktreeUsesMain(t *testing.T) {
	root := testutil.Repo(t, "")
	testutil.Write(t, BootstrapPath(root), "[project]\nhome='custom'\n")
	other := filepath.Join(t.TempDir(), "wt")
	testutil.Git(t, root, "worktree", "add", "-q", "-b", "other", other)
	t.Setenv("KANBANZAI_PROJECT", "")
	if _, err := Upgrade(other, true); err != nil {
		t.Fatal(err)
	}
	if _, err := Find(other); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(BootstrapPath(root) + ".v0.bak"); err != nil {
		t.Fatal(err)
	}
}

func TestUpgradeRefusesSymlink(t *testing.T) {
	root := testutil.Repo(t, "")
	target := filepath.Join(root, "legacy.toml")
	testutil.Write(t, target, "[project]\nowner='Sam'\n")
	if err := os.MkdirAll(filepath.Dir(BootstrapPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, BootstrapPath(root)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := Upgrade(root, true); err == nil {
		t.Fatal("rewrote symlink")
	}
}
