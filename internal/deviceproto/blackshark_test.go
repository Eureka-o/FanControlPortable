package deviceproto

import (
	"bytes"
	"testing"
)

func TestBlackSharkFrameBuildersAndStatusParser(t *testing.T) {
	if got := BuildBlackSharkGetVersion(); !bytes.Equal(got, []byte{0xA5, 0x03, 0x01, 0xA9}) {
		t.Fatalf("version frame = % X", got)
	}
	if got := BuildBlackSharkSetSpeed(1600); !bytes.Equal(got, []byte{0xA5, 0x09, 0x24, 0x00, 0x00, 0x01, 0x40, 0x06, 0x19}) {
		t.Fatalf("speed frame = % X", got)
	}
	if got := BuildBlackSharkSetSpeed(4500); !bytes.Equal(got, []byte{0xA5, 0x09, 0x24, 0x00, 0x00, 0x01, 0xA0, 0x0F, 0x82}) {
		t.Fatalf("speed frame should clamp to 4000 RPM, got % X", got)
	}
	frame, ok := ParseBlackSharkFrame([]byte{0xA5, 0x07, 0x06, 0xAC, 0x08, 0x00, 0x66})
	if !ok || !frame.ChecksumOK {
		t.Fatalf("status frame parse = %#v/%v", frame, ok)
	}
	rpm, flag, ok := ParseBlackSharkStatus(frame)
	if !ok || rpm != 2220 || flag != 0 {
		t.Fatalf("status = %d/%d/%v", rpm, flag, ok)
	}
}

func TestBlackSharkModeNameMatchesFlyDigiConvention(t *testing.T) {
	if got := BlackSharkModeName(0); got != "manual/fixed gear mode" {
		t.Fatalf("manual mode = %q", got)
	}
	if got := BlackSharkModeName(1); got != "auto/realtime RPM mode" {
		t.Fatalf("automatic mode = %q", got)
	}
}

func TestBlackSharkLightingFrame(t *testing.T) {
	frame, err := BuildBlackSharkSetLighting("blackshark_breathing", "medium", 80, 0x19, 0x00, 0xFF)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(frame, []byte{0xA5, 0x0C, 0x12, 0x03, 0xB8, 0x0B, 0x50, 0x01, 0x19, 0x00, 0xFF, 0xF2}) {
		t.Fatalf("lighting frame = % X", frame)
	}
	if _, err := BuildBlackSharkSetLighting("static_multi", "slow", 80, 0x19, 0x00, 0xFF); err != nil {
		t.Fatalf("generic static_multi lighting mode should map to a native mode: %v", err)
	}
}
