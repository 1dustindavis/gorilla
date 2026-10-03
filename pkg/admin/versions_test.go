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
