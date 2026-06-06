package deviceprofileexec

import (
	"bytes"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestEncodeCommandAppendsChecksumModesToHexPayloads(t *testing.T) {
	tests := []struct {
		name     string
		checksum string
		command  string
		want     []byte
	}{
		{
			name:     "none",
			checksum: "none",
			command:  "01 02 03",
			want:     []byte{0x01, 0x02, 0x03},
		},
		{
			name:     "sum8",
			checksum: "sum8",
			command:  "01 02 03",
			want:     []byte{0x01, 0x02, 0x03, 0x06},
		},
		{
			name:     "xor8",
			checksum: "xor8",
			command:  "01 02 03",
			want:     []byte{0x01, 0x02, 0x03, 0x00},
		},
		{
			name:     "crc16 modbus",
			checksum: "crc16",
			command:  "31 32 33 34 35 36 37 38 39",
			want:     []byte{'1', '2', '3', '4', '5', '6', '7', '8', '9', 0x37, 0x4b},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, contentType, err := EncodeCommand(types.DeviceCommandTemplate{
				Name:     "setSpeed",
				Command:  tt.command,
				Encoding: "hex",
				Checksum: tt.checksum,
			}, SpeedVars{})
			if err != nil {
				t.Fatalf("EncodeCommand() error = %v", err)
			}
			if contentType != "application/octet-stream" {
				t.Fatalf("content type = %q, want application/octet-stream", contentType)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("payload = % X, want % X", got, tt.want)
			}
		})
	}
}

func TestEncodeCommandAppendsChecksumModesToAsciiPayloads(t *testing.T) {
	got, contentType, err := EncodeCommand(types.DeviceCommandTemplate{
		Name:     "setSpeed",
		Command:  "P{{percent}}",
		Encoding: "ascii",
		Checksum: "sum8",
	}, SpeedVarsFromValue(types.NewPercentTickSpeed(125)))
	if err != nil {
		t.Fatalf("EncodeCommand() error = %v", err)
	}
	want := []byte{'P', '1', '3', 0xb4}
	if !bytes.Equal(got, want) {
		t.Fatalf("payload = % X, want % X", got, want)
	}
	if contentType != "application/octet-stream" {
		t.Fatalf("content type = %q, want application/octet-stream", contentType)
	}
}

func TestEncodeCommandRejectsJSONChecksum(t *testing.T) {
	_, _, err := EncodeCommand(types.DeviceCommandTemplate{
		Name:     "setSpeed",
		Command:  `{"speed":{{percent}}}`,
		Encoding: "json",
		Checksum: "sum8",
	}, SpeedVarsFromValue(types.NewPercentSpeed(20)))
	if err == nil {
		t.Fatal("expected json checksum to be rejected")
	}
}
