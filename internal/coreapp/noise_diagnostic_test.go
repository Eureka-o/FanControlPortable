package coreapp

import (
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestNoiseDiagnosticLeaseActiveAndCancellationAreIdempotent(t *testing.T) {
	app := &CoreApp{}
	app.noiseDiagnosticLease = &noiseDiagnosticLease{
		sessionID: "session-1",
		expiresAt: time.Now().Add(time.Minute),
		done:      make(chan struct{}),
	}
	if !app.noiseDiagnosticLeaseActive() {
		t.Fatal("active noise diagnostic lease reported inactive")
	}
	if err := app.CancelNoiseDiagnostic("session-1"); err != nil {
		t.Fatalf("CancelNoiseDiagnostic() error = %v", err)
	}
	if app.noiseDiagnosticLeaseActive() {
		t.Fatal("noise diagnostic lease remains active after cancellation")
	}
	if err := app.CancelNoiseDiagnostic("session-1"); err != nil {
		t.Fatalf("second CancelNoiseDiagnostic() error = %v", err)
	}
}

func TestNoiseDiagnosticLeaseExpires(t *testing.T) {
	app := &CoreApp{}
	app.noiseDiagnosticLease = &noiseDiagnosticLease{
		sessionID: "expired",
		expiresAt: time.Now().Add(-time.Second),
		done:      make(chan struct{}),
	}
	if app.noiseDiagnosticLeaseActive() {
		t.Fatal("expired noise diagnostic lease reported active")
	}
	if _, err := app.noiseDiagnosticLeaseFor("expired"); err == nil {
		t.Fatal("expired lease lookup unexpectedly succeeded")
	}
}

func TestNoiseDiagnosticResultNormalizationBeforePersistence(t *testing.T) {
	result, changed := types.NormalizeNoiseDiagnosticResult(types.NoiseDiagnosticResult{
		Unit: "rpm",
		Points: []types.NoiseDiagnosticPoint{
			{Requested: 2000, Actual: 2000, LevelDB: 3, SpreadDB: 1, Valid: true},
			{Requested: 1000, Actual: 1000, LevelDB: 1, SpreadDB: 1, Valid: true},
			{Requested: 0, Actual: 0, Valid: false},
		},
	})
	if !changed || len(result.Points) != 2 {
		t.Fatalf("normalized result = %#v, changed=%v", result, changed)
	}
	if result.Points[0].Actual != 1000 || result.Points[1].Actual != 2000 {
		t.Fatalf("result points not sorted: %#v", result.Points)
	}
}
