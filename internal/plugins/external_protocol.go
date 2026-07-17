package plugins

import (
	"encoding/json"
	"strings"
)

const (
	ExternalProtocolVersion = 1
	maxProtocolMessageBytes = 256 * 1024
)

type TelemetryValue struct {
	Value float64 `json:"value"`
	Valid bool    `json:"valid"`
}

type TelemetryPayload struct {
	CPUTemp       *TelemetryValue `json:"cpuTemp,omitempty"`
	GPUTemp       *TelemetryValue `json:"gpuTemp,omitempty"`
	CPUPowerWatts *TelemetryValue `json:"cpuPowerWatts,omitempty"`
	GPUPowerWatts *TelemetryValue `json:"gpuPowerWatts,omitempty"`
	BridgeOK      bool            `json:"bridgeOk"`
}

type TelemetrySnapshot struct {
	Sequence  uint64           `json:"sequence"`
	SampledAt int64            `json:"sampledAt"`
	Payload   TelemetryPayload `json:"payload"`
}

func (snapshot TelemetrySnapshot) Filter(inputs []string) (TelemetrySnapshot, bool) {
	filtered := TelemetrySnapshot{
		Sequence:  snapshot.Sequence,
		SampledAt: snapshot.SampledAt,
		Payload: TelemetryPayload{
			BridgeOK: snapshot.Payload.BridgeOK,
		},
	}
	for _, input := range inputs {
		switch strings.TrimSpace(input) {
		case "cpu-temp":
			filtered.Payload.CPUTemp = snapshot.Payload.CPUTemp
		case "gpu-temp":
			filtered.Payload.GPUTemp = snapshot.Payload.GPUTemp
		case "cpu-power":
			filtered.Payload.CPUPowerWatts = snapshot.Payload.CPUPowerWatts
		case "gpu-power":
			filtered.Payload.GPUPowerWatts = snapshot.Payload.GPUPowerWatts
		}
	}
	hasValue := filtered.Payload.CPUTemp != nil || filtered.Payload.GPUTemp != nil ||
		filtered.Payload.CPUPowerWatts != nil || filtered.Payload.GPUPowerWatts != nil
	return filtered, hasValue
}

type RuntimeStatus struct {
	ID           string
	State        CatalogState
	LastError    string
	Capabilities []string
}

type RuntimeEvent struct {
	PluginID string          `json:"pluginId"`
	Event    string          `json:"event"`
	Payload  json.RawMessage `json:"payload"`
}

type protocolEnvelope struct {
	Type            string          `json:"type"`
	ProtocolVersion int             `json:"protocolVersion,omitempty"`
	PluginID        string          `json:"pluginId,omitempty"`
	Version         string          `json:"version,omitempty"`
	Capabilities    []string        `json:"capabilities,omitempty"`
	Supported       *bool           `json:"supported,omitempty"`
	Reason          string          `json:"reason,omitempty"`
	ID              string          `json:"id,omitempty"`
	Method          string          `json:"method,omitempty"`
	OK              bool            `json:"ok,omitempty"`
	Error           string          `json:"error,omitempty"`
	Event           string          `json:"event,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
}

type hostInitMessage struct {
	Type                string `json:"type"`
	ProtocolVersion     int    `json:"protocolVersion"`
	InstanceID          string `json:"instanceId"`
	DataDir             string `json:"dataDir"`
	HeartbeatIntervalMS int    `json:"heartbeatIntervalMs"`
}

type protocolRequest struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Method  string          `json:"method"`
	Payload json.RawMessage `json:"payload"`
}

type protocolTelemetry struct {
	Type      string           `json:"type"`
	Sequence  uint64           `json:"sequence"`
	SampledAt int64            `json:"sampledAt"`
	Payload   TelemetryPayload `json:"payload"`
}

func validProtocolName(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 80 {
		return false
	}
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '-', char == '_', char == '.', char == ':':
		default:
			return false
		}
	}
	return true
}
