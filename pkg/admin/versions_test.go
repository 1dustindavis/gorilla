package admin

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{a: "1.9", b: "1.10", want: -1},
		{a: "2.47.1", b: "2.48.0", want: -1},
		{a: "24.09", b: "24.10", want: -1},
		{a: "145.0.7632.75", b: "145.0.7632.76", want: -1},
		{a: "1", b: "1.0", want: 0},
		{a: "1.0", b: "1.0.0", want: 0},
		{a: "145.0.10", b: "145.0.9", want: 1},
		{a: "1.2-beta", b: "1.2", want: -1},
		{a: "1.2-rc1", b: "1.2-beta9", want: 1},
		{a: "1.2b2", b: "1.2b10", want: -1},
		{a: "2026-Q3", b: "2026-Q4", want: -1},
		{a: "v2.4.1", b: "2.4.1", want: 0},
		{a: "1.2-alpha", b: "1.2-beta", want: -1},
		{a: "1.2-preview", b: "1.2-rc", want: -1},
		{a: "1.2-hotfix2", b: "1.2-hotfix10", want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got, err := compareVersions(tt.a, tt.b)
			if err != nil {
				t.Fatalf("compareVersions() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestParseVersionRejectsNonConcreteLabels(t *testing.T) {
	for _, version := range []string{"latest", "stable", "beta", ""} {
		t.Run(version, func(t *testing.T) {
			if _, err := parseVersion(version); err == nil {
				t.Fatalf("parseVersion(%q) unexpectedly succeeded", version)
			}
		})
	}
}

func TestParseVersionRejectsUnsupportedCharacters(t *testing.T) {
	for _, version := range []string{"1.2/3", "1.2@beta", "1 2"} {
		t.Run(version, func(t *testing.T) {
			if _, err := parseVersion(version); err == nil {
				t.Fatalf("parseVersion(%q) unexpectedly succeeded", version)
			}
		})
	}
}
