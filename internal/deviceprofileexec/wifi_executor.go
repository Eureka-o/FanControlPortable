package deviceprofileexec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

const (
	defaultHTTPTimeout     = 2 * time.Second
	defaultMinSendInterval = 100 * time.Millisecond
	defaultRetryBackoff    = 150 * time.Millisecond
	maxHTTPBodyBytes       = 64 * 1024
	maxProfileHTTPTimeout  = 30 * time.Second
	maxProfileSendInterval = 60 * time.Second
	maxProfileRetryBackoff = 10 * time.Second
	maxProfileRetryCount   = 5
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type WiFiExecutor struct {
	profile         types.DeviceProfile
	client          HTTPDoer
	baseURL         string
	stateEndpoint   string
	speedEndpoint   string
	method          string
	setCommand      types.DeviceCommandTemplate
	hasSetCommand   bool
	parsers         CompiledResponseParsers
	requestTimeout  time.Duration
	minSendInterval time.Duration
	maxRetries      int
	retryBackoff    time.Duration

	sendMutex sync.Mutex
	lastSend  time.Time
	now       func() time.Time
	sleep     func(context.Context, time.Duration) error
}

func NewWiFiExecutor(profile types.DeviceProfile, fallbackEndpoint string, client HTTPDoer) (*WiFiExecutor, error) {
	profile = types.NormalizeDeviceProfile(profile, fallbackEndpoint)
	if profile.Transport != types.DeviceTransportWiFi {
		return nil, fmt.Errorf("wifi executor requires a wifi profile")
	}
	baseURL, err := normalizeBaseEndpoint(profile.Connection.Endpoint)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	command, hasCommand := FindCommand(profile.Commands, "setSpeed", "set-speed", "speed")
	method := strings.ToUpper(strings.TrimSpace(profile.Connection.HTTPMethod))
	if method == "" {
		method = http.MethodPost
	}
	parsers, err := CompileResponseParsers(profile.ResponseParsers)
	if err != nil {
		return nil, err
	}
	requestTimeout := durationFromMillis(profile.Connection.RequestTimeoutMs, defaultHTTPTimeout, maxProfileHTTPTimeout)
	minSendInterval := durationFromMillis(profile.Connection.MinSendIntervalMs, defaultMinSendInterval, maxProfileSendInterval)
	retryBackoff := durationFromMillis(profile.Connection.RetryBackoffMs, defaultRetryBackoff, maxProfileRetryBackoff)
	maxRetries := clampInt(profile.Connection.MaxRetries, 0, maxProfileRetryCount)
	return &WiFiExecutor{
		profile:         profile,
		client:          client,
		baseURL:         baseURL,
		stateEndpoint:   profile.Connection.StateEndpoint,
		speedEndpoint:   profile.Connection.SpeedEndpoint,
		method:          method,
		setCommand:      command,
		hasSetCommand:   hasCommand,
		parsers:         parsers,
		requestTimeout:  requestTimeout,
		minSendInterval: minSendInterval,
		maxRetries:      maxRetries,
		retryBackoff:    retryBackoff,
		now:             time.Now,
		sleep:           sleepContext,
	}, nil
}

func (e *WiFiExecutor) Profile() types.DeviceProfile {
	return e.profile
}

func (e *WiFiExecutor) ReadState(ctx context.Context) (*types.FanData, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	body, err := e.do(ctx, http.MethodGet, e.stateEndpoint, nil, "")
	if err != nil {
		return nil, err
	}
	return e.fanDataFromBody(body, 0)
}

func (e *WiFiExecutor) SetSpeed(ctx context.Context, speed types.FanSpeedValue) (*types.FanData, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	speed = speed.Normalized()
	if speed.Unit != types.NormalizeFanSpeedUnit(e.profile.SpeedUnit) {
		return nil, fmt.Errorf("speed unit %q does not match profile unit %q", speed.Unit, e.profile.SpeedUnit)
	}

	vars := SpeedVarsFromValue(speed)
	endpoint := RenderTemplate(e.speedEndpoint, vars)
	body, contentType, err := e.speedRequestBody(vars)
	if err != nil {
		return nil, err
	}
	if err := e.waitForSendSlot(ctx); err != nil {
		return nil, err
	}
	responseBody, err := e.do(ctx, e.method, endpoint, body, contentType)
	if err != nil {
		return nil, err
	}
	if err := validateSetSpeedResponse(responseBody); err != nil {
		return nil, err
	}

	fanData, err := e.ReadState(ctx)
	if err != nil {
		return nil, err
	}
	if fanData.TargetRPM == 0 {
		fanData.TargetRPM = uint16(clampUint16(vars.Value))
	}
	return fanData, nil
}

func (e *WiFiExecutor) speedRequestBody(vars SpeedVars) ([]byte, string, error) {
	if e.method == http.MethodGet {
		return nil, "", nil
	}
	if e.hasSetCommand {
		return EncodeCommand(e.setCommand, vars)
	}

	payload, err := json.Marshal(map[string]int{"speed": vars.Value})
	if err != nil {
		return nil, "", err
	}
	return payload, "application/json", nil
}

func (e *WiFiExecutor) do(ctx context.Context, method, endpoint string, body []byte, contentType string) ([]byte, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}
	urlString, err := e.urlFor(endpoint)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		responseBody, err := e.doOnce(ctx, method, endpoint, urlString, body, contentType)
		if err == nil {
			return responseBody, nil
		}
		lastErr = err
		if attempt >= e.maxRetries || !shouldRetryHTTPError(err) {
			break
		}
		if err := e.sleep(ctx, e.retryDelay(attempt)); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (e *WiFiExecutor) doOnce(ctx context.Context, method, endpoint, urlString string, body []byte, contentType string) ([]byte, error) {
	requestCtx, cancel := context.WithTimeout(ctx, e.requestTimeout)
	defer cancel()

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(requestCtx, method, urlString, reader)
	if err != nil {
		return nil, err
	}
	if contentType != "" && len(body) > 0 {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpStatusError{method: method, endpoint: endpoint, statusCode: resp.StatusCode}
	}
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPBodyBytes))
	if err != nil {
		return nil, err
	}
	return responseBody, nil
}

