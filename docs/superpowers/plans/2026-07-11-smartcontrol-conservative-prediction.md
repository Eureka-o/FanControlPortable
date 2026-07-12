# SmartControl Conservative Prediction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve high-load response under ramp smoothing without adding polling, workers, persisted learning state, or user-facing settings.

**Architecture:** Keep one existing monitor tick and one existing in-memory history. Mark bridge-cache data as non-fresh, make prediction samples time/domain aware, and use a bounded temporary ramp-up multiplier only after power-rise and sustained-temperature-rise evidence. Keep learned offsets slow: reliable high-temperature samples adjust immediately; quiet down-learning requires confirmation and never mixes known-power with unknown-power efficiency history.

**Tech Stack:** Go, existing `internal/smartcontrol`, `internal/coreapp`, Go standard-library tests.

---

### Task 1: Telemetry freshness and learning history integrity

**Files:**
- Modify: `internal/types/types.go`
- Modify: `internal/temperature/temperature.go`
- Modify: `internal/temperature/temperature_test.go`
- Modify: `internal/smartcontrol/learning.go`
- Modify: `internal/smartcontrol/target_test.go`

- [ ] **Step 1: Write failing tests**

Add a test proving a transient bridge-cache result is marked non-fresh, and a test proving a known-power efficiency estimate does not reuse unknown-power equilibrium history.

- [ ] **Step 2: Run focused tests to verify failure**

Run: `go test ./internal/temperature ./internal/smartcontrol -run 'Fresh|KnownPower' -count=1`

Expected: FAIL because freshness and strict known-power matching do not yet exist.

- [ ] **Step 3: Write minimal implementation**

Add a non-serialized `TelemetryFresh` field to `TemperatureData`; set it only for a successful direct bridge read. In `localEfficiencyForPower`, when the reference has known power, include only history points that also have known comparable power.

- [ ] **Step 4: Run focused tests to verify pass**

Run: `go test ./internal/temperature ./internal/smartcontrol -run 'Fresh|KnownPower' -count=1`

Expected: PASS.

### Task 2: Time-aware, domain-aware prediction core

**Files:**
- Modify: `internal/smartcontrol/rise_prediction.go`
- Modify: `internal/smartcontrol/rise_prediction_test.go`

- [ ] **Step 1: Write failing tests**

Add tests proving a recent domain power rise earns only a bounded ramp-up factor before temperature confirmation, sustained temperature rise earns the higher bounded factor, and stale sample timing or fan lag suppresses prediction.

- [ ] **Step 2: Run focused test to verify failure**

Run: `go test ./internal/smartcontrol -run RisePrediction -count=1`

Expected: FAIL because the current result has no ramp multiplier or time/domain/fan-lag checks.

- [ ] **Step 3: Write minimal implementation**

Extend the in-memory sample with timestamp, control source, per-domain availability, target speed, and actual speed. Add one pure helper returning prediction boost and a bounded ramp-up factor: power rise alone gives only a small factor; continuous time-valid temperature rise gives the larger factor; a fan that has not caught up returns no factor/boost.

- [ ] **Step 4: Run focused test to verify pass**

Run: `go test ./internal/smartcontrol -run RisePrediction -count=1`

Expected: PASS.

### Task 3: Slow safe offset steps and monitor integration

**Files:**
- Modify: `internal/smartcontrol/learning.go`
- Modify: `internal/smartcontrol/target_test.go`
- Modify: `internal/coreapp/monitoring.go`
- Modify: `internal/coreapp/monitoring_test.go`

- [ ] **Step 1: Write failing tests**

Add tests proving known-efficiency learning does not receive the old fixed power gain, quiet down-learning is half-size and requires a second matching steady result, and a telemetry context change clears in-flight prediction/learning state.

- [ ] **Step 2: Run focused tests to verify failure**

Run: `go test ./internal/smartcontrol ./internal/coreapp -run 'Quiet|PowerGain|PredictionContext' -count=1`

Expected: FAIL because fixed power gain and one-shot quiet learning remain.

- [ ] **Step 3: Write minimal implementation**

Remove the fixed 35W/120W multiplier from known-efficiency steps. Add a tiny in-memory confirmation state for negative offset steps. In the monitor, build context/sample data from the existing tick result, reset state on resume/context/freshness changes, and apply only the prediction's bounded effective `RampUpLimit` before existing curve/device limits.

- [ ] **Step 4: Run focused tests to verify pass**

Run: `go test ./internal/smartcontrol ./internal/coreapp -run 'Quiet|PowerGain|PredictionContext' -count=1`

Expected: PASS.

### Task 4: Regression verification

**Files:**
- Verify: `internal/smartcontrol/*_test.go`
- Verify: `internal/temperature/*_test.go`
- Verify: `internal/coreapp/*_test.go`

- [ ] **Step 1: Format changed Go files**

Run: `gofmt -w internal/types/types.go internal/temperature/temperature.go internal/temperature/temperature_test.go internal/smartcontrol/learning.go internal/smartcontrol/target_test.go internal/smartcontrol/rise_prediction.go internal/smartcontrol/rise_prediction_test.go internal/coreapp/monitoring.go internal/coreapp/monitoring_test.go`

- [ ] **Step 2: Run package regression tests**

Run: `go test ./internal/smartcontrol ./internal/temperature ./internal/coreapp -count=1`

Expected: PASS.

- [ ] **Step 3: Run broader Go verification**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 4: Inspect scope**

Run: `git diff --check` and `git diff --stat`

Expected: no whitespace errors and only smart-control implementation plus tests/plan/memory changes.
