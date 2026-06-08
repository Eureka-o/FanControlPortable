package device

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

const (
	wifiDiscoveryDefaultTimeout = 450 * time.Millisecond
	wifiDiscoveryDeepTimeout    = 180 * time.Millisecond
	wifiDiscoveryNormalLimit    = 8 * time.Second
	wifiDiscoveryDeepLimit      = 75 * time.Second
	wifiDiscoveryDynamicLimit   = 6 * time.Second
)

type wifiDiscoveryEndpoint struct {
	Scheme string
	Host   string
	Port   string
}

type wifiDiscoveryCandidate struct {
	Endpoint string
	IP       string
	Port     string
	Source   string
	Network  string
}

func DiscoverWiFiDevices(ctx context.Context, params types.WiFiDiscoveryParams) (result types.WiFiDiscoveryResult) {
	start := time.Now()
	mode := normalizeWiFiDiscoveryMode(params.Mode)
	result = types.WiFiDiscoveryResult{Mode: mode}
	defer func() {
		result.ElapsedMs = time.Since(start).Milliseconds()
		if params.Control != nil && params.Control.IsCanceled() {
			result.Canceled = true
		}
	}()

	timeout := wifiDiscoveryTimeout(params.TimeoutMs, mode)
	candidates, scopes, err := buildWiFiDiscoveryCandidates(params, mode)
	result.Scopes = scopes
	result.CandidateCount = len(candidates)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if len(candidates) == 0 {
		result.Error = "no WiFi discovery candidates"
		return result
	}

	limit := wifiDiscoveryOverallLimit(mode)
	if limit > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, limit)
		defer cancel()
	}

	client := newWiFiDiscoveryHTTPClient(timeout)
	stateEndpoint := normalizeWiFiDiscoveryStateEndpoint(params.StateEndpoint)
	devices, scannedCount := scanWiFiDiscoveryCandidatesByMode(ctx, client, candidates, stateEndpoint, params, mode)
	result.Devices = devices
	result.ScannedCount = scannedCount
	result.Found = len(devices) > 0
	return result
}

func normalizeWiFiDiscoveryMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case types.WiFiDiscoveryModeDeep:
		return types.WiFiDiscoveryModeDeep
	case types.WiFiDiscoveryModeDynamic:
		return types.WiFiDiscoveryModeDynamic
	default:
		return types.WiFiDiscoveryModeNormal
	}
}

func wifiDiscoveryTimeout(timeoutMs int, mode string) time.Duration {
	if timeoutMs > 0 {
		if timeoutMs < 120 {
			timeoutMs = 120
		}
		if timeoutMs > 2000 {
			timeoutMs = 2000
		}
		return time.Duration(timeoutMs) * time.Millisecond
	}
	if mode == types.WiFiDiscoveryModeDeep {
		return wifiDiscoveryDeepTimeout
	}
	return wifiDiscoveryDefaultTimeout
}

func wifiDiscoveryOverallLimit(mode string) time.Duration {
	switch mode {
	case types.WiFiDiscoveryModeDeep:
		return wifiDiscoveryDeepLimit
	case types.WiFiDiscoveryModeDynamic:
		return wifiDiscoveryDynamicLimit
	default:
		return wifiDiscoveryNormalLimit
	}
}

func newWiFiDiscoveryHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: (&net.Dialer{
				Timeout:   timeout,
				KeepAlive: 15 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   timeout,
			ResponseHeaderTimeout: timeout,
			IdleConnTimeout:       10 * time.Second,
			MaxIdleConns:          512,
			MaxIdleConnsPerHost:   4,
		},
	}
}

