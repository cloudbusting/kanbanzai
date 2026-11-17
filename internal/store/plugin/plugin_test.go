package plugin

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/store"
	"github.com/cloudbusting/kanbanzai/internal/store/files"
	"github.com/cloudbusting/kanbanzai/internal/store/mirror"
	"github.com/cloudbusting/kanbanzai/internal/store/protocol"
	"github.com/cloudbusting/kanbanzai/internal/store/reference"
	"github.com/cloudbusting/kanbanzai/internal/store/storetest"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// The test binary is also the plugin: run as kanbanzai-store-fake with this variable set, it serves the reference
// backend over the protocol, and leaves a marker so a test can prove it ran (or never did).
const pluginMode = "KANBANZAI_TEST_STORE_PLUGIN"

func TestMain(m *testing.M) {
	if mode := os.Getenv(pluginMode); mode != "" {
		if marker := os.Getenv("KANBANZAI_TEST_STORE_MARKER"); marker != "" {
			_ = os.WriteFile(marker, []byte("ran\n"), 0o644)
		}
		switch mode {
		case "serve":
			if err := protocol.Serve(os.Stdin, os.Stdout, reference.Open); err != nil {
				os.Exit(1)
			}
		case "garbage":
			_, _ = io.WriteString(os.Stdout, "this is not JSON")
		case "fail":
			_, _ = io.WriteString(os.Stderr, "the plugin's own complaint\n")
			os.Exit(3)
		case "hang":
			time.Sleep(10 * time.Second)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

const fakeConfig = "[project]\nowner = \"Sam\"\n\n[store]\nplugin = \"fake\"\nprefix = \"REF\"\n\n[store.settings]\nroot = \"%s\"\n"

// install puts a copy of this test binary on PATH as kanbanzai-store-fake, serving in mode.
func install(t *testing.T, mode string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	name := "kanbanzai-store-fake"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(pluginMode, mode)
	return path
}

func project(t *testing.T, tickets string) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, strings.Replace(fakeConfig, "%s", filepath.ToSlash(tickets), 1))
	p, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	return p
}

func trusted(t *testing.T, p *config.Project) *Client {
	t.Helper()
	program, err := Find(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Trust(p, program, "2026-11-17T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	client, err := Open(p, io.Discard)
	if err != nil {
		t.Fatalf("Open after trust: %v", err)
	}
	return client
}

func TestConformanceOverTheProtocol(t *testing.T) {
	install(t, "serve")
	storetest.Run(t, func(t *testing.T) storetest.Harness {
		tickets := t.TempDir()
		p := project(t, tickets)
		s := mirror.New(p, trusted(t, p), mirror.Options{Name: "fake"})
		return storetest.Harness{
			Store: s, Folders: [2]string{"1-open", "10-done"}, Prefix: "REF",
			Seed: func(t *testing.T, id, title, stage, folder string) {
				testutil.Write(t, filepath.Join(tickets, id+"-"+files.Slug(title)+".md"), storetest.Brief(id, title, stage))
				if err := s.Sync(); err != nil {
					t.Fatalf("Sync: %v", err)
				}
			},
			Tamper: func(t *testing.T, id string) {
				found, _ := filepath.Glob(filepath.Join(tickets, id+"-*.md"))
				data, _ := os.ReadFile(found[0])
				testutil.Write(t, found[0], strings.Replace(string(data), "Body.", "Theirs.", 1))
			},
		}
	})
}

func TestAnUntrustedPluginIsNeverRun(t *testing.T) {
	path := install(t, "serve")
	marker := filepath.Join(t.TempDir(), "ran")
	t.Setenv("KANBANZAI_TEST_STORE_MARKER", marker)
	p := project(t, t.TempDir())
	_, err := Open(p, io.Discard)
	if !errors.Is(err, store.ErrRefused) || !strings.Contains(err.Error(), "kanbanzai store trust") {
		t.Fatalf("Open of an untrusted plugin = %v, want a refusal naming `kanbanzai store trust`", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("the untrusted plugin ran")
	}
	client := trusted(t, p)
	if _, err := client.Hello(); err != nil {
		t.Fatalf("Hello after trust: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("the trusted plugin did not run")
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("changed"))
	_ = file.Close()
	if _, err := Open(p, io.Discard); !errors.Is(err, store.ErrRefused) || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("Open after the program changed = %v, want a refusal saying it changed", err)
	}
}

func TestAPluginThatFailsIsUnavailableAndItsComplaintIsShown(t *testing.T) {
	for mode, want := range map[string]string{"garbage": "not JSON", "fail": "failed on hello", "hang": "did not answer"} {
		t.Run(mode, func(t *testing.T) {
			install(t, mode)
			p := project(t, t.TempDir())
			program, _ := Find(p)
			if _, err := Trust(p, program, "now"); err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			client, err := Open(p, &stderr)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "hang" {
				old := Timeout
				Timeout = 300 * time.Millisecond
				defer func() { Timeout = old }()
			}
			_, err = client.Hello()
			if !errors.Is(err, store.ErrUnavailable) || !strings.Contains(err.Error(), want) {
				t.Fatalf("Hello = %v, want unavailable with %q", err, want)
			}
			if mode == "fail" && !strings.Contains(stderr.String(), "the plugin's own complaint") {
				t.Fatalf("the plugin's stderr was not passed on: %q", stderr.String())
			}
		})
	}
}

func TestAPluginIsABareNameOnPathNeverAPath(t *testing.T) {
	p := project(t, t.TempDir())
	t.Setenv("PATH", t.TempDir())
	if _, err := Find(p); !errors.Is(err, store.ErrUnavailable) || !strings.Contains(err.Error(), "kanbanzai-store-fake is not on PATH") {
		t.Fatalf("Find with no plugin installed = %v", err)
	}
}
