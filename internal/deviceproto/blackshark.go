package deviceproto

import (
	"encoding/binary"
	"fmt"
	"image"
	"strings"
)

const (
	BlackSharkCmdVersion       byte = 0x01
	BlackSharkCmdStatusNotify  byte = 0x06
	BlackSharkCmdSetSpeed      byte = 0x24
	BlackSharkCmdGetStatus     byte = 0x25
	BlackSharkCmdRGBEnable     byte = 0x10
	BlackSharkCmdRGBStatus     byte = 0x11
	BlackSharkCmdSetLighting   byte = 0x12
	BlackSharkCmdGetLighting   byte = 0x13
	BlackSharkCmdImageMetadata byte = 0xC4
	BlackSharkCmdImageCommit   byte = 0xC5
	BlackSharkCmdImageBlock    byte = 0xA4
)

const (
	BlackSharkImageWidth      = 428
	BlackSharkImageHeight     = 142
	BlackSharkImageBytes      = BlackSharkImageWidth * BlackSharkImageHeight * 2
	blackSharkImageBlockBytes = 58
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

// BlackSharkRGB565 converts a 428x142 image to the display's RGB565 big-endian canvas.
func BlackSharkRGB565(src image.Image) ([]byte, error) {
	if src == nil {
		return nil, fmt.Errorf("black shark image is nil")
	}
	bounds := src.Bounds()
	if bounds.Dx() != BlackSharkImageWidth || bounds.Dy() != BlackSharkImageHeight {
		return nil, fmt.Errorf("black shark image must be %dx%d, got %dx%d", BlackSharkImageWidth, BlackSharkImageHeight, bounds.Dx(), bounds.Dy())
	}
	canvas := make([]byte, 0, BlackSharkImageBytes)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := src.At(x, y).RGBA()
			pixel := uint16(r>>11)<<11 | uint16(g>>10)<<5 | uint16(b>>11)
			canvas = append(canvas, byte(pixel>>8), byte(pixel))
		}
	}
	return canvas, nil
}

// BlackSharkCRC16XMODEM returns CRC-16/XMODEM (poly 0x1021, init 0).
func BlackSharkCRC16XMODEM(data []byte) uint16 {
	var crc uint16
	for _, value := range data {
		crc ^= uint16(value) << 8
		for bit := 0; bit < 8; bit++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func validateBlackSharkCanvas(canvas []byte) error {
	if len(canvas) != BlackSharkImageBytes {
		return fmt.Errorf("black shark canvas must contain %d bytes, got %d", BlackSharkImageBytes, len(canvas))
	}
	return nil
}

// BuildBlackSharkImageMetadata builds the C4 image handshake frame.
func BuildBlackSharkImageMetadata(canvas []byte) ([]byte, error) {
	if err := validateBlackSharkCanvas(canvas); err != nil {
		return nil, err
	}
	metadata := []byte{0x18, 0xDE, 0xC0, 0x6A, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(metadata[4:8], uint32(len(canvas)))
	binary.LittleEndian.PutUint16(metadata[8:10], BlackSharkCRC16XMODEM(canvas))
	return buildBlackSharkFrameWithLength(0x0F, BlackSharkCmdImageMetadata, metadata...), nil
}

// BuildBlackSharkImageDataBlock builds one 64-byte A4 data block. Block 2096 is first.
func BuildBlackSharkImageDataBlock(blockNumber int, data []byte, first bool) ([]byte, error) {
	if blockNumber < 2 || blockNumber > 2096 {
		return nil, fmt.Errorf("black shark image block number %d out of range", blockNumber)
	}
	if len(data) != blackSharkImageBlockBytes {
		return nil, fmt.Errorf("black shark image data block must contain %d bytes, got %d", blackSharkImageBlockBytes, len(data))
	}
	high := byte(blockNumber>>8) & 0x0F
	if first {
		high |= 0x80
	}
	params := append([]byte{byte(blockNumber), high, 0xC6}, data...)
	return buildBlackSharkImageDataFrame(0x40, params), nil
}

// BuildBlackSharkImageTail builds the 48-byte A4 tail frame for block 1.
func BuildBlackSharkImageTail(data []byte) ([]byte, error) {
	if len(data) != 42 {
		return nil, fmt.Errorf("black shark image tail must contain 42 bytes, got %d", len(data))
	}
	params := append([]byte{0x01, 0x00, 0xC6}, data...)
	return buildBlackSharkImageDataFrame(0x30, params), nil
}

func buildBlackSharkImageDataFrame(length byte, params []byte) []byte {
	frame := make([]byte, 0, int(length))
	frame = append(frame, BlackSharkCmdImageBlock, length)
	frame = append(frame, params...)
	return append(frame, blackSharkChecksum(frame))
}

// BuildBlackSharkImageCommit builds the C5 image commit frame.
func BuildBlackSharkImageCommit() []byte {
	return []byte{0xA5, 0x04, BlackSharkCmdImageCommit, 0x6E}
}

// BuildBlackSharkImageUpload builds the complete C4, A4, tail, and C5 sequence.
func BuildBlackSharkImageUpload(canvas []byte) ([][]byte, error) {
	if err := validateBlackSharkCanvas(canvas); err != nil {
		return nil, err
	}
	frames := make([][]byte, 0, 2098)
	metadata, err := BuildBlackSharkImageMetadata(canvas)
	if err != nil {
		return nil, err
	}
	frames = append(frames, metadata)
	for block := 2096; block >= 2; block-- {
		start := (2096 - block) * blackSharkImageBlockBytes
		data, err := BuildBlackSharkImageDataBlock(block, canvas[start:start+blackSharkImageBlockBytes], block == 2096)
		if err != nil {
			return nil, err
		}
		frames = append(frames, data)
	}
	tail, err := BuildBlackSharkImageTail(canvas[2095*blackSharkImageBlockBytes:])
	if err != nil {
		return nil, err
	}
	frames = append(frames, tail, BuildBlackSharkImageCommit())
	return frames, nil
}
