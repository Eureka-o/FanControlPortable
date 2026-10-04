package deviceproto

import (
	"encoding/binary"
	"fmt"
	"strings"
)

const (
	BlackSharkCmdVersion      byte = 0x01
	BlackSharkCmdStatusNotify byte = 0x06
	BlackSharkCmdSetSpeed     byte = 0x24
	BlackSharkCmdGetStatus    byte = 0x25
	BlackSharkCmdRGBEnable    byte = 0x10
	BlackSharkCmdRGBStatus    byte = 0x11
	BlackSharkCmdSetLighting  byte = 0x12
	BlackSharkCmdGetLighting  byte = 0x13
)

const (
	BlackSharkModeColorCycle byte = 0x02
	BlackSharkModeBreathing  byte = 0x03
	BlackSharkModeStatic     byte = 0x04
	BlackSharkModeFlashing   byte = 0x05
	BlackSharkModeRefresh    byte = 0x06
	BlackSharkModeResponse   byte = 0x08
	BlackSharkModeColorFlow  byte = 0x11
)

type BlackSharkFrame struct {
	Command    byte
	Payload    []byte
	Checksum   byte
	ChecksumOK bool
	Raw        []byte
}

func blackSharkChecksum(data []byte) byte {
	var sum byte
	for _, value := range data {
		sum += value
	}
	return sum
}

// BuildBlackSharkFrame follows the BRB02 short-frame wire format.
func BuildBlackSharkFrame(command byte, params ...byte) []byte {
	return buildBlackSharkFrameWithLength(byte(3+len(params)), command, params...)
}

func buildBlackSharkFrameWithLength(length byte, command byte, params ...byte) []byte {
	body := make([]byte, 0, int(length)+1)
	body = append(body, 0xA5, length, command)
	body = append(body, params...)
	return append(body, blackSharkChecksum(body))
}

func ParseBlackSharkFrame(data []byte) (BlackSharkFrame, bool) {
	if len(data) < 4 || data[0] != 0xA5 {
		return BlackSharkFrame{}, false
	}
	raw := append([]byte(nil), data...)
	for len(raw) > 4 && raw[len(raw)-1] == 0 {
		raw = raw[:len(raw)-1]
	}
	declared := int(raw[1])
	checksumIndex := -1
	for _, candidate := range []int{declared, declared - 1, len(raw) - 1} {
		if candidate < 3 || candidate >= len(raw) {
			continue
		}
		if blackSharkChecksum(raw[:candidate]) == raw[candidate] {
			checksumIndex = candidate
			break
		}
	}
	payloadEnd := len(raw)
	checksum := byte(0)
	checksumOK := checksumIndex >= 0
	if checksumOK {
		payloadEnd = checksumIndex
		checksum = raw[checksumIndex]
	}
	if payloadEnd < 3 {
		return BlackSharkFrame{}, false
	}
	return BlackSharkFrame{
		Command:    raw[2],
		Payload:    append([]byte(nil), raw[3:payloadEnd]...),
		Checksum:   checksum,
		ChecksumOK: checksumOK,
		Raw:        raw,
	}, true
}

func BuildBlackSharkGetVersion() []byte { return BuildBlackSharkFrame(BlackSharkCmdVersion) }

func BuildBlackSharkGetStatus() []byte { return BuildBlackSharkFrame(BlackSharkCmdGetStatus) }

func BuildBlackSharkSetSpeed(rpm int) []byte {
	if rpm < 0 {
		rpm = 0
	}
	if rpm > 4000 {
		rpm = 4000
	}
	// v3.0.3 write frames use the device's long-form LEN field.
	return buildBlackSharkFrameWithLength(0x09, BlackSharkCmdSetSpeed, 0x00, 0x00, 0x01, byte(rpm), byte(rpm>>8))
}

// BlackSharkModeName uses the same user-facing work-mode labels as FlyDigi.
func BlackSharkModeName(mode byte) string {
	return ModeName(mode)
}

func BuildBlackSharkSetRGBEnabled(enabled bool) []byte {
	value := byte(0)
	if enabled {
		value = 1
	}
	return BuildBlackSharkFrame(BlackSharkCmdRGBEnable, value)
}

func BuildBlackSharkGetLighting() []byte { return BuildBlackSharkFrame(BlackSharkCmdGetLighting) }

func blackSharkLightingMode(mode string) (byte, bool) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "blackshark_color_cycle", "rotation":
		return BlackSharkModeColorCycle, true
	case "blackshark_breathing", "breathing", "smart_temp":
		return BlackSharkModeBreathing, true
	case "blackshark_static", "static_single":
		return BlackSharkModeStatic, true
	case "static_multi":
		// BRB02 has no confirmed multi-zone static mode; use its native
		// color-cycle mode while keeping the common frontend option available.
		return BlackSharkModeColorCycle, true
	case "blackshark_flashing":
		return BlackSharkModeFlashing, true
	case "blackshark_refresh":
		return BlackSharkModeRefresh, true
	case "blackshark_response":
		return BlackSharkModeResponse, true
	case "blackshark_color_flow", "flowing":
		return BlackSharkModeColorFlow, true
	default:
		return 0, false
	}
}

func blackSharkLightingSpeed(speed string) uint16 {
	switch strings.ToLower(strings.TrimSpace(speed)) {
	case "fast":
		return 1000
	case "slow":
		return 5000
	default:
		return 3000
	}
}

func BuildBlackSharkSetLighting(modeName, speedName string, brightness int, r, g, b byte) ([]byte, error) {
	mode, ok := blackSharkLightingMode(modeName)
	if !ok {
		return nil, fmt.Errorf("unsupported Black Shark lighting mode %q", modeName)
	}
	if brightness < 0 {
		brightness = 0
	}
	if brightness > 100 {
		brightness = 100
	}
	colorMode := byte(0x01)
	if mode == BlackSharkModeColorCycle || mode == BlackSharkModeColorFlow {
		colorMode = 0x0A
	}
	speed := make([]byte, 2)
	binary.LittleEndian.PutUint16(speed, blackSharkLightingSpeed(speedName))
	return buildBlackSharkFrameWithLength(
		0x0C,
		BlackSharkCmdSetLighting,
		mode, speed[0], speed[1], byte(brightness), colorMode, r, g, b,
	), nil
}

func ParseBlackSharkStatus(frame BlackSharkFrame) (currentRPM int, flag byte, ok bool) {
	if frame.Command == BlackSharkCmdStatusNotify && len(frame.Payload) >= 3 {
		return int(binary.LittleEndian.Uint16(frame.Payload[:2])), frame.Payload[2], true
	}
	if frame.Command == BlackSharkCmdGetStatus && len(frame.Payload) >= 5 {
		return int(binary.LittleEndian.Uint16(frame.Payload[3:5])), frame.Payload[2], true
	}
	return 0, 0, false
}
