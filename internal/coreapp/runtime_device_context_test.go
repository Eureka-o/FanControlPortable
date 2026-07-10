package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestDeviceCurveScopeKeyForNativeRuntimeProfile(t *testing.T) {
	got := deviceCurveScopeKeyForProfile(types.FlyDigiBS1Profile())
	want := types.DeviceTransportBLE + deviceCurveScopeSeparator + types.FlyDigiBS1ProfileID
	if got != want {
		t.Fatalf("deviceCurveScopeKeyForProfile() = %q, want %q", got, want)
	}
}

func TestCompatibilityConnectionEnabledOnlyForManualCompatibilityTransports(t *testing.T) {
	tests := []struct {
		name string
		cfg  types.AppConfig
		want bool
	}{
		{
			name: "wifi enabled",
			cfg: func() types.AppConfig {
				cfg := types.GetDefaultConfig(false)
				cfg.DeviceTransport = types.DeviceTransportWiFi
				cfg.ActiveDeviceProfileID = types.DefaultWiFiPercentProfileID
				cfg.WiFiCompatibilityEnabled = true
				return cfg
			}(),
			want: true,
		},
		{
			name: "wifi disabled",
			cfg: func() types.AppConfig {
				cfg := types.GetDefaultConfig(false)
				cfg.DeviceTransport = types.DeviceTransportWiFi
				cfg.ActiveDeviceProfileID = types.DefaultWiFiPercentProfileID
				cfg.WiFiCompatibilityEnabled = false
				return cfg
			}(),
			want: false,
		},
		{
			name: "serial enabled",
			cfg: func() types.AppConfig {
				serial := testSerialDeviceProfile()
				cfg := types.GetDefaultConfig(false)
				cfg.DeviceTransport = types.DeviceTransportSerial
				cfg.ActiveDeviceProfileID = serial.ID
				cfg.SerialCompatibilityEnabled = true
				cfg.DeviceProfiles = append(cfg.DeviceProfiles, serial)
				cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
					types.DeviceTransportSerial: serial.ID,
				}
				return cfg
			}(),
			want: true,
		},
		{
			name: "native persisted state normalizes away from manual compatibility",
			cfg: func() types.AppConfig {
				cfg := types.GetDefaultConfig(false)
				cfg.DeviceTransport = types.DeviceTransportBLE
				cfg.ActiveDeviceProfileID = types.FlyDigiBS1ProfileID
				cfg.WiFiCompatibilityEnabled = false
				cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
					types.DeviceTransportBLE: types.FlyDigiBS1ProfileID,
				}
				return cfg
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compatibilityConnectionEnabled(tt.cfg); got != tt.want {
				t.Fatalf("compatibilityConnectionEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldTryDynamicWiFiCompatibilityOnlyForEnabledWiFiCompatibility(t *testing.T) {
	wifi := types.DefaultWiFiPercentProfile("10.0.0.25")
	serial := testSerialDeviceProfile()
	tests := []struct {
		name string
		cfg  types.AppConfig
		want bool
	}{
		{
			name: "wifi dynamic compatibility enabled",
			cfg: func() types.AppConfig {
				cfg := types.GetDefaultConfig(false)
				cfg.DeviceTransport = types.DeviceTransportWiFi
				cfg.ActiveDeviceProfileID = wifi.ID
				cfg.WiFiCompatibilityEnabled = true
				cfg.WiFiDynamicIPCompatibilityEnabled = true
				cfg.DeviceProfiles = []types.DeviceProfile{wifi, serial}
				cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
					types.DeviceTransportWiFi: wifi.ID,
				}
				return cfg
			}(),
			want: true,
		},
		{
			name: "wifi dynamic compatibility disabled",
			cfg: func() types.AppConfig {
				cfg := types.GetDefaultConfig(false)
				cfg.DeviceTransport = types.DeviceTransportWiFi
				cfg.ActiveDeviceProfileID = wifi.ID
				cfg.WiFiCompatibilityEnabled = true
				cfg.WiFiDynamicIPCompatibilityEnabled = false
				cfg.DeviceProfiles = []types.DeviceProfile{wifi, serial}
				cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
					types.DeviceTransportWiFi: wifi.ID,
				}
				return cfg
			}(),
			want: false,
		},
		{
			name: "wifi compatibility disabled",
			cfg: func() types.AppConfig {
				cfg := types.GetDefaultConfig(false)
				cfg.DeviceTransport = types.DeviceTransportWiFi
				cfg.ActiveDeviceProfileID = wifi.ID
				cfg.WiFiCompatibilityEnabled = false
				cfg.DeviceProfiles = []types.DeviceProfile{wifi, serial}
				cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
					types.DeviceTransportWiFi: wifi.ID,
				}
				return cfg
			}(),
			want: false,
		},
		{
			name: "serial compatibility active",
			cfg: func() types.AppConfig {
				cfg := types.GetDefaultConfig(false)
				cfg.DeviceTransport = types.DeviceTransportSerial
				cfg.ActiveDeviceProfileID = serial.ID
				cfg.WiFiCompatibilityEnabled = true
				cfg.SerialCompatibilityEnabled = true
				cfg.DeviceProfiles = []types.DeviceProfile{wifi, serial}
				cfg.ActiveDeviceProfileIDsByTransport = map[string]string{
					types.DeviceTransportWiFi:   wifi.ID,
					types.DeviceTransportSerial: serial.ID,
				}
				return cfg
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldTryDynamicWiFiCompatibility(tt.cfg); got != tt.want {
				t.Fatalf("shouldTryDynamicWiFiCompatibility() = %v, want %v", got, tt.want)
			}
		})
	}
}
