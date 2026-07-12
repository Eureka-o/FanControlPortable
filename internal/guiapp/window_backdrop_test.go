package guiapp

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

func TestResolveWindowBackdrop(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		theme     string
		supported bool
		want      windows.BackdropType
	}{
		{name: "acrylic", mode: "acrylic", theme: "light", supported: true, want: windows.Acrylic},
		{name: "mica", mode: "mica", theme: "dark", supported: true, want: windows.Mica},
		{name: "mica alt", mode: "tabbed", theme: "system", supported: true, want: windows.Tabbed},
		{name: "off", mode: "off", theme: "light", supported: true, want: windows.None},
		{name: "unsupported windows", mode: "acrylic", theme: "light", supported: false, want: windows.None},
		{name: "custom theme", mode: "mica", theme: "custom-theme", supported: true, want: windows.None},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveWindowBackdrop(tt.mode, tt.theme, tt.supported); got != tt.want {
				t.Fatalf("resolveWindowBackdrop(%q, %q, %v) = %v, want %v", tt.mode, tt.theme, tt.supported, got, tt.want)
			}
		})
	}
}
