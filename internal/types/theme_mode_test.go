package types

import "testing"

func TestNormalizeThemeModeMigratesLegacyTHRM(t *testing.T) {
	if got := NormalizeThemeMode(ThemeModeTHRM); got != ThemeModeClassic {
		t.Fatalf("NormalizeThemeMode(%q) = %q, want %q", ThemeModeTHRM, got, ThemeModeClassic)
	}
}
