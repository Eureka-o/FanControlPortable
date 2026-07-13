package version

import "testing"

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		candidate string
		want      bool
	}{
		{name: "same version", current: "2.5.2", candidate: "v2.5.2", want: false},
		{name: "uppercase tag prefix", current: "V2.5.2", candidate: "v2.5.2", want: false},
		{name: "newer preview", current: "2.5.2-preview.1", candidate: "v2.5.2-preview.2", want: true},
		{name: "older preview", current: "2.5.2-preview.2", candidate: "v2.5.2-preview.1", want: false},
		{name: "stable follows prerelease", current: "2.5.2-preview.2", candidate: "v2.5.2", want: true},
		{name: "prerelease does not replace stable", current: "2.5.2", candidate: "v2.5.2-preview.2", want: false},
		{name: "newer base prerelease", current: "2.5.1", candidate: "v2.5.2-preview.2", want: true},
		{name: "newer nightly", current: "nightly-20260712", candidate: "nightly-20260713", want: true},
		{name: "older nightly", current: "nightly.20260713", candidate: "nightly.20260712", want: false},
		{name: "text prerelease follows numeric", current: "2.5.2-preview.1", candidate: "2.5.2-preview.beta", want: true},
		{name: "numeric prerelease precedes text", current: "2.5.2-preview.beta", candidate: "2.5.2-preview.1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNewer(tt.current, tt.candidate); got != tt.want {
				t.Fatalf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.candidate, got, tt.want)
			}
		})
	}
}
