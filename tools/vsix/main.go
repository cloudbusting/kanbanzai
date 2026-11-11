// tools/vsix packs board-view into a .vsix.
//
// It is the release tool, not a runtime: the extension runs `kanbanzai` from the `kanbanzai.command` setting or PATH,
// so the archive carries no machine path and no bin.json. Run from the repository root:
//
//	go run ./tools/vsix -o /tmp/kanbanzai-board.vsix
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
	`<Default Extension=".xml" ContentType="text/xml"/>` +
	`</Types>`

// BuildVSIX packs dir into dest and returns the extension's metadata.
func BuildVSIX(dir, dest string) (Package, error) {
	var pkg Package
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
	if err := writeString(writer, "[Content_Types].xml", contentTypes); err != nil {
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

func main() {
	dir := flag.String("dir", "board-view", "the board-view folder to pack")
	dest := flag.String("o", "", "the .vsix path to write")
	flag.Parse()
	if *dest == "" {
		fmt.Fprintln(os.Stderr, "vsix: -o <path> is needed")
		os.Exit(2)
	}
	pkg, err := BuildVSIX(*dir, *dest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vsix: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%s.%s %s); not installed\n", *dest, pkg.Publisher, pkg.Name, pkg.Version)
}
