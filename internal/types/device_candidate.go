package types

const (
	DeviceScanModeNormal = "normal"
	DeviceScanModeDeep   = "deep"
)

type DeviceCandidate struct {
	ID          string `json:"id"`
	Transport   string `json:"transport"`
	Name        string `json:"name"`
	ProfileID   string `json:"profileId,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	Source      string `json:"source,omitempty"`
	Network     string `json:"network,omitempty"`
	Speed       int    `json:"speed,omitempty"`
	TargetSpeed int    `json:"targetSpeed,omitempty"`
	Temperature int    `json:"temperature,omitempty"`
	LatencyMs   int64  `json:"latencyMs,omitempty"`
	Connected   bool   `json:"connected,omitempty"`
	Connectable bool   `json:"connectable"`
	Error       string `json:"error,omitempty"`
}

type DeviceScanResult struct {
	Mode          string            `json:"mode"`
	Connected     bool              `json:"connected"`
	Devices       []DeviceCandidate `json:"devices"`
	WiFiEnabled   bool              `json:"wifiEnabled"`
	SerialEnabled bool              `json:"serialEnabled"`
	ShowDeepScan  bool              `json:"showDeepScan,omitempty"`
	Error         string            `json:"error,omitempty"`
}

type DeviceConnectRequest struct {
	ID        string `json:"id,omitempty"`
	Transport string `json:"transport,omitempty"`
	ProfileID string `json:"profileId,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
}
