package types

import "testing"

func TestNormalizeWindowBlurMaterial(t *testing.T) {
	tests := map[string]string{
		"acrylic": "acrylic",
		"mica":    "mica",
		"tabbed":  "tabbed",
		"off":     "off",
		"on":      "acrylic",
		"auto":    "acrylic",
		"":        "acrylic",
		"unknown": "acrylic",
	}
	for input, want := range tests {
		if got := NormalizeWindowBlur(input); got != want {
			t.Errorf("NormalizeWindowBlur(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDefaultConfigUsesAcrylicAndTemperatureRisePrediction(t *testing.T) {
	cfg := GetDefaultConfig(false)
	if cfg.WindowBlur != "acrylic" {
		t.Fatalf("WindowBlur = %q, want acrylic", cfg.WindowBlur)
	}
	if !cfg.SmartControl.TemperatureRisePrediction {
		t.Fatal("temperature rise prediction should default to enabled")
	}

	rpm := GetDefaultSmartControlConfigForUnit(GetDefaultRPMFanCurve(), FanSpeedUnitRPM)
	if !rpm.TemperatureRisePrediction {
		t.Fatal("RPM temperature rise prediction should default to enabled")
	}
}
