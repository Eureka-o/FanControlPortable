package types

import (
	"fmt"
	"reflect"
	"strings"
)

const (
	FlyDigiBS1ProfileID    = "builtin.flydigi.bs1.ble.rpm"
	FlyDigiBS2ProfileID    = "builtin.flydigi.bs2.hid.rpm"
	FlyDigiBS2PROProfileID = "builtin.flydigi.bs2pro.hid.rpm"
	FlyDigiBS3ProfileID    = "builtin.flydigi.bs3.hid.rpm"
	FlyDigiBS3PROProfileID = "builtin.flydigi.bs3pro.hid.rpm"

	FlyDigiFrameProtocolTemplateID  = "flydigi-frame-v1"
	FlyDigiHIDRPMProtocolTemplateID = "flydigi-hid-rpm-v1"
	FlyDigiBS1BLEProtocolTemplateID = "flydigi-bs1-ble-rpm-v1"

	FlyDigiHIDVendorID     uint16 = 0x37D7
	FlyDigiBS2ProductID    uint16 = 0x1001
	FlyDigiBS2PROProductID uint16 = 0x1002
	FlyDigiBS3ProductID    uint16 = 0x1003
	FlyDigiBS3PROProductID uint16 = 0x1004
	flyDigiBS1ServiceUUID         = "fff0"
	flyDigiBS1WriteUUID           = "fff2"
	flyDigiBS1NotifyUUID          = "fff1"
	flyDigiVendorName             = "飞智（FlyDigi）"
)

type flyDigiProfileDefinition struct {
	id                     string
	model                  string
	transport              string
	protocolTemplateID     string
	productID              uint16
	referenceCompatible    bool
	supportsGearLight      bool
	supportsLighting       bool
	supportsBrightness     bool
	supportsScreen         bool
	supportsPowerOnStart   bool
	supportsSmartStartStop bool
	connection             DeviceConnectionSettings
}

func FlyDigiBS1Profile() DeviceProfile {
	return flyDigiRPMProfile(flyDigiProfileDefinition{
		id:                     FlyDigiBS1ProfileID,
		model:                  "BS1",
		transport:              DeviceTransportBLE,
		protocolTemplateID:     FlyDigiBS1BLEProtocolTemplateID,
		supportsPowerOnStart:   true,
		supportsLighting:       false,
		supportsSmartStartStop: false,
		connection: DeviceConnectionSettings{
			BLENameFilter:           "BS1",
			BLEServiceUUID:          flyDigiBS1ServiceUUID,
			BLEWriteCharacteristic:  flyDigiBS1WriteUUID,
			BLENotifyCharacteristic: flyDigiBS1NotifyUUID,
			BLEWriteWithResponse:    false,
		},
	})
}

func FlyDigiBS2Profile() DeviceProfile {
	return flyDigiHIDProfile(FlyDigiBS2ProfileID, "BS2", FlyDigiBS2ProductID, false)
}

func FlyDigiBS2PROProfile() DeviceProfile {
	return flyDigiHIDProfile(FlyDigiBS2PROProfileID, "BS2PRO", FlyDigiBS2PROProductID, false)
}

func FlyDigiBS3Profile() DeviceProfile {
	return flyDigiHIDProfile(FlyDigiBS3ProfileID, "BS3", FlyDigiBS3ProductID, true)
}

func FlyDigiBS3PROProfile() DeviceProfile {
	return flyDigiHIDProfile(FlyDigiBS3PROProfileID, "BS3PRO", FlyDigiBS3PROProductID, true)
}

func FlyDigiBuiltInProfiles() []DeviceProfile {
	return []DeviceProfile{
		FlyDigiBS1Profile(),
		FlyDigiBS2Profile(),
		FlyDigiBS2PROProfile(),
		FlyDigiBS3Profile(),
		FlyDigiBS3PROProfile(),
	}
}

func BuiltInDeviceProfiles(endpoint string) []DeviceProfile {
	return []DeviceProfile{DefaultWiFiPercentProfile(endpoint)}
}

func IsFlyDigiDeviceProfileID(profileID string) bool {
	switch strings.TrimSpace(profileID) {
	case FlyDigiBS1ProfileID,
		FlyDigiBS2ProfileID,
		FlyDigiBS2PROProfileID,
		FlyDigiBS3ProfileID,
		FlyDigiBS3PROProfileID:
		return true
	default:
		return false
	}
}

func IsBuiltInDeviceProfileID(profileID string) bool {
	switch strings.TrimSpace(profileID) {
	case DefaultWiFiPercentProfileID,
		DefaultWiFiPercentTemplateProfileID,
		LegacyRPMProfileID:
		return true
	default:
		return false
	}
}

