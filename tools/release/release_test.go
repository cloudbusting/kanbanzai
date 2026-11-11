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
	names, err := Build(root, "v0.0.0-test", out)
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
