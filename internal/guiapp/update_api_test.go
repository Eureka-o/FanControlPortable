package guiapp

import "testing"

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
