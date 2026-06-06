package deviceprofileexec

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestNormalizeSerialPortInfosSortsAndDeduplicatesCOMPorts(t *testing.T) {
	ports := normalizeSerialPortInfos([]types.SerialPortInfo{
		{Name: "COM10", Path: `\Device\VCP10`},
		{Name: "COM2", Path: `\Device\Serial2`},
		{Name: " com1 ", Path: `\Device\Serial1`},
		{Name: "COM2", Path: `\Device\Duplicate`},
		{Name: "USB0"},
		{Name: ""},
	})

	names := make([]string, 0, len(ports))
	for _, port := range ports {
		names = append(names, port.Name)
	}
	want := []string{"com1", "COM2", "COM10", "USB0"}
	if len(names) != len(want) {
		t.Fatalf("ports = %#v, want %v", ports, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v", names, want)
		}
	}
	if ports[0].DisplayName == "" || ports[1].DisplayName == "" {
		t.Fatalf("expected display names to be filled: %#v", ports)
	}
}

func TestSerialCOMNumber(t *testing.T) {
	tests := []struct {
		name   string
		number int
		ok     bool
	}{
		{name: "COM1", number: 1, ok: true},
		{name: " com42 ", number: 42, ok: true},
		{name: "USB0", ok: false},
		{name: "COMx", ok: false},
	}
	for _, tt := range tests {
		number, ok := serialCOMNumber(tt.name)
		if number != tt.number || ok != tt.ok {
			t.Fatalf("serialCOMNumber(%q) = %d/%v, want %d/%v", tt.name, number, ok, tt.number, tt.ok)
		}
	}
}
