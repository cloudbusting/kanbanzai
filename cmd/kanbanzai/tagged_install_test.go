package main

import (
	"archive/zip"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// Exercise Go's real module metadata without relying on a remote proxy or a public tag.
func TestTaggedInstallVersion(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	const module = "github.com/cloudbusting/kanbanzai"
	const ver = "v0.99.0"
	tmp := t.TempDir()
	proxy := filepath.Join(tmp, "proxy")
	dir := filepath.Join(proxy, filepath.FromSlash(module), "@v")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ver+".mod"), mod, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ver+".info"), []byte(`{"Version":"v0.99.0","Time":"2026-01-01T00:00:00Z"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "list"), []byte(ver+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, ver+".zip"))
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	files, err := exec.Command("git", "-C", root, "ls-files").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range strings.Split(strings.TrimSpace(string(files)), "\n") {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		w, err := z.Create(module + "@" + ver + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(tmp, "bin")
	proxyPath := filepath.ToSlash(proxy)
	if runtime.GOOS == "windows" {
		proxyPath = "/" + proxyPath
	}
	proxyURL := (&url.URL{Scheme: "file", Path: proxyPath}).String()
	cmd := exec.Command("go", "install", module+"/cmd/kanbanzai@"+ver)
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(), "GOPROXY="+proxyURL, "GOSUMDB=off", "GOWORK=off", "GOFLAGS=-modcacherw", "GOBIN="+bin, "GOMODCACHE="+filepath.Join(tmp, "modcache"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go install: %v\n%s", err, out)
	}
	name := "kanbanzai"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	out, err := exec.Command(filepath.Join(bin, name), "--version").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "kanbanzai "+ver {
		t.Fatalf("installed --version = %q, %v; want %s", out, err, fmt.Sprintf("kanbanzai %s", ver))
	}
	project := testutil.Repo(t, "[project]\nowner = \"test\"\n")
	context := exec.Command(filepath.Join(bin, name), "context", "--adapter-version", "0.1.0")
	context.Dir = project
	out, err = context.CombinedOutput()
	if err != nil || !strings.Contains(string(out), ver) {
		t.Fatalf("context mismatch = %q, %v; want module version", out, err)
	}
	doctor := exec.Command(filepath.Join(bin, name), "doctor", "--json")
	doctor.Dir = project
	out, _ = doctor.CombinedOutput()
	if !strings.Contains(string(out), "Running binary: kanbanzai "+ver) {
		t.Fatalf("doctor output: %s", out)
	}
	installEnv := append(os.Environ(), "XDG_DATA_HOME="+filepath.Join(tmp, "data"), "LOCALAPPDATA="+filepath.Join(tmp, "data"))
	where := exec.Command(filepath.Join(bin, name), "install", "--where")
	where.Env = installEnv
	out, err = where.CombinedOutput()
	if err != nil {
		t.Fatalf("install --where: %v %s", err, out)
	}
	destination := strings.TrimSpace(string(out))
	if err := os.MkdirAll(destination, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, name), []byte("old command"), 0755); err != nil {
		t.Fatal(err)
	}
	refresh := exec.Command(filepath.Join(bin, name), "install", "--refresh")
	refresh.Env = installEnv
	out, err = refresh.CombinedOutput()
	if err != nil || !strings.Contains(string(out), " -> "+ver) {
		t.Fatalf("install --refresh = %q, %v", out, err)
	}
	cmd = exec.Command("go", "install", "-ldflags=-X "+module+"/internal/version.Version=v9.8.7", module+"/cmd/kanbanzai@"+ver)
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(), "GOPROXY="+proxyURL, "GOSUMDB=off", "GOWORK=off", "GOFLAGS=-modcacherw", "GOBIN="+bin, "GOMODCACHE="+filepath.Join(tmp, "modcache"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("stamped go install: %v\n%s", err, out)
	}
	out, err = exec.Command(filepath.Join(bin, name), "--version").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "kanbanzai v9.8.7" {
		t.Fatalf("stamped --version = %q, %v", out, err)
	}
}
