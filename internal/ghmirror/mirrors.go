package ghmirror

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	probeTimeout     = 4 * time.Second
	probeGraceWindow = 250 * time.Millisecond
)

// Candidate describes one downloadable URL route (GitHub origin or a China mirror).
type Candidate struct {
	Name  string
	Label string
	URL   string
}

// RankedCandidate pairs a Candidate with measured probe latency.
type RankedCandidate struct {
	Candidate
	LatencyMs int64
	Probed    bool
}

var trustedMirrorHosts = map[string]bool{
	"gh-proxy.com":     true,
	"hk.gh-proxy.org":  true,
	"gh-proxy.org":     true,
	"ghfast.top":       true,
	"cdn.jsdelivr.net": true,
}

// IsTrustedMirrorHost reports whether host is a built-in GitHub acceleration mirror.
func IsTrustedMirrorHost(host string) bool {
	return trustedMirrorHosts[strings.ToLower(strings.TrimSpace(host))]
}

// BuildCandidates expands GitHub URLs into origin + China mirror candidates.
// Non-GitHub URLs (local servers, custom hosts) are returned unchanged as a single candidate.
func BuildCandidates(rawURL string) []Candidate {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return []Candidate{{Name: "origin", Label: "源站", URL: trimmed}}
	}

	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "raw.githubusercontent.com":
		candidates := []Candidate{
			{Name: "origin", Label: "GitHub 源站", URL: trimmed},
			{Name: "gh-proxy.com", Label: "gh-proxy.com 镜像", URL: "https://gh-proxy.com/" + trimmed},
			{Name: "hk.gh-proxy.org", Label: "hk.gh-proxy.org 镜像", URL: "https://hk.gh-proxy.org/" + trimmed},
			{Name: "gh-proxy.org", Label: "gh-proxy.org 镜像", URL: "https://gh-proxy.org/" + trimmed},
			{Name: "ghfast.top", Label: "ghfast.top 镜像", URL: "https://ghfast.top/" + trimmed},
		}
		if jsd := toJSDelivrRawURL(parsed); jsd != "" {
			candidates = append(candidates, Candidate{Name: "jsdelivr", Label: "jsDelivr 镜像", URL: jsd})
		}
		return candidates

	case "github.com", "api.github.com":
		return []Candidate{
			{Name: "origin", Label: "GitHub 源站", URL: trimmed},
			{Name: "gh-proxy.com", Label: "gh-proxy.com 镜像", URL: "https://gh-proxy.com/" + trimmed},
			{Name: "hk.gh-proxy.org", Label: "hk.gh-proxy.org 镜像", URL: "https://hk.gh-proxy.org/" + trimmed},
			{Name: "gh-proxy.org", Label: "gh-proxy.org 镜像", URL: "https://gh-proxy.org/" + trimmed},
			{Name: "ghfast.top", Label: "ghfast.top 镜像", URL: "https://ghfast.top/" + trimmed},
		}

	default:
		return []Candidate{{Name: "origin", Label: "源站", URL: trimmed}}
	}
}

func toJSDelivrRawURL(parsed *url.URL) string {
	// /<owner>/<repo>/<branch>/<path...>
	parts := strings.SplitN(strings.TrimPrefix(parsed.Path, "/"), "/", 4)
	if len(parts) < 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		return ""
	}
	return fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s/%s@%s/%s", parts[0], parts[1], parts[2], parts[3])
}

type raceResult struct {
	idx       int
	data      []byte
	latencyMs int64
	err       error
}

