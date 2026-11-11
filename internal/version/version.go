// Package version reports the command's version.
package version

// Version is the release version. A release build sets it with
//
//	-ldflags "-X github.com/cloudbusting/kanbanzai/internal/version.Version=v1.2.3"
//
// A source build says "dev".
var Version = "dev"

// String is what `kanbanzai --version` prints.
func String() string {
	return "kanbanzai " + Version
}
