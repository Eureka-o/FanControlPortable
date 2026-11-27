package ghmirror

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsTrustedMirrorHost(t *testing.T) {
	for _, host := range []string{"gh-proxy.com", "HK.GH-PROXY.ORG", "gh-proxy.org", "ghfast.top", "cdn.jsdelivr.net"} {
		if !IsTrustedMirrorHost(host) {
			t.Fatalf("expected %q to be trusted", host)
		}
	}
	for _, host := range []string{"evil.example", "gh-proxy.com.evil.test", ""} {
		if IsTrustedMirrorHost(host) {
			t.Fatalf("expected %q to be rejected", host)
		}
	}
}

func TestBuildCandidates(t *testing.T) {
	raw := "https://raw.githubusercontent.com/Eureka-o/FanControl-Plugins/main/catalog.json"
	cands := BuildCandidates(raw)
	if len(cands) != 6 {
		t.Fatalf("expected 6 candidates for raw github URL, got %d", len(cands))
	}
	if cands[len(cands)-1].URL != "https://cdn.jsdelivr.net/gh/Eureka-o/FanControl-Plugins@main/catalog.json" {
		t.Fatalf("unexpected jsdelivr URL: %q", cands[len(cands)-1].URL)
	}

	rel := "https://github.com/Eureka-o/FanControl-Plugins/releases/download/omen-fan-v1.0.0/omen-fan.zip"
	relCands := BuildCandidates(rel)
	if len(relCands) != 5 {
		t.Fatalf("expected 5 candidates for github release URL, got %d", len(relCands))
	}

	local := "https://custom.local.test/pkg.zip"
	localCands := BuildCandidates(local)
	if len(localCands) != 1 || localCands[0].URL != local {
		t.Fatalf("expected non-GitHub URL untouched, got %+v", localCands)
	}
}

func TestRaceGetSelectsFastestValidCandidate(t *testing.T) {
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(350 * time.Millisecond):
			_, _ = w.Write([]byte("valid-slow"))
		}
	}))
	defer slowSrv.Close()

	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>corrupt-fast</html>"))
	}))
	defer badSrv.Close()

	fastSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte("valid-fast"))
	}))
	defer fastSrv.Close()

	cands := []Candidate{
		{Name: "origin", Label: "GitHub 源站", URL: slowSrv.URL},
		{Name: "bad-mirror", Label: "故障镜像", URL: badSrv.URL},
		{Name: "fast-mirror", Label: "高速镜像", URL: fastSrv.URL},
	}

	data, winner, latencyMs, err := RaceGet(context.Background(), cands, &http.Client{Timeout: 2 * time.Second}, nil, 1024, func(b []byte) error {
		if !strings.HasPrefix(string(b), "valid-") {
			return fmt.Errorf("unexpected payload")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RaceGet failed: %v", err)
	}
	if string(data) != "valid-fast" || winner.Name != "fast-mirror" || latencyMs <= 0 {
		t.Fatalf("unexpected winner=%+v latency=%d data=%q", winner, latencyMs, string(data))
	}
}

func TestRankByLatencyOrdersFastestFirstAndFallsBack(t *testing.T) {
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(110 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowSrv.Close()

	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer errSrv.Close()

	fastSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Force HEAD 405 -> fallback 1-byte range GET probe.
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer fastSrv.Close()

	cands := []Candidate{
		{Name: "origin", Label: "GitHub 源站", URL: slowSrv.URL},
		{Name: "broken", Label: "失效镜像", URL: errSrv.URL},
		{Name: "fast", Label: "低延迟镜像", URL: fastSrv.URL},
	}
	ranked := RankByLatency(context.Background(), cands, &http.Client{Timeout: 2 * time.Second}, "TestAgent")
	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked entries, got %d", len(ranked))
	}
	if ranked[0].Name != "fast" || !ranked[0].Probed {
		t.Fatalf("expected fast mirror first, got %+v", ranked[0])
	}
	if ranked[2].Name != "broken" || ranked[2].Probed {
		t.Fatalf("expected broken mirror at tail unprobed, got %+v", ranked[2])
	}
}
