package deviceprofileexec

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/TIANLI0/THRM/internal/types"
)

type SpeedVars struct {
	Unit           string
	Value          int
	Percent        int
	PercentTicks   int
	DecimalPercent float64
	RPM            int
}

func SpeedVarsFromValue(speed types.FanSpeedValue) SpeedVars {
	speed = speed.Normalized()
	vars := SpeedVars{
		Unit:  speed.Unit,
		Value: speed.Value,
	}
	if types.IsRPMSpeedUnit(speed.Unit) {
		vars.RPM = types.ClampRPM(speed.Value)
		vars.Value = vars.RPM
		return vars
	}

	vars.PercentTicks = types.ClampPercentTicks(speed.Value)
	vars.Percent = types.PercentTicksToIntegerPercent(vars.PercentTicks)
	vars.DecimalPercent = types.PercentTicksToDecimalPercent(vars.PercentTicks)
	vars.Value = vars.Percent
	return vars
}

func FindCommand(commands []types.DeviceCommandTemplate, names ...string) (types.DeviceCommandTemplate, bool) {
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[normalizeKey(name)] = true
	}
	for _, command := range commands {
		if wanted[normalizeKey(command.Name)] {
			return command, true
		}
	}
	return types.DeviceCommandTemplate{}, false
}

func RenderTemplate(template string, vars SpeedVars) string {
	replacements := map[string]string{
		"speed":           strconv.Itoa(vars.Value),
		"value":           strconv.Itoa(vars.Value),
		"unit":            vars.Unit,
		"percent":         strconv.Itoa(vars.Percent),
		"percentTicks":    strconv.Itoa(vars.PercentTicks),
		"percent_ticks":   strconv.Itoa(vars.PercentTicks),
		"decimalPercent":  formatDecimal(vars.DecimalPercent),
		"decimal_percent": formatDecimal(vars.DecimalPercent),
		"rpm":             strconv.Itoa(vars.RPM),
	}
	rendered := template
	for key, value := range replacements {
		rendered = strings.ReplaceAll(rendered, "{{"+key+"}}", value)
	}
	return rendered
}

func EncodeCommand(command types.DeviceCommandTemplate, vars SpeedVars) ([]byte, string, error) {
	rendered := RenderTemplate(command.Command, vars)
	encoding := strings.ToLower(strings.TrimSpace(command.Encoding))
	if encoding == "" {
		encoding = "json"
	}

	switch encoding {
	case "json":
		body := []byte(rendered)
		if len(strings.TrimSpace(rendered)) == 0 {
			return nil, "", fmt.Errorf("json command template is empty")
		}
		if !json.Valid(body) {
			return nil, "", fmt.Errorf("json command template rendered invalid JSON")
		}
		if checksumMode(command.Checksum) != "" {
			return nil, "", fmt.Errorf("checksum mode %q is not supported for json command encoding", command.Checksum)
		}
		return body, "application/json", nil
	case "hex":
		body, err := decodeHexPayload(rendered)
		if err != nil {
			return nil, "", err
		}
		body, err = appendChecksum(body, checksumMode(command.Checksum))
		if err != nil {
			return nil, "", err
		}
		return body, "application/octet-stream", nil
	case "ascii", "raw":
		body, err := appendChecksum([]byte(rendered), checksumMode(command.Checksum))
		if err != nil {
			return nil, "", err
		}
		contentType := "text/plain; charset=utf-8"
		if checksumMode(command.Checksum) != "" {
			contentType = "application/octet-stream"
		}
		return body, contentType, nil
	default:
		return nil, "", fmt.Errorf("unsupported command encoding %q", command.Encoding)
	}
}

func checksumMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "none" {
		return ""
	}
	return mode
}

func normalizeKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, " ", "")
	return value
}

func formatDecimal(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func decodeHexPayload(value string) ([]byte, error) {
	cleaned := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "", "0x", "", "0X", "").Replace(value)
	if cleaned == "" {
		return nil, fmt.Errorf("hex command template rendered empty payload")
	}
	if len(cleaned)%2 != 0 {
		return nil, fmt.Errorf("hex command template rendered odd-length payload")
	}
	payload, err := hex.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("hex command template rendered invalid hex: %w", err)
	}
	return payload, nil
}

func appendChecksum(payload []byte, mode string) ([]byte, error) {
	switch mode {
	case "":
		return payload, nil
	case "sum8":
		var sum byte
		for _, b := range payload {
			sum += b
		}
		return append(payload, sum), nil
	case "xor8":
		var x byte
		for _, b := range payload {
			x ^= b
		}
		return append(payload, x), nil
	case "crc16":
		crc := crc16Modbus(payload)
		out := append([]byte(nil), payload...)
		out = binary.LittleEndian.AppendUint16(out, crc)
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported checksum mode %q", mode)
	}
}

func crc16Modbus(payload []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range payload {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}
