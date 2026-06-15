package deviceprofileexec

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestRenderTemplateKeepsPercentTicks(t *testing.T) {
	rendered := RenderTemplate(
		`{"percent":{{percent}},"ticks":{{percentTicks}},"decimal":{{decimalPercent}}}`,
		SpeedVarsFromValue(types.NewPercentTickSpeed(555)),
	)
	if rendered != `{"percent":56,"ticks":555,"decimal":55.5}` {
		t.Fatalf("rendered template = %s", rendered)
	}
}

func TestWiFiExecutorUsesCustomTemplateEndpointAndParsers(t *testing.T) {
	var posted struct {
		Duty    int     `json:"duty"`
		Ticks   int     `json:"ticks"`
		Decimal float64 `json:"decimal"`
	}
	setSeen := false
	getCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/state":
			if r.Method != http.MethodGet {
				t.Fatalf("state method = %s, want GET", r.Method)
			}
			getCount++
			speed := 42
			if setSeen {
				speed = 56
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"fan": map[string]any{
					"current": speed,
					"target":  speed,
				},
				"mode": "wifi",
			})
		case "/v1/control":
			if r.Method != http.MethodPut {
				t.Fatalf("control method = %s, want PUT", r.Method)
			}
			if r.URL.Query().Get("ticks") != "555" {
				t.Fatalf("control query ticks = %q, want 555", r.URL.Query().Get("ticks"))
			}
			if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
				t.Fatalf("decode posted body: %v", err)
			}
			setSeen = true
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor, err := NewWiFiExecutor(types.DeviceProfile{
		ID:          "user.custom.percent",
		DisplayName: "Custom percent WiFi",
		Transport:   types.DeviceTransportWiFi,
		SpeedUnit:   types.FanSpeedUnitPercent,
		SpeedRange:  types.DefaultPercentSpeedRange(),
		Connection: types.DeviceConnectionSettings{
			Endpoint:      server.URL,
			StateEndpoint: "/v1/state",
			SpeedEndpoint: "/v1/control?ticks={{percentTicks}}",
			HTTPMethod:    http.MethodPut,
		},
		Commands: []types.DeviceCommandTemplate{
			{
				Name:     "setSpeed",
				Command:  `{"duty":{{percent}},"ticks":{{percentTicks}},"decimal":{{decimalPercent}}}`,
				Encoding: "json",
			},
		},
		ResponseParsers: []types.DeviceResponseParser{
			{Name: "current speed", Type: "json_path", Expression: "$.fan.current"},
			{Name: "target speed", Type: "json_path", Expression: "$.fan.target"},
		},
		Capabilities: types.DefaultWiFiPercentCapabilities(),
	}, "", nil)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	initial, err := executor.ReadState(nil)
	if err != nil {
		t.Fatalf("read initial state: %v", err)
	}
	if initial.CurrentRPM != 42 || initial.TargetRPM != 42 {
		t.Fatalf("initial speed = %d/%d, want 42/42", initial.CurrentRPM, initial.TargetRPM)
	}

	next, err := executor.SetSpeed(nil, types.NewPercentTickSpeed(555))
	if err != nil {
		t.Fatalf("set speed: %v", err)
	}
	if !setSeen {
		t.Fatal("expected custom set endpoint to be called")
	}
	if posted.Duty != 56 || posted.Ticks != 555 || posted.Decimal != 55.5 {
		t.Fatalf("posted body = %+v, want duty 56 ticks 555 decimal 55.5", posted)
	}
	if getCount != 2 {
		t.Fatalf("state reads = %d, want 2", getCount)
	}
	if next.CurrentRPM != 56 || next.TargetRPM != 56 || next.SpeedUnit != types.FanSpeedUnitPercent {
		t.Fatalf("next fan data = %#v", next)
	}
}

func TestWiFiExecutorRejectsFailureResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			_ = json.NewEncoder(w).Encode(map[string]any{"speed": 30, "targetSpeed": 30})
		case "/speed":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "rejected"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	profile := types.DefaultWiFiPercentProfile(server.URL)
	profile.Connection.StateEndpoint = "/state"
	profile.Connection.SpeedEndpoint = "/speed"
	executor, err := NewWiFiExecutor(profile, "", nil)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	if _, err := executor.SetSpeed(nil, types.NewPercentSpeed(55)); err == nil {
		t.Fatal("expected failure response to be rejected")
	}
}

func TestWiFiExecutorDefaultParserUsesFanSpeedAsCurrentWhenSpeedIsSetpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"speed":       80,
				"fanSpeed":    35,
				"controlMode": "manual",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	profile := types.DefaultWiFiPercentProfile(server.URL)
	profile.Connection.StateEndpoint = "/state"
	executor, err := NewWiFiExecutor(profile, "", nil)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	fanData, err := executor.ReadState(nil)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if fanData.CurrentRPM != 35 || fanData.TargetRPM != 80 {
		t.Fatalf("fan data speed = current %d target %d, want 35/80", fanData.CurrentRPM, fanData.TargetRPM)
	}
}

func TestWiFiExecutorRetriesRetryableHTTPStatus(t *testing.T) {
	speedAttempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			_ = json.NewEncoder(w).Encode(map[string]any{"speed": 55, "targetSpeed": 55})
		case "/speed":
			speedAttempts++
			if speedAttempts == 1 {
				http.Error(w, "busy", http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	profile := types.DefaultWiFiPercentProfile(server.URL)
	profile.Connection.StateEndpoint = "/state"
	profile.Connection.SpeedEndpoint = "/speed"
	profile.Connection.MaxRetries = 1
	profile.Connection.RetryBackoffMs = 25
	executor, err := NewWiFiExecutor(profile, "", nil)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	var slept []time.Duration
	executor.sleep = func(_ context.Context, duration time.Duration) error {
		slept = append(slept, duration)
		return nil
	}

	if _, err := executor.SetSpeed(nil, types.NewPercentSpeed(55)); err != nil {
		t.Fatalf("set speed with retry: %v", err)
	}
	if speedAttempts != 2 {
		t.Fatalf("speed attempts = %d, want 2", speedAttempts)
	}
	if len(slept) != 1 || slept[0] != 25*time.Millisecond {
		t.Fatalf("retry sleeps = %v, want [25ms]", slept)
	}
}

func TestWiFiExecutorRateLimitsSetSpeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			_ = json.NewEncoder(w).Encode(map[string]any{"speed": 55, "targetSpeed": 55})
		case "/speed":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	profile := types.DefaultWiFiPercentProfile(server.URL)
	profile.Connection.StateEndpoint = "/state"
	profile.Connection.SpeedEndpoint = "/speed"
	profile.Connection.MinSendIntervalMs = 250
	executor, err := NewWiFiExecutor(profile, "", nil)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	now := time.Unix(100, 0)
	var slept []time.Duration
	executor.now = func() time.Time { return now }
	executor.sleep = func(_ context.Context, duration time.Duration) error {
		slept = append(slept, duration)
		now = now.Add(duration)
		return nil
	}

	if _, err := executor.SetSpeed(nil, types.NewPercentSpeed(40)); err != nil {
		t.Fatalf("first set speed: %v", err)
	}
	if _, err := executor.SetSpeed(nil, types.NewPercentSpeed(55)); err != nil {
		t.Fatalf("second set speed: %v", err)
	}
	if len(slept) != 1 || slept[0] != 250*time.Millisecond {
		t.Fatalf("rate-limit sleeps = %v, want [250ms]", slept)
	}
}
