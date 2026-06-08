package types

const (
	WiFiDiscoveryModeNormal  = "normal"
	WiFiDiscoveryModeDeep    = "deep"
	WiFiDiscoveryModeDynamic = "dynamic"
)

type WiFiDiscoveryParams struct {
	Mode          string `json:"mode"`
	Endpoint      string `json:"endpoint,omitempty"`
	ProfileID     string `json:"profileId,omitempty"`
	ProfileName   string `json:"profileName,omitempty"`
	StateEndpoint string `json:"stateEndpoint,omitempty"`
	TimeoutMs     int    `json:"timeoutMs,omitempty"`
}

type WiFiDiscoveryScope struct {
	Source         string `json:"source"`
	Network        string `json:"network"`
	CandidateCount int    `json:"candidateCount"`
}

type WiFiDiscoveredDevice struct {
	Name          string `json:"name"`
	ProfileID     string `json:"profileId,omitempty"`
	Transport     string `json:"transport"`
	Endpoint      string `json:"endpoint"`
	IP            string `json:"ip"`
	Port          string `json:"port,omitempty"`
	Source        string `json:"source"`
	Network       string `json:"network,omitempty"`
	Speed         int    `json:"speed,omitempty"`
	TargetSpeed   int    `json:"targetSpeed,omitempty"`
	Temperature   int    `json:"temperature,omitempty"`
	LatencyMs     int64  `json:"latencyMs,omitempty"`
	StateEndpoint string `json:"stateEndpoint,omitempty"`
}

type WiFiDiscoveryResult struct {
	Mode           string                 `json:"mode"`
	Found          bool                   `json:"found"`
	Devices        []WiFiDiscoveredDevice `json:"devices,omitempty"`
	Scopes         []WiFiDiscoveryScope   `json:"scopes,omitempty"`
	CandidateCount int                    `json:"candidateCount"`
	ElapsedMs      int64                  `json:"elapsedMs"`
	Error          string                 `json:"error,omitempty"`
}
