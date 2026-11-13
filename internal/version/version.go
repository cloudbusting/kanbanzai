// Package version reports the command's version.
package version

import "runtime/debug"

// Version is the release version. A release build sets it with
//
//	-ldflags "-X github.com/cloudbusting/kanbanzai/internal/version.Version=v1.2.3"
//
// When not stamped, module build information supplies the version when available.
var Version = "dev"

// Effective is the version used by all command consumers.
func Effective() string {
	info, ok := debug.ReadBuildInfo()
	module := ""
	if ok {
		module = info.Main.Version
	}
	return resolve(Version, module)
}

func resolve(stamped, module string) string {
	if stamped != "" && stamped != "dev" {
		return stamped
	}
	if module != "" && module != "(devel)" {
		return module
	}
	return "dev"
}

// String is what `kanbanzai --version` prints.
func String() string {
	return "kanbanzai " + Effective()
}
