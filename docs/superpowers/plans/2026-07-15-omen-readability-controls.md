# OMEN Readability And Direct Controls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Simplify the OMEN overview, remove duplicated device controls, and render performance limits with direct sliders plus numeric inputs.

**Architecture:** Keep the existing plugin IIFE, host UI API, backend methods, and readback flow. Recompose only `overviewPage`, `performancePage`, and `devicePage`, reusing `ui.Slider`, `ui.NumberInput`, `ui.Select`, `ui.ToggleSwitch`, and existing card helpers.

**Tech Stack:** React 19 host runtime, JavaScript IIFE plugin, scoped CSS, Node test runner.

---

### Task 1: Lock The New Information Architecture

**Files:**
- Modify: `frontend/tests/omen-plugin-frontend.test.mjs`
- Test: `frontend/tests/omen-plugin-frontend.test.mjs`

- [ ] **Step 1: Write the failing contract assertions**

Add assertions that the overview block contains no `setActivePage`, CPU-tuning summary, or custom-curve summary; that it directly calls GPU-mode, charge, overdrive, and GPU-power commands; that the performance block uses `ui.Slider`; and that the device block no longer owns GPU mode, charge, or overdrive commands. Assert the Chinese label is `masterMode: '大师'`.

- [ ] **Step 2: Run the focused test and confirm RED**

Run: `cd frontend; node --test tests/omen-plugin-frontend.test.mjs`

Expected: FAIL because the overview still contains navigation cards, performance uses only number inputs, and device controls are duplicated.

### Task 2: Recompose The Three Plugin Pages

**Files:**
- Modify: `plugins/omen-fan/src/ui/index.js`
- Modify: `plugins/omen-fan/src/ui/index.css`
- Test: `frontend/tests/omen-plugin-frontend.test.mjs`

- [ ] **Step 1: Add one shared numeric tuning renderer**

Inside `OmenPage`, add a local `tuningControl(field, label, min, max, suffix)` helper that renders the existing `ui.Slider` and `ui.NumberInput` against `powerDraft`, both calling `setPowerField`.

- [ ] **Step 2: Simplify the overview**

Keep the enlarged performance-mode card. Render only four direct quick controls in two columns: Dynamic Boost toggle, screen-overdrive toggle, 80/100 charge buttons, and GPU-mode `ui.Select` with the existing restart confirmation. Remove CPU tuning and custom curve navigation cards.

- [ ] **Step 3: Reflow performance tuning**

Replace numeric-only fields with `tuningControl`. Keep current ranges, draft state, Apply buttons, boost policy, power bias, and Dynamic Boost. Mark the GPU tuning card to span the full desktop grid width.

- [ ] **Step 4: Remove duplicated device controls**

Delete GPU-mode, screen-overdrive, and charge-protection cards from `devicePage`. Keep OMEN key and diagnostics only.

- [ ] **Step 5: Tighten scoped CSS**

Make the mode card taller, enlarge its segmented buttons, change quick controls to two columns, add slider/input row styles, span the GPU tuning card, improve muted-text contrast, and retain one-column narrow layouts. Remove CSS used only by deleted navigation summaries.

- [ ] **Step 6: Run focused GREEN checks**

Run: `cd frontend; node --test tests/omen-plugin-frontend.test.mjs; npx tsc --noEmit`

Expected: all OMEN tests pass and TypeScript exits 0.

### Task 3: Verify And Repackage

**Files:**
- Verify: `frontend/`
- Verify: `plugins/omen-fan/`
- Output: `build/bin/plugins/omen-fan/`
- Output: `build/bin/omen-fan-setup.exe`
- Output: `build/bin/omen-fan-0.1.0-windows-amd64.zip`

- [ ] **Step 1: Run full frontend verification**

Run: `cd frontend; npm test; npx tsc --noEmit; npm run build`

Expected: zero failed tests, zero type errors, and a successful production build.

- [ ] **Step 2: Check desktop and narrow rendering**

Verify overview and performance at desktop and 390px widths, in light and dark themes. Confirm no duplicated controls, no horizontal scroll, readable labels, stable control sizing, and no console errors.

- [ ] **Step 3: Rebuild the plugin preview**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File plugins/omen-fan/build.ps1`

Expected: the staged plugin directory, ZIP, and NSIS installer are regenerated without warnings.

- [ ] **Step 4: Refresh the running Core plugin and relaunch GUI**

Use the existing Core IPC `RefreshPlugins` request, verify `omen-fan` returns `ready`, and relaunch `build/bin/FanControl.exe` for review. Preserve the dirty tree; do not commit or publish product changes.
