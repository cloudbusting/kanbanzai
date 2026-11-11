//go:build windows

package sessions

import "testing"

// makeFIFO has no Windows equivalent; the rest of the junk-file cases still run.
func makeFIFO(t *testing.T, path string) {}