func buildWiFiDiscoveryCandidates(params types.WiFiDiscoveryParams, mode string) ([]wifiDiscoveryCandidate, []types.WiFiDiscoveryScope, error) {
	base, err := parseWiFiDiscoveryEndpoint(params.Endpoint)
	if err != nil && mode == types.WiFiDiscoveryModeDynamic {
		return nil, nil, err
	}
	if err != nil {
		base = wifiDiscoveryEndpoint{Scheme: "http"}
	}
	if base.Scheme == "" {
		base.Scheme = "http"
	}

	var candidates []wifiDiscoveryCandidate
	var scopes []types.WiFiDiscoveryScope
	seen := map[string]bool{}
	addCandidate := func(source, network, ip string) bool {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			return false
		}
		endpoint := wifiDiscoveryBaseURL(base.Scheme, ip, base.Port)
		if seen[endpoint] {
			return false
		}
		seen[endpoint] = true
		candidates = append(candidates, wifiDiscoveryCandidate{
			Endpoint: endpoint,
			IP:       ip,
			Port:     base.Port,
			Source:   source,
			Network:  network,
		})
		return true
	}
	addScope := func(source, network string, ips []string) {
		count := 0
		for _, ip := range ips {
			if addCandidate(source, network, ip) {
				count++
			}
		}
		if count > 0 {
			scopes = append(scopes, types.WiFiDiscoveryScope{
				Source:         source,
				Network:        network,
				CandidateCount: count,
			})
		}
	}
	addSubnetScopes := func(source string, subnets []string) {
		for _, subnet := range subnets {
			addScope(source, subnet+".0/24", subnetHosts(subnet))
		}
	}

	if mode != types.WiFiDiscoveryModeDynamic && base.Host != "" {
		if addCandidate("exact", base.Host, base.Host) {
			scopes = append(scopes, types.WiFiDiscoveryScope{Source: "exact", Network: base.Host, CandidateCount: 1})
		}
	}

	if subnet := ipv4Subnet24(base.Host); subnet != "" {
		addScope("savedSubnet", subnet+".0/24", subnetHosts(subnet))
	} else if mode == types.WiFiDiscoveryModeDynamic {
		return candidates, scopes, fmt.Errorf("dynamic IP compatibility requires an IPv4 WiFi endpoint")
	}

	if mode == types.WiFiDiscoveryModeDynamic {
		return candidates, scopes, nil
	}

	for _, subnet := range localIPv4Subnets24() {
		addScope("adapterSubnet", subnet+".0/24", subnetHosts(subnet))
	}
	addScope("windowsHotspot", "192.168.137.0/24", subnetHosts("192.168.137"))
	addScope("deviceAP", "192.168.4.1-2", []string{"192.168.4.1", "192.168.4.2"})

	if mode == types.WiFiDiscoveryModeDeep {
		addSubnetScopes("commonSubnet", commonWiFiDiscoverySubnets())
		addSubnetScopes("expandedSubnet", expandedWiFiDiscoverySubnets())
	}

	return candidates, scopes, nil
}

func parseWiFiDiscoveryEndpoint(endpoint string) (wifiDiscoveryEndpoint, error) {
	normalized, err := normalizeWiFiEndpoint(endpoint)
	if err != nil {
		return wifiDiscoveryEndpoint{}, err
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return wifiDiscoveryEndpoint{}, err
	}
	return wifiDiscoveryEndpoint{
		Scheme: parsed.Scheme,
		Host:   parsed.Hostname(),
		Port:   parsed.Port(),
	}, nil
}

func wifiDiscoveryBaseURL(scheme, ip, port string) string {
	if scheme == "" {
		scheme = "http"
	}
	host := ip
	if port != "" {
		host = net.JoinHostPort(ip, port)
	}
	return scheme + "://" + host
}

