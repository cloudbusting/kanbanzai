package version

import "testing"

func TestResolve(t *testing.T) {
	for _, tc := range []struct{ stamp, module, want string }{
		{"dev", "v1.2.3", "v1.2.3"},
		{"", "v1.2.3", "v1.2.3"},
		{"dev", "v0.0.0-20260101000000-abcdef123456", "v0.0.0-20260101000000-abcdef123456"},
		{"dev", "(devel)", "dev"},
		{"", "", "dev"},
		{"v2.0.0", "v1.2.3", "v2.0.0"},
	} {
		if got := resolve(tc.stamp, tc.module); got != tc.want {
			t.Errorf("resolve(%q, %q) = %q, want %q", tc.stamp, tc.module, got, tc.want)
		}
	}
}
