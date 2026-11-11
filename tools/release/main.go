// tools/release builds the five supported release archives and their checksums, and checks that the tag, the
// manifests and CHANGELOG.md carry one version.
//
// Run from the repository root:
//
//	go run ./tools/release -check -version v0.5.0
//	go run ./tools/release -version v0.5.0 -out dist
//
// It cross-compiles with CGO_ENABLED=0 and -trimpath; every binary is self-contained. A release is built from a tag
// by .github/workflows/release.yml, which runs the same tool.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type target struct {
	OS   string
	Arch string
}

var targets = []target{
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"windows", "amd64"},
}

// Build compiles every target and writes its archive and SHA256SUMS into out.
func Build(root, version, out string) ([]string, error) {
	artifactVersion := strings.TrimPrefix(version, "v")
	if artifactVersion == "" {
		artifactVersion = version
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, err
	}
	names := []string{}
	for _, platform := range targets {
		name := "kanbanzai"
		if platform.OS == "windows" {
			name += ".exe"
		}
		temporary, err := os.MkdirTemp("", "kanbanzai-release-*")
		if err != nil {
			return nil, err
		}
		binary := filepath.Join(temporary, name)
		ldflags := "-s -w -X github.com/cloudbusting/kanbanzai/internal/version.Version=" + version
		command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags", ldflags,
			"-o", binary, "./cmd/kanbanzai")
		command.Dir = root
		command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+platform.OS, "GOARCH="+platform.Arch)
		var stderr bytes.Buffer
		command.Stderr = &stderr
		if err := command.Run(); err != nil {
			os.RemoveAll(temporary)
			return nil, fmt.Errorf("%s/%s: %v: %s", platform.OS, platform.Arch, err, stderr.String())
		}
		archive := fmt.Sprintf("kanbanzai_%s_%s_%s", artifactVersion, platform.OS, platform.Arch)
		if platform.OS == "windows" {
			archive += ".zip"
			err = writeZip(filepath.Join(out, archive), binary, name)
		} else {
			archive += ".tar.gz"
			err = writeTarGz(filepath.Join(out, archive), binary, name)
		}
		os.RemoveAll(temporary)
		if err != nil {
			return nil, err
		}
		names = append(names, archive)
	}
	sort.Strings(names)
	sums, err := os.Create(filepath.Join(out, "SHA256SUMS"))
	if err != nil {
		return nil, err
	}
	defer sums.Close()
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(sums, "%x  %s\n", sha256.Sum256(data), name)
	}
	return names, nil
}

func writeTarGz(destination, binary, name string) error {
	info, err := os.Stat(binary)
	if err != nil {
		return err
	}
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	header := &tar.Header{Name: name, Mode: 0o755, Size: info.Size(), ModTime: time.Now()}
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}
	source, err := os.Open(binary)
	if err != nil {
		return err
	}
	defer source.Close()
	if _, err := io.Copy(tarWriter, source); err != nil {
		return err
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		return err
	}
	return file.Close()
}

func writeZip(destination, binary, name string) error {
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o755)
	header.Modified = time.Now()
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	source, err := os.Open(binary)
	if err != nil {
		return err
	}
	defer source.Close()
	if _, err := io.Copy(entry, source); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return file.Close()
}

func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above the current directory")
		}
		dir = parent
	}
}

func main() {
	version := flag.String("version", "", "the release tag, like v1.2.3")
	out := flag.String("out", "dist", "the directory for the archives and SHA256SUMS")
	root := flag.String("root", "", "the repository root (default: found from the current directory)")
	check := flag.Bool("check", false, "check that the manifests, CHANGELOG.md and -version agree, then exit")
	flag.Parse()
	directory := *root
	if directory == "" {
		found, err := findRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "release: %v\n", err)
			os.Exit(1)
		}
		directory = found
	}
	if *check {
		checked, err := Check(directory, *version)
		if err != nil {
			fmt.Fprintf(os.Stderr, "release: %v\n", err)
			os.Exit(1)
		}
		if *version == "" {
			fmt.Printf("version %s: the manifests and CHANGELOG.md agree\n", checked)
		} else {
			fmt.Printf("version %s: the manifests, CHANGELOG.md and tag %s agree\n", checked, *version)
		}
		return
	}
	if *version == "" {
		fmt.Fprintln(os.Stderr, "release: -version <tag> is needed")
		os.Exit(2)
	}
	names, err := Build(directory, *version, *out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "release: %v\n", err)
		os.Exit(1)
	}
	for _, name := range names {
		fmt.Println(name)
	}
}
