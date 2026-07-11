package guiapp

import (
	"path/filepath"
	"testing"
)

func TestWindowStateRoundTripAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "window-state.json")
	want := windowState{X: 120, Y: 80, Width: 1280, Height: 800, Maximised: true}
	if err := saveWindowStateFile(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadWindowStateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("window state = %#v, want %#v", got, want)
	}
	if _, ok := normaliseWindowState(windowState{Width: 10, Height: 10}, 1920, 1080); ok {
		t.Fatal("invalid window dimensions should be rejected")
	}
}
