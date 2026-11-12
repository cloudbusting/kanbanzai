// tools/vsix packs board-view into a .vsix.
//
// It is the release tool, not a runtime: the extension runs `kanbanzai` from the `kanbanzai.command` setting or PATH,
// so the archive carries no machine path and no bin.json. Run from the repository root:
//
//	go run ./tools/vsix -o /tmp/kanbanzai-board.vsix
//
// With -bin DIR (the layout tools/release -bins leaves: DIR/<os>-<arch>/kanbanzai[.exe] for linux/amd64,
// linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64) the archive also carries the command at
// extension/bin/<os>-<arch>/kanbanzai[.exe], executable. Without it the .vsix is a dev build with no command.
package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Package is the part of board-view/package.json the manifest needs.
type Package struct {
	Name          string            `json:"name"`
	Version       string            `json:"version"`
	Publisher     string            `json:"publisher"`
	DisplayName   string            `json:"displayName"`
	Description   string            `json:"description"`
	Categories    []string          `json:"categories"`
	ExtensionKind []string          `json:"extensionKind"`
	Engines       map[string]string `json:"engines"`
	Icon          string            `json:"icon"`
}

const contentTypes = `<?xml version="1.0" encoding="utf-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
	`<Default Extension=".json" ContentType="application/json"/>` +
	`<Default Extension=".js" ContentType="application/javascript"/>` +
	`<Default Extension=".md" ContentType="text/markdown"/>` +
	`<Default Extension=".png" ContentType="image/png"/>` +
	`<Default Extension=".vsixmanifest" ContentType="text/xml"/>` +
	`<Default Extension=".xml" ContentType="text/xml"/>`

// The command is an extensionless file (and .exe on Windows), so each binary gets an Override on its own path.
const binaryContentType = "application/octet-stream"

// bundledTargets are the platforms the .vsix carries, as tools/release builds them.
var bundledTargets = []struct{ dir, name string }{
	{"linux-amd64", "kanbanzai"},
	{"linux-arm64", "kanbanzai"},
	{"darwin-amd64", "kanbanzai"},
	{"darwin-arm64", "kanbanzai"},
	{"windows-amd64", "kanbanzai.exe"},
}

type binary struct {
	entry string // the path in the archive
	path  string // the file to read
}

// findBinaries checks that bins holds all five targets as regular files (not links, not folders).
func findBinaries(bins string) ([]binary, error) {
	var found []binary
	for _, target := range bundledTargets {
		path := filepath.Join(bins, target.dir, target.name)
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("-bin %s lacks the %s command (%s/%s): %w", bins, target.dir, target.dir, target.name, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("-bin: %s/%s is not a regular file (%v)", target.dir, target.name, info.Mode().Type())
		}
		found = append(found, binary{entry: "extension/bin/" + target.dir + "/" + target.name, path: path})
	}
	return found, nil
}

func typesXML(binaries []binary) string {
	var overrides strings.Builder
	for _, bin := range binaries {
		overrides.WriteString(`<Override PartName="/` + html.EscapeString(bin.entry) + `" ContentType="` + binaryContentType + `"/>`)
	}
	return contentTypes + overrides.String() + `</Types>`
}

// BuildVSIX packs dir into dest and returns the extension's metadata. When bins is not empty it also packs the
// command for the five platforms from bins/<os>-<arch>/kanbanzai[.exe], with the executable bit set.
func BuildVSIX(dir, dest, bins string) (Package, error) {
	var pkg Package
	var binaries []binary
	if bins != "" {
		var err error
		if binaries, err = findBinaries(bins); err != nil {
			return pkg, err
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return pkg, err
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return pkg, err
	}
	names := []string{"package.json", "README.md", "icon.png"}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return pkg, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".js") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names[2:])
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return pkg, err
	}
	file, err := os.Create(dest)
	if err != nil {
		return pkg, err
	}
	writer := zip.NewWriter(file)
	fail := func(err error) (Package, error) {
		writer.Close()
		file.Close()
		return pkg, err
	}
	if err := writeString(writer, "extension.vsixmanifest", manifest(pkg)); err != nil {
		return fail(err)
	}
	if err := writeString(writer, "[Content_Types].xml", typesXML(binaries)); err != nil {
		return fail(err)
	}
	for _, name := range names {
		source, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fail(err)
		}
		if err := writeBytes(writer, "extension/"+name, source); err != nil {
			return fail(err)
		}
	}
	for _, bin := range binaries {
		source, err := os.ReadFile(bin.path)
		if err != nil {
			return fail(err)
		}
		if err := writeExecutable(writer, bin.entry, source); err != nil {
			return fail(err)
		}
	}
	if err := writer.Close(); err != nil {
		file.Close()
		return pkg, err
	}
	if err := file.Close(); err != nil {
		return pkg, err
	}
	return pkg, nil
}

func manifest(pkg Package) string {
	engine := pkg.Engines["vscode"]
	if engine == "" {
		engine = "*"
	}
	kinds := pkg.ExtensionKind
	if len(kinds) == 0 {
		kinds = []string{"workspace"}
	}
	categories := pkg.Categories
	if len(categories) == 0 {
		categories = []string{"Other"}
	}
	display := pkg.DisplayName
	if display == "" {
		display = pkg.Name
	}
	return `<?xml version="1.0" encoding="utf-8"?>