func ipv4Subnet24(host string) string {
	ip := net.ParseIP(strings.TrimSpace(host)).To4()
	if ip == nil {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d", ip[0], ip[1], ip[2])
}

func subnetHosts(subnet string) []string {
	out := make([]string, 0, 254)
	for i := 1; i <= 254; i++ {
		out = append(out, fmt.Sprintf("%s.%d", subnet, i))
	}
	return out
}

func commonWiFiDiscoverySubnets() []string {
	return []string{
		"192.168.0",
		"192.168.1",
		"192.168.2",
		"192.168.3",
		"192.168.4",
		"192.168.5",
		"192.168.10",
		"192.168.31",
		"192.168.50",
		"192.168.88",
		"192.168.100",
		"192.168.137",
		"10.0.0",
		"10.0.1",
		"10.1.1",
		"10.10.0",
		"10.10.10",
		"172.16.0",
		"172.20.10",
		"172.31.0",
	}
}

func expandedWiFiDiscoverySubnets() []string {
	seen := map[string]bool{}
	subnets := make([]string, 0, 320)
	add := func(subnet string) {
		subnet = strings.TrimSpace(subnet)
		if subnet == "" || seen[subnet] {
			return
		}
		seen[subnet] = true
		subnets = append(subnets, subnet)
	}
	for i := 0; i <= 255; i++ {
		add(fmt.Sprintf("192.168.%d", i))
	}
	for _, subnet := range commonWiFiDiscoverySubnets() {
		add(subnet)
	}
	for _, subnet := range []string{
		"10.0.2",
		"10.0.4",
		"10.0.8",
		"10.0.10",
		"10.0.20",
		"10.0.50",
		"10.0.88",
		"10.0.100",
		"10.0.137",
		"10.1.0",
		"10.1.2",
		"10.10.1",
		"10.137.137",
		"172.16.1",
		"172.16.2",
		"172.16.4",
		"172.16.10",
		"172.16.20",
		"172.20.0",
		"172.20.1",
		"172.31.1",
		"172.31.137",
	} {
		add(subnet)
	}
	return subnets
}

func localIPv4Subnets24() []string {
	seen := map[string]bool{}
	var subnets []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			ip = ip.To4()
			if ip == nil || ip[0] == 169 && ip[1] == 254 {
				continue
			}
			subnet := fmt.Sprintf("%d.%d.%d", ip[0], ip[1], ip[2])
			if !seen[subnet] {
				seen[subnet] = true
				subnets = append(subnets, subnet)
			}
		}
	}
	sort.Strings(subnets)
	return subnets
}

func normalizeWiFiDiscoveryStateEndpoint(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/api/data"
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		parsed, err := url.Parse(path)
		if err == nil {
			path = parsed.EscapedPath()
			if path == "" {
				path = "/"
			}
			if parsed.RawQuery != "" {
				path += "?" + parsed.RawQuery
			}
			return path
		}
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func scanWiFiDiscoveryCandidatesByMode(
	ctx context.Context,
	client *http.Client,
	candidates []wifiDiscoveryCandidate,
	stateEndpoint string,
	params types.WiFiDiscoveryParams,
	mode string,
) ([]types.WiFiDiscoveredDevice, int) {
	if mode != types.WiFiDiscoveryModeDeep {
		return scanWiFiDiscoveryCandidates(ctx, client, candidates, stateEndpoint, params)
	}

	common, expanded := splitWiFiDiscoveryCandidates(candidates)
	devices, scannedCount := scanWiFiDiscoveryCandidates(ctx, client, common, stateEndpoint, params)
	if len(devices) > 0 || ctx.Err() != nil || len(expanded) == 0 {
		return devices, scannedCount
	}

	expandedDevices, expandedScannedCount := scanWiFiDiscoveryCandidates(ctx, client, expanded, stateEndpoint, params)
	devices = append(devices, expandedDevices...)
	return devices, scannedCount + expandedScannedCount
}

func splitWiFiDiscoveryCandidates(candidates []wifiDiscoveryCandidate) ([]wifiDiscoveryCandidate, []wifiDiscoveryCandidate) {
	common := make([]wifiDiscoveryCandidate, 0, len(candidates))
	expanded := make([]wifiDiscoveryCandidate, 0)
	for _, candidate := range candidates {
		if candidate.Source == "expandedSubnet" {
			expanded = append(expanded, candidate)
			continue
		}
		common = append(common, candidate)
	}
	return common, expanded
}

func scanWiFiDiscoveryCandidates(
	ctx context.Context,
	client *http.Client,
	candidates []wifiDiscoveryCandidate,
	stateEndpoint string,
	params types.WiFiDiscoveryParams,
) ([]types.WiFiDiscoveredDevice, int) {
	workerCount := 64
	if len(candidates) < workerCount {
		workerCount = len(candidates)
	}
	if workerCount < 1 {
		workerCount = 1
	}
	if len(candidates) > 800 {
		workerCount = 96
	}
	if len(candidates) > 50000 {
		workerCount = 384
	}

	jobs := make(chan wifiDiscoveryCandidate)
	results := make(chan types.WiFiDiscoveredDevice, len(candidates))
	var wg sync.WaitGroup
	var scannedCount int64
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for candidate := range jobs {
				if !params.Control.Wait(ctx) {
					continue
				}
				atomic.AddInt64(&scannedCount, 1)
				device, ok := probeWiFiDiscoveryCandidate(ctx, client, candidate, stateEndpoint, params)
				if ok {
					results <- device
				}
			}
		}()
	}

