//go:build windows

package powernotify

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
	"unsafe"
)

func TestHIDNotifyFilterLayout(t *testing.T) {
	if got := unsafe.Sizeof(hidNotifyFilter{}); got != 416 {
		t.Fatalf("HID notify filter size = %d, want 416", got)
	}
}

func TestHIDInterfacePathAndMatch(t *testing.T) {
	want := `\\?\HID#VID_37D7&PID_1002#abc`
	encoded := append(utf16.Encode([]rune(want)), 0)
	eventData := make([]byte, hidNotifyEventPathOffset+len(encoded)*2)
	for i, value := range encoded {
		binary.LittleEndian.PutUint16(eventData[hidNotifyEventPathOffset+i*2:], value)
	}

	got := hidInterfacePath(unsafe.Pointer(&eventData[0]), uintptr(len(eventData)))
	if got != want {
		t.Fatalf("hidInterfacePath() = %q, want %q", got, want)
	}
	if !matchesHIDInterfacePath(got, []string{"vid_37d7&pid_1002"}) {
		t.Fatal("supported FlyDigi HID path was not matched")
	}
	if matchesHIDInterfacePath(got, []string{"vid_37d7&pid_1003"}) {
		t.Fatal("unrelated FlyDigi HID product was matched")
	}
}