<PackageManifest Version="2.0.0" xmlns="http://schemas.microsoft.com/developer/vsx-schema/2011" xmlns:d="http://schemas.microsoft.com/developer/vsx-schema-design/2011">
  <Metadata>
    <Identity Language="en-US" Id="` + html.EscapeString(pkg.Name) + `" Version="` + html.EscapeString(pkg.Version) +
		`" Publisher="` + html.EscapeString(pkg.Publisher) + `" />
    <DisplayName>` + html.EscapeString(display) + `</DisplayName>
    <Description xml:space="preserve">` + html.EscapeString(pkg.Description) + `</Description>
    <Categories>` + html.EscapeString(strings.Join(categories, ",")) + `</Categories>
    <GalleryFlags>Public</GalleryFlags>
    <Properties>
      <Property Id="Microsoft.VisualStudio.Code.Engine" Value="` + html.EscapeString(engine) + `" />
      <Property Id="Microsoft.VisualStudio.Code.ExtensionDependencies" Value="" />
      <Property Id="Microsoft.VisualStudio.Code.ExtensionPack" Value="" />
      <Property Id="Microsoft.VisualStudio.Code.ExtensionKind" Value="` + html.EscapeString(strings.Join(kinds, ",")) + `" />
      <Property Id="Microsoft.VisualStudio.Code.LocalizedLanguages" Value="" />
      <Property Id="Microsoft.VisualStudio.Code.EnabledApiProposals" Value="" />
      <Property Id="Microsoft.VisualStudio.Code.ExecutesCode" Value="true" />
    </Properties>
  </Metadata>
  <Installation>
    <InstallationTarget Id="Microsoft.VisualStudio.Code"/>
  </Installation>
  <Dependencies/>
  <Assets>
    <Asset Type="Microsoft.VisualStudio.Code.Manifest" Path="extension/package.json" Addressable="true" />
    <Asset Type="Microsoft.VisualStudio.Services.Content.Details" Path="extension/README.md" Addressable="true" />
    <Asset Type="Microsoft.VisualStudio.Services.Icons.Default" Path="extension/icon.png" Addressable="true" />
  </Assets>
</PackageManifest>
`
}

func writeString(writer *zip.Writer, name, text string) error {
	return writeBytes(writer, name, []byte(text))
}

func writeBytes(writer *zip.Writer, name string, data []byte) error {
	entry, err := writer.Create(name)
	if err != nil {
		return err
	}
	_, err = entry.Write(data)
	return err
}

// writeExecutable adds a file with Unix mode 0755 in the high 16 bits of the external attributes, so an extractor
// that honours them leaves the command runnable.
func writeExecutable(writer *zip.Writer, name string, data []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o755)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = entry.Write(data)
	return err
}

// describe is the one line the packer prints.
func describe(dest string, pkg Package, binaries int) string {
	contents := "no command: a dev build, the extension uses kanbanzai.command or PATH"
	if binaries > 0 {
		contents = fmt.Sprintf("with the command for %d platforms", binaries)
	}
	return fmt.Sprintf("wrote %s (%s.%s %s); %s; not installed", dest, pkg.Publisher, pkg.Name, pkg.Version, contents)
}

func main() {
	dir := flag.String("dir", "board-view", "the board-view folder to pack")
	bins := flag.String("bin", "", "the folder of commands, DIR/<os>-<arch>/kanbanzai[.exe] (tools/release -bins); empty packs a dev .vsix with no command")
	dest := flag.String("o", "", "the .vsix path to write")
	flag.Parse()
	if *dest == "" {
		fmt.Fprintln(os.Stderr, "vsix: -o <path> is needed")
		os.Exit(2)
	}
	pkg, err := BuildVSIX(*dir, *dest, *bins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vsix: %v\n", err)
		os.Exit(1)
	}
	count := 0
	if *bins != "" {
		count = len(bundledTargets)
	}
	fmt.Println(describe(*dest, pkg, count))
}
