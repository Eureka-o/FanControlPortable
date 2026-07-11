package deviceprofileexec

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestDefaultBLEAdapterLeaseWaitIsCancelable(t *testing.T) {
	release, err := acquireDefaultBLEAdapter(context.Background())
	if err != nil {
		t.Fatalf("first adapter lease failed: %v", err)
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := acquireDefaultBLEAdapter(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("second adapter lease error = %v, want context canceled", err)
	}
}

func TestNormalizeAndMatchBLEDevicesMatchesNameAndService(t *testing.T) {
	devices := []types.BLEDeviceInfo{
		{
			Address:      "AA:BB:CC:00:00:01",
			Name:         "DIY Cooler",
			RSSI:         -42,
			ServiceUUIDs: []string{"0000fff0-0000-1000-8000-00805f9b34fb"},
		},
		{
			Address:      "AA:BB:CC:00:00:02",
			Name:         "Keyboard",
			RSSI:         -20,
			ServiceUUIDs: []string{"180f"},
		},
	}

	got := NormalizeAndMatchBLEDevices(devices, types.BLEScanParams{
		NameFilter:  "cooler",
		ServiceUUID: "fff0",
	})

	if len(got) != 2 {
		t.Fatalf("got %d devices, want 2", len(got))
	}
	if got[0].Address != "AA:BB:CC:00:00:01" || !got[0].Matched {
		t.Fatalf("first match = %#v, want DIY cooler", got[0])
	}
	if got[0].MatchScore != 90 {
		t.Fatalf("match score = %d, want 90", got[0].MatchScore)
	}
	if got[0].SuggestedNameFilter != "DIY Cooler" || got[0].SuggestedServiceUUID == "" {
		t.Fatalf("suggestions not filled: %#v", got[0])
	}
}

func TestNormalizeAndMatchBLEDevicesMatchesCharacteristicsWhenKnown(t *testing.T) {
	devices := []types.BLEDeviceInfo{
		{
			Address:                   "AA:BB:CC:00:00:03",
			Name:                      "GATT Cooler",
			RSSI:                      -55,
			WriteCharacteristicUUIDs:  []string{"fff2"},
			NotifyCharacteristicUUIDs: []string{"0000fff1-0000-1000-8000-00805f9b34fb"},
		},
	}

	got := NormalizeAndMatchBLEDevices(devices, types.BLEScanParams{
		WriteCharacteristicUUID:  "0000fff2-0000-1000-8000-00805f9b34fb",
		NotifyCharacteristicUUID: "fff1",
	})

	if len(got) != 1 || !got[0].Matched {
		t.Fatalf("expected characteristic match, got %#v", got)
	}
	if got[0].MatchScore != 50 {
		t.Fatalf("match score = %d, want 50", got[0].MatchScore)
	}
	if got[0].SuggestedWriteCharacteristic != "fff2" || got[0].SuggestedNotifyCharacteristic == "" {
		t.Fatalf("characteristic suggestions not filled: %#v", got[0])
	}
}

func TestNormalizeAndMatchBLEDevicesMatchesProfiles(t *testing.T) {
	profile := types.DeviceProfile{
		ID:          "user.ble.percent",
		DisplayName: "DIY BLE percent",
		Transport:   types.DeviceTransportBLE,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			BLENameFilter:           "cooler",
			BLEServiceUUID:          "fff0",
			BLEWriteCharacteristic:  "fff2",
			BLENotifyCharacteristic: "fff1",
		},
		Capabilities: types.DeviceCapabilities{
			Transport:         types.DeviceTransportBLE,
			SpeedUnit:         types.FanSpeedUnitPercent,
			SpeedRange:        types.DefaultPercentSpeedRange(),
			SupportsReadState: true,
			SupportsSetSpeed:  true,
		},
	}

	got := NormalizeAndMatchBLEDevices([]types.BLEDeviceInfo{
		{
			Address:                   "AA:BB:CC:00:00:04",
			Name:                      "DIY Cooler v2",
			RSSI:                      -45,
			ServiceUUIDs:              []string{"0000fff0-0000-1000-8000-00805f9b34fb"},
			WriteCharacteristicUUIDs:  []string{"fff2"},
			NotifyCharacteristicUUIDs: []string{"fff1"},
		},
	}, types.BLEScanParams{Profiles: []types.DeviceProfile{profile}})

	if len(got) != 1 || got[0].MatchedProfileID != profile.ID {
		t.Fatalf("profile match = %#v, want %s", got, profile.ID)
	}
	if got[0].MatchedProfileDisplayName != profile.DisplayName {
		t.Fatalf("matched display = %q, want %q", got[0].MatchedProfileDisplayName, profile.DisplayName)
	}
}

func TestNormalizeAndMatchBLEDevicesOnlyMatched(t *testing.T) {
	got := NormalizeAndMatchBLEDevices([]types.BLEDeviceInfo{
		{Address: "AA:BB:CC:00:00:05", Name: "Cooler", RSSI: -70},
		{Address: "AA:BB:CC:00:00:06", Name: "Other", RSSI: -30},
	}, types.BLEScanParams{NameFilter: "cooler", OnlyMatched: true})

	if len(got) != 1 || got[0].Name != "Cooler" {
		t.Fatalf("got %#v, want only Cooler", got)
	}
}

func TestScanBLEDevicesWithScannerReturnsEmptyResult(t *testing.T) {
	got, err := ScanBLEDevicesWithScanner(context.Background(), BLEScannerFunc(func(ctx context.Context, params types.BLEScanParams) ([]types.BLEDeviceInfo, error) {
		return nil, nil
	}), types.BLEScanParams{TimeoutMs: 10})
	if err != nil {
		t.Fatalf("ScanBLEDevicesWithScanner returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %#v, want empty result", got)
	}
}

func TestMatchedBLEAdvertisementStopsScanWithoutWaitingForTimeout(t *testing.T) {
	stopScan := make(chan struct{}, 1)
	started := time.Now()
	devices, err := scanBLEAdvertisements(
		context.Background(),
		types.BLEScanParams{NameFilter: "BS1", OnlyMatched: true},
		func(report func(types.BLEDeviceInfo)) error {
			report(types.BLEDeviceInfo{Address: "AA:BB:CC:00:00:07", Name: "FlyDigi BS1"})
			select {
			case <-stopScan:
				return nil
			case <-time.After(500 * time.Millisecond):
				return errors.New("matched advertisement did not stop scan")
			}
		},
		func() error {
			select {
			case stopScan <- struct{}{}:
			default:
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("scanBLEAdvertisements returned error: %v", err)
	}
	if elapsed := time.Since(started); elapsed >= 200*time.Millisecond {
		t.Fatalf("matched BLE scan took %v, want immediate stop", elapsed)
	}
	if len(devices) != 1 || devices[0].Name != "FlyDigi BS1" {
		t.Fatalf("devices = %#v, want matched BS1", devices)
	}
}
