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
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func (t *httpWiFiTransport) do(ctx context.Context, method, endpoint string, body []byte, contentType string) ([]byte, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}
	urlString, err := t.urlFor(endpoint)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		responseBody, err := t.doOnce(ctx, method, endpoint, urlString, body, contentType)
		if err == nil {
			return responseBody, nil
		}
		lastErr = err
		if attempt >= t.maxRetries || !shouldRetryHTTPError(err) {
			break
		}
		if err := t.sleepFor(ctx, t.retryDelay(attempt)); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (t *httpWiFiTransport) doOnce(ctx context.Context, method, endpoint, urlString string, body []byte, contentType string) ([]byte, error) {
	requestCtx, cancel := context.WithTimeout(ctx, t.requestTimeout)
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

	resp, err := t.client.Do(req)
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

type wifiTransport interface {
	Do(context.Context, wifiRequest) ([]byte, error)
}

type httpWiFiTransport struct {
	client         HTTPDoer
	baseURL        string
	requestTimeout time.Duration
	maxRetries     int
	retryBackoff   time.Duration
	sleep          *func(context.Context, time.Duration) error
}

func newHTTPWiFiTransport(settings types.DeviceConnectionSettings, baseURL string, client HTTPDoer, sleep *func(context.Context, time.Duration) error) *httpWiFiTransport {
	return &httpWiFiTransport{
		client:         client,
		baseURL:        baseURL,
		requestTimeout: durationFromMillis(settings.RequestTimeoutMs, defaultHTTPTimeout, maxProfileHTTPTimeout),
		maxRetries:     clampInt(settings.MaxRetries, 0, maxProfileRetryCount),
		retryBackoff:   durationFromMillis(settings.RetryBackoffMs, defaultRetryBackoff, maxProfileRetryBackoff),
		sleep:          sleep,
	}
}

func (t *httpWiFiTransport) Do(ctx context.Context, req wifiRequest) ([]byte, error) {
	return t.do(ctx, req.method, req.endpoint, req.body, req.contentType)
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

func (t *httpWiFiTransport) retryDelay(attempt int) time.Duration {
	if t.retryBackoff <= 0 {
		return 0
	}
	multiplier := attempt + 1
	if multiplier < 1 {
		multiplier = 1
	}
	return time.Duration(multiplier) * t.retryBackoff
}

func (t *httpWiFiTransport) sleepFor(ctx context.Context, duration time.Duration) error {
	if t.sleep != nil && *t.sleep != nil {
		return (*t.sleep)(ctx, duration)
	}
	return sleepContext(ctx, duration)
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

func (t *httpWiFiTransport) urlFor(endpoint string) (string, error) {
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
	return t.baseURL + endpoint, nil
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
	if ctx == nil {
		ctx = context.Background()
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
