package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestTheReleaseBuildsTheFiveArchivesAndChecksums(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	names, err := Build(root, "v0.0.0-test", out, "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"kanbanzai_0.0.0-test_darwin_amd64.tar.gz",
		"kanbanzai_0.0.0-test_darwin_arm64.tar.gz",
		"kanbanzai_0.0.0-test_linux_amd64.tar.gz",
		"kanbanzai_0.0.0-test_linux_arm64.tar.gz",
		"kanbanzai_0.0.0-test_windows_amd64.zip",
	}
	if strings.Join(names, " ") != strings.Join(want, " ") {
		t.Fatalf("artifacts = %v, want %v", names, want)
	}
	sums, err := os.ReadFile(filepath.Join(out, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(sums)), "\n")
	if len(lines) != 5 {
		t.Fatalf("SHA256SUMS = %q", sums)
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("checksum line = %q", line)
		}
		data, err := os.ReadFile(filepath.Join(out, fields[1]))
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != fields[0] {
			t.Fatalf("bad checksum for %s", fields[1])
		}
	}
	checkTar(t, filepath.Join(out, "kanbanzai_0.0.0-test_linux_amd64.tar.gz"), "kanbanzai")
	checkZip(t, filepath.Join(out, "kanbanzai_0.0.0-test_windows_amd64.zip"), "kanbanzai.exe")

	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		binary := extractTar(t, filepath.Join(out, "kanbanzai_0.0.0-test_linux_amd64.tar.gz"), "kanbanzai")
		command := exec.Command(binary, "--version")
		version, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("the built binary: %v: %s", err, version)
		}
		if strings.TrimSpace(string(version)) != "kanbanzai v0.0.0-test" {
			t.Fatalf("--version = %q", version)
		}
	}
}

func TestTheReleaseLeavesRawBinariesWhenAskedTo(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dist")
	bins := filepath.Join(t.TempDir(), "bins")
	if _, err := Build(root, "v0.0.0-bins", out, bins); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(bins)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, entry := range entries {
		found = append(found, entry.Name())
	}
	wantDirs := []string{"darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64"}
	if strings.Join(found, " ") != strings.Join(wantDirs, " ") {
		t.Fatalf("bins holds %v, want %v", found, wantDirs)
	}
	for _, dir := range wantDirs {
		name := "kanbanzai"
		if strings.HasPrefix(dir, "windows") {
			name += ".exe"
		}
		info, err := os.Lstat(filepath.Join(bins, dir, name))
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			t.Fatalf("%s: mode %v, size %d", dir, info.Mode(), info.Size())
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
			t.Fatalf("%s: permissions %v, want 0755", dir, info.Mode().Perm())
		}
		inner, err := os.ReadDir(filepath.Join(bins, dir))
		if err != nil || len(inner) != 1 {
			t.Fatalf("%s holds %v, %v; want only the binary", dir, inner, err)
		}
	}
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		version, err := exec.Command(filepath.Join(bins, "linux-amd64", "kanbanzai"), "--version").CombinedOutput()
		if err != nil || strings.TrimSpace(string(version)) != "kanbanzai v0.0.0-bins" {
			t.Fatalf("--version = %q, %v", version, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "SHA256SUMS")); err != nil {
		t.Fatalf("the archives are still built: %v", err)
	}
}

func checkTar(t *testing.T, path, name string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(gzipReader)
	header, err := reader.Next()
	if err != nil {
		t.Fatal(err)
	}
	if header.Name != name || header.Size == 0 {
		t.Fatalf("tar entry = %#v", header)
	}
}

func checkZip(t *testing.T, path, name string) {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if len(reader.File) != 1 || reader.File[0].Name != name || reader.File[0].UncompressedSize64 == 0 {
		t.Fatalf("zip entries = %v", reader.File)
	}
}

func extractTar(t *testing.T, path, name string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(gzipReader)
	header, err := reader.Next()
	if err != nil || header.Name != name {
		t.Fatalf("tar entry = %#v, %v", header, err)
	}
	destination := filepath.Join(t.TempDir(), name)
	output, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(output, reader); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	return destination
}