func FlyDigiProfileIDForModel(model string) string {
	switch strings.ToUpper(strings.TrimSpace(model)) {
	case "BS1":
		return FlyDigiBS1ProfileID
	case "BS2":
		return FlyDigiBS2ProfileID
	case "BS2PRO":
		return FlyDigiBS2PROProfileID
	case "BS3":
		return FlyDigiBS3ProfileID
	case "BS3PRO":
		return FlyDigiBS3PROProfileID
	default:
		return ""
	}
}

func FlyDigiProfileIDForHIDProductID(productID uint16) string {
	switch productID {
	case FlyDigiBS2ProductID:
		return FlyDigiBS2ProfileID
	case FlyDigiBS2PROProductID:
		return FlyDigiBS2PROProfileID
	case FlyDigiBS3ProductID:
		return FlyDigiBS3ProfileID
	case FlyDigiBS3PROProductID:
		return FlyDigiBS3PROProfileID
	default:
		return ""
	}
}

func flyDigiHIDProfile(id, model string, productID uint16, referenceCompatible bool) DeviceProfile {
	return flyDigiRPMProfile(flyDigiProfileDefinition{
		id:                     id,
		model:                  model,
		transport:              DeviceTransportHID,
		protocolTemplateID:     FlyDigiHIDRPMProtocolTemplateID,
		productID:              productID,
		referenceCompatible:    referenceCompatible,
		supportsGearLight:      true,
		supportsPowerOnStart:   true,
		supportsLighting:       true,
		supportsBrightness:     true,
		supportsScreen:         false,
		supportsSmartStartStop: true,
	})
}

func flyDigiRPMProfile(def flyDigiProfileDefinition) DeviceProfile {
	displayName := flyDigiVendorName + def.model
	caps := DeviceCapabilities{
		ProfileID:              def.id,
		DisplayName:            displayName,
		Transport:              def.transport,
		SpeedUnit:              FanSpeedUnitRPM,
		SpeedRange:             DefaultRPMSpeedRange(),
		SupportsReadState:      true,
		SupportsSetSpeed:       true,
		SupportsManualGears:    true,
		SupportsCustomSpeed:    true,
		SupportsDebugFrames:    false,
		SupportsRawCommands:    false,
		SupportsGearLight:      def.supportsGearLight,
		SupportsLighting:       def.supportsLighting,
		SupportsBrightness:     def.supportsBrightness,
		SupportsScreen:         def.supportsScreen,
		SupportsPowerOnStart:   def.supportsPowerOnStart,
		SupportsSmartStartStop: def.supportsSmartStartStop,
	}
	return DeviceProfile{
		ID:           def.id,
		DisplayName:  displayName,
		Vendor:       flyDigiVendorName,
		Model:        def.model,
		Notes:        flyDigiProfileNotes(def),
		BuiltIn:      true,
		Transport:    def.transport,
		SpeedUnit:    caps.SpeedUnit,
		SpeedRange:   caps.SpeedRange,
		Connection:   def.connection,
		Capabilities: caps,
	}
}

func flyDigiProfileNotes(def flyDigiProfileDefinition) string {
	if def.transport == DeviceTransportBLE {
		return fmt.Sprintf(
			"Protocol %s over BLE GATT; service %s, write %s, notify %s. BS1 supports speed control, state read, manual gears, custom RPM, and power-on-start; smart start/stop and lighting stay disabled.",
			def.protocolTemplateID,
			flyDigiBS1ServiceUUID,
			flyDigiBS1WriteUUID,
			flyDigiBS1NotifyUUID,
		)
	}
	note := fmt.Sprintf(
		"Protocol %s over HID; VID 0x%04X, PID 0x%04X. Optional device functions are enabled by this built-in profile whitelist.",
		def.protocolTemplateID,
		FlyDigiHIDVendorID,
		def.productID,
	)
	if def.referenceCompatible {
		note += " BS3/BS3PRO optional functions are reference-compatible and still need more real-device feedback."
	}
	return note
}

func ensureBuiltInDeviceProfiles(cfg *AppConfig) bool {
	if cfg == nil {
		return false
	}
	changed := false
	for _, builtIn := range BuiltInDeviceProfiles(cfg.FanControlDeviceIp) {
		builtIn = NormalizeDeviceProfile(builtIn, cfg.FanControlDeviceIp)
		idx := -1
		for i := range cfg.DeviceProfiles {
			if cfg.DeviceProfiles[i].ID == builtIn.ID {
				idx = i
				break
			}
		}
		if idx < 0 {
			cfg.DeviceProfiles = append(cfg.DeviceProfiles, builtIn)
			changed = true
			continue
		}
		if !reflect.DeepEqual(cfg.DeviceProfiles[idx], builtIn) {
			cfg.DeviceProfiles[idx] = builtIn
			changed = true
		}
	}
	return changed
}
