package deviceproto

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
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

func TestBlackSharkCaptured07FrameIsKeptOpaque(t *testing.T) {
	// This is a captured BLE write from the BRB02 report. 0x07 is deliberately
	// treated as an opaque command here; the report does not establish the
	// meaning of its tagged bytes.
	raw := []byte{0xA5, 0x1A, 0x07, 0x07, 0x00, 0x40, 0x00, 0x01, 0x3B, 0x00, 0x02, 0x1E, 0x00, 0x03, 0x26, 0x00, 0x05, 0x2F, 0x00, 0x06, 0x33, 0x00, 0x07, 0x8F, 0x05, 0x9A}
	frame, ok := ParseBlackSharkFrame(raw)
	if !ok || !frame.ChecksumOK {
		t.Fatalf("captured 0x07 frame parse = %#v/%v", frame, ok)
	}
	if frame.Command != 0x07 || !bytes.Equal(frame.Payload, raw[3:25]) {
		t.Fatalf("captured 0x07 frame fields = command 0x%02X payload % X", frame.Command, frame.Payload)
	}
	// This captured command uses the long-form convention where LEN includes
	// the checksum byte; short commands in this package use the legacy form.
	if got := buildBlackSharkFrameWithLength(0x1A, frame.Command, frame.Payload...); !bytes.Equal(got, raw) {
		t.Fatalf("captured 0x07 frame serialization = % X, want % X", got, raw)
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

func TestBlackSharkRGB565AndCRC(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, BlackSharkImageWidth, BlackSharkImageHeight))
	img.SetRGBA(0, 0, color.RGBA{R: 0xFF, A: 0xFF})
	canvas, err := BlackSharkRGB565(img)
	if err != nil {
		t.Fatal(err)
	}
	if len(canvas) != BlackSharkImageBytes || !bytes.Equal(canvas[:2], []byte{0xF8, 0x00}) {
		t.Fatalf("canvas size/pixel = %d/% X", len(canvas), canvas[:2])
	}
	if got := BlackSharkCRC16XMODEM([]byte("123456789")); got != 0x31C3 {
		t.Fatalf("CRC16/XMODEM = %04X", got)
	}
	if _, err := BlackSharkRGB565(image.NewRGBA(image.Rect(0, 0, 1, 1))); err == nil {
		t.Fatal("expected image dimension error")
	}
}

func TestBlackSharkImageUploadFrames(t *testing.T) {
	canvas := bytes.Repeat([]byte{0x12, 0x34}, BlackSharkImageWidth*BlackSharkImageHeight)
	frames, err := BuildBlackSharkImageUpload(canvas)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2098 || len(frames[0]) != 15 || len(frames[1]) != 64 || len(frames[len(frames)-2]) != 48 || len(frames[len(frames)-1]) != 4 {
		t.Fatalf("frame count/lengths = %d/%d/%d/%d/%d", len(frames), len(frames[0]), len(frames[1]), len(frames[len(frames)-2]), len(frames[len(frames)-1]))
	}
	if !bytes.Equal(frames[0][0:4], []byte{0xA5, 0x0F, 0xC4, 0x18}) || binary.LittleEndian.Uint32(frames[0][7:11]) != uint32(BlackSharkImageBytes) {
		t.Fatalf("metadata header/size = % X/%d", frames[0][:11], binary.LittleEndian.Uint32(frames[0][7:11]))
	}
	if !bytes.Equal(frames[1][:6], []byte{0xA4, 0x40, 0x30, 0x88, 0xC6, 0x12}) {
		t.Fatalf("first block header = % X", frames[1][:6])
	}
	if !bytes.Equal(frames[2095][:5], []byte{0xA4, 0x40, 0x02, 0x00, 0xC6}) {
		t.Fatalf("last full block header = % X", frames[2095][:6])
	}
	if !bytes.Equal(frames[2096][:5], []byte{0xA4, 0x30, 0x01, 0x00, 0xC6}) {
		t.Fatalf("tail header = % X", frames[2096][:6])
	}
	if !bytes.Equal(frames[len(frames)-1], []byte{0xA5, 0x04, 0xC5, 0x6E}) {
		t.Fatalf("commit frame = % X", frames[len(frames)-1])
	}
	for _, frame := range frames {
		if len(frame) < 4 || int(frame[1]) != len(frame) {
			t.Fatalf("invalid frame length: % X", frame)
		}
		if blackSharkChecksum(frame[:len(frame)-1]) != frame[len(frame)-1] {
			t.Fatalf("invalid frame checksum: % X", frame)
		}
	}
}

func TestBlackSharkImageBuilderRejectsLengths(t *testing.T) {
	if _, err := BuildBlackSharkImageMetadata(nil); err == nil {
		t.Fatal("expected metadata canvas length error")
	}
	if _, err := BuildBlackSharkImageDataBlock(1, make([]byte, 58), false); err == nil {
		t.Fatal("expected block number error")
	}
	if _, err := BuildBlackSharkImageDataBlock(2, make([]byte, 57), false); err == nil {
		t.Fatal("expected data block length error")
	}
	if _, err := BuildBlackSharkImageTail(make([]byte, 41)); err == nil {
		t.Fatal("expected tail length error")
	}
}
