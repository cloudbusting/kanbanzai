package main

import (
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestVersion(t *testing.T) {
	out := testutil.Run(t, t.TempDir(), nil, "--version")
	if out.Code != 0 {
		t.Fatalf("--version exit %d, stderr %q", out.Code, out.Stderr)
	}
	if got, want := strings.TrimSpace(out.Stdout), "kanbanzai dev"; got != want {
		t.Fatalf("--version printed %q, want %q", got, want)
	}
}
