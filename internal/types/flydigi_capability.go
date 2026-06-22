package types

import "fmt"

const (
	FlyDigiGearCodeQuiet       = 0x0
	FlyDigiGearCodeStandard    = 0x1
	FlyDigiGearCodePerformance = 0x2
	FlyDigiGearCodeExtreme     = 0x3

	FlyDigiMaxGearCodeStandard    = 0x2
	FlyDigiMaxGearCodePerformance = 0x4
	FlyDigiMaxGearCodeExtreme     = 0x6
)

type FlyDigiRuntimeCapability struct {
	Available        bool   `json:"available"`
	GearSettings     uint8  `json:"gearSettings"`
	MaxGearCode      int    `json:"maxGearCode,omitempty"`
	MaxGearLabel     string `json:"maxGearLabel,omitempty"`
	MaxGearIndex     int    `json:"maxGearIndex,omitempty"`
	MaxRPM           int    `json:"maxRpm,omitempty"`
	SelectedGearCode int    `json:"selectedGearCode,omitempty"`
	SelectedGear     string `json:"selectedGear,omitempty"`
	Source           string `json:"source,omitempty"`
	Reason           string `json:"reason,omitempty"`
}

func DecodeFlyDigiRuntimeCapability(fanData *FanData, table []DeviceGearRPM) FlyDigiRuntimeCapability {
	if fanData == nil {
		return FlyDigiRuntimeCapability{Reason: "missingFanData"}
	}
	return DecodeFlyDigiRuntimeCapabilityFromGearSettings(fanData.GearSettings, table)
}

func DecodeFlyDigiRuntimeCapabilityFromGearSettings(gearSettings uint8, table []DeviceGearRPM) FlyDigiRuntimeCapability {
	maxCode := int((gearSettings >> 4) & 0x0F)
	selectedCode := int(gearSettings & 0x0F)
	index, label, ok := flyDigiMaxGearCodeToIndex(maxCode)
	capability := FlyDigiRuntimeCapability{
		Available:        ok,
		GearSettings:     gearSettings,
		MaxGearCode:      maxCode,
		MaxGearLabel:     label,
		MaxGearIndex:     index,
		SelectedGearCode: selectedCode,
		SelectedGear:     flyDigiSelectedGearCodeToLabel(selectedCode),
	}
	if !ok {
		capability.Reason = fmt.Sprintf("unknownMaxGearCode:0x%X", maxCode)
		return capability
	}
	if rpm, tableOK := flyDigiRPMForGearIndex(table, index); tableOK {
		capability.MaxRPM = rpm
		capability.Source = "gearRpmTable"
		return capability
	}
	capability.MaxRPM = FlyDigiDefaultMaxRPMForGearIndex(index)
	capability.Source = "default"
	return capability
}

func FlyDigiClampRPMForCapability(rpm int, capability FlyDigiRuntimeCapability) (int, bool) {
	rpm = ClampRPM(rpm)
	if !capability.Available || capability.MaxRPM <= 0 {
		return rpm, false
	}
	if rpm > capability.MaxRPM {
		return capability.MaxRPM, true
	}
	return rpm, false
}

func FlyDigiIsGearAllowed(gear string, capability FlyDigiRuntimeCapability) bool {
	if !capability.Available || capability.MaxGearIndex <= 0 {
		return true
	}
	idx, ok := GearIndex(gear)
	return ok && idx <= capability.MaxGearIndex
}

func FlyDigiDefaultMaxRPMForGearIndex(index int) int {
	switch index {
	case FlyDigiGearCodeQuiet:
		return DefaultGearRPMForUnit("静音", "高", FanSpeedUnitRPM)
	case FlyDigiGearCodeStandard:
		return DefaultGearRPMForUnit("标准", "高", FanSpeedUnitRPM)
	case FlyDigiGearCodePerformance:
		return DefaultGearRPMForUnit("强劲", "高", FanSpeedUnitRPM)
	case FlyDigiGearCodeExtreme:
		return DefaultGearRPMForUnit("超频", "高", FanSpeedUnitRPM)
	default:
		return 0
	}
}

func flyDigiMaxGearCodeToIndex(code int) (int, string, bool) {
	switch code {
	case FlyDigiMaxGearCodeStandard:
		return FlyDigiGearCodeStandard, "标准", true
	case FlyDigiMaxGearCodePerformance:
		return FlyDigiGearCodePerformance, "强劲", true
	case FlyDigiMaxGearCodeExtreme:
		return FlyDigiGearCodeExtreme, "超频", true
	default:
		return 0, "", false
	}
}

func flyDigiSelectedGearCodeToLabel(code int) string {
	switch code {
	case 0x8:
		return "静音"
	case 0xA:
		return "标准"
	case 0xC:
		return "强劲"
	case 0xE:
		return "超频"
	default:
		return fmt.Sprintf("未知(0x%X)", code)
	}
}

func flyDigiRPMForGearIndex(table []DeviceGearRPM, index int) (int, bool) {
	for _, item := range table {
		if item.Gear == index && item.RPM > 0 {
			return item.RPM, true
		}
	}
	return 0, false
}
