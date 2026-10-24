package coreapp

import (
	"testing"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/deviceproto"
	"github.com/Eureka-o/FanControlPortable/internal/types"
)

func TestBlackSharkScreenItemsDefaultAndLimit(t *testing.T) {
	defaults := types.GetDefaultConfig(false).BlackSharkScreenItems
	if len(defaults) != 4 {
		t.Fatalf("default items = %#v, want all four", defaults)
	}

	items := types.NormalizeBlackSharkScreenItems([]string{
		"unknown", types.BlackSharkScreenItemTime,
		types.BlackSharkScreenItemGPUPower, types.BlackSharkScreenItemGPUTemperature,
		types.BlackSharkScreenItemCPUTemperature, types.BlackSharkScreenItemTime,
	})
	want := []string{
		types.BlackSharkScreenItemTime, types.BlackSharkScreenItemGPUPower,
		types.BlackSharkScreenItemGPUTemperature, types.BlackSharkScreenItemCPUTemperature,
	}
	if len(items) != len(want) {
		t.Fatalf("normalized items = %#v, want %#v", items, want)
	}
	for i := range want {
		if items[i] != want[i] {
			t.Fatalf("normalized items = %#v, want %#v", items, want)
		}
	}
}

func TestBlackSharkHostInfoEntriesAndFrame(t *testing.T) {
	now := time.Date(2026, time.October, 6, 13, 42, 0, 0, time.Local)
	cfg := types.GetDefaultConfig(false)
	temp := types.TemperatureData{CPUTemp: 61, GPUTemp: 72, GPUPowerWatts: 183.4}
	entries := blackSharkHostInfoEntries(cfg, temp, now)
	wantIDs := []byte{0x00, 0x01, 0x02, 0x03, 0x05, 0x06, 0x07}
	if len(entries) != len(wantIDs) {
		t.Fatalf("entries = %#v, want IDs %#v", entries, wantIDs)
	}
	for i, id := range wantIDs {
		if entries[i].ID != id {
			t.Fatalf("entry %d ID = %#x, want %#x", i, entries[i].ID, id)
		}
	}
	if entries[0].Value != 61 || entries[1].Value != 72 || entries[2].Value != 183 ||
		entries[3].Value != 0 || entries[4].Value != 74 || entries[5].Value != 72 || entries[6].Value != 822 {
		t.Fatalf("entries = %#v, values do not match telemetry/keepalive/time", entries)
	}

	frame := deviceproto.BuildBlackSharkHostInfo(entries)
	parsed, ok := deviceproto.ParseBlackSharkFrame(frame)
	if !ok || parsed.Command != deviceproto.BlackSharkCmdHostInfo || len(frame) != 26 {
		t.Fatalf("frame = % X, parsed=%+v ok=%v", frame, parsed, ok)
	}
}

func TestBlackSharkHostInfoKeepsSlotsWhenUsersSelectNone(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.BlackSharkScreenItems = []string{}
	entries := blackSharkHostInfoEntries(cfg, types.TemperatureData{GPUTemp: 55}, time.Time{})
	wantIDs := []byte{0x03, 0x05, 0x06}
	if len(entries) != len(wantIDs) {
		t.Fatalf("entries = %#v, want only keepalive slots", entries)
	}
	for i, id := range wantIDs {
		if entries[i].ID != id {
			t.Fatalf("entry %d ID = %#x, want %#x", i, entries[i].ID, id)
		}
	}
}
