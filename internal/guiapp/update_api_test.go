package guiapp

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestValidateUpdateURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		ok   bool
	}{
		{name: "release asset", url: "https://github.com/Eureka-o/FanControlPortable/releases/download/v2.4.4/FanControl-2.4.4-amd64-installer.exe", ok: true},
		{name: "cdn rejected as initial URL", url: "https://objects.githubusercontent.com/github-production-release-asset/example", ok: false},
		{name: "plain github page", url: "https://github.com/Eureka-o/FanControlPortable", ok: false},
		{name: "wrong repository", url: "https://github.com/example/FanControl/releases/download/v2.4.4/FanControl-amd64-installer.exe", ok: false},
		{name: "insecure", url: "http://github.com/Eureka-o/FanControlPortable/releases/download/v2.4.4/update.exe", ok: false},
		{name: "untrusted host", url: "https://githubusercontent.com.evil.test/update.exe", ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := validateReleaseAssetURL(test.url)
			if (err == nil) != test.ok {
				t.Fatalf("validateUpdateURL(%q) error = %v, want success=%v", test.url, err, test.ok)
			}
		})
	}
}

func TestValidateUpdateRedirectURL(t *testing.T) {
	if _, err := validateUpdateURL("https://objects.githubusercontent.com/github-production-release-asset/example"); err != nil {
		t.Fatalf("trusted GitHub redirect rejected: %v", err)
	}
}

func TestParseWindowsProxyServer(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		scheme string
		want   string
	}{
		{name: "shared proxy", value: "127.0.0.1:1423", scheme: "https", want: "http://127.0.0.1:1423"},
		{name: "protocol proxy", value: "http=127.0.0.1:8080;https=127.0.0.1:1423", scheme: "https", want: "http://127.0.0.1:1423"},
		{name: "socks fallback", value: "socks=127.0.0.1:1080", scheme: "https", want: "socks5://127.0.0.1:1080"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proxyURL, err := parseWindowsProxyServer(test.value, test.scheme)
			if err != nil {
				t.Fatalf("parseWindowsProxyServer() error = %v", err)
			}
			if got := proxyURL.String(); got != test.want {
				t.Fatalf("parseWindowsProxyServer() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDownloadWithRetryResumesPartialFile(t *testing.T) {
	payload := append([]byte{'M', 'Z'}, bytes.Repeat([]byte("fancontrol-update"), 8192)...)
	var requests atomic.Int32
	var resumed atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestNumber := requests.Add(1)
		if requestNumber == 1 {
			response.Header().Set("Content-Length", fmt.Sprint(len(payload)))
			response.WriteHeader(http.StatusOK)
			_, _ = response.Write(payload[:len(payload)/2])
			return
		}

		rangeHeader := request.Header.Get("Range")
		if !strings.HasPrefix(rangeHeader, "bytes=") {
			t.Errorf("retry request Range = %q", rangeHeader)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		var start int
		if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &start); err != nil {
			t.Errorf("parse Range %q: %v", rangeHeader, err)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		resumed.Store(true)
		response.Header().Set("Content-Length", fmt.Sprint(len(payload)-start))
		response.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
		response.WriteHeader(http.StatusPartialContent)
		_, _ = response.Write(payload[start:])
	}))
	defer server.Close()

	partPath := filepath.Join(t.TempDir(), "update.part")
	err := downloadWithRetry(
		context.Background(),
		func(int) *http.Client { return server.Client() },
		server.URL,
		partPath,
		3,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("downloadWithRetry() error = %v", err)
	}
	if requests.Load() != 2 || !resumed.Load() {
		t.Fatalf("requests = %d, resumed = %v", requests.Load(), resumed.Load())
	}
	downloaded, err := os.ReadFile(partPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(downloaded, payload) {
		t.Fatalf("downloaded payload length = %d, want %d", len(downloaded), len(payload))
	}
}

func TestCheckLatestReleaseSelectsPrerelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`[
			{"tag_name":"v2.5.0","draft":false,"prerelease":false},
			{"tag_name":"v2.6.0-beta.1","html_url":"https://github.com/Eureka-o/FanControlPortable/releases/tag/v2.6.0-beta.1","body":"beta","draft":false,"prerelease":true,"assets":[{"name":"FanControl-2.6.0-beta.1-amd64-installer.exe","browser_download_url":"https://github.com/Eureka-o/FanControlPortable/releases/download/v2.6.0-beta.1/FanControl-2.6.0-beta.1-amd64-installer.exe"}]}
		]`))
	}))
	defer server.Close()

	release, err := checkLatestRelease(context.Background(), server.Client(), "prerelease", server.URL, server.URL)
	if err != nil {
		t.Fatalf("checkLatestRelease() error = %v", err)
	}
	if release.TagName != "v2.6.0-beta.1" || release.InstallerURL == "" || !release.Prerelease {
		t.Fatalf("unexpected release: %+v", release)
	}
}

func TestHasUpdateCompletedArg(t *testing.T) {
	if !hasUpdateCompletedArg([]string{"FanControl.exe", "--update-complete"}) {
		t.Fatal("update completion launch argument was not detected")
	}
	if hasUpdateCompletedArg([]string{"FanControl.exe", "--autostart"}) {
		t.Fatal("ordinary launch was treated as an update completion")
	}
}

func TestShouldBypassUpdateProxyOnlyOnFinalRetry(t *testing.T) {
	if shouldBypassUpdateProxy(1, 3) || shouldBypassUpdateProxy(2, 3) {
		t.Fatal("proxy was bypassed before the final retry")
	}
	if !shouldBypassUpdateProxy(3, 3) {
		t.Fatal("final retry did not fall back to a direct connection")
	}
}
