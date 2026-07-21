# Noise Benefit Learning And Diagnostic Guidance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Feed trustworthy device-scoped noise measurements into safe downward learning and clarify both diagnostic workflows.

**Architecture:** The monitoring loop selects the active device's saved result and asks `smartcontrol` for a bounded noise gain. Existing steady-learning and target-output guards remain authoritative. The frontend reuses the two current dialogs and computes time guidance from the already selected sweep points.

**Tech Stack:** Go, React 19, TypeScript, Tailwind CSS, i18next, Node test runner.

---

### Task 1: Lock Down Noise-Aware Learning

**Files:**
- Modify: `internal/smartcontrol/target_test.go`
- Modify: `internal/smartcontrol/learning.go`

- [ ] **Step 1: Write failing tests**

Add tests that construct one medium-confidence RPM result with four valid points and assert:

```go
gain := NoiseLearningDownGain(3000, types.FanSpeedUnitRPM, cfg, result)
if gain <= 1 { t.Fatalf("gain = %v, want > 1", gain) }
```

Also assert low-confidence and unit-mismatched results return `1`, and a positive safety learning step is identical with gain `1` and gain `1.8`.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/smartcontrol -run "TestNoiseLearning|TestNoiseGain" -count=1`

Expected: FAIL because `NoiseLearningDownGain` and the gain-aware learning entry point do not exist.

- [ ] **Step 3: Implement the minimum learning hook**

In `internal/smartcontrol/learning.go`:

```go
func NoiseLearningDownGain(steadySpeed int, unit string, cfg types.SmartControlConfig, result types.NoiseDiagnosticResult) float64
```

Normalize the result, validate confidence/unit/point count/rise, convert percent ticks to integer percent, calculate a centered local slope, compare it with the overall slope, blend by `NoiseWeight`, and clamp the final gain. Add a gain-aware learning function while preserving the existing public function as a neutral-gain wrapper. Multiply only a negative step.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/smartcontrol -run "TestNoiseLearning|TestNoiseGain" -count=1`

Expected: PASS.

### Task 2: Select The Active Device Result

**Files:**
- Modify: `internal/coreapp/monitoring.go`
- Modify: `internal/coreapp/noise_diagnostic_test.go`

- [ ] **Step 1: Write the failing device-scope test**

Add a test that gives two saved diagnostics and asserts only the active key and matching speed unit produce a non-neutral gain.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/coreapp -run "TestNoiseLearningUsesOnlyActiveDevice" -count=1`

Expected: FAIL because monitoring does not consume saved diagnostics.

- [ ] **Step 3: Wire the existing monitoring path**

At the existing long-term learning call, read `cfg.NoiseDiagnosticsByDevice[a.activeDeviceCurveScopeKey(cfg)]`, calculate the gain with `steady.MeanRPM`, and pass it to the gain-aware learning entry point. Do not alter the surrounding prediction, hardware-limit, axis-noise, or diagnostic-lease guards.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/coreapp -run "TestNoiseLearningUsesOnlyActiveDevice" -count=1`

Expected: PASS.

### Task 3: Add Time And Operation Guidance

**Files:**
- Modify: `frontend/src/app/components/NoiseDiagnostic.tsx`
- Modify: `frontend/src/app/components/AxisNoiseScan.tsx`
- Modify: `frontend/src/app/locales/zh-CN/translation.json`
- Modify: `frontend/src/app/locales/en-US/translation.json`
- Modify: `frontend/src/app/locales/ja-JP/translation.json`
- Modify: `frontend/tests/noise-diagnostic-ui.test.mjs`

- [ ] **Step 1: Write failing UI assertions**

Assert both dialogs derive `plannedSteps`, display translated estimated-time and operation-reminder keys, expose running status with `aria-live="polite"`, and retain `min-[560px]:grid-cols-3` for axis rating buttons.

- [ ] **Step 2: Verify RED**

Run: `npm test -- --test-name-pattern="diagnostic guidance"`

Expected: FAIL because the new reminder keys and markup are absent.

- [ ] **Step 3: Implement the dialog refinements**

Use `buildDiagnosticSteps(range).length` and a small component-local calculation for approximate minutes. Add compact themed information rows with Lucide `Clock3`, `Info`, and `ShieldCheck` icons; keep the existing amber safety warning. Add live running copy and explain fine rescans in the axis dialog. Do not add a dependency or a third workflow.

- [ ] **Step 4: Verify GREEN**

Run from `frontend`: `npm test -- --test-name-pattern="diagnostic guidance"`

Expected: PASS.

### Task 4: Full Verification

- [ ] Run `gofmt` on changed Go files.
- [ ] Run `go test ./internal/smartcontrol ./internal/coreapp -count=1`.
- [ ] Run `go test ./...`.
- [ ] Run `npm test`, `npx tsc --noEmit`, and `npm run build` from `frontend`.
- [ ] Run `git diff --check`.
- [ ] Run `codegraph sync` and `codegraph status`.