// RaceGet concurrently fetches candidates and returns the first valid HTTP 200 payload.
func RaceGet(
	ctx context.Context,
	candidates []Candidate,
	client *http.Client,
	headers map[string]string,
	maxBytes int64,
	validate func([]byte) error,
) ([]byte, Candidate, int64, error) {
	if len(candidates) == 0 {
		return nil, Candidate{}, 0, fmt.Errorf("no download candidates provided")
	}
	if client == nil {
		client = http.DefaultClient
	}

	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan raceResult, len(candidates))
	for i, c := range candidates {
		go func(idx int, cand Candidate) {
			start := time.Now()
			req, err := http.NewRequestWithContext(raceCtx, http.MethodGet, cand.URL, nil)
			if err != nil {
				ch <- raceResult{idx: idx, err: err}
				return
			}
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			if req.Header.Get("User-Agent") == "" {
				req.Header.Set("User-Agent", "FanControl-Downloader")
			}

			resp, err := client.Do(req)
			if err != nil {
				ch <- raceResult{idx: idx, err: err}
				return
			}
			var data []byte
			var readErr error
			if maxBytes > 0 {
				data, readErr = io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
			} else {
				data, readErr = io.ReadAll(resp.Body)
			}
			_ = resp.Body.Close()

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				ch <- raceResult{idx: idx, err: fmt.Errorf("%s HTTP %d", cand.Name, resp.StatusCode)}
				return
			}
			if readErr != nil {
				ch <- raceResult{idx: idx, err: readErr}
				return
			}
			if maxBytes > 0 && int64(len(data)) > maxBytes {
				ch <- raceResult{idx: idx, err: fmt.Errorf("%s payload exceeds limit", cand.Name)}
				return
			}
			if validate != nil {
				if vErr := validate(data); vErr != nil {
					ch <- raceResult{idx: idx, err: vErr}
					return
				}
			}
			ms := time.Since(start).Milliseconds()
			if ms < 1 {
				ms = 1
			}
			ch <- raceResult{idx: idx, data: data, latencyMs: ms}
		}(i, c)
	}

	var lastErr error
	for received := 0; received < len(candidates); received++ {
		select {
		case res := <-ch:
			if res.err == nil {
				cancel()
				return res.data, candidates[res.idx], res.latencyMs, nil
			}
			lastErr = res.err
		case <-ctx.Done():
			cancel()
			return nil, Candidate{}, 0, ctx.Err()
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all download candidates failed")
	}
	return nil, Candidate{}, 0, lastErr
}

type probeResult struct {
	idx       int
	latencyMs int64
	err       error
}

// RankByLatency concurrently probes candidates and sorts healthy responders by lowest latency first.
// Remaining unprobed or failed candidates are appended in their original order as fallbacks.
func RankByLatency(ctx context.Context, candidates []Candidate, client *http.Client, userAgent string) []RankedCandidate {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return []RankedCandidate{{Candidate: candidates[0]}}
	}
	if client == nil {
		client = http.DefaultClient
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "FanControl-Downloader"
	}

	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	ch := make(chan probeResult, len(candidates))
	for i, c := range candidates {
		go func(idx int, cand Candidate) {
			ms, err := probeSingleCandidate(probeCtx, client, cand.URL, userAgent)
			ch <- probeResult{idx: idx, latencyMs: ms, err: err}
		}(i, c)
	}

	healthy := make([]RankedCandidate, 0, len(candidates))
	seenHealthy := make(map[int]bool, len(candidates))
	var graceTimer *time.Timer
	var graceC <-chan time.Time

	completed := 0
	doneCollecting := false
	for completed < len(candidates) && !doneCollecting {
		select {
		case res := <-ch:
			completed++
			if res.err == nil {
				seenHealthy[res.idx] = true
				healthy = append(healthy, RankedCandidate{
					Candidate: candidates[res.idx],
					LatencyMs: res.latencyMs,
					Probed:    true,
				})
				if graceTimer == nil {
					graceTimer = time.NewTimer(probeGraceWindow)
					graceC = graceTimer.C
				}
			}
		case <-graceC:
			doneCollecting = true
		case <-probeCtx.Done():
			doneCollecting = true
		}
	}
	if graceTimer != nil {
		graceTimer.Stop()
	}
	cancel()

	sort.SliceStable(healthy, func(i, j int) bool {
		return healthy[i].LatencyMs < healthy[j].LatencyMs
	})

	ranked := make([]RankedCandidate, 0, len(candidates))
	ranked = append(ranked, healthy...)
	for i, c := range candidates {
		if !seenHealthy[i] {
			ranked = append(ranked, RankedCandidate{
				Candidate: c,
				LatencyMs: -1,
				Probed:    false,
			})
		}
	}
	return ranked
}

func probeSingleCandidate(ctx context.Context, client *http.Client, targetURL, userAgent string) (int64, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return maxProbeMs(start), nil
		}
		if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusNotImplemented {
			return 0, fmt.Errorf("HEAD HTTP %d", resp.StatusCode)
		}
	}

	// Fallback probe using 1-byte range GET when mirrors disallow HEAD.
	reqGet, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return 0, err
	}
	reqGet.Header.Set("User-Agent", userAgent)
	reqGet.Header.Set("Range", "bytes=0-0")

	start = time.Now()
	respGet, err := client.Do(reqGet)
	if err != nil {
		return 0, err
	}
	_, _ = io.CopyN(io.Discard, respGet.Body, 64)
	_ = respGet.Body.Close()
	if respGet.StatusCode < 200 || respGet.StatusCode >= 300 {
		return 0, fmt.Errorf("GET range HTTP %d", respGet.StatusCode)
	}
	return maxProbeMs(start), nil
}

func maxProbeMs(start time.Time) int64 {
	ms := time.Since(start).Milliseconds()
	if ms < 1 {
		return 1
	}
	return ms
}
