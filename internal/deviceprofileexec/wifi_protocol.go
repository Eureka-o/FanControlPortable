package deviceprofileexec

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/TIANLI0/THRM/internal/types"
)

type wifiRequest struct {
	method      string
	endpoint    string
	body        []byte
	contentType string
}

type wifiProtocol interface {
	StateRequest() wifiRequest
	SpeedRequest(types.FanSpeedValue) (wifiRequest, error)
	ParseState([]byte, int) (*types.FanData, error)
	ValidateSetSpeedResponse([]byte) error
}

type defaultWiFiPercentProtocol struct {
	profile       types.DeviceProfile
	stateEndpoint string
	speedEndpoint string
	method        string
}

type profileWiFiProtocol struct {
	profile       types.DeviceProfile
	stateEndpoint string
	speedEndpoint string
	method        string
	setCommand    types.DeviceCommandTemplate
	hasSetCommand bool
	parsers       CompiledResponseParsers
}

func newWiFiProtocol(profile types.DeviceProfile) (wifiProtocol, error) {
	if shouldUseDefaultWiFiPercentProtocol(profile) {
		return newDefaultWiFiPercentProtocol(profile), nil
	}
	return newProfileWiFiProtocol(profile)
}

func shouldUseDefaultWiFiPercentProtocol(profile types.DeviceProfile) bool {
	return profile.Transport == types.DeviceTransportWiFi &&
		types.NormalizeFanSpeedUnit(profile.SpeedUnit) == types.FanSpeedUnitPercent &&
		len(profile.Commands) == 0 &&
		len(profile.ResponseParsers) == 0
}

func newDefaultWiFiPercentProtocol(profile types.DeviceProfile) *defaultWiFiPercentProtocol {
	return &defaultWiFiPercentProtocol{
		profile:       profile,
		stateEndpoint: profile.Connection.StateEndpoint,
		speedEndpoint: profile.Connection.SpeedEndpoint,
		method:        wifiHTTPMethod(profile.Connection.HTTPMethod),
	}
}

func newProfileWiFiProtocol(profile types.DeviceProfile) (*profileWiFiProtocol, error) {
	command, hasCommand := FindCommand(profile.Commands, "setSpeed", "set-speed", "speed")
	parsers, err := CompileResponseParsers(profile.ResponseParsers)
	if err != nil {
		return nil, err
	}
	return &profileWiFiProtocol{
		profile:       profile,
		stateEndpoint: profile.Connection.StateEndpoint,
		speedEndpoint: profile.Connection.SpeedEndpoint,
		method:        wifiHTTPMethod(profile.Connection.HTTPMethod),
		setCommand:    command,
		hasSetCommand: hasCommand,
		parsers:       parsers,
	}, nil
}

func wifiHTTPMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return http.MethodPost
	}
	return method
}

func (p *defaultWiFiPercentProtocol) StateRequest() wifiRequest {
	return wifiRequest{
		method:   http.MethodGet,
		endpoint: p.stateEndpoint,
	}
}

func (p *defaultWiFiPercentProtocol) SpeedRequest(speed types.FanSpeedValue) (wifiRequest, error) {
	vars := SpeedVarsFromValue(speed)
	endpoint := RenderTemplate(p.speedEndpoint, vars)
	if p.method == http.MethodGet {
		return wifiRequest{method: p.method, endpoint: endpoint}, nil
	}

	body, err := json.Marshal(map[string]int{"speed": vars.Value})
	if err != nil {
		return wifiRequest{}, err
	}
	return wifiRequest{
		method:      p.method,
		endpoint:    endpoint,
		body:        body,
		contentType: "application/json",
	}, nil
}

func (p *defaultWiFiPercentProtocol) ParseState(body []byte, fallbackTarget int) (*types.FanData, error) {
	state, ok := parseDefaultWiFiState(body)
	if !ok {
		return nil, fmt.Errorf("wifi profile state response did not contain speed data")
	}
	return fanDataFromWiFiState(p.profile, state, fallbackTarget)
}

func (p *defaultWiFiPercentProtocol) ValidateSetSpeedResponse(body []byte) error {
	return validateSetSpeedResponse(body)
}

func (p *profileWiFiProtocol) StateRequest() wifiRequest {
	return wifiRequest{
		method:   http.MethodGet,
		endpoint: p.stateEndpoint,
	}
}

func (p *profileWiFiProtocol) SpeedRequest(speed types.FanSpeedValue) (wifiRequest, error) {
	vars := SpeedVarsFromValue(speed)
	endpoint := RenderTemplate(p.speedEndpoint, vars)
	if p.method == http.MethodGet {
		return wifiRequest{method: p.method, endpoint: endpoint}, nil
	}

	body, contentType, err := p.speedRequestBody(vars)
	if err != nil {
		return wifiRequest{}, err
	}
	return wifiRequest{
		method:      p.method,
		endpoint:    endpoint,
		body:        body,
		contentType: contentType,
	}, nil
}

func (p *profileWiFiProtocol) speedRequestBody(vars SpeedVars) ([]byte, string, error) {
	if p.hasSetCommand {
		return EncodeCommand(p.setCommand, vars)
	}

	payload, err := json.Marshal(map[string]int{"speed": vars.Value})
	if err != nil {
		return nil, "", err
	}
	return payload, "application/json", nil
}

func (p *profileWiFiProtocol) ParseState(body []byte, fallbackTarget int) (*types.FanData, error) {
	state, err := p.parsers.Parse(body)
	if err != nil {
		return nil, err
	}
	return fanDataFromWiFiState(p.profile, state, fallbackTarget)
}

func (p *profileWiFiProtocol) ValidateSetSpeedResponse(body []byte) error {
	return validateSetSpeedResponse(body)
}

func fanDataFromWiFiState(profile types.DeviceProfile, state ParsedState, fallbackTarget int) (*types.FanData, error) {
	if !completeStateTarget(&state, fallbackTarget) {
		return nil, fmt.Errorf("wifi profile state response did not contain speed data")
	}

	current := clampForProfile(state.CurrentSpeed, profile.SpeedUnit)
	target := clampForProfile(state.TargetSpeed, profile.SpeedUnit)
	workMode := strings.TrimSpace(state.WorkMode)
	if workMode == "" {
		workMode = "software"
	}
	return &types.FanData{
		CurrentRPM: uint16(current),
		TargetRPM:  uint16(target),
		WorkMode:   workMode,
		Transport:  types.DeviceTransportWiFi,
		SpeedUnit:  types.NormalizeFanSpeedUnit(profile.SpeedUnit),
	}, nil
}
