package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type testEnvelope struct {
	Type            string          `json:"type"`
	ProtocolVersion int             `json:"protocolVersion"`
	PluginID        string          `json:"pluginId"`
	Version         string          `json:"version"`
	Supported       *bool           `json:"supported"`
	ID              string          `json:"id"`
	OK              bool            `json:"ok"`
	Payload         json.RawMessage `json:"payload"`
}

func runProtocol(t *testing.T, input string) []testEnvelope {
	t.Helper()

	var output bytes.Buffer
	server := newProtocolServer(strings.NewReader(input), &output)
	if err := server.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	messages := make([]testEnvelope, 0, len(lines))
	for _, line := range lines {
		var message testEnvelope
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			t.Fatalf("decode output %q: %v", line, err)
		}
		messages = append(messages, message)
	}
	return messages
}

func TestProtocolServerHandshakesAndReturnsPreviewStatus(t *testing.T) {
	messages := runProtocol(t, strings.Join([]string{
		`{"type":"host-init","protocolVersion":1,"instanceId":"test"}`,
		`{"type":"request","id":"1","method":"get-status","payload":{}}`,
		`{"type":"request","id":"2","method":"host.stop","payload":{"reason":"disabled"}}`,
	}, "\n")+"\n")

	if len(messages) != 3 {
		t.Fatalf("message count = %d, want 3", len(messages))
	}
	hello := messages[0]
	if hello.Type != "hello" || hello.ProtocolVersion != 1 || hello.PluginID != pluginID || hello.Version != pluginVersion {
		t.Fatalf("unexpected hello: %+v", hello)
	}
	if hello.Supported == nil || !*hello.Supported {
		t.Fatalf("hello supported = %v, want true", hello.Supported)
	}

	if messages[1].Type != "response" || messages[1].ID != "1" || !messages[1].OK {
		t.Fatalf("unexpected get-status response: %+v", messages[1])
	}
	var status previewStatus
	if err := json.Unmarshal(messages[1].Payload, &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.DeviceName != "HP OMEN Preview" || status.CPUModel != "AMD Ryzen 9 8945HX" || status.GPUModel != "NVIDIA GeForce RTX 5060 Laptop GPU" {
		t.Fatalf("unexpected preview identity: %+v", status)
	}
	if status.ProcessorFamily != "amd" {
		t.Fatalf("unexpected preview processor family: %q", status.ProcessorFamily)
	}
	if !status.Capabilities.ThermalMode || !status.Capabilities.FanCurves || !status.Capabilities.Diagnostics {
		t.Fatalf("preview capabilities are incomplete: %+v", status.Capabilities)
	}

	if messages[2].Type != "response" || messages[2].ID != "2" || !messages[2].OK {
		t.Fatalf("unexpected stop response: %+v", messages[2])
	}
}

func TestProtocolServerAppliesTelemetryAndInMemoryCommands(t *testing.T) {
	messages := runProtocol(t, strings.Join([]string{
		`{"type":"host-init","protocolVersion":1,"instanceId":"test"}`,
		`{"type":"telemetry","sequence":7,"sampledAt":1234,"payload":{"cpuTemp":{"value":72,"valid":true},"gpuTemp":{"value":66,"valid":true},"cpuPowerWatts":{"value":48.5,"valid":true},"gpuPowerWatts":{"value":92,"valid":true},"bridgeOk":true}}`,
		`{"type":"request","id":"1","method":"set-thermal-mode","payload":{"mode":"master"}}`,
		`{"type":"request","id":"2","method":"set-joint-learning","payload":{"enabled":true}}`,
		`{"type":"request","id":"3","method":"get-status","payload":{}}`,
		`{"type":"request","id":"4","method":"host.stop","payload":{"reason":"disabled"}}`,
	}, "\n")+"\n")

	var status previewStatus
	for _, message := range messages {
		if message.Type == "response" && message.ID == "3" {
			if err := json.Unmarshal(message.Payload, &status); err != nil {
				t.Fatalf("decode status: %v", err)
			}
		}
	}
	if status.Mode != "master" || status.CPUTemp != 72 || status.GPUTemp != 66 || status.CPUPower != 48.5 || status.GPUPower != 92 {
		t.Fatalf("telemetry or mode was not retained: %+v", status)
	}
	if !status.JointLearning.Enabled {
		t.Fatalf("joint learning was not retained: %+v", status.JointLearning)
	}
}
