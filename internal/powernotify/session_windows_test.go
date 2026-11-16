//go:build windows

package powernotify

import "testing"

func TestHandleSessionEndMessage(t *testing.T) {
	ended := 0
	onEnd := func() { ended++ }

	if result, handled := handleSessionEndMessage(wmQueryEndSession, 0, onEnd); !handled || result != 1 {
		t.Fatalf("WM_QUERYENDSESSION = (%d, %v), want (1, true)", result, handled)
	}
	if ended != 0 {
		t.Fatal("querying shutdown must not trigger cleanup before shutdown is confirmed")
	}
	if result, handled := handleSessionEndMessage(wmEndSession, 0, onEnd); !handled || result != 0 {
		t.Fatalf("canceled WM_ENDSESSION = (%d, %v), want (0, true)", result, handled)
	}
	if ended != 0 {
		t.Fatal("canceled shutdown triggered cleanup")
	}
	if result, handled := handleSessionEndMessage(wmEndSession, 1, onEnd); !handled || result != 0 {
		t.Fatalf("confirmed WM_ENDSESSION = (%d, %v), want (0, true)", result, handled)
	}
	if ended != 1 {
		t.Fatalf("cleanup calls = %d, want 1", ended)
	}
}