type httpStatusError struct {
	method     string
	endpoint   string
	statusCode int
}

func (e httpStatusError) Error() string {
	return fmt.Sprintf("%s %s returned HTTP %d", e.method, e.endpoint, e.statusCode)
}

func shouldRetryHTTPError(err error) bool {
	if err == nil {
		return false
	}
	if statusErr, ok := err.(httpStatusError); ok {
		return statusErr.statusCode == http.StatusRequestTimeout ||
			statusErr.statusCode == http.StatusTooManyRequests ||
			statusErr.statusCode >= http.StatusInternalServerError
	}
	return true
}

func (e *WiFiExecutor) retryDelay(attempt int) time.Duration {
	if e.retryBackoff <= 0 {
		return 0
	}
	multiplier := attempt + 1
	if multiplier < 1 {
		multiplier = 1
	}
	return time.Duration(multiplier) * e.retryBackoff
}

func (e *WiFiExecutor) waitForSendSlot(ctx context.Context) error {
	if e.minSendInterval <= 0 {
		return nil
	}

	e.sendMutex.Lock()
	defer e.sendMutex.Unlock()

	now := e.now()
	if !e.lastSend.IsZero() {
		wait := e.minSendInterval - now.Sub(e.lastSend)
		if wait > 0 {
			if err := e.sleep(ctx, wait); err != nil {
				return err
			}
			now = e.now()
		}
	}
	e.lastSend = now
	return nil
}

func (e *WiFiExecutor) urlFor(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = "/"
	}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint, nil
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	return e.baseURL + endpoint, nil
}

func (e *WiFiExecutor) fanDataFromBody(body []byte, fallbackTarget int) (*types.FanData, error) {
	state, err := e.parsers.Parse(body)
	if err != nil {
		return nil, err
	}
	if !state.HasCurrent && fallbackTarget > 0 {
		state.CurrentSpeed = fallbackTarget
		state.HasCurrent = true
	}
	if !state.HasTarget {
		if fallbackTarget > 0 {
			state.TargetSpeed = fallbackTarget
		} else {
			state.TargetSpeed = state.CurrentSpeed
		}
		state.HasTarget = state.HasCurrent || fallbackTarget > 0
	}
	if !state.HasCurrent && !state.HasTarget {
		return nil, fmt.Errorf("wifi profile state response did not contain speed data")
	}

	current := clampForProfile(state.CurrentSpeed, e.profile.SpeedUnit)
	target := clampForProfile(state.TargetSpeed, e.profile.SpeedUnit)
	workMode := strings.TrimSpace(state.WorkMode)
	if workMode == "" {
		workMode = "software"
	}
	return &types.FanData{
		CurrentRPM: uint16(current),
		TargetRPM:  uint16(target),
		WorkMode:   workMode,
		Transport:  types.DeviceTransportWiFi,
		SpeedUnit:  types.NormalizeFanSpeedUnit(e.profile.SpeedUnit),
	}, nil
}

func normalizeBaseEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = types.DefaultFanDeviceIP
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func validateSetSpeedResponse(body []byte) error {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil
	}
	lower := strings.ToLower(trimmed)
	if lower == "ok" || lower == "success" {
		return nil
	}

	var data struct {
		Status      string `json:"status"`
		ControlMode string `json:"controlMode"`
		Success     *bool  `json:"success"`
		Message     string `json:"message"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("invalid speed response: %w", err)
	}
	if data.Success != nil {
		if *data.Success {
			return nil
		}
		return fmt.Errorf("speed response success=false")
	}
	status := strings.ToLower(strings.TrimSpace(data.Status))
	controlMode := strings.ToLower(strings.TrimSpace(data.ControlMode))
	if status == "" && controlMode == "" && data.Message == "" && data.Error == "" {
		return nil
	}
	if status == "success" || status == "ok" || controlMode == "wifi" || controlMode == "software" {
		return nil
	}
	if strings.TrimSpace(data.Error) != "" {
		return fmt.Errorf("%s", strings.TrimSpace(data.Error))
	}
	if strings.TrimSpace(data.Message) != "" {
		return fmt.Errorf("%s", strings.TrimSpace(data.Message))
	}
	return fmt.Errorf("status=%q controlMode=%q", data.Status, data.ControlMode)
}

func clampForProfile(value int, unit string) int {
	if types.IsRPMSpeedUnit(unit) {
		return clampUint16(types.ClampRPM(value))
	}
	return types.ClampFanPercent(value)
}

func clampUint16(value int) int {
	if value < 0 {
		return 0
	}
	if value > 65535 {
		return 65535
	}
	return value
}

func durationFromMillis(value int, fallback, maxValue time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	duration := time.Duration(value) * time.Millisecond
	if duration > maxValue {
		return maxValue
	}
	return duration
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: defaultHTTPTimeout,
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: (&net.Dialer{
				Timeout:   defaultHTTPTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   defaultHTTPTimeout,
			ResponseHeaderTimeout: defaultHTTPTimeout,
			IdleConnTimeout:       30 * time.Second,
		},
	}
}
