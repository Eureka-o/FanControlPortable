package deviceproto

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

const (
	Magic0   byte = 0x5A
	Magic1   byte = 0xA5
	ReportID byte = 0x02
)

type Frame struct {
	ReportID   byte
	Offset     int
	Command    byte
	Length     byte
	Payload    []byte
	Checksum   byte
	ChecksumOK bool
	Frame      []byte
}

func Checksum(cmd byte, payload ...byte) byte {
	sum := uint16(cmd) + uint16(2+len(payload))
	for _, b := range payload {
		sum += uint16(b)
	}
	return byte(sum & 0xFF)
}

func BuildFrame(cmd byte, payload ...byte) []byte {
	frame := make([]byte, 0, 5+len(payload))
	frame = append(frame, Magic0, Magic1, cmd, byte(2+len(payload)))
	frame = append(frame, payload...)
	frame = append(frame, Checksum(cmd, payload...))
	return frame
}

func BuildReport(frame []byte, reportLen int) []byte {
	if reportLen <= 0 || reportLen < len(frame)+1 {
		reportLen = len(frame) + 1
	}
	report := make([]byte, reportLen)
	report[0] = ReportID
	copy(report[1:], frame)
	return report
}

func ParseFrame(data []byte) (Frame, bool) {
	offset := -1
	reportID := byte(0)
	switch {
	case len(data) >= 2 && data[0] == Magic0 && data[1] == Magic1:
		offset = 0
	case len(data) >= 3 && data[1] == Magic0 && data[2] == Magic1:
		offset = 1
		reportID = data[0]
	default:
		return Frame{}, false
	}

	if len(data) < offset+5 {
		return Frame{}, false
	}

	length := int(data[offset+3])
	if length < 2 {
		return Frame{}, false
	}

	frameLen := 2 + length + 1
	if len(data) < offset+frameLen {
		return Frame{}, false
	}

	frame := data[offset : offset+frameLen]
	checksumIndex := offset + 2 + length
	var sum uint16
	for _, b := range data[offset+2 : checksumIndex] {
		sum += uint16(b)
	}

	payloadLen := length - 2
	payload := make([]byte, payloadLen)
	copy(payload, data[offset+4:offset+4+payloadLen])

	copiedFrame := make([]byte, len(frame))
	copy(copiedFrame, frame)

	return Frame{
		ReportID:   reportID,
		Offset:     offset,
		Command:    data[offset+2],
		Length:     byte(length),
		Payload:    payload,
		Checksum:   data[checksumIndex],
		ChecksumOK: byte(sum&0xFF) == data[checksumIndex],
		Frame:      copiedFrame,
	}, true
}

func NormalizeDebugInput(input string) ([]byte, error) {
	data, err := ParseHex(input)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	if len(data) >= 3 && data[0] == ReportID && data[1] == Magic0 && data[2] == Magic1 {
		parsed, ok := ParseFrame(data)
		if !ok {
			return nil, fmt.Errorf("invalid HID report frame")
		}
		return parsed.Frame, nil
	}
	if len(data) >= 2 && data[0] == Magic0 && data[1] == Magic1 {
		parsed, ok := ParseFrame(data)
		if !ok {
			return nil, fmt.Errorf("invalid protocol frame")
		}
		return parsed.Frame, nil
	}

	cmd := data[0]
	payload := data[1:]
	return BuildFrame(cmd, payload...), nil
}

func ParseHex(input string) ([]byte, error) {
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == ',' || r == ';' || r == ':' || r == '-' {
			return ' '
		}
		return r
	}, strings.TrimSpace(input))

	fields := strings.Fields(normalized)
	var cleaned string
	if len(fields) <= 1 {
		cleaned = normalized
	} else {
		var b strings.Builder
		for _, field := range fields {
			b.WriteString(strings.TrimPrefix(strings.TrimPrefix(field, "0x"), "0X"))
		}
		cleaned = b.String()
	}
	cleaned = strings.ReplaceAll(cleaned, "0x", "")
	cleaned = strings.ReplaceAll(cleaned, "0X", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	if len(cleaned)%2 != 0 {
		cleaned = "0" + cleaned
	}
	data, err := hex.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("invalid hex command: %w", err)
	}
	return data, nil
}

func Hex(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	encoded := strings.ToUpper(hex.EncodeToString(data))
	var b strings.Builder
	b.Grow(len(encoded) + len(encoded)/2)
	for i := 0; i < len(encoded); i += 2 {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(encoded[i : i+2])
	}
	return b.String()
}
