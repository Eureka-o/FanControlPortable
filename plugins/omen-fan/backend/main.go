package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const (
	pluginID      = "omen-fan"
	pluginVersion = "0.1.0"
	protocolV1    = 1
)

type protocolMessage struct {
	Type     string          `json:"type"`
	ID       string          `json:"id,omitempty"`
	Method   string          `json:"method,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	Sequence uint64          `json:"sequence,omitempty"`
	Sampled  int64           `json:"sampledAt,omitempty"`
}

type protocolHello struct {
	Type            string   `json:"type"`
	ProtocolVersion int      `json:"protocolVersion"`
	PluginID        string   `json:"pluginId"`
	Version         string   `json:"version"`
	Capabilities    []string `json:"capabilities"`
	Supported       bool     `json:"supported"`
}

type protocolResponse struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

type protocolEvent struct {
	Type    string `json:"type"`
	Event   string `json:"event"`
	Payload any    `json:"payload"`
}

type telemetryValue struct {
	Value float64 `json:"value"`
	Valid bool    `json:"valid"`
}

type telemetryPayload struct {
	CPUTemp       *telemetryValue `json:"cpuTemp"`
	GPUTemp       *telemetryValue `json:"gpuTemp"`
	CPUPowerWatts *telemetryValue `json:"cpuPowerWatts"`
	GPUPowerWatts *telemetryValue `json:"gpuPowerWatts"`
	BridgeOK      bool            `json:"bridgeOk"`
}

type previewCapabilities struct {
	ThermalMode      bool `json:"thermalMode"`
	CPUPowerLimits   bool `json:"cpuPowerLimits"`
	CPUPl4           bool `json:"cpuPl4"`
	CPUTempLimit     bool `json:"cpuTempLimit"`
	CPUBoostPolicy   bool `json:"cpuBoostPolicy"`
	PowerBias        bool `json:"powerBias"`
	GPUPower         bool `json:"gpuPower"`
	ScreenOverdrive  bool `json:"screenOverdrive"`
	ChargeProtection bool `json:"chargeProtection"`
	GPUMode          bool `json:"gpuMode"`
	FanCurves        bool `json:"fanCurves"`
	CurveResponse    bool `json:"curveResponse"`
	JointLearning    bool `json:"jointLearning"`
	OmenKey          bool `json:"omenKey"`
	Diagnostics      bool `json:"diagnostics"`
}

type previewPower struct {
	PL1          float64 `json:"pl1"`
	PL2          float64 `json:"pl2"`
	PL4          float64 `json:"pl4"`
	SPL          float64 `json:"spl"`
	SPPT         float64 `json:"sppt"`
	FPPT         float64 `json:"fppt"`
	TempLimit    float64 `json:"tempLimit"`
	BoostPolicy  string  `json:"boostPolicy"`
	PowerBias    string  `json:"powerBias"`
	TGP          float64 `json:"tgp"`
	PPAB         float64 `json:"ppab"`
	DynamicBoost bool    `json:"dynamicBoost"`
}

type previewFeatures struct {
	ScreenOverdrive bool   `json:"screenOverdrive"`
	ChargeLimit     int    `json:"chargeLimit"`
	GPUMode         string `json:"gpuMode"`
	OmenKeyAction   string `json:"omenKeyAction"`
}

type curvePoint struct {
	Temperature float64 `json:"temperature"`
	RPM         float64 `json:"rpm"`
}

type previewCurve struct {
	Points                    []curvePoint `json:"points"`
	ResponseTime              float64      `json:"responseTime"`
	LoweringDelay             float64      `json:"loweringDelay"`
	HighTemperatureProtection bool         `json:"highTemperatureProtection"`
}

type previewCurves struct {
	CPU previewCurve `json:"cpu"`
	GPU previewCurve `json:"gpu"`
}

type previewJointLearning struct {
	Enabled bool `json:"enabled"`
	Paused  bool `json:"paused"`
}

type previewDiagnostics struct {
	Model      string `json:"model"`
	BIOS       string `json:"bios"`
	HardwareID string `json:"hardwareId"`
}

type previewStatus struct {
	Supported               bool                 `json:"supported"`
	ControlActive           bool                 `json:"controlActive"`
	ConnectionState         string               `json:"connectionState"`
	DeviceName              string               `json:"deviceName"`
	CPUModel                string               `json:"cpuModel"`
	GPUModel                string               `json:"gpuModel"`
	CPUTemp                 float64              `json:"cpuTemp"`
	GPUTemp                 float64              `json:"gpuTemp"`
	CPUPower                float64              `json:"cpuPower"`
	GPUPower                float64              `json:"gpuPower"`
	CPURPM                  float64              `json:"cpuRpm"`
	GPURPM                  float64              `json:"gpuRpm"`
	MinFanRPM               float64              `json:"minFanRpm"`
	MaxFanRPM               float64              `json:"maxFanRpm"`
	Mode                    string               `json:"mode"`
	ProcessorFamily         string               `json:"processorFamily"`
	Capabilities            previewCapabilities  `json:"capabilities"`
	Power                   previewPower         `json:"power"`
	Features                previewFeatures      `json:"features"`
	Curves                  previewCurves        `json:"curves"`
	JointLearning           previewJointLearning `json:"jointLearning"`
	Diagnostics             previewDiagnostics   `json:"diagnostics"`
	AvailableGPUModes       []string             `json:"availableGpuModes"`
	AvailableOmenKeyActions []string             `json:"availableOmenKeyActions"`
	RPMEstimated            bool                 `json:"rpmEstimated"`
	LastUpdated             int64                `json:"lastUpdated"`
}

type protocolServer struct {
	input  io.Reader
	output *json.Encoder
	status previewStatus
}

func newProtocolServer(input io.Reader, output io.Writer) *protocolServer {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return &protocolServer{input: input, output: encoder, status: newPreviewStatus()}
}

func newPreviewStatus() previewStatus {
	return previewStatus{
		Supported:       true,
		ConnectionState: "connected",
		DeviceName:      "HP OMEN Preview",
		CPUModel:        "AMD Ryzen 9 8945HX",
		GPUModel:        "NVIDIA GeForce RTX 5060 Laptop GPU",
		CPUTemp:         58,
		GPUTemp:         53,
		CPUPower:        35,
		GPUPower:        62,
		CPURPM:          2280,
		GPURPM:          2140,
		MinFanRPM:       1200,
		MaxFanRPM:       6500,
		Mode:            "balanced",
		ProcessorFamily: "amd",
		Capabilities: previewCapabilities{
			ThermalMode: true, CPUPowerLimits: true, CPUPl4: true, CPUTempLimit: true,
			CPUBoostPolicy: true, PowerBias: true, GPUPower: true, ScreenOverdrive: true,
			ChargeProtection: true, GPUMode: true, FanCurves: true, CurveResponse: true,
			JointLearning: true, OmenKey: true, Diagnostics: true,
		},
		Power: previewPower{
			PL1: 55, PL2: 115, PL4: 150, SPL: 45, SPPT: 65, FPPT: 85, TempLimit: 95,
			BoostPolicy: "enabled", PowerBias: "balanced", TGP: 115, PPAB: 15, DynamicBoost: true,
		},
		Features: previewFeatures{
			ScreenOverdrive: true, ChargeLimit: 80, GPUMode: "hybrid", OmenKeyAction: "fancontrol",
		},
		Curves:                  defaultPreviewCurves(),
		JointLearning:           previewJointLearning{},
		Diagnostics:             previewDiagnostics{Model: "OMEN Preview Device", BIOS: "Preview 0.1", HardwareID: "PREVIEW-ONLY"},
		AvailableGPUModes:       []string{"hybrid", "discrete", "integrated"},
		AvailableOmenKeyActions: []string{"fancontrol", "omen-hub", "ignore"},
		RPMEstimated:            true,
		LastUpdated:             time.Now().UnixMilli(),
	}
}

func defaultPreviewCurves() previewCurves {
	return previewCurves{
		CPU: newPreviewCurve([]float64{1700, 2000, 2500, 3200, 4100, 5100, 6200}),
		GPU: newPreviewCurve([]float64{1600, 1900, 2350, 3000, 3900, 4900, 6000}),
	}
}

func newPreviewCurve(rpm []float64) previewCurve {
	temperatures := []float64{40, 50, 60, 70, 80, 90, 100}
	points := make([]curvePoint, 0, len(temperatures))
	for index, temperature := range temperatures {
		points = append(points, curvePoint{Temperature: temperature, RPM: rpm[index]})
	}
	return previewCurve{Points: points, ResponseTime: 3, LoweringDelay: 15, HighTemperatureProtection: true}
}

func (server *protocolServer) Run() error {
	if err := server.output.Encode(protocolHello{
		Type: "hello", ProtocolVersion: protocolV1, PluginID: pluginID, Version: pluginVersion,
		Capabilities: []string{"status", "fan-control"}, Supported: true,
	}); err != nil {
		return fmt.Errorf("write hello: %w", err)
	}

	scanner := bufio.NewScanner(server.input)
	scanner.Buffer(make([]byte, 4096), 256*1024)
	for scanner.Scan() {
		var message protocolMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			return fmt.Errorf("decode host message: %w", err)
		}
		switch message.Type {
		case "host-init":
			continue
		case "telemetry":
			if err := server.applyTelemetry(message.Payload); err != nil {
				return err
			}
			if err := server.output.Encode(protocolEvent{Type: "event", Event: "status-changed", Payload: server.status}); err != nil {
				return fmt.Errorf("write status event: %w", err)
			}
		case "request":
			stop, err := server.handleRequest(message)
			if err != nil {
				if writeErr := server.output.Encode(protocolResponse{Type: "response", ID: message.ID, OK: false, Error: err.Error()}); writeErr != nil {
					return fmt.Errorf("write error response: %w", writeErr)
				}
				continue
			}
			if stop {
				return nil
			}
		default:
			return fmt.Errorf("unsupported host message type %q", message.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read host message: %w", err)
	}
	return nil
}

func (server *protocolServer) applyTelemetry(raw json.RawMessage) error {
	var payload telemetryPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode telemetry: %w", err)
	}
	if payload.CPUTemp != nil && payload.CPUTemp.Valid {
		server.status.CPUTemp = payload.CPUTemp.Value
	}
	if payload.GPUTemp != nil && payload.GPUTemp.Valid {
		server.status.GPUTemp = payload.GPUTemp.Value
	}
	if payload.CPUPowerWatts != nil && payload.CPUPowerWatts.Valid {
		server.status.CPUPower = payload.CPUPowerWatts.Value
	}
	if payload.GPUPowerWatts != nil && payload.GPUPowerWatts.Valid {
		server.status.GPUPower = payload.GPUPowerWatts.Value
	}
	server.touch()
	return nil
}

func (server *protocolServer) handleRequest(message protocolMessage) (bool, error) {
	if message.ID == "" {
		return false, fmt.Errorf("request id is required")
	}
	stop := false
	switch message.Method {
	case "get-status":
	case "set-thermal-mode":
		var payload struct {
			Mode string `json:"mode"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return false, err
		}
		if !oneOf(payload.Mode, "eco", "balanced", "performance", "master") {
			return false, fmt.Errorf("unsupported thermal mode %q", payload.Mode)
		}
		server.status.Mode = payload.Mode
		server.status.ControlActive = true
	case "set-cpu-power":
		if err := decodePayload(message.Payload, &server.status.Power); err != nil {
			return false, err
		}
	case "set-cpu-boost-policy":
		var payload struct {
			Policy string `json:"policy"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return false, err
		}
		server.status.Power.BoostPolicy = payload.Policy
	case "set-power-bias":
		var payload struct {
			Bias string `json:"bias"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return false, err
		}
		server.status.Power.PowerBias = payload.Bias
	case "set-gpu-power":
		var payload struct {
			TGP          float64 `json:"tgp"`
			PPAB         float64 `json:"ppab"`
			DynamicBoost bool    `json:"dynamicBoost"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return false, err
		}
		server.status.Power.TGP = payload.TGP
		server.status.Power.PPAB = payload.PPAB
		server.status.Power.DynamicBoost = payload.DynamicBoost
	case "set-screen-overdrive":
		var payload struct {
			Enabled bool `json:"enabled"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return false, err
		}
		server.status.Features.ScreenOverdrive = payload.Enabled
	case "set-charge-limit":
		var payload struct {
			Limit int `json:"limit"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return false, err
		}
		if payload.Limit != 80 && payload.Limit != 100 {
			return false, fmt.Errorf("charge limit must be 80 or 100")
		}
		server.status.Features.ChargeLimit = payload.Limit
	case "set-gpu-mode":
		var payload struct {
			Mode string `json:"mode"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return false, err
		}
		if !oneOf(payload.Mode, server.status.AvailableGPUModes...) {
			return false, fmt.Errorf("unsupported GPU mode %q", payload.Mode)
		}
		server.status.Features.GPUMode = payload.Mode
	case "set-omen-key":
		var payload struct {
			Action string `json:"action"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return false, err
		}
		server.status.Features.OmenKeyAction = payload.Action
	case "set-fan-curve":
		var payload struct {
			Target string       `json:"target"`
			Curve  previewCurve `json:"curve"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return false, err
		}
		if payload.Target == "cpu" {
			server.status.Curves.CPU = payload.Curve
		} else if payload.Target == "gpu" {
			server.status.Curves.GPU = payload.Curve
		} else {
			return false, fmt.Errorf("fan curve target must be cpu or gpu")
		}
	case "reset-fan-curve":
		var payload struct {
			Target string `json:"target"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return false, err
		}
		defaults := defaultPreviewCurves()
		if payload.Target == "cpu" {
			server.status.Curves.CPU = defaults.CPU
		} else if payload.Target == "gpu" {
			server.status.Curves.GPU = defaults.GPU
		} else {
			return false, fmt.Errorf("fan curve target must be cpu or gpu")
		}
	case "set-joint-learning":
		var payload struct {
			Enabled bool `json:"enabled"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return false, err
		}
		server.status.JointLearning.Enabled = payload.Enabled
		server.status.JointLearning.Paused = false
	case "export-diagnostics":
	case "host.stop", "host.prepare-suspend":
		stop = true
	default:
		return false, fmt.Errorf("unsupported preview method %q", message.Method)
	}
	server.touch()
	if err := server.output.Encode(protocolResponse{Type: "response", ID: message.ID, OK: true, Payload: server.status}); err != nil {
		return false, fmt.Errorf("write response: %w", err)
	}
	return stop, nil
}

func decodePayload(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode request payload: %w", err)
	}
	return nil
}

func oneOf(value string, options ...string) bool {
	value = strings.TrimSpace(value)
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}

func (server *protocolServer) touch() {
	server.status.LastUpdated = time.Now().UnixMilli()
}

func main() {
	if err := newProtocolServer(os.Stdin, os.Stdout).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
