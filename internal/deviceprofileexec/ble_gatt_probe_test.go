package deviceprofileexec

import (
	"context"
	"reflect"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestProbeBLEGATTWithProberSuggestsWritableAndNotifyCharacteristics(t *testing.T) {
	result, err := ProbeBLEGATTWithProber(context.Background(), BLEGATTProberFunc(func(ctx context.Context, params types.BLEGATTProbeParams) (*types.BLEGATTProbeResult, error) {
		if params.TimeoutMs <= 0 {
			t.Fatal("expected normalized timeout")
		}
		if params.Profile.Transport != types.DeviceTransportBLE {
			t.Fatalf("profile transport = %q, want ble", params.Profile.Transport)
		}
		return &types.BLEGATTProbeResult{
			Address: "AA:BB:CC:DD:EE:01",
			Services: []types.BLEGATTServiceInfo{
				{
					UUID: "FFF0",
					Characteristics: []types.BLEGATTCharacteristicInfo{
						{UUID: "FFF1", Properties: []string{"notify", "read"}},
						{UUID: "FFF2", Properties: []string{"writeWithoutResponse", "write"}},
					},
				},
			},
		}, nil
	}), types.BLEGATTProbeParams{
		Profile: types.DeviceProfile{
			DisplayName: "DIY BLE",
			Transport:   types.DeviceTransportBLE,
			SpeedUnit:   types.FanSpeedUnitPercent,
		},
	})
	if err != nil {
		t.Fatalf("ProbeBLEGATTWithProber() error = %v", err)
	}
	if result.SuggestedServiceUUID != "fff0" {
		t.Fatalf("suggested service = %q, want fff0", result.SuggestedServiceUUID)
	}
	if result.SuggestedWriteCharacteristic != "fff2" {
		t.Fatalf("suggested write = %q, want fff2", result.SuggestedWriteCharacteristic)
	}
	if result.SuggestedNotifyCharacteristic != "fff1" {
		t.Fatalf("suggested notify = %q, want fff1", result.SuggestedNotifyCharacteristic)
	}
	if !result.Services[0].Characteristics[0].CanRead || !result.Services[0].Characteristics[0].CanNotify {
		t.Fatalf("notify characteristic flags not applied: %#v", result.Services[0].Characteristics[0])
	}
}

func TestNormalizeBLEGATTProbeResultKeepsPreferredCharacteristics(t *testing.T) {
	result := NormalizeBLEGATTProbeResult(&types.BLEGATTProbeResult{
		Services: []types.BLEGATTServiceInfo{
			{
				UUID: "180f",
				Characteristics: []types.BLEGATTCharacteristicInfo{
					{UUID: "2a19", Properties: []string{"read"}},
				},
			},
			{
				UUID: "fff0",
				Characteristics: []types.BLEGATTCharacteristicInfo{
					{UUID: "fff3", Properties: []string{"notify"}},
					{UUID: "fff2", Properties: []string{"write"}},
				},
			},
		},
	}, types.BLEGATTProbeParams{
		ServiceUUID: "FFF0",
		Profile: types.DeviceProfile{
			Transport: types.DeviceTransportBLE,
			Connection: types.DeviceConnectionSettings{
				BLEWriteCharacteristic:  "FFF2",
				BLENotifyCharacteristic: "FFF3",
			},
		},
	})

	if result.SuggestedServiceUUID != "fff0" || result.SuggestedWriteCharacteristic != "fff2" || result.SuggestedNotifyCharacteristic != "fff3" {
		t.Fatalf("suggestions = %#v, want fff0/fff2/fff3", result)
	}
	gotOrder := []string{result.Services[0].UUID, result.Services[1].UUID}
	if !reflect.DeepEqual(gotOrder, []string{"180f", "fff0"}) {
		t.Fatalf("service order = %#v, want normalized sort", gotOrder)
	}
}
