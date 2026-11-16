package screenimg

import "testing"

func TestPresetByCRCMatchesEmbeddedPresets(t *testing.T) {
	ordered, err := PresetsInOfficialOrder()
	if err != nil {
		t.Fatalf("PresetsInOfficialOrder() failed: %v", err)
	}
	if len(ordered) != len(OfficialUILabelOrder) {
		t.Fatalf("ordered length = %d, want %d", len(ordered), len(OfficialUILabelOrder))
	}

	seen := make(map[uint16]int, len(ordered))
	for i, wantPreset := range ordered {
		wantPos := i + 1
		_, _, crc, err := PresetImage(wantPreset.Index)
		if err != nil {
			t.Fatalf("PresetImage(%d) failed: %v", wantPreset.Index, err)
		}
		if prevPos, dup := seen[crc]; dup {
			t.Fatalf("duplicate CRC 0x%04X for position %d and %d", crc, prevPos, wantPos)
		}
		seen[crc] = wantPos

		gotPos, gotPreset, ok := PresetByCRC(crc)
		t.Logf("pos=%d index=%d file=%s name=%s crc=0x%04X", wantPos, wantPreset.Index, wantPreset.File, wantPreset.Name, crc)
		if !ok {
			t.Fatalf("PresetByCRC(0x%04X) ok = false for position %d (%s)", crc, wantPos, wantPreset.File)
		}
		if gotPos != wantPos {
			t.Fatalf("PresetByCRC(0x%04X) pos = %d, want %d", crc, gotPos, wantPos)
		}
		if gotPreset != wantPreset {
			t.Fatalf("PresetByCRC(0x%04X) preset = %#v, want %#v", crc, gotPreset, wantPreset)
		}
	}

	if pos, _, ok := PresetByCRC(0x0000); ok {
		t.Fatalf("PresetByCRC(0x0000) unexpectedly matched pos %d", pos)
	}
}