dispatch:
	for _, candidate := range candidates {
		if !params.Control.Wait(ctx) {
			break dispatch
		}
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- candidate:
		}
	}
	close(jobs)
	wg.Wait()
	close(results)

	devices := make([]types.WiFiDiscoveredDevice, 0)
	for device := range results {
		devices = append(devices, device)
	}
	sort.SliceStable(devices, func(i, j int) bool {
		if devices[i].Source != devices[j].Source {
			return wifiDiscoverySourceRank(devices[i].Source) < wifiDiscoverySourceRank(devices[j].Source)
		}
		return devices[i].Endpoint < devices[j].Endpoint
	})
	return devices, int(atomic.LoadInt64(&scannedCount))
}

func wifiDiscoverySourceRank(source string) int {
	switch source {
	case "exact":
		return 0
	case "savedSubnet":
		return 1
	case "adapterSubnet":
		return 2
	case "windowsHotspot":
		return 3
	case "deviceAP":
		return 4
	case "commonSubnet":
		return 5
	case "expandedSubnet":
		return 6
	default:
		return 9
	}
}

func probeWiFiDiscoveryCandidate(
	ctx context.Context,
	client *http.Client,
	candidate wifiDiscoveryCandidate,
	stateEndpoint string,
	params types.WiFiDiscoveryParams,
) (types.WiFiDiscoveredDevice, bool) {
	targetURL, err := wifiDiscoveryStateURL(candidate.Endpoint, stateEndpoint)
	if err != nil {
		return types.WiFiDiscoveredDevice{}, false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return types.WiFiDiscoveredDevice{}, false
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return types.WiFiDiscoveredDevice{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return types.WiFiDiscoveredDevice{}, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil || !looksLikeWiFiDiscoveryState(body) {
		return types.WiFiDiscoveredDevice{}, false
	}
	var data wifiDataResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return types.WiFiDiscoveredDevice{}, false
	}
	fanData := fanDataFromWiFiResponse(data)
	name := strings.TrimSpace(params.ProfileName)
	if name == "" {
		name = wifiOnlyModelName
	}
	temperature, _ := intFromAny(data.Temperature)
	return types.WiFiDiscoveredDevice{
		Name:          name,
		ProfileID:     strings.TrimSpace(params.ProfileID),
		Transport:     types.DeviceTransportWiFi,
		Endpoint:      candidate.Endpoint,
		IP:            candidate.IP,
		Port:          candidate.Port,
		Source:        candidate.Source,
		Network:       candidate.Network,
		Speed:         int(fanData.CurrentRPM),
		TargetSpeed:   int(fanData.TargetRPM),
		Temperature:   temperature,
		LatencyMs:     time.Since(start).Milliseconds(),
		StateEndpoint: stateEndpoint,
	}, true
}

func wifiDiscoveryStateURL(endpoint, stateEndpoint string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	stateEndpoint = normalizeWiFiDiscoveryStateEndpoint(stateEndpoint)
	path := stateEndpoint
	rawQuery := ""
	if idx := strings.IndexByte(stateEndpoint, '?'); idx >= 0 {
		path = stateEndpoint[:idx]
		rawQuery = stateEndpoint[idx+1:]
	}
	parsed.Path = path
	parsed.RawQuery = rawQuery
	parsed.Fragment = ""
	return parsed.String(), nil
}

func looksLikeWiFiDiscoveryState(body []byte) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return false
	}
	for _, key := range []string{
		"speed",
		"fanSpeed",
		"targetSpeed",
		"wifiTargetSpeed",
		"wifiControl",
		"controlMode",
		"mode",
		"temperature",
		"power",
	} {
		if _, ok := raw[key]; ok {
			return true
		}
	}
	return false
}
