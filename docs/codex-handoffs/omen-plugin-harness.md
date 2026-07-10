# OMEN Plugin Harness

## Current Status

- Current phase: P9 - OMEN backend capability contract before frontend rewrite
- Phase state: P1-P8 released by main-agent verification; debug/mock UI visibility slice released; OMEN page crash/installer write-path fixes released; OMEN curve UI/unit/sidebar icon fix released; OMEN submenu/error/card/mock-port polish released; OMEN frontend mock HTTP integration plus CORS released; P9 backend capability/status contract released; main app preview artifacts remain `2.5.0-preview`; OMEN plugin installer version remains `0.1.0`
- Main-agent recovery source: `../AGENTS.md`, `../MEMORY_INDEX.md`, `../SKILL_ROUTER_MEMORY.md`, `../OMEN_PLUGIN_PLAN.md`
- Current repo state before P1 dispatch: earlier handoff recorded pre-existing `?? Cache/`; implementation start observed `?? Cache/` and untracked `docs/codex-handoffs/`
- Subproject memory files: `FanControlPortable/MEMORY_INDEX.md` and `FanControlPortable/SKILL_ROUTER_MEMORY.md` do not currently exist
- Current recovery note: after any context compaction, reread only the necessary agreements/current progress first, then verify repo status and CodeGraph before dispatching the next lightweight subagent slice.
- Current workflow note: use `@superpowers`/PAL first for fast analysis/review/test design when available; if PAL/external model access fails, record it and continue with local CodeGraph, diff review, and validation commands.

## 2026-07-05 OMEN Backend Capability Contract Before Frontend Rewrite

Request:
- User stopped the current frontend polish direction and asked to first connect backend logic from third-party OMEN control tools where safe, excluding lighting, then rewrite a product-grade frontend around stable backend capabilities.
- Main-agent decision: freeze the current OMEN page as a temporary debug shell. P9 focuses on backend capability/status shape only, not final UI and not real hardware writes.

Reference findings used:
- OmenMon, OmenSuperHub, and OmenHubLighter agree that the safe first backend layer is read-only fan capability/status: `0x28`, `0x10`, `0x2C`, `0x2D`, and `0x2F`.
- `0x2E` fan level write, `0x1A` mode write, `0x29` CPU power, and `0x22` GPU power are feasible later but remain gated behind explicit hardware-write self-tests, readback, and restore.
- Lighting, HP OMEN SDK DLL bundling, PerformanceControl.dll, NVAPI, driver patchers, OpenHardwareMonitor/Ring0, and preset bundles remain out of scope.

Implementation subagents:
- Herschel (`019f3247-5aa6-7031-a46f-1faabbb9fb51`) added `plugins/omen-fan/src/OmenCapabilityModel.cs` and the `--self-test-parsers` entry in `Program.cs`.
  - Pure parsers cover SystemDesignData, fan state, fan type nibbles, current levels, and OmenMon-style fan table triples.
  - Main-agent review found a fan-table trailing-zero risk; Herschel fixed parsing to honor positive `levelCount` and added self-test coverage.
- Locke (`019f3255-e85a-71d2-a90c-de9e1e553062`) updated only `plugins/omen-fan/src/WmiReadOnlyProbe.cs`.
  - Supported detect output now uses the pure capability parsers.
  - Backward-compatible fields are preserved: `type`, `supported`, `mock`, `mode`, `readOnly`, `swFanControl`, levels, estimated RPM, max RPM/level, `fanState`, `fanTypes`, and `lastUpdated`.
  - Added richer read-only fields: `thermalPolicyVersion`, `adapterWatts`, `fanCount`, and flat `fanTable`.
- Helmholtz (`019f325f-d566-7dc3-9794-8ee905bb66f6`) updated only Go plugin status/test files.
  - `internal/plugins/plugin.go` now carries comparable OMEN capability fields through `OmenFanStatus` and `OmenFanTableStatus`.
  - `fanTypes` remains intentionally ignored on the Go side for now to keep `OmenFanStatus` comparable and avoid broad test churn.
- Descartes (`019f3264-27a7-7510-9e2f-e146274cb01d`) updated only `plugins/omen-fan/src/Program.cs`.
  - Mock daemon/HTTP status now mirrors the stable capability shape, including flat `fanTable` fields.
  - Endpoints and request semantics are unchanged; `--detect-only --mock` output is unchanged.

Main-agent verification:
- `dotnet build plugins/omen-fan/src/OmenFanDriver.csproj -c Release` passed.
- `plugins/omen-fan/src/bin/Release/net472/omen-fan-driver.exe --self-test-parsers` passed with `{"type":"self-test","ok":true,"tests":38}`.
- `plugins/omen-fan/src/bin/Release/net472/omen-fan-driver.exe --detect-only --mock` passed.
- `plugins/omen-fan/src/bin/Release/net472/omen-fan-driver.exe --detect-only` passed on this non-OMEN machine with `supported:false`, `category:"wmi-missing"`, exit 0.
- Mock HTTP `GET /status` on port 8787 returned `type=status`, `readOnly=false`, `swFanControl=true`, `fanCount=2`, flat `fanTable.fan1MinLevel/fan1MaxLevel`, and no nested `fan1`/`fan2` objects.
- Go validation passed: `gofmt -l`, `go test ./internal/plugins/omenfan/... -count=1`, `go test ./internal/plugins/... -count=1`, `go build ./internal/plugins/...`, and `go vet ./internal/plugins/...`.
- Scoped forbidden-token scans found no new real hardware write path, no `0x2E`, `0x1A`, `0x29`, `0x22`, `SetFanLevel`, `SetFanMode`, lighting, HP SDK, NVAPI, driver patcher, OpenHardwareMonitor/Ring0, installer, frontend, theme, or packaging changes.
- `git diff --check` for P9 files passed with only the existing LF-to-CRLF warning on `internal/plugins/plugin.go`.
- `codegraph sync; codegraph status` passed; index is up to date.
- C# validation `bin`/`obj` folders were cleaned; no `omen-fan-driver` process remained after mock HTTP verification.

Boundary:
- P9 does not implement real non-mock daemon launch, real `0x2E` fan writes, `0x1A` mode writes, `0x29`/`0x22` power writes, restore-on-stop hardware writes, frontend rewrite, generated Wails bindings, installer/package changes, or public release changes.
- The main installer and distributable portable package still must not intentionally include or manage `plugins/omen-fan`; any copy under `build/bin/plugins/omen-fan` remains local debug convenience only.

Next step:
- P10 should be a backend write-gate planning step before any product frontend rewrite. Recommended first P10 slice: a C# dry-run command encoder for mode/fan writes and a hardware-write self-test design that writes the current fan level back to itself only behind an explicit `--hardware-write` flag, then readback/restores.
- Frontend rewrite should wait until the backend contract includes real capability labels and clearly exposes which controls are read-only, mock-only, dry-run-only, or hardware-enabled.

## 2026-07-05 OMEN Frontend Mock HTTP Integration And CORS

Request:
- User reported the OMEN status cards still showed mismatched small text and asked to add/use a plugin-source debug interface so a local mock port could be connected to inspect the UI.

Implementation subagents:
- Raman (`019f31fe-737e-7cc3-8535-a95220806cf3`) updated only `frontend/src/app/services/api.ts` and `frontend/src/app/types/app.ts`.
  - Added OMEN mock HTTP client methods for `/health`, `/status`, `/set-fan`, `/mode`, and `/power`.
  - The client uses `http://127.0.0.1:8787`, short timeout, and unwraps `{type,status}` envelopes.
  - Extended `OmenFanStatus` with `mock`, `powerMode`, and `powerLimitWatts`.
- Tesla (`019f3202-ce3a-7643-a513-639400f89cb4`) updated only `frontend/src/app/components/OmenPage.tsx`.
  - Added `Mock 8787` connected/offline state.
  - Uses `displayFanData = mockStatus ?? omenFanData` for status cards.
  - Mode, power, and curve apply actions update mock state when connected.
  - Card subtext is explicitly tied to each metric: fan cards show level/estimate, CPU temperature shows mode/source, GPU temperature shows update time.
- Ptolemy (`019f320d-449c-77e0-9707-104ec7fd9d57`) updated only `plugins/omen-fan/src/Program.cs`.
  - Added CORS headers and OPTIONS preflight to `--mock-http` so WebView fetch can call the local mock port.
  - No real WMI/hardware write path was added.

Main-agent verification:
- `cd frontend; npx tsc --noEmit` passed.
- `cd frontend; npm run build` passed with only the known `baseline-browser-mapping` age warning.
- `dotnet build plugins/omen-fan/src/OmenFanDriver.csproj -c Release` passed.
- `build/bin/plugins/omen-fan/omen-fan-driver.exe --detect-only --mock` passed.
- Mock HTTP from copied `build/bin/plugins/omen-fan/omen-fan-driver.exe` passed:
  - OPTIONS `/status` returned 204 with `Access-Control-Allow-Origin: *`.
  - GET `/status` returned a status envelope.
  - POST `/mode` and POST `/set-fan` updated mock status.
- Go validation passed: `go test`, `go build`, and `go vet` for `internal/coreapp`, `internal/plugins`, and `internal/smartcontrol`.
- `cmd /c build.bat` passed and rebuilt main artifacts.
- `C:\Program Files (x86)\NSIS\makensis.exe plugins/omen-fan/installer/omen-fan-setup.nsi` passed and rebuilt the separate plugin installer.
- Portable zip inspection passed: no `omen`, `omen-fan`, or `plugins/` entries.
- `git diff --check` passed with only LF-to-CRLF warnings.
- `codegraph sync; codegraph status` passed.
- `plugins/omen-fan/src/bin` and `plugins/omen-fan/src/obj` were cleaned after C# validation.

Artifacts:
- `build/bin/FanControl.exe` length `24571392`, SHA256 `76349C09EAAFEF187A703AB92ED25CB28D83AAC122A98A3B1F80CB2E0DC972F4`.
- `build/bin/FanControl-2.5.0-preview-amd64-installer.exe` length `28152351`, SHA256 `27172FFEB1A6F7DB0657091EF02D17F67D637A9325B35731E79705B029492DD6`.
- `build/bin/FanControl-2.5.0-preview-portable.zip` length `26027055`, SHA256 `FF6B13A010AC35A6D9E90B2A4BC4C1233CABFED94B958B4BFD2AF49F532115D2`.
- `build/bin/omen-fan-setup.exe` length `357022`, SHA256 `72E2AA712C81394AFE06815877095A1F64F8412816D2816DD87A6A3EC8FDA800`.
- `build/bin/plugins/omen-fan/omen-fan-driver.exe` length `28672`, SHA256 `57AE385A78C4E7BEB312667E50A054E033EA57B9AAC595DF39CA1B05441DE901`.

Local debug usage:
- Launch `build/bin/FanControl.exe` for portable preview.
- To see the page switch to connected mock data, start `build/bin/plugins/omen-fan/omen-fan-driver.exe --mock-http --port 8787`.

Boundary:
- This remains a debug/mock UI route. Mode/power/fan updates through the page only update local mock state.
- Main installer and distributable portable zip still do not bundle or manage `plugins/omen-fan`.
- Real OMEN/Shadow Elf 11 WMI writes, real power control, and restore-on-stop validation remain undone and gated.

## 2026-07-05 OMEN Submenus, Error Polish, And Mock HTTP Debug Port

Request:
- User reported remaining OMEN page issues: unsupported error detail showed mojibake, sidebar OMEN icon did not match the existing line-icon style, metric card details did not match their labels, the curve chart had an extra standalone degree label, the bottom target input list was too heavy, and the page needed basic plugin submenus such as mode switching and power adjustment.
- User also requested a plugin-side local mock port so the UI/debug flow can connect to a local mock interface before real OMEN WMI write support is complete.
- Lighting remains intentionally out of scope for this slice.

Implementation subagents:
- Halley (`019f31d9-f400-7ec1-830d-63b714e3541a`) updated only `frontend/src/app/components/AppShell.tsx`.
  - Replaced the colorful OMEN gradient icon with a `currentColor` line SVG that inherits the same active/inactive colors as the other sidebar icons.
- Lorentz (`019f31da-fbd1-7ec0-a05d-0c8eddbd3288`) updated only `frontend/src/app/components/OmenPage.tsx`.
  - Unsupported error display now shows a stable Chinese summary and a short sanitized diagnostic hint instead of raw `plugin.lastError`.
  - Added `模式切换 / 功耗调整 / 自定义风扇` segmented subpages.
  - Mode and power controls are clearly local/debug preview and do not write hardware.
  - Fan curve, bias, and joint-learning controls only show in custom mode.
  - Removed the bottom per-point RPM `NumberInput` target list.
  - Removed the extra standalone `°C` x-axis label and adjusted curve margins.
  - Metric-card details now match their cards: fan cards show fan level and estimate marker, CPU temperature shows mode/source, GPU temperature shows update time.
- Russell (`019f31dd-6d98-7090-b7b5-1878ab47edb0`) updated only `plugins/omen-fan/src/Program.cs`.
  - Added `omen-fan-driver.exe --mock-http [--port 8787]`, also accepting `--port=8787`.
  - The mock HTTP server binds only to `127.0.0.1` and `localhost`.
  - Added `/health`, `/status`, `/set-fan`, `/mode`, and `/power` JSON endpoints with `application/json; charset=utf-8`.
  - Reuses the existing mock fan-level writer and status envelope, adding `powerMode` and `powerLimitWatts` fields for debug.

Main-agent verification:
- PAL/Superpowers review was attempted with `gpt-5.5` but failed with insufficient balance; local CodeGraph/source review and validation commands were used.
- CodeGraph was checked before edits and synced after the slices.
- Source checks passed: `OmenPage.tsx` has no U+FFFD replacement character, no raw `{plugin.lastError}` render, no `NumberInput`, the three submenus are present, and the standalone degree-axis label marker is absent.
- `rg -n "[ \t]+$" frontend/src/app/components/OmenPage.tsx frontend/src/app/components/AppShell.tsx plugins/omen-fan/src/Program.cs` found no trailing whitespace.
- `cd frontend; npx tsc --noEmit` passed.
- `cd frontend; npm run build` passed with only the existing `baseline-browser-mapping` age warning.
- `dotnet build plugins/omen-fan/src/OmenFanDriver.csproj -c Release` passed.
- Mock HTTP was tested from the source build output and from copied `build/bin/plugins/omen-fan/omen-fan-driver.exe`.
  - `/health` returned `ok=true`.
  - `/set-fan` updated CPU/GPU RPM.
  - `/mode` accepted `custom`.
  - `/power` accepted `performance` and `55W`.
- `C:\Program Files (x86)\NSIS\makensis.exe plugins/omen-fan/installer/omen-fan-setup.nsi` passed and rebuilt the plugin installer.
- `cmd /c build.bat` passed and rebuilt `FanControl.exe`, the main installer, and the portable zip.
- Copied the new plugin payload into `build/bin/plugins/omen-fan/` for local portable preview.
- `build/bin/plugins/omen-fan/omen-fan-driver.exe --detect-only --mock` passed.
- Portable zip inspection passed: `build/bin/FanControl-2.5.0-preview-portable.zip` has no `omen`, `omen-fan`, or `plugins/` entries.
- `git diff --check` passed with only Git LF-to-CRLF warnings.
- `codegraph sync; codegraph status` passed; index is up to date.

Local debug usage:
- Open `build/bin/FanControl.exe` to preview the plugin from `build/bin/plugins/omen-fan/`.
- To inspect the mock HTTP interface manually:
  - `build/bin/plugins/omen-fan/omen-fan-driver.exe --mock-http --port 8787`
  - `GET http://127.0.0.1:8787/status`
  - `POST http://127.0.0.1:8787/set-fan` with `{"cpuRpm":2400,"gpuRpm":2300}`
  - `POST http://127.0.0.1:8787/mode` with `{"mode":"custom"}`
  - `POST http://127.0.0.1:8787/power` with `{"powerMode":"performance","powerLimitWatts":55}`

Artifacts:
- `build/bin/FanControl.exe` length `24568320`, time `2026/7/5 18:59:32`.
- `build/bin/FanControl-2.5.0-preview-amd64-installer.exe` length `28150513`, SHA256 `E6A117183E5F7D0FEC6C2A86E4A523387B60716A5F74357EFA05A68C27CCEA9B`.
- `build/bin/FanControl-2.5.0-preview-portable.zip` length `26025616`, SHA256 `61CB8F305A3E0A627AC57BC5D71B32E7CA3183F9F41F0FC10B00A3753463029C`.
- `build/bin/omen-fan-setup.exe` length `356750`, SHA256 `20ADDA100FAE6B27A82CB609092259C19C579BB6711B1F7BC2BA62617F35B796`.
- `build/bin/plugins/omen-fan/omen-fan-driver.exe` length `28160`, SHA256 `36203D6AE0398A7BC054CE5530ECFA61121483B91A69397A38DDE649710ED93D`.

Boundary and remaining work:
- Mode and power controls are UI/mock preview only; they do not yet write OMEN BIOS/WMI state.
- Lighting is still out of scope.
- Main installer and distributable portable zip still do not bundle or manage the plugin; the copied plugin under `build/bin/plugins/omen-fan` is only a local preview convenience.
- Real OMEN/Shadow Elf 11 WMI writes, real power mode control, and restore-on-stop validation remain undone and gated.

## 2026-07-05 OMEN UI Curve/Unit/Icon Fix And Local Portable Preview

Request:
- User reported that the OMEN page display still had issues: temperature unit rendered incorrectly, the OMEN fan allocation UI should use a curve-editor style instead of per-gear sliders, applied RPM should round to 100-RPM steps while local learning/preview can use finer values, and the sidebar OMEN tab should use an OMEN-like rotated gradient square icon.
- User also clarified that the portable runnable exe is under `build/bin`, and for debugging the plugin can be placed beside that exe so opening `build/bin/FanControl.exe` shows the OMEN UI directly.

Implementation subagents:
- Darwin (`019f31c0-f3ea-7431-ac39-eb52a299ad96`) updated only `frontend/src/app/components/AppShell.tsx`.
  - Replaced the OMEN sidebar laptop icon with an inline rotated gradient square mark.
  - Preserved existing sidebar button dimensions and active/inactive behavior.
- Anscombe (`019f31c1-d033-7321-bdc8-5e7b9c2a3d7b`) updated only `frontend/src/app/components/OmenPage.tsx`.
  - Added a lightweight SVG line-curve editor with draggable points.
  - Changed temperature formatting and curve labels to real `°C`.
  - Changed OMEN RPM apply/input normalization to 100-RPM rounding while pointer drag remains smooth.
  - Changed bias/learning slider step from 5 to 1.

Main-agent verification:
- PAL/Superpowers review was attempted with `gpt-5.5`, but that model endpoint rejected the screenshot because image input was not supported; local review continued with CodeGraph/source inspection and validation commands.
- `cd frontend; npx tsc --noEmit` passed.
- `cd frontend; npm run build` passed with only the existing `baseline-browser-mapping` age warning.
- `cmd /c build.bat` passed and rebuilt `build/bin/FanControl.exe`, the 2.5.0-preview installer, and the 2.5.0-preview portable zip.
- `git diff --check` passed with only Git LF-to-CRLF warnings.
- UTF-8 source check confirmed `frontend/src/app/components/OmenPage.tsx` contains real `°C` and no mojibake marker.
- `build/bin/plugins/omen-fan/omen-fan-driver.exe --detect-only --mock` returned supported mock output after copying the plugin payload into the local runnable bin tree.
- Portable zip inspection passed: `build/bin/FanControl-2.5.0-preview-portable.zip` contains no `omen`, `omen-fan`, or `plugins/` entries.
- `codegraph sync; codegraph status` passed; index is up to date.

Local preview setup:
- Copied plugin payload to `build/bin/plugins/omen-fan/` for local debug preview by launching `build/bin/FanControl.exe`.
- Copied files: `plugin.json`, `omen-fan-driver.exe`, `omen-fan-driver.exe.config`, and `Newtonsoft.Json.dll`.
- This local folder copy is intentionally not part of the distributable portable zip.

Artifacts:
- `build/bin/FanControl.exe` length `24564736`, time `2026/7/5 18:24:12`.
- `build/bin/FanControl-2.5.0-preview-amd64-installer.exe` length `28149307`, SHA256 `D4368A62836C8B12AB8FFC0716AE7C8FB2B7855A6FB0277BD1AA41FD4D39DAFE`.
- `build/bin/FanControl-2.5.0-preview-portable.zip` length `26024656`, SHA256 `60B2A90B6B7AF1360C9A298A9065872E22D7B993BAD0895B0D2790AAF9A080CE`.
- `build/bin/omen-fan-setup.exe` length `354054`, SHA256 `39AB9D20EB03DCE8ED55A789F128FA220A1663EE99A27011B5A00E684BFD93F7`.

Boundary and remaining work:
- `build/bin/plugins/omen-fan` is a local debugging convenience only.
- Main installer and distributable portable zip still do not bundle or manage the plugin.
- Real OMEN/Shadow Elf 11 non-mock WMI write and restore-on-stop validation remain undone and gated.
- If the page still looks wrong when launched from `build/bin/FanControl.exe`, next step is a screenshot-driven frontend polish pass with a browser/WebView capture rather than changing backend control code.

## 2026-07-05 Debug Mock UI Slice

Request:
- User installed the separate OMEN plugin on a non-OMEN/HONOR machine and saw no visible UI change.
- Temporary debug goal: ignore real machine support for the frontend entry, show the OMEN page when the plugin is installed, and use mock/status data for UI testing without real WMI writes.
- Installer goal: warn when unsupported/non-OMEN is detected, but allow installation for UI/mock/debug preview.

Implementation subagents:
- Pascal (`019f3197-156f-7f21-a16b-725229226d29`) updated `frontend/src/app/store/app-store.ts` and `frontend/src/app/components/OmenPage.tsx`.
  - OMEN active-tab recovery now redirects away only when the plugin is not installed.
  - Unsupported state no longer clears cached OMEN page data while the plugin remains installed.
  - Enable/refresh avoids unsupported hardware/control calls unless the refreshed store reports supported.
- Leibniz (`019f3197-854f-7952-ba8f-5f1b86324ea0`) updated `internal/coreapp/plugin_request_handlers.go` and `internal/coreapp/plugin_request_handlers_test.go`.
  - DebugMode exposes unsupported installed `omen-fan` as UI-level `supported=true` with `status="debug-mock"`.
  - `GetOmenFanStatus` returns populated mock status only in debug mode when the runtime plugin is unsupported.
  - The real `DevicePlugin.HardwareSupported()` and `SetFanTargets` gates remain unchanged, so monitoring/hardware-write logic still sees unsupported hardware as unsupported.
- Mendel (`019f3198-5f7f-7c41-91a8-253c12665390`) updated `plugins/omen-fan/installer/omen-fan-setup.nsi`.
  - Installer stages the built driver in `$PLUGINSDIR`, runs `--detect-only`, warns on unsupported/non-OMEN, and continues installation.
  - Payload/output remain unchanged; no `RMDir /r`, `taskkill`, or `schtasks` was added.
- Linnaeus (`019f31a2-a393-7573-a7cd-90d220ec295f`) updated `frontend/src/app/page.tsx`.
  - Top-level OMEN tab visibility is now installed-only: `omenVisible={view.omenInstalled}`.

Main-agent verification:
- PAL/Superpowers chat was attempted with `gpt-5.5` but failed with insufficient balance; release used local CodeGraph, source review, diff review, and validation commands.
- CodeGraph was used before broad code review and synced after generated bindings changed.
- `cd frontend; npx tsc --noEmit` passed.
- `cd frontend; npm run build` passed.
- `go test ./internal/coreapp/... ./internal/plugins/omenfan/... -count=1` passed.
- `go build ./internal/coreapp/... ./internal/plugins/omenfan/...` passed.
- `go vet ./internal/coreapp/... ./internal/plugins/omenfan/...` passed.
- `go test ./internal/coreapp/... ./internal/plugins/... ./internal/smartcontrol/... -count=1` passed.
- `go build ./internal/coreapp/... ./internal/plugins/... ./internal/smartcontrol/...` passed.
- `go vet ./internal/coreapp/... ./internal/plugins/... ./internal/smartcontrol/...` passed.
- `dotnet build plugins/omen-fan/src/OmenFanDriver.csproj -c Release` passed.
- `C:\Program Files (x86)\NSIS\makensis.exe plugins/omen-fan/installer/omen-fan-setup.nsi` passed and rebuilt `build\bin\omen-fan-setup.exe`.
- `cmd /c build.bat` passed and rebuilt `build\bin\FanControl-2.5.0-preview-amd64-installer.exe` plus `build\bin\FanControl-2.5.0-preview-portable.zip`.
- Driver checks on this HONOR/non-OMEN machine: `--detect-only` returns `supported:false`, `category:"wmi-missing"`; `--detect-only --mock` returns `supported:true`, `mock:true`.
- Portable zip inspection passed: no `omen-fan`, `OMEN`, `Omen`, or `plugins/` entries.
- Main packaging scripts still have no OMEN/plugin path references.
- `git diff --check` and scoped trailing-whitespace checks passed, with only Git LF-to-CRLF warnings.

Artifacts:
- `build\bin\FanControl-2.5.0-preview-amd64-installer.exe`
- `build\bin\FanControl-2.5.0-preview-portable.zip`
- `build\bin\omen-fan-setup.exe`

Boundary and remaining work:
- This is a temporary debug/mock UI route, not a release-ready support policy.
- Main installer/portable still do not include or manage `plugins\omen-fan`; plugin remains a separate installer / future plugin-market artifact.
- Real non-mock OMEN 11 hardware write and restore-on-stop validation remain undone and gated.
- Before public release, restore or tighten unsupported UI semantics and validate on real OMEN/Shadow Elf 11 hardware.
- UI polish follow-up: on unsupported debug mock, plugin runtime may still show stopped because real `Start` remains unsupported; consider a clearer "debug mock preview" label if needed.

## Harness Agent-Sizing Rule

- Product code changes still must be done by implementation subagents, with the main agent responsible for task cards, diff review, validation reruns, and phase release.
- Do not assign a whole large phase to one subagent when the phase can be split safely.
- Do not let one subagent carry too much work. Prefer opening several lightweight subagents, each scoped to one small responsibility, one package, or 1-2 files with its own checklist and validation evidence.
- Prefer multiple lightweight subagents with narrow, disjoint write scopes. Each subagent should own one small slice with explicit allowed files, forbidden files, validation commands, and checklist evidence.
- Default to multiple lightweight subagents when a phase can be split. A single subagent should usually own only one small responsibility, one package, or 1-2 files; assigning a whole phase to one subagent requires the main agent to write the reason in the task card.
- Each slice must be independently verifiable, independently checkable, and small enough to review without trusting a large bundled change.
- Avoid parallel subagents that write the same files. If one slice depends on another slice's types or API, run it after the dependency is merged and main-agent-verified.
- If a dispatched subagent task is discovered to be too broad, interrupt it at a safe point, collect its changed files/validation status, then re-split the work before continuing.
- Main-agent phase release still requires checking every slice, rerunning key validation, confirming forbidden files stayed untouched, and updating this handoff before moving to the next phase.

## Subagent Model And Effort Rule

- Future implementation/review subagents for this OMEN Harness must use `gpt-5.5`.
- Do not dispatch OMEN Harness subagents with `gpt-5.4`, `gpt-5.4-mini`, or any lower/downgraded model.
- If `gpt-5.5` dispatch fails due to service/API availability, record the failure and retry later with `gpt-5.5`; do not silently substitute a smaller model.
- Assign reasoning effort by task risk: low/medium effort is acceptable for documentation, isolated tests, and single-file low-risk edits; high effort is preferred for ordinary product-code slices; xhigh/max effort is reserved for WMI, real hardware write gates, monitor-loop concurrency, installer/uninstaller, and other high-risk slices.
- This rule supersedes older task-card wording that required `xhigh` for every subagent. Historical entries that mention `xhigh` describe earlier dispatches and should not be copied into new task cards blindly.

## P6 Feasibility Preflight

Date: 2026-07-05
Main-agent status: feasibility reviewed; P6 product code is now active through lightweight implementation subagent slices

Structure findings:
- CodeGraph status is up to date, and P1-P5 Go-side plugin, config, smartcontrol, CoreApp IPC, and Wails API slices are present in the dirty worktree.
- `frontend/src/app/store/app-store.ts` currently has `ActiveTab = 'status' | 'curve' | 'control' | 'devices' | 'about'`; P6 needs to add `omen` plus plugin/status state without breaking existing tabs.
- `frontend/src/app/services/api.ts` already uses `(window as any).go?.main?.App?...` for newer APIs not in generated Wails bindings. Current generated files under `frontend/wailsjs/go/main/App.*` do not yet expose P5 methods, so P6 should either use runtime fallback or create a separate Wails generation slice.
- `frontend/src/app/components/AppShell.tsx` has a compact icon sidebar using lucide icons and tooltip behavior. The OMEN tab should be dynamic and should not leave `activeTab='omen'` stuck if the plugin disappears.
- `frontend/src/app/page.tsx` mounts the existing content panes through `AppShell` props; P6 should add `OmenPage` as a separate pane after store/API support is verified.
- The plan's old locale paths are stale: `frontend/src/app/locales/zh-CN.json` does not exist in the current tree. P6 must locate the real i18n resources before editing translations.
- The existing `FanCurve` component is a large FlyDigi-focused page with many props; P6 should not paste the old `curve/onChange/unit/maxRpm` example. Prefer a small OMEN-specific RPM editor or a carefully scoped reusable extraction in a later task card.

Feasibility conclusion:
- P6 is feasible as a frontend-only Harness step, but it should be split into small serial/parallel slices with non-overlapping write scopes.
- Recommended P6 slices: API/types/store first; AppShell dynamic tab after store verification; `OmenPage` component after API/store verification; page mount after AppShell/OmenPage verification; locale edits as a separate slice after locating real i18n files.
- Do not edit `themes/`, `exported-themes/`, C# driver/installer files, generated Wails bindings, or packaging files in P6 unless a separate task card explicitly adds a binding generation gate.

Control-path conclusion:
- No command-mapping blocker is visible for the later P7 direction. Local references still support fan-level control via `0x2D` read level, `0x2E` write level, `0x2F` fan table, `0x28` SystemDesignData support, `0x10` fan/protection state, `0x2C` fan type/capability, and `0x1A` mode switching/restoration.
- P7 remains high risk because 暗影精灵 11 may require OMEN Hub or EC initialization before WMI writes are accepted. P7 task cards must start with mock/unsupported/read-only detection and only then allow a conservative OMEN 11 write test.

Current progress: P6 released by main-agent verification. Next action is P7 C# OMEN driver planning/task card, with 暗影精灵 11 fan-level control as the first target.

## P6 Task Card

Goal:
- Add the frontend OMEN surface after the released P5 Go-side Wails/API layer.
- Show the "laptop fan" / OMEN tab only when the `omen-fan` plugin is installed and hardware support is positive.
- Keep P6 frontend-only unless a later explicit task card creates a Wails binding-generation gate.
- Do not edit theme files, generated Wails files, C# driver code, installer/packaging scripts, or Go product code in P6.

Allowed files:
- `frontend/src/app/types/app.ts`
- `frontend/src/app/services/api.ts`
- `frontend/src/app/store/app-store.ts`
- `frontend/src/app/components/AppShell.tsx`
- `frontend/src/app/components/OmenPage.tsx`
- `frontend/src/app/page.tsx`
- `frontend/src/app/locales/zh-CN/translation.json`
- `frontend/src/app/locales/en-US/translation.json`
- `frontend/src/app/locales/ja-JP/translation.json`
- `docs/codex-handoffs/omen-plugin-harness.md`

Forbidden files:
- `themes/`
- `exported-themes/`
- `build.bat`
- `scripts/package_portable.ps1`
- `internal/`
- `bridge/`
- `frontend/wailsjs/`
- plugin installer/package files

Implementation adaptation notes:
- Use current repo structure from CodeGraph and current files, not stale P6 snippets from the plan.
- Do not manually edit `frontend/wailsjs/go/models.ts` or `frontend/wailsjs/go/main/App.*`.
- Because generated bindings do not yet expose P5 methods, new frontend API methods should use `(window as any).go?.main?.App?...`, matching the existing fallback style in `api.ts`.
- `frontend/src/app/locales/<locale>/translation.json` are the real locale files. Do not create stale `frontend/src/app/locales/zh-CN.json` style files.
- The existing `FanCurve` component is FlyDigi-focused and does not accept the stale `curve/onChange/unit/maxRpm` props from the old plan. `OmenPage` should use a small OMEN-specific RPM editor/preview in P6.
- OMEN curve editing must be local-preview first; hardware is not called while dragging/editing. Apply should call only `SetOmenFanCurve` through the P5 API.
- If the installed/supported state changes and `activeTab` is `omen`, the store/AppShell/page layer must recover to a valid visible tab instead of leaving the UI on a hidden tab.

Lightweight slice plan:
- Slice A - Frontend OMEN API/types/store: one worker owns only `frontend/src/app/types/app.ts`, `frontend/src/app/services/api.ts`, and `frontend/src/app/store/app-store.ts`.
- Slice B - AppShell dynamic tab: after Slice A main-agent verification, one worker owns only `frontend/src/app/components/AppShell.tsx`.
- Slice C - OmenPage UI: after Slice A main-agent verification, one worker owns only `frontend/src/app/components/OmenPage.tsx`.
- Slice D - Page mount: after Slices B/C main-agent verification, one worker owns only `frontend/src/app/page.tsx`.
- Slice E - Locale strings: one worker owns only the three `translation.json` files; may run after page/UI key names are stable.

Phase checklist:
- [x] Slice A API/types/store added / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Slice A main-agent verification passed / Validation: `cd frontend; npx tsc --noEmit`; scoped diff/boundary review; `git diff --check -- frontend/src/app/types/app.ts frontend/src/app/services/api.ts frontend/src/app/store/app-store.ts`
- [x] Slice B AppShell dynamic OMEN tab added / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Slice B main-agent verification passed / Validation: `cd frontend; npx tsc --noEmit`; scoped diff/boundary review; `git diff --check -- frontend/src/app/components/AppShell.tsx`
- [x] Slice A2 store tab type refinement completed / Validation: worker ran `cd frontend; npx tsc --noEmit`; main-agent ran `cd frontend; npx tsc --noEmit`; `git diff --check -- frontend/src/app/store/app-store.ts`
- [x] Slice C OmenPage added / Validation: worker ran `cd frontend; npx tsc --noEmit`; main-agent reviewed source and reran `cd frontend; npx tsc --noEmit`
- [x] Slice C2 OMEN bias unit conversion fixed / Validation: worker ran `cd frontend; npx tsc --noEmit`; main-agent reviewed source and reran `cd frontend; npx tsc --noEmit`
- [x] Slice D page mount added / Validation: worker ran `cd frontend; npx tsc --noEmit`; main-agent ran `cd frontend; npx tsc --noEmit`; `git diff --check -- frontend/src/app/page.tsx`
- [x] Slice E locale strings added / Validation: worker ran JSON parse/key checks and `cd frontend; npx tsc --noEmit`; main-agent ran JSON parse/key checks, `git diff --check --` locale files, and `cd frontend; npx tsc --noEmit`
- [x] P6 full validation passed / Validation: `cd frontend; npx tsc --noEmit`; `cd frontend; npm run build`; JSON parse for locales
- [x] Forbidden files unchanged for P6 / Validation: scoped allowed-list status plus explicit forbidden-path check
- [x] CodeGraph updated / Validation: `codegraph sync`; `codegraph status`

Current progress: P6 released by main-agent verification; P7 not started.

## P6 Slice A - Frontend OMEN API/Types/Store

Date: 2026-07-05
Implementation subagent: Parfit (`019f2eec-3423-7001-97a8-dcedb010032d`)
Allowed scope: `frontend/src/app/types/app.ts`, `frontend/src/app/services/api.ts`, `frontend/src/app/store/app-store.ts`

Checklist:
- [x] Added local frontend OMEN/plugin config and status types without generated Wails binding edits / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Added runtime-fallback `ApiService` methods/events for P5 APIs and IPC events / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Extended Zustand store with plugin snapshot and OMEN installed/supported/status/curve state / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Added defensive recovery when hidden `omen` tab becomes invalid / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Main-agent verification passed / Validation: `cd frontend; npx tsc --noEmit`; `git diff --check -- frontend/src/app/types/app.ts frontend/src/app/services/api.ts frontend/src/app/store/app-store.ts`; CodeGraph sync

Actual changes: `frontend/src/app/types/app.ts`, `frontend/src/app/services/api.ts`, `frontend/src/app/store/app-store.ts`

Main-agent review:
- Slice A stayed inside its allowed files and did not touch `frontend/wailsjs/`, themes, Go/internal files, C# driver files, packaging scripts, page/AppShell/OmenPage, or locale files.
- Frontend plugin types align with Go `types.PluginInfo` fields including `installed`, `supported`, and `running`.
- New frontend API calls use the existing runtime fallback style through `window.go.main.App`, which avoids manual generated-binding edits in P6.
- Store currently supports `omen` only defensively; AppShell/page exposure belongs to later slices.

Current progress: P6 Slice A released by main-agent verification.

## P6 Slice A2 - Store ActiveTab Type Refinement

Date: 2026-07-05
Implementation subagent: Pasteur (`019f2eff-8041-7c60-a1aa-c8774fa90748`)
Allowed scope: `frontend/src/app/store/app-store.ts`

Checklist:
- [x] Added `omen` to the store `ActiveTab` union instead of relying on casts / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Removed unnecessary OMEN tab recovery casts / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Main-agent verification passed / Validation: `cd frontend; npx tsc --noEmit`; `git diff --check -- frontend/src/app/store/app-store.ts`

Actual changes: `frontend/src/app/store/app-store.ts`

Main-agent review:
- Slice A2 stayed inside its single allowed file and kept the store tab model aligned with the dynamic AppShell tab.
- This was a narrow follow-up to Slice A and should not be expanded into page mounting or locale work.

Current progress: P6 Slice A2 released by main-agent verification.

## P6 Slice B - AppShell Dynamic OMEN Tab

Date: 2026-07-05
Implementation subagent: Curie (`019f2ef6-d6e0-7431-8953-f444c729f945`)
Allowed scope: `frontend/src/app/components/AppShell.tsx`

Checklist:
- [x] Added optional `omenVisible` and `omenContent` props with hidden defaults / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Extended AppShell tab union, content map, transition ordering, and sidebar rendering for dynamic `omen` / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Added hidden-tab recovery effect to call `onTabChange('status')` when `activeTab === 'omen'` and OMEN is no longer visible / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Main-agent verification passed / Validation: `cd frontend; npx tsc --noEmit`; `git diff --check -- frontend/src/app/components/AppShell.tsx`

Actual changes: `frontend/src/app/components/AppShell.tsx`

Main-agent review:
- Slice B stayed inside its single allowed file and did not touch page mounting, OmenPage, store/API/types, locale files, generated Wails bindings, themes, Go/internal files, C# driver files, or packaging scripts.
- The OMEN tab remains hidden by default and requires later page/store props to render.
- Follow-up before page mount: recheck whether the temporary `TabChangeHandler` bivariance type is still needed after page wiring.

Current progress: P6 Slice B released by main-agent verification.

## P6 Slice C - OmenPage UI

Date: 2026-07-05
Implementation subagent: Nash (`019f2ef7-d112-76e1-87aa-92e182aafe8c`)
Allowed scope: `frontend/src/app/components/OmenPage.tsx`

Checklist:
- [x] Added standalone OMEN page component without importing the FlyDigi-focused `FanCurve` page / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Added plugin unavailable/unsupported states, status metrics, local RPM curve draft editor, bias slider, and joint-learning toggle / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Kept curve editing local-preview first; hardware is only called through `setOmenFanCurve` on Apply / Validation: main-agent source review
- [x] Main-agent verification passed / Validation: source review; `cd frontend; npx tsc --noEmit`

Actual changes: `frontend/src/app/components/OmenPage.tsx`

Main-agent review:
- Slice C stayed inside its single allowed file and did not touch page mounting, locale files, generated Wails bindings, themes, Go/internal files, C# driver files, or packaging scripts.
- `OmenPage` uses translation keys under `omenPage.*`; Slice E must add these keys before final P6 build release.
- `git diff --check -- frontend/src/app/components/OmenPage.tsx` is not reliable while the file is untracked, so direct source review plus TypeScript validation is the current evidence.

Current progress: P6 Slice C released by main-agent verification with Slice C2 unit-conversion refinement applied.

## P6 Slice C2 - OmenPage Bias Unit Conversion

Date: 2026-07-05
Implementation subagent: Nietzsche (`019f2f01-a54c-7e42-a536-5f48ca46ab47`)
Allowed scope: `frontend/src/app/components/OmenPage.tsx`

Checklist:
- [x] Normalized backend `omen.fanBias` float values in `[-1,1]` to frontend slider percent values in `[-100,100]` / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Preserved legacy percent-like values when absolute value is greater than 1 / Validation: main-agent source review
- [x] Main-agent verification passed / Validation: source review; `cd frontend; npx tsc --noEmit`

Actual changes: `frontend/src/app/components/OmenPage.tsx`

Main-agent review:
- Slice C2 stayed inside the single allowed file and fixed the UI/Core unit boundary without changing API or store code.
- This keeps the backend config contract (`-1.0..+1.0`) separate from the UI slider contract (`-100..+100`).

Current progress: P6 Slices A, A2, B, C, and C2 released; Slice D page mount completed next.

## P6 Slice D - Page Mount

Date: 2026-07-05
Implementation subagent: Carson (`019f2f12-a147-7641-9de6-e647190f2203`)
Allowed scope: `frontend/src/app/page.tsx`

Checklist:
- [x] Imported `OmenPage` into the top-level page / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Selected `omenInstalled` and `omenSupported` from the app store view snapshot / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Passed `omenVisible={view.omenInstalled && view.omenSupported}` and `omenContent={<OmenPage />}` into `AppShell` / Validation: worker ran `cd frontend; npx tsc --noEmit`
- [x] Main-agent verification passed / Validation: source diff review; `git diff --check -- frontend/src/app/page.tsx`; `cd frontend; npx tsc --noEmit`

Actual changes: `frontend/src/app/page.tsx`

Main-agent review:
- Slice D stayed inside its single allowed file and did not touch locale files, `OmenPage.tsx`, `AppShell.tsx`, generated Wails bindings, themes, Go/internal files, C# driver files, or packaging scripts.
- The OMEN tab now becomes visible only when both `omenInstalled` and `omenSupported` are true.

Current progress: P6 Slice D released by main-agent verification.

## P6 Slice E - Locale Strings

Date: 2026-07-05
Implementation subagent: Boyle (`019f2f12-b554-7ca1-a861-63493b0cd559`)
Allowed scope:
- `frontend/src/app/locales/zh-CN/translation.json`
- `frontend/src/app/locales/en-US/translation.json`
- `frontend/src/app/locales/ja-JP/translation.json`

Checklist:
- [x] Added `appShell.tabs.omen` to all three locale files / Validation: worker ran JSON parse/key checks
- [x] Added all `omenPage.*` keys referenced by `OmenPage.tsx` / Validation: worker ran JSON parse/key checks
- [x] Preserved required interpolation variables `{{error}}`, `{{level}}`, `{{rpm}}`, and `{{time}}` / Validation: main-agent JSON key/interpolation check
- [x] Main-agent verification passed / Validation: JSON key/interpolation check; `git diff --check --` locale files; `cd frontend; npx tsc --noEmit`

Actual changes:
- `frontend/src/app/locales/zh-CN/translation.json`
- `frontend/src/app/locales/en-US/translation.json`
- `frontend/src/app/locales/ja-JP/translation.json`

Main-agent review:
- Slice E stayed inside its three allowed JSON files and did not touch `page.tsx`, `OmenPage.tsx`, `AppShell.tsx`, generated Wails bindings, themes, Go/internal files, C# driver files, or packaging scripts.
- Locale paths match the current repository structure; no stale `frontend/src/app/locales/zh-CN.json` style files were created.

Current progress: P6 Slices A, A2, B, C, C2, D, and E are released.

## P6 Main-Agent Verification

Date: 2026-07-05
Main-agent status: passed; P6 released

Main-agent validation commands:
- `cd frontend; npx tsc --noEmit` / passed
- `cd frontend; npm run build` / passed; only the existing `baseline-browser-mapping` age warning appeared
- `python -c "import json,sys; [json.load(open(f,encoding='utf-8')) for f in sys.argv[1:]]; print('json ok')" src/app/locales/zh-CN/translation.json src/app/locales/en-US/translation.json src/app/locales/ja-JP/translation.json` / passed
- JSON key/interpolation check for `appShell.tabs.omen`, all `omenPage.*` keys, and `{{error}}`, `{{level}}`, `{{rpm}}`, `{{time}}` / passed
- `git diff --check --` scoped P6 files / passed with only LF-to-CRLF Git warnings
- P6 allowed-list status / showed only expected P6 frontend files plus this handoff
- Explicit forbidden-path status for `frontend/wailsjs`, `themes`, `exported-themes`, `build.bat`, `scripts/package_portable.ps1`, and `bridge` / no changes
- `codegraph sync` / passed
- `codegraph status` / passed, index up to date

Main-agent review:
- The OMEN tab is now mounted from `page.tsx` and visible only when both `omenInstalled` and `omenSupported` are true.
- `OmenPage` remains frontend-only and local-preview-first: editing the curve does not call hardware; Apply calls the OMEN fan-curve API.
- P6 used runtime API fallbacks and did not manually edit generated Wails bindings.
- P6 did not edit theme files, generated Wails files, C# driver files, installer/packaging scripts, or default main package behavior.
- The wider worktree remains dirty from released P1-P5 and untracked `Cache/`; do not treat those as P6 changes.

Next step:
- Start P7 only after a fresh P7 Harness task card. P7 must stay separate-driver only, prioritize 暗影精灵 11, use fan-level WMI commands (`0x28`, `0x10`, `0x2C`, `0x2D`, `0x2F`, `0x2E`, `0x1A`), and keep non-OMEN/unverified machines on mock, read-only, or unsupported paths until validated.

## P7 Task Card

Goal:
- Add the independent OMEN fan driver module as a separate C# executable for the future standalone OMEN plugin installer.
- Prioritize 暗影精灵 11 / OMEN 11 safe control, using fan-level commands rather than raw RPM writes.
- Start with mock, unsupported, and read-only gates before any real write path.
- Keep the default FanControl main installer and portable package unchanged; P8 will handle the separate OMEN installer.

Allowed files:
- `plugins/omen-fan/src/OmenFanDriver.csproj`
- `plugins/omen-fan/src/plugin.json`
- `plugins/omen-fan/src/*.cs`
- `plugins/omen-fan/tests/**` if a later test slice needs it
- `internal/plugins/omenfan/**` only in a later Go-wrapper slice
- `internal/coreapp/plugin_discovery.go`, `internal/coreapp/plugin_request_handlers.go`, `internal/coreapp/*_test.go` only in later registration/API integration slices
- `docs/codex-handoffs/omen-plugin-harness.md`

Forbidden files:
- `build.bat`
- `scripts/package_portable.ps1`
- `build/windows/installer/**`
- `frontend/wailsjs/**`
- `themes/**`
- `exported-themes/**`
- `bridge/TempBridge/**`
- default main installer or portable packaging files
- existing FlyDigi device-control files under `internal/device/**`

Implementation adaptation notes:
- The repository currently has no `plugins/` source directory; P7 may create `plugins/omen-fan/src` for source only. Do not wire it into the main default package in P7.
- Existing C# code is only `bridge/TempBridge`; P7 must not merge OMEN fan control into TempBridge.
- Local reference checks point to fan-level control:
  - `0x28` SystemDesignData / SwFanControl support bit
  - `0x10` fan/protection/fan-count state
  - `0x2C` fan type/capability
  - `0x2D` current fan levels
  - `0x2F` fan table/bounds
  - `0x2E` write fan levels
  - `0x1A` mode switching/restoration
- Core/UI curves remain RPM targets only. The C# driver converts RPM to fan level conservatively with clamped bounds from fan table, observed levels, or safe defaults.
- No WMI write is allowed until mock, unsupported, WMI availability, support-bit, fan mapping, and bounds inference are implemented and verified.
- If OMEN hardware is not present, unsupported must be treated as a valid safe path rather than a test failure.
- Later Go wrapper must keep `DevicePlugin.SetFanTargets(cpuRPM,gpuRPM)` non-blocking through last-target coalescing / bounded queue and a dedicated writer goroutine.

Lightweight slice plan:
- Slice A - C# driver scaffold and safe mock/unsupported CLI: one worker owns only `plugins/omen-fan/src/OmenFanDriver.csproj`, `plugins/omen-fan/src/plugin.json`, and a minimal `plugins/omen-fan/src/Program.cs`. This slice may use three files because the project, manifest, and entrypoint must compile together as a minimal executable.
- Slice B - C# mock daemon protocol: after Slice A verification, one worker owns only the small protocol/mock worker files under `plugins/omen-fan/src`, with `--daemon --mock` status output and `set-fan` queue behavior.
- Slice C - C# WMI read-only adapter: one worker owns only WMI adapter/read-only probe files; implement `hpqBIntM` / `hpqBDataIn` availability, `0x28`, `0x10`, `0x2C`, `0x2D`, and `0x2F` reads with timeout/error classification, no writes.
- Slice D - C# fan-level conversion and write adapter behind mock/test gate: one worker owns only conversion/write adapter files; `0x2E` must be tested through a mock adapter first and disabled for real hardware unless explicitly gated.
- Slice E - Go runtime plugin wrapper: one worker owns only a new `internal/plugins/omenfan` package and necessary CoreApp registration tests; implement non-blocking target coalescing and process protocol.
- Slice F - integration validation: main agent and optional read-only reviewer verify build, mock/unsupported paths, Go tests, and hardware-gated behavior.

Phase checklist:
- [x] Slice A C# scaffold builds and safe detect paths run / Validation: worker and main-agent ran temp-output `dotnet build`; driver `--detect-only` returns unsupported safely; driver `--detect-only --mock` returns supported mock JSON
- [x] Slice B mock daemon protocol works / Validation: worker and main-agent verified mock daemon status envelopes, `set-fan`, invalid JSON, and unknown command behavior
- [x] Slice C WMI read-only detection works for safe/non-OMEN path / Validation: subagents implemented read-only WMI probe; main-agent verified non-OMEN `wmi-missing` returns unsupported with exit 0, source contains no write commands, and OMEN hardware supported path remains a hardware-only verification gap
- [x] Slice D fan-level write adapter is mock-tested / Validation: conversion/bounds helper and mock writer pass; non-mock daemon remains disabled and real hardware write remains absent/hardware-gated
- [ ] Slice E Go wrapper integrated / Validation: Go tests for non-blocking writer, process protocol, enable/disable lifecycle, and unsupported errors pass
- [ ] P7 full validation passed / Validation: `dotnet build`; mock/unsupported CLI reruns; relevant Go tests/build/vet; scoped forbidden-path check
- [x] CodeGraph updated / Validation: main-agent ran `codegraph sync`; `codegraph status`

Current progress: P7 Slices A, B, C, D, E1a1, and E1a2 released by main-agent verification. Slice E1b Go wrapper process lifecycle is next and not started.

## P7 Slice A - C# Driver Scaffold And Safe Detect Paths

Date: 2026-07-05
Implementation subagent: Schrodinger (`019f2f27-457c-79f1-8f31-975321e93adb`)
Allowed scope:
- `plugins/omen-fan/src/OmenFanDriver.csproj`
- `plugins/omen-fan/src/plugin.json`
- `plugins/omen-fan/src/Program.cs`

Checklist:
- [x] Added independent C# OMEN driver project with assembly name `omen-fan-driver` / Validation: worker ran `dotnet build plugins/omen-fan/src/OmenFanDriver.csproj -c Release`
- [x] Added safe `plugin.json` manifest for `omen-fan` with `type=device` and relative `omen-fan-driver.exe` executable / Validation: main-agent manifest field check
- [x] Implemented `--detect-only` unsupported path and `--detect-only --mock` supported mock path / Validation: worker and main-agent CLI reruns
- [x] Implemented minimal `--daemon --mock` status output and stdin EOF exit / Validation: worker and main-agent CLI reruns
- [x] Main-agent verification passed / Validation: temp-output `dotnet build`; `--detect-only`; `--detect-only --mock`; `--daemon --mock`; source review; no build residue; forbidden-path status; `go test ./internal/plugins/... -count=1`

Actual changes:
- `plugins/omen-fan/src/OmenFanDriver.csproj`
- `plugins/omen-fan/src/plugin.json`
- `plugins/omen-fan/src/Program.cs`

Main-agent review:
- Slice A stayed inside its three allowed files and did not touch main installer/portable packaging, `build.bat`, `scripts/package_portable.ps1`, `bridge/TempBridge`, `frontend`, themes, generated Wails files, Go/internal files, or existing device-control files.
- The C# code contains no real WMI implementation and no `0x2E` write path; only a safe unsupported message mentions WMI as not implemented.
- `plugins/omen-fan/src` contains only the three source files after validation; build outputs were produced in `%TEMP%` and removed.
- Slice A mock status currently outputs a minimal top-level status JSON. Slice B should standardize the daemon protocol to the Go wrapper envelope: `{"type":"status","status":{...}}` plus `{"type":"error","message":"..."}`.

Current progress: P7 Slice A released by main-agent verification; Slice B completed next.

## P7 Slice B - Mock Daemon Protocol

Date: 2026-07-05
Implementation subagent: Peirce (`019f2f33-4545-7833-8e72-227d541f1dcd`)
Allowed scope:
- `plugins/omen-fan/src/Program.cs`
- `plugins/omen-fan/src/OmenFanDriver.csproj`

Checklist:
- [x] Standardized mock daemon status envelope as `{"type":"status","status":{...}}` / Validation: worker and main-agent daemon CLI reruns
- [x] Added stdin line loop for `--daemon --mock` with `set-fan` command support / Validation: worker and main-agent piped `set-fan` reruns
- [x] Added safe mock RPM clamps and level estimation without real WMI writes / Validation: source review and CLI output
- [x] Invalid JSON and unknown commands emit `{"type":"error","message":"..."}` and exit normally on EOF / Validation: worker and main-agent invalid/unknown command reruns
- [x] Main-agent verification passed / Validation: temp-output `dotnet build`; `--detect-only`; `--detect-only --mock`; `--daemon --mock` with `set-fan`, invalid JSON, and unknown command; source review; no build residue; forbidden-path status

Actual changes:
- `plugins/omen-fan/src/Program.cs`
- `plugins/omen-fan/src/OmenFanDriver.csproj`

Main-agent review:
- Slice B stayed inside its two allowed files and did not touch `plugin.json`, main installer/portable packaging, `build.bat`, `scripts/package_portable.ps1`, `bridge/TempBridge`, `frontend`, themes, generated Wails files, Go/internal files, or existing device-control files.
- `Newtonsoft.Json` 13.0.3 was added to the OMEN driver project, matching TempBridge's JSON dependency style.
- The code still contains no real WMI implementation and no `0x2E` write path; the only WMI text remains the safe unsupported detect message.
- `plugins/omen-fan/src` contains only `OmenFanDriver.csproj`, `plugin.json`, and `Program.cs` after validation; build outputs were produced in `%TEMP%` and removed.

Current progress: P7 Slices A and B released by main-agent verification; Slice C read-only WMI detection is next.

## P7 Slice C Dispatch Note

Date: 2026-07-05
First implementation subagent: Socrates (`019f2f40-737c-70d2-9b4a-dc5da9183cc8`)
Result: failed before producing code due to external API quota/precharge error.

Main-agent handling:
- Closed the failed subagent and verified `git status --short -- plugins/omen-fan/src` still only showed the expected untracked source directory from Slices A/B.
- Re-dispatched the same Slice C task to a lightweight worker, Faraday (`019f3005-3f9f-71a2-86ae-a1055dfee83d`), with the same read-only WMI scope and write-path prohibitions.
- Faraday also failed before producing code due to a 503 model/channel error during the attempted dispatch route. No Faraday code changes were accepted.
- User clarified after this failure that all future subagents must use `gpt-5.5` with `xhigh` reasoning and must not use `gpt-5.4` or `gpt-5.4-mini`.
- Re-dispatched Slice C to lightweight worker Maxwell (`019f300f-c8fc-7f62-8b1e-2658eb360792`) with explicit `gpt-5.5` + `xhigh` settings, read-only WMI scope, and the same write-path prohibitions.

Main-agent review after Maxwell:
- Maxwell added `WmiReadOnlyProbe.cs` and kept mock/unsupported CLI behavior working, but main-agent source review found a hardware-path risk: read commands were sent with `Size=0` and empty `hpqBData`, while OMEN references use a 4-byte zero payload for fan reads such as `0x2C`, `0x2D`, and `0x2F`.
- Slice C was not released yet. A single-file fix was dispatched to Avicenna (`019f3020-24bc-7a13-9451-fc0df8c609da`) with explicit `gpt-5.5` + `xhigh`, allowed only to edit `plugins/omen-fan/src/WmiReadOnlyProbe.cs`.

## P7 Slice C - C# WMI Read-Only Detection

Date: 2026-07-05
Implementation subagents:
- Maxwell (`019f300f-c8fc-7f62-8b1e-2658eb360792`) implemented the read-only WMI probe.
- Avicenna (`019f3020-24bc-7a13-9451-fc0df8c609da`) fixed the read payload shape after main-agent review.

Allowed scope:
- `plugins/omen-fan/src/Program.cs`
- `plugins/omen-fan/src/OmenFanDriver.csproj`
- `plugins/omen-fan/src/WmiReadOnlyProbe.cs`

Checklist:
- [x] Added read-only OMEN WMI probe for `root\wmi`, `hpqBIntM`, `hpqBDataIn`, and `hpqBIOSInt128` / Validation: source review and temp-output `dotnet build`
- [x] Limited real WMI operations to read commands `0x28`, `0x10`, `0x2C`, `0x2D`, and `0x2F` / Validation: source review and forbidden-token search
- [x] Safe unsupported path returns JSON and exit 0 on non-OMEN/no-WMI machines / Validation: main-agent `--detect-only` returned `supported:false`, `category:"wmi-missing"`, exit 0
- [x] Mock detect and mock daemon behavior preserved / Validation: main-agent verified `--detect-only --mock`, `--daemon --mock` `set-fan`, invalid JSON, and unknown command
- [x] WMI read payload shape aligned with OMEN fan read references / Validation: Avicenna changed read input to a four-byte zero payload with `Size=4`
- [x] Main-agent verification passed / Validation: temp-output `dotnet build`; CLI reruns; no forbidden write tokens; no plugin `bin/obj` residue; scoped forbidden-path status; `codegraph sync`; `codegraph status`
- [ ] OMEN 11 supported hardware path verified / Validation: still pending real 暗影精灵 11 hardware because this machine only exercised safe unsupported/mock paths

Actual changes:
- `plugins/omen-fan/src/Program.cs`
- `plugins/omen-fan/src/OmenFanDriver.csproj`
- `plugins/omen-fan/src/WmiReadOnlyProbe.cs`

Main-agent review:
- Slice C stayed inside the allowed OMEN C# driver files and did not touch `plugin.json`, main installer/portable packaging, `build.bat`, `scripts/package_portable.ps1`, `bridge/TempBridge`, frontend, themes, generated Wails files, Go/internal files, or existing device-control files.
- `--detect-only` now uses the read-only WMI probe for non-mock runs and catches failures into safe `detect-result` JSON.
- The probe reports unsupported safely for no-WMI/non-OMEN conditions; unsupported is a valid outcome without OMEN hardware.
- No real write path was added. Source search found no `0x2E`, `0x1A`, `SetFanLevel`, or `SetFanMode`.
- Build outputs were produced in `%TEMP%`, removed afterward, and `plugins/omen-fan/src/bin` plus `plugins/omen-fan/src/obj` are absent.

Current progress: P7 Slice C released by main-agent verification. Slice D task card follows; implementation not started.

## P7 Slice D Task Card - Fan-Level Conversion And Mock Write Adapter

Goal:
- Add conservative RPM-to-fan-level conversion for the OMEN C# driver.
- Add a mock-only fan-level write adapter so `set-fan` exercises the same conversion and write-command shape without touching real WMI hardware.
- Keep real WMI writes disabled until a later explicit hardware-gated slice.

Allowed files:
- `plugins/omen-fan/src/Program.cs`
- `plugins/omen-fan/src/OmenFanDriver.csproj`
- `plugins/omen-fan/src/FanLevelConverter.cs`
- `plugins/omen-fan/src/MockFanLevelWriter.cs`
- `plugins/omen-fan/src/*SelfTest*.cs` only if the worker needs a tiny in-driver self-test helper
- `docs/codex-handoffs/omen-plugin-harness.md`

Forbidden files:
- `build.bat`
- `scripts/package_portable.ps1`
- `build/windows/installer/**`
- `frontend/**`
- `themes/**`
- `exported-themes/**`
- `bridge/TempBridge/**`
- `internal/**`
- `plugins/omen-fan/src/plugin.json`
- reference projects under `Cache/**` or `THRM-reference-git/**`

Implementation constraints:
- Use `gpt-5.5` with `xhigh` for every Slice D subagent; no `gpt-5.4` or mini fallback.
- Keep the split lightweight and serial if files overlap.
- RPM remains the UI/Core target unit. The C# driver converts to fan level with `round(rpm / 100.0)` and clamps to bounds.
- Bounds must be conservative: prefer probe/table/observed bounds when available in future, but for this mock slice use explicit safe mock/default bounds such as the existing mock RPM limits expressed as levels.
- Mock write should record/apply target levels and emit status, but must not call real WMI.
- Do not add a real hardware write path, mode switch, EC initialization, service installer, or Go wrapper in Slice D.
- If product source mentions the real fan-level write command, it must remain behind mock/test naming and must not be reachable by non-mock `--daemon`.

Lightweight slice plan:
- Slice D1 - fan-level conversion helper: one worker owns only `plugins/omen-fan/src/FanLevelConverter.cs` and, if needed, a tiny self-test helper file. It should add pure conversion/bounds types with no Program wiring and no WMI calls.
- Slice D2 - mock writer and daemon integration: after D1 main-agent verification, one worker owns only `plugins/omen-fan/src/Program.cs` and `plugins/omen-fan/src/MockFanLevelWriter.cs` plus minimal csproj updates if needed. It should route mock `set-fan` through the converter/mock writer and preserve existing JSON envelopes.
- Slice D3 - fix/review only if main-agent verification finds a scoped issue.

Checklist:
- [x] Slice D1 conversion helper added / Validation: Ampere and main-agent ran temp-output `dotnet build`; source review confirms pure conversion only
- [x] Slice D2 mock writer integration added / Validation: temp-output `dotnet build`; mock daemon `set-fan` exercises conversion/bounds and returns status
- [ ] Real WMI write path remains disabled / Validation: source review confirms non-mock `--daemon` is still unavailable and no hardware write call is reachable
- [ ] Mock/unsupported CLI regressions passed / Validation: `--detect-only`, `--detect-only --mock`, mock daemon set-fan, invalid JSON, and unknown command
- [ ] Main-agent verification passed / Validation: scoped forbidden-path status, no `bin/obj` residue, `git diff --check`, `codegraph sync`, `codegraph status`

## P7 Slice D1 - Fan-Level Conversion Helper

Date: 2026-07-05
Implementation subagent: Ampere (`019f3035-7d50-7f60-9cee-f65c288ad742`)
Allowed scope:
- `plugins/omen-fan/src/FanLevelConverter.cs`

Checklist:
- [x] Added pure conversion helper types `FanLevelBounds`, `FanLevelTarget`, and `FanLevelConverter` / Validation: source review
- [x] Implemented `round(rpm / 100.0, AwayFromZero)`, clamped bounds, and `estimatedRpm = level * 100` / Validation: source review
- [x] Kept the helper unconnected to `Program.cs` and free of WMI calls / Validation: source review and forbidden-token search
- [x] Main-agent verification passed / Validation: temp-output `dotnet build`; `--detect-only`; `--detect-only --mock`; mock daemon `set-fan`; forbidden-token search; no `bin/obj` residue; `git diff --check`

Actual changes:
- `plugins/omen-fan/src/FanLevelConverter.cs`

Main-agent review:
- D1 stayed inside the single allowed file and did not touch `Program.cs`, WMI probe, project file, manifest, Go/internal files, frontend, packaging, themes, or bridge files.
- The helper is pure .NET 4.7.2-compatible C# and imports only `System`.
- D2 should wire mock `set-fan` through this converter and a mock writer without enabling real hardware writes.

## P7 Slice D2 - Mock Fan-Level Writer Integration

Date: 2026-07-05
Implementation subagent: Aristotle (`019f3040-007b-78a2-92c3-4c7d53fd3f67`)
Allowed scope:
- `plugins/omen-fan/src/Program.cs`
- `plugins/omen-fan/src/MockFanLevelWriter.cs`

Checklist:
- [x] Added mock-only fan-level writer using `FanLevelConverter` / Validation: source review
- [x] Routed mock daemon `set-fan` through the converter/writer / Validation: main-agent CLI rerun with `cpuRpm=2600`, `gpuRpm=2400` produced levels `26` and `24`
- [x] Verified conservative mock bounds clamp / Validation: main-agent CLI rerun with `cpuRpm=1`, `gpuRpm=99999` produced levels `8` and `50`
- [x] Kept non-mock daemon disabled / Validation: main-agent `--daemon` returned usage and exit 2
- [x] Preserved detect/mock/error envelopes / Validation: `--detect-only`, `--detect-only --mock`, invalid JSON, and unknown command reruns
- [x] Main-agent verification passed / Validation: temp-output `dotnet build`; CLI reruns; forbidden-token search; no `bin/obj` residue; scoped forbidden-path status; `git diff --check`; `codegraph sync`; `codegraph status`

Actual changes:
- `plugins/omen-fan/src/Program.cs`
- `plugins/omen-fan/src/MockFanLevelWriter.cs`

Main-agent review:
- D2 stayed inside the allowed OMEN C# mock-driver files and did not touch WMI probe, manifest, Go/internal files, frontend, packaging, themes, or bridge files.
- Mock writer is pure mock logic and imports no WMI/Management APIs.
- No real hardware write path was added. Source search found no `0x2E`, `0x1A`, `SetFanLevel`, or `SetFanMode` in the D1/D2 files.
- `--daemon --mock` now exercises the same fan-level conversion path that later real writes can reuse behind explicit hardware gates.
- Build outputs were produced in `%TEMP%`, removed afterward, and `plugins/omen-fan/src/bin` plus `plugins/omen-fan/src/obj` are absent.

Current progress: P7 Slice D released by main-agent verification. Slice E Go runtime plugin wrapper task card follows; implementation not started.

## P7 Slice E Task Card - Go Runtime OMEN Plugin Wrapper

Goal:
- Add a Go-side `plugins.DevicePlugin` wrapper for the independent `omen-fan` C# driver.
- Keep `SetFanTargets(cpuRPM,gpuRPM)` non-blocking through last-target coalescing or a bounded queue.
- Parse the C# stdout protocol and cache `plugins.OmenFanStatus`.
- Preserve safe unsupported behavior when the driver executable is missing, non-OMEN, no-WMI, or not running.

Allowed files:
- `internal/plugins/omenfan/**`
- `internal/plugins/omenfan/*_test.go`
- `internal/coreapp/app.go` only in a later registration slice
- `internal/coreapp/lifecycle.go` only in a later lifecycle slice if needed
- `internal/coreapp/omen_plugin_test.go` only for registration/dispatch tests if needed
- `docs/codex-handoffs/omen-plugin-harness.md`

Forbidden files:
- `build.bat`
- `scripts/package_portable.ps1`
- `build/windows/installer/**`
- `frontend/**`
- `themes/**`
- `exported-themes/**`
- `bridge/TempBridge/**`
- `plugins/omen-fan/src/**` unless a later C# fix slice is explicitly written
- existing FlyDigi device-control files under `internal/device/**`
- reference projects under `Cache/**` or `THRM-reference-git/**`

Implementation constraints:
- Use `gpt-5.5` for every Slice E subagent; assign reasoning effort by slice risk per the top Subagent Model And Effort Rule. No `gpt-5.4` or mini fallback.
- Keep the wrapper independent from the default main installer/portable package. Slice E may reference a runtime executable path under the plugin install directory, but must not package or embed it.
- Do not call WMI from Go. Go communicates with the C# child process over stdin/stdout only.
- `SetFanTargets` must return quickly and must not block on process stdin, process start, WMI, or a long lock.
- Queue behavior must coalesce latest CPU/GPU targets; a full channel must drop stale target and keep newest.
- The worker goroutine may write `{"type":"set-fan","cpuRpm":...,"gpuRpm":...}` to stdin only after the process is running.
- stdout parsing must handle `{"type":"status","status":{...}}`, `{"type":"error","message":"..."}`, malformed lines, EOF, and process exit without panics.
- `HardwareSupported()` should be based on successful detect result and remain false for missing executable/unsupported JSON/errors.

Lightweight slice plan:
- Slice E1a - protocol/coalescer primitives: one worker owns only `internal/plugins/omenfan/protocol.go`, `internal/plugins/omenfan/target_queue.go`, and matching tests. Implement detect/status/error parsing and non-blocking latest-target coalescing. No process start, no CoreApp registration.
- Slice E1b - wrapper process lifecycle: after E1a main-agent verification, one worker owns only `internal/plugins/omenfan/plugin.go` plus focused tests. Implement constructor/options, `plugins.DevicePlugin`, injected command/process seams, detect-only support, daemon stdout/stdin loop, and stop handling.
- Slice E2 - CoreApp registration: after E1b main-agent verification, one worker owns only the minimal CoreApp registration/lifecycle files needed to register `omen-fan` when the discovered plugin manifest/runtime exists. Add focused tests.
- Slice E3 - fix/review only if main-agent verification finds a scoped issue.

Checklist:
- [x] Slice E1a1 protocol parsing primitives added / Validation: `gofmt -l`; `go test ./internal/plugins/omenfan/...`; `go build`; `go vet`; forbidden-token search
- [x] Slice E1a2 target coalescer primitives added / Validation: `gofmt -l`; `go test ./internal/plugins/omenfan/... -count=1`; `go build`; `go vet`; forbidden-token search; source review confirms non-blocking latest-target behavior
- [x] Slice E1b Go wrapper package added / Validation: E1b1/E1b2/E1b3a/E1b3b/E1b3c released by main-agent verification
- [x] Slice E2 CoreApp registration added / Validation: E2-A focused CoreApp tests/build prove plugin registers without affecting missing-runtime paths
- [x] SetFanTargets non-blocking verified / Validation: target queue and mock-daemon writer tests cover latest-target coalescing and non-blocking submission
- [x] Process protocol verified / Validation: unit tests cover detect result, status, error, malformed JSON, daemon stdout, stderr, and self-exit cleanup
- [x] Main-agent verification passed / Validation: relevant Go tests/build/vet, scoped forbidden-path status, `git diff --check`, trailing-whitespace check, `codegraph sync`, `codegraph status`

Dispatch note:
- Cicero (`019f304e-4577-7db1-af21-344081bc1c3c`) was dispatched for the original broad E1 package with `gpt-5.5` + `xhigh`, but produced no code before interruption.
- Main-agent response: split E1 into smaller E1a protocol/coalescer primitives and E1b wrapper process lifecycle.
- Boole (`019f3059-841a-7593-8aff-d04f439e4d9b`) was dispatched for E1a with `gpt-5.5` + `xhigh`, but produced no code before interruption. Main-agent response: split E1a again into E1a1 protocol parsing only and E1a2 target coalescer only.
- Aquinas (`019f3063-4058-7ec1-96c1-857099f72fbf`) was dispatched for E1a1 protocol parsing with `gpt-5.5` + `xhigh`, but produced no code before interruption. Main-agent response: re-dispatch E1a1 with an even narrower instruction to write only `protocol.go` and `protocol_test.go`, skip external sanity-checks, and avoid broader context work.

## P7 Slice E1a1 - Driver Protocol Parsing

Date: 2026-07-05
Implementation subagent: Poincare (`019f306b-3053-7603-8143-2c84e7cb6d07`)
Allowed scope:
- `internal/plugins/omenfan/protocol.go`
- `internal/plugins/omenfan/protocol_test.go`

Checklist:
- [x] Added status/error event parser for C# driver stdout / Validation: unit tests and source review
- [x] Added detect-result parser / Validation: supported, unsupported, and wrong-type unit tests
- [x] Kept parser free of process startup, CoreApp registration, C# edits, and WMI/write logic / Validation: source review and forbidden-token search
- [x] Main-agent verification passed / Validation: `gofmt -l`; `go test ./internal/plugins/omenfan/... -count=1`; `go build`; `go vet`; forbidden-token search

Actual changes:
- `internal/plugins/omenfan/protocol.go`
- `internal/plugins/omenfan/protocol_test.go`

Main-agent review:
- E1a1 stayed inside the two allowed files. It did not touch target queue, process wrapper, CoreApp, C# driver, plugin interfaces, frontend, packaging, themes, or bridge files.
- Parser functions are package-private, ready for the later wrapper slice.

## P7 Slice E1a2 Dispatch Note

Date: 2026-07-05
Implementation subagent: Heisenberg (`019f3072-602b-72e2-81f5-f698571f392e`)
Result: no code produced before interruption.

Main-agent handling:
- Closed the no-code subagent and verified `internal/plugins/omenfan` still contains only E1a1 protocol parser files.
- Slice E1a2 remains not started and should be re-dispatched as an even narrower two-file target-queue task.

## P7 Slice E1a2 Task Card - Target Coalescer Primitives

Date: 2026-07-05
Implementation subagent: Euclid (`019f30a5-280c-79a1-b2b0-b25c4099f38e`)
Main-agent status: released by main-agent verification

Goal:
- Add only the package-private target coalescer primitives needed by the later Go wrapper.
- Keep future `SetFanTargets(cpuRPM,gpuRPM)` non-blocking by proving latest-target coalescing behavior in unit tests.
- Do not start a process, write stdin, call WMI, register CoreApp hooks, or touch C# driver files in this slice.

Allowed files:
- `internal/plugins/omenfan/target_queue.go`
- `internal/plugins/omenfan/target_queue_test.go`

Forbidden files:
- `internal/plugins/omenfan/protocol.go`
- `internal/plugins/omenfan/protocol_test.go`
- `internal/plugins/omenfan/plugin.go`
- `internal/coreapp/**`
- `internal/plugins/plugin.go`
- `plugins/omen-fan/src/**`
- `frontend/**`
- `themes/**`
- `exported-themes/**`
- `build.bat`
- `scripts/package_portable.ps1`
- `bridge/**`
- `Cache/**`
- `THRM-reference-git/**`

Implementation constraints:
- The implementation subagent must use `gpt-5.5` with `xhigh` reasoning. Do not use `gpt-5.4`, `gpt-5.4-mini`, or any downgrade.
- Keep the slice lightweight and two-file only.
- Suggested package-private API: `type fanTarget struct { cpuRPM int; gpuRPM int }`, `type targetQueue struct { ch chan fanTarget }`, `newTargetQueue()`, `submit(cpuRPM, gpuRPM int)`, and `channel() <-chan fanTarget`.
- `submit` must not block. If the channel is full, it must drop the stale queued target and keep the newest target.
- Use only Go concurrency primitives. Do not import `os/exec`, WMI/Windows packages, or plugin lifecycle code.

Checklist:
- [x] Added package-private fan target queue/coalescer primitives / Validation: source review
- [x] Added tests for normal submit/receive behavior / Validation: `go test ./internal/plugins/omenfan/... -count=1`
- [x] Added tests proving overflow keeps the latest target / Validation: `go test ./internal/plugins/omenfan/... -count=1`
- [x] Added tests proving submit returns quickly while full / Validation: `go test ./internal/plugins/omenfan/... -count=1`
- [x] Worker validation passed / Validation: `gofmt -l internal/plugins/omenfan`; `go test ./internal/plugins/omenfan/... -count=1`; `go build ./internal/plugins/omenfan/...`; `go vet ./internal/plugins/omenfan/...`; forbidden-token search
- [x] Main-agent verification passed / Validation: source review; `gofmt -l internal/plugins/omenfan`; `go test ./internal/plugins/omenfan/... -count=1`; `go build ./internal/plugins/omenfan/...`; `go vet ./internal/plugins/omenfan/...`; forbidden-token search; `git diff --check`; `codegraph sync`; `codegraph status`

Actual changes:
- `internal/plugins/omenfan/target_queue.go`
- `internal/plugins/omenfan/target_queue_test.go`

Main-agent review:
- E1a2 stayed inside the two allowed files. It did not touch protocol parser files, process wrapper, CoreApp, C# driver, plugin interfaces, frontend, packaging, themes, bridge, Cache, or THRM reference files.
- `submit` uses non-blocking send/drain/send over a capacity-1 channel, so a full queue drops the stale target and keeps the newest target without blocking the monitoring path.
- No WMI, real hardware write, process launch, stdin writer, `0x2E`, `0x1A`, `SetFanLevel`, `SetFanMode`, `exec.Command`, or `os/exec` code was added.

Current progress: P7 Slice E1a2 released by main-agent verification. Slice E1b Go wrapper process lifecycle is next and not started.

## P7 Slice E1b1 Task Card - Plugin Shell And Non-Blocking Queue

Date: 2026-07-05
Implementation subagent: Hilbert (`019f30b9-414c-77e0-952d-24520e289912`)
Main-agent status: released by main-agent verification

Goal:
- Add the first tiny Go wrapper shell for the OMEN runtime plugin.
- Implement only the `plugins.DevicePlugin` surface that can be verified without starting the C# process.
- Wire `SetFanTargets(cpuRPM,gpuRPM)` to the released E1a2 target coalescer so the monitoring path stays non-blocking.
- Keep process start, detect-only execution, stdin writer, stdout scanner, CoreApp registration, and installer/package work for later E1b/E2 slices.

Allowed files:
- `internal/plugins/omenfan/plugin.go`
- `internal/plugins/omenfan/plugin_test.go`

Forbidden files:
- `internal/plugins/omenfan/protocol.go`
- `internal/plugins/omenfan/protocol_test.go`
- `internal/plugins/omenfan/target_queue.go`
- `internal/plugins/omenfan/target_queue_test.go`
- `internal/coreapp/**`
- `internal/plugins/plugin.go`
- `plugins/omen-fan/src/**`
- `frontend/**`
- `themes/**`
- `exported-themes/**`
- `build.bat`
- `scripts/package_portable.ps1`
- `bridge/**`
- `Cache/**`
- `THRM-reference-git/**`

Implementation constraints:
- The implementation subagent must use `gpt-5.5` with `xhigh` reasoning. Do not use `gpt-5.4`, `gpt-5.4-mini`, or any downgrade.
- Keep the slice lightweight and two-file only.
- Do not import or use `os/exec`, `exec.Command`, WMI/Windows APIs, process pipes, or C# driver code in this slice.
- Suggested package-private/exported API: `const PluginID = "omen-fan"`, `const PluginName = "HP OMEN Fan Control"` or equivalent, `type Options struct { Logger ...; DriverPath string }`, `func New(options Options) *Plugin`.
- `Plugin` must implement `plugins.DevicePlugin` but may keep `Start` as a safe no-process stub for this slice if process lifecycle is deferred. If `Start` is a stub, it must not set `Running` true or claim hardware support.
- `Status`, `FanStatus`, and `HardwareSupported` must return cached state without blocking.
- `SetFanTargets` must return quickly and must never write stdin or start a process. For a running/supported test state, it should enqueue through `targetQueue.submit`; for not-running/unsupported state, return a clear error quickly.

Checklist:
- [ ] Added OMEN plugin shell implementing `plugins.DevicePlugin` / Validation: compile-time interface assertion and `go test`
- [ ] Added non-blocking `SetFanTargets` queue integration without process/stdin writes / Validation: unit test covers quick return and latest-target queue behavior
- [ ] Added cached status/support/runtime status tests / Validation: `go test ./internal/plugins/omenfan/... -count=1`
- [ ] Kept process lifecycle out of scope / Validation: forbidden-token search has no matches for `os/exec`, `exec.Command`, WMI, `0x2E`, `0x1A`, `SetFanLevel`, `SetFanMode`
- [ ] Worker validation passed / Validation: `gofmt -l internal/plugins/omenfan`; `go test ./internal/plugins/omenfan/... -count=1`; `go build ./internal/plugins/omenfan/...`; `go vet ./internal/plugins/omenfan/...`; forbidden-token search
- [ ] Main-agent verification passed / Validation: pending

Implementation subagent: Dalton (`019f3093-0269-7060-b966-246d3f871e65`)

Checklist:
- [x] Added OMEN plugin shell implementing `plugins.DevicePlugin` / Validation: compile-time interface assertion and `go test`
- [x] Added non-blocking `SetFanTargets` queue integration without process/stdin writes / Validation: unit test covers quick return and latest-target queue behavior
- [x] Added cached status/support/runtime status tests / Validation: `go test ./internal/plugins/omenfan/... -count=1`
- [x] Kept process lifecycle out of scope / Validation: forbidden-token search has no matches for `os/exec`, `exec.Command`, WMI, `0x2E`, `0x1A`, `SetFanLevel`, `SetFanMode`, `StdinPipe`, or `StdoutPipe`
- [x] Worker validation passed / Validation: `gofmt -l internal/plugins/omenfan`; `go test ./internal/plugins/omenfan/... -count=1`; `go build ./internal/plugins/omenfan/...`; `go vet ./internal/plugins/omenfan/...`; forbidden-token search
- [x] Main-agent verification passed / Validation: source review; `gofmt -l internal/plugins/omenfan`; `go test ./internal/plugins/omenfan/... -count=1`; `go build ./internal/plugins/omenfan/...`; `go vet ./internal/plugins/omenfan/...`; forbidden-token search; `git diff --check`

Actual changes:
- `internal/plugins/omenfan/plugin.go`
- `internal/plugins/omenfan/plugin_test.go`

Main-agent review:
- E1b1 stayed inside the two allowed files. It did not touch protocol parser files, target queue files, CoreApp, C# driver, plugin interfaces, frontend, packaging, themes, bridge, Cache, or THRM reference files.
- `Start` and `Stop` are honest no-process stubs for this slice: they do not launch a process, do not set `Running` true, and do not claim hardware support.
- `SetFanTargets` returns clear errors for not-running/unsupported state, and only queues through the E1a2 target coalescer in test-controlled running/supported state.
- No WMI, real hardware write, process launch, stdin/stdout pipe, `0x2E`, `0x1A`, `SetFanLevel`, `SetFanMode`, `exec.Command`, or `os/exec` code was added.

Current progress: P7 Slice E1b1 released by main-agent verification. Slice E1b2 detect/process seam is next and not started.

## P7 Slice E1b2 Task Card - Detect-Only Process Seam

Date: 2026-07-05
Implementation subagent: Arendt (`019f30c4-e2f4-7bc0-ba76-f8fd59d3a453`)
Main-agent status: released by main-agent verification

Goal:
- Add only the detect-only process seam for the Go OMEN wrapper.
- Let `Start(ctx)` run a bounded `--detect-only` command, parse the released detect-result protocol, and cache supported/unsupported state honestly.
- Keep daemon mode, stdin writer, stdout scanner, target writer goroutine, CoreApp registration, C# edits, and installer/package work for later slices.

Allowed files:
- `internal/plugins/omenfan/plugin.go`
- `internal/plugins/omenfan/plugin_test.go`

Forbidden files:
- `internal/plugins/omenfan/protocol.go`
- `internal/plugins/omenfan/protocol_test.go`
- `internal/plugins/omenfan/target_queue.go`
- `internal/plugins/omenfan/target_queue_test.go`
- `internal/coreapp/**`
- `internal/plugins/plugin.go`
- `plugins/omen-fan/src/**`
- `frontend/**`
- `themes/**`
- `exported-themes/**`
- `build.bat`
- `scripts/package_portable.ps1`
- `bridge/**`
- `Cache/**`
- `THRM-reference-git/**`

Implementation constraints:
- The implementation subagent must use `gpt-5.5` with `xhigh` reasoning. Do not use `gpt-5.4`, `gpt-5.4-mini`, or any downgrade.
- Keep the slice lightweight and two-file only.
- This slice may introduce an injected detect command seam and use `os/exec` only for `--detect-only`. Do not start `--daemon`.
- Do not use `StdinPipe`, `StdoutPipe`, `StderrPipe`, persistent process fields, daemon loops, target writer goroutines, or any hardware/WMI code in Go.
- `Start(ctx)` must return quickly under a bounded timeout, update cached supported/lastErr state from `parseDetectResult`, and leave `running` false until a later daemon slice actually starts the long-running process.
- Missing `DriverPath`, command errors, malformed JSON, wrong protocol type, and unsupported detect JSON must all leave `HardwareSupported()` false with a useful `Status().LastError`.
- A supported detect result may set `supported=true` while still keeping `running=false` for this slice.
- `Stop()` remains idempotent and no-process.

Checklist:
- [x] Added injected detect-only command seam / Validation: unit tests avoid launching a real process
- [x] `Start(ctx)` runs bounded detect-only path and caches supported result without claiming daemon running / Validation: `go test ./internal/plugins/omenfan/... -count=1`
- [x] Missing driver path, command error, malformed JSON, wrong type, and unsupported JSON remain safe unsupported paths / Validation: unit tests
- [x] Kept daemon lifecycle out of scope / Validation: source review and search show no `--daemon`, `StdinPipe`, `StdoutPipe`, `StderrPipe`, persistent process fields, or writer goroutine
- [x] Worker validation passed / Validation: `gofmt -l internal/plugins/omenfan`; `go test ./internal/plugins/omenfan/... -count=1`; `go build ./internal/plugins/omenfan/...`; `go vet ./internal/plugins/omenfan/...`; forbidden-token/boundary search
- [x] Main-agent verification passed / Validation: source review; `gofmt -l internal/plugins/omenfan`; `go test ./internal/plugins/omenfan/... -count=1`; `go build ./internal/plugins/omenfan/...`; `go vet ./internal/plugins/omenfan/...`; forbidden-token search; `os/exec` boundary search; `git diff --check`; CodeGraph sync/status

Actual changes:
- `internal/plugins/omenfan/plugin.go`
- `internal/plugins/omenfan/plugin_test.go`

Main-agent review:
- E1b2 stayed inside the two allowed product files.
- `Start(ctx)` now validates a driver path, runs an injected bounded `--detect-only` command, parses `detect-result`, and caches supported/unsupported state.
- `Start(ctx)` still leaves `running=false`; `SetFanTargets` therefore still returns `ErrNotRunning` after detect-only until a later daemon slice actually starts a persistent process.
- Missing driver path, command error, malformed detect JSON, wrong protocol type, and unsupported detect JSON all cache safe unsupported/error state.
- The only `os/exec` use in `internal/plugins/omenfan` is `exec.CommandContext(ctx, driverPath, "--detect-only").Output()`.
- No daemon mode, stdin/stdout/stderr pipes, writer goroutine, CoreApp registration, C# edit, WMI call, real hardware write, `0x2E`, `0x1A`, `SetFanLevel`, or `SetFanMode` was added.

Main-agent validation:
- Passed `gofmt -l internal/plugins/omenfan`.
- Passed `go test ./internal/plugins/omenfan/... -count=1`.
- Passed `go build ./internal/plugins/omenfan/...`.
- Passed `go vet ./internal/plugins/omenfan/...`.
- Passed forbidden-token search: `rg -n -- "--daemon|StdinPipe|StdoutPipe|StderrPipe|SetFanLevel|SetFanMode|0x2E|0x1A|WMI" internal/plugins/omenfan` returned no matches.
- Passed `os/exec` boundary search: `os/exec` appears only in `plugin.go` for the default detect-only runner.
- Passed `git diff --check -- internal/plugins/omenfan/plugin.go internal/plugins/omenfan/plugin_test.go docs/codex-handoffs/omen-plugin-harness.md`.
- Passed trailing-whitespace check on E1b2 files and handoff.
- Passed `codegraph sync` and `codegraph status`; index is up to date.

Current progress: P7 Slice E1b2 released by main-agent verification. Slice E1b3a daemon command serialization task card follows; implementation not started.

## P7 Slice E1b3a Task Card - Daemon Command Serialization

Date: 2026-07-05
Implementation subagent: Volta (`019f30d3-2081-7ee1-9304-8ebbc1f18487`)
Main-agent status: released by main-agent verification

Goal:
- Add only a pure Go helper for encoding OMEN daemon `set-fan` commands.
- Match the released C# mock daemon protocol: compact JSON line with `type:"set-fan"`, `cpuRpm`, and `gpuRpm`.
- Keep process lifecycle, `--daemon` launch, stdin writer, stdout scanner, `Stop()` process teardown, CoreApp registration, C# edits, and installer/package work for later slices.

Allowed files:
- `internal/plugins/omenfan/protocol.go`
- `internal/plugins/omenfan/protocol_test.go`

Forbidden files:
- `internal/plugins/omenfan/plugin.go`
- `internal/plugins/omenfan/plugin_test.go`
- `internal/plugins/omenfan/target_queue.go`
- `internal/plugins/omenfan/target_queue_test.go`
- `internal/coreapp/**`
- `internal/plugins/plugin.go`
- `plugins/omen-fan/src/**`
- `frontend/**`
- `themes/**`
- `exported-themes/**`
- `build.bat`
- `scripts/package_portable.ps1`
- `bridge/**`
- `Cache/**`
- `THRM-reference-git/**`

Implementation constraints:
- The implementation subagent must use `gpt-5.5` with `xhigh` reasoning. Do not use `gpt-5.4`, `gpt-5.4-mini`, or any downgrade.
- Keep the slice lightweight and two-file only.
- Prefer structured JSON encoding over string concatenation.
- The helper should be package-private and directly usable by a later stdin writer. Returning a single newline-terminated JSON command line is acceptable and preferred.
- Do not start a process, do not mention or pass `--daemon`, do not use `os/exec`, and do not add `StdinPipe`, `StdoutPipe`, `StderrPipe`, goroutines, persistent process fields, timers, or WMI/hardware code.
- The helper must preserve integer RPM targets exactly; clamping/conversion to OMEN fan levels remains in the C# layer.

Checklist:
- [x] Added package-private set-fan command encoder / Validation: unit test unmarshals the encoded JSON and checks `type`, `cpuRpm`, and `gpuRpm`
- [x] Encoded command is one newline-terminated line suitable for stdin writing / Validation: unit test checks the trailing newline and compact JSON line behavior
- [x] Kept process lifecycle out of scope / Validation: source review and forbidden-token search show no `--daemon`, pipes, WMI, `0x2E`, or `0x1A`; no `os/exec` in the edited protocol files
- [x] Worker validation passed / Validation: `gofmt -l internal/plugins/omenfan/protocol.go internal/plugins/omenfan/protocol_test.go`; `go test ./internal/plugins/omenfan/... -count=1`; `go build ./internal/plugins/omenfan/...`; `go vet ./internal/plugins/omenfan/...`; forbidden-token/boundary search
- [x] Main-agent verification passed / Validation: source review; `gofmt -l internal/plugins/omenfan/protocol.go internal/plugins/omenfan/protocol_test.go`; `go test ./internal/plugins/omenfan/... -count=1`; `go build ./internal/plugins/omenfan/...`; `go vet ./internal/plugins/omenfan/...`; forbidden-token search; `os/exec` boundary search; `git diff --check`; CodeGraph sync

Actual changes:
- `internal/plugins/omenfan/protocol.go`
- `internal/plugins/omenfan/protocol_test.go`

Main-agent review:
- E1b3a stayed inside the two allowed protocol files.
- `encodeSetFanCommand(cpuRpm, gpuRpm int)` uses structured JSON encoding for `{"type":"set-fan","cpuRpm":...,"gpuRpm":...}` and appends exactly one newline for future stdin writing.
- Tests parse the encoded payload back to structured fields and verify compact single-line JSON plus trailing newline.
- No process start, daemon argument, stdin/stdout/stderr pipe, writer goroutine, CoreApp registration, C# edit, WMI call, real hardware write, `0x2E`, `0x1A`, `SetFanLevel`, or `SetFanMode` was added.

Main-agent validation:
- Passed `gofmt -l internal/plugins/omenfan/protocol.go internal/plugins/omenfan/protocol_test.go`.
- Passed `go test ./internal/plugins/omenfan/... -count=1`.
- Passed `go build ./internal/plugins/omenfan/...`.
- Passed `go vet ./internal/plugins/omenfan/...`.
- Passed forbidden-token search over `internal/plugins/omenfan`: no `--daemon`, `StdinPipe`, `StdoutPipe`, `StderrPipe`, `SetFanLevel`, `SetFanMode`, `0x2E`, `0x1A`, or `WMI`.
- Passed `os/exec` boundary search: `os/exec` remains only in `plugin.go` for E1b2 `--detect-only`.
- Passed `git diff --check -- internal/plugins/omenfan/protocol.go internal/plugins/omenfan/protocol_test.go docs/codex-handoffs/omen-plugin-harness.md`.
- Passed trailing-whitespace check on E1b3a files and handoff.
- Passed `codegraph sync`; index was refreshed.

Current progress: P7 Slice E1b3a released by main-agent verification. Slice E1b3b driver event application task card follows; implementation not started.

## P7 Slice E1b3b Task Card - Driver Event Application

Date: 2026-07-05
Main-agent status: task card prepared; implementation not started

Goal:
- Add only a small Go helper that applies already-released driver JSON event lines to the plugin cache.
- Prepare for a later stdout scanner by reusing `parseDriverEvent(line)`.
- Keep process lifecycle, `--daemon` launch, stdin writer, stdout scanner loop, `Stop()` process teardown, CoreApp registration, C# edits, and installer/package work for later slices.

Allowed files:
- `internal/plugins/omenfan/plugin.go`
- `internal/plugins/omenfan/plugin_test.go`

Forbidden files:
- `internal/plugins/omenfan/protocol.go`
- `internal/plugins/omenfan/protocol_test.go`
- `internal/plugins/omenfan/target_queue.go`
- `internal/plugins/omenfan/target_queue_test.go`
- `internal/coreapp/**`
- `internal/plugins/plugin.go`
- `plugins/omen-fan/src/**`
- `frontend/**`
- `themes/**`
- `exported-themes/**`
- `build.bat`
- `scripts/package_portable.ps1`
- `bridge/**`
- `Cache/**`
- `THRM-reference-git/**`

Implementation constraints:
- The implementation subagent must use `gpt-5.5` with `xhigh` reasoning. Do not use `gpt-5.4`, `gpt-5.4-mini`, or any downgrade.
- Keep the slice lightweight and two-file only.
- Add a package-private helper, for example `handleDriverLine(line []byte) error`, that parses one line and updates cached state under the existing mutex.
- For `status` events, update `fanStatus`, update `supported` from `status.Supported`, and clear `lastErr`.
- For `error` events, cache `lastErr` and leave the last known fan status intact.
- For parse errors, cache the parse error in `lastErr` and return the error.
- Do not start a process, do not pass or mention `--daemon` in code, do not add pipes, goroutines, persistent process fields, timers, or WMI/hardware code.
- Do not change `Start`, `Stop`, `SetFanTargets`, or `defaultDetectCommand` behavior except if absolutely required for tests; this slice is only event application.

Checklist:
- [x] Added package-private driver-event application helper / Validation: unit test applies a status event and checks cached `FanStatus`, `HardwareSupported`, and cleared `LastError`
- [x] Error and parse-error events cache useful `Status().LastError` without mutating previous status / Validation: unit tests
- [x] Kept process lifecycle out of scope / Validation: source review and forbidden-token search show no `--daemon`, pipes, WMI, `0x2E`, or `0x1A`
- [x] Worker validation passed / Validation: `gofmt -l internal/plugins/omenfan/plugin.go internal/plugins/omenfan/plugin_test.go`; `go test ./internal/plugins/omenfan/... -count=1`; `go build ./internal/plugins/omenfan/...`; `go vet ./internal/plugins/omenfan/...`; forbidden-token/boundary search
- [x] Main-agent verification passed / Validation: source review; CodeGraph review; `gofmt -l internal/plugins/omenfan/plugin.go internal/plugins/omenfan/plugin_test.go`; `go test ./internal/plugins/omenfan/... -count=1`; `go build ./internal/plugins/omenfan/...`; `go vet ./internal/plugins/omenfan/...`; forbidden-token search; `os/exec` boundary search; `git diff --check`; CodeGraph sync/status

Actual changes:
- `internal/plugins/omenfan/plugin.go`
- `internal/plugins/omenfan/plugin_test.go`

Main-agent review:
- E1b3b stayed inside the two allowed product files.
- `handleDriverLine(line []byte)` reuses `parseDriverEvent(line)`, then updates cached fan status/support/error state under the existing plugin mutex.
- Status events update `fanStatus`, set `supported` from the raw status envelope, and clear `lastErr`.
- Driver error events and parse errors cache useful `lastErr` text while preserving the prior `fanStatus` and support state.
- `Start`, `Stop`, `SetFanTargets`, and `defaultDetectCommand` behavior were not changed.
- Because `plugins.OmenFanStatus` does not currently carry `supported`, this slice uses a tiny local raw-envelope parse for support while still using `parseDriverEvent` for the released event path. A later protocol cleanup may formalize this if protocol files are in scope.
- No process start, daemon argument, stdin/stdout/stderr pipe, scanner loop, writer goroutine, CoreApp registration, C# edit, WMI call, real hardware write, `0x2E`, `0x1A`, `SetFanLevel`, or `SetFanMode` was added.

Main-agent validation:
- Passed `gofmt -l internal/plugins/omenfan/plugin.go internal/plugins/omenfan/plugin_test.go`.
- Passed `go test ./internal/plugins/omenfan/... -count=1`.
- Passed `go build ./internal/plugins/omenfan/...`.
- Passed `go vet ./internal/plugins/omenfan/...`.
- Passed forbidden-token search over `internal/plugins/omenfan`: no `--daemon`, `StdinPipe`, `StdoutPipe`, `StderrPipe`, `SetFanLevel`, `SetFanMode`, `0x2E`, `0x1A`, or `WMI`.
- Passed `os/exec` boundary search: `os/exec` remains only in `plugin.go` for E1b2 `--detect-only`.
- Passed `git diff --check -- internal/plugins/omenfan/plugin.go internal/plugins/omenfan/plugin_test.go docs/codex-handoffs/omen-plugin-harness.md`.
- Passed trailing-whitespace check on E1b3b files and handoff.
- Passed `codegraph sync` and `codegraph status`; index is up to date.

Current progress: P7 Slice E1b3b released by main-agent verification. Slice E1b3c mock daemon process lifecycle task card follows; implementation not started.

## P7 Slice E1b3c Task Card - Mock Daemon Process Lifecycle

Date: 2026-07-05
Main-agent status: task card prepared; implementation not started

Goal:
- Add the first real Go-side daemon process lifecycle, but only for the already-safe C# mock daemon path.
- `Start(ctx)` may, after successful detect-only support, launch the driver with `--daemon --mock`, connect stdout/stderr/stdin pipes, read stdout events through the released `handleDriverLine`, and write queued fan targets through the released `encodeSetFanCommand`.
- Keep CoreApp registration, real non-mock daemon launch, C# edits, real WMI write, packaging, and installer work for later slices.

Allowed files:
- `internal/plugins/omenfan/plugin.go`
- `internal/plugins/omenfan/plugin_test.go`

Forbidden files:
- `internal/plugins/omenfan/protocol.go`
- `internal/plugins/omenfan/protocol_test.go`
- `internal/plugins/omenfan/target_queue.go`
- `internal/plugins/omenfan/target_queue_test.go`
- `internal/coreapp/**`
- `internal/plugins/plugin.go`
- `plugins/omen-fan/src/**`
- `frontend/**`
- `themes/**`
- `exported-themes/**`
- `build.bat`
- `scripts/package_portable.ps1`
- `bridge/**`
- `Cache/**`
- `THRM-reference-git/**`

Implementation constraints:
- The implementation subagent must use `gpt-5.5` with `xhigh` reasoning. Do not use `gpt-5.4`, `gpt-5.4-mini`, or any downgrade.
- Keep the slice lightweight and two-file only.
- Add dependency-injected process seams so unit tests do not need to launch the real C# executable.
- Non-mock real hardware daemon launch must remain unreachable in this slice. Only detect results with `mock:true` may proceed to daemon startup.
- The command args for the daemon path must be exactly `--daemon --mock`; do not add a reachable plain `--daemon` path.
- `Start(ctx)` should still return safe unsupported/errors when detect-only says unsupported, missing driver path, command failure, parse failure, or supported non-mock.
- The writer path must consume the existing target queue, encode commands with `encodeSetFanCommand`, and avoid blocking `SetFanTargets`.
- The stdout path must call `handleDriverLine`; stderr should cache error text without crashing the process.
- `Stop()` must be idempotent, cancel the process context, close stdin when present, and set `running=false`.
- Do not touch CoreApp, C# driver files, WMI code, real `0x2E`, `0x1A`, installer, frontend, themes, or packaging.

Checklist:
- [x] Added injected process runner/pipes for tests / Validation: unit tests use fakes and do not launch a real executable
- [x] `Start(ctx)` launches only mock daemon after supported mock detect and marks running / Validation: unit tests assert args and status
- [x] stdout events update cached fan status; stderr/driver errors cache `LastError` / Validation: unit tests
- [x] target writer consumes existing non-blocking queue and writes encoded `set-fan` lines without blocking `SetFanTargets` / Validation: unit test
- [x] `Stop()` is idempotent and tears down the fake daemon state / Validation: unit tests
- [x] Daemon self-exit cancels the run context and stops stale writer state / Validation: follow-up regression test after main-agent review
- [x] Real/non-mock daemon remains gated out / Validation: unit test for supported non-mock detect returns safe error and does not call daemon runner
- [x] Worker validation passed / Validation: `gofmt -l internal/plugins/omenfan/plugin.go internal/plugins/omenfan/plugin_test.go`; `go test ./internal/plugins/omenfan/... -count=1`; `go build ./internal/plugins/omenfan/...`; `go vet ./internal/plugins/omenfan/...`; boundary search confirming no WMI, `0x2E`, `0x1A`, `SetFanLevel`, or `SetFanMode`
- [x] Main-agent verification passed / Validation: source review; CodeGraph review; targeted lifecycle tests; `go test ./internal/plugins/omenfan/... -count=1`; `go build ./internal/plugins/omenfan/...`; `go vet ./internal/plugins/omenfan/...`; `git diff --check`; trailing-whitespace check; CodeGraph sync

Actual changes:
- `internal/plugins/omenfan/plugin.go`
- `internal/plugins/omenfan/plugin_test.go`

Main-agent review:
- E1b3c stayed inside the two allowed Go files.
- `Start(ctx)` now runs detect-only first, launches daemon only when the detect result is `supported:true` and `mock:true`, and passes exactly `--daemon --mock`.
- Supported non-mock detect results are deliberately gated with `omen fan non-mock daemon launch is disabled`, leaving real hardware daemon launch unreachable in this slice.
- The daemon process is dependency-injected for tests; default process adapter uses `exec.CommandContext` only with caller-supplied args from the gated mock path.
- Stdout is scanned into `handleDriverLine`; stderr and daemon stdout/target write errors cache `LastError`.
- The target writer consumes the existing non-blocking queue and serializes commands through `encodeSetFanCommand`.
- `Stop()` is idempotent and cancels the process context, closes stdin, clears process state, and sets `running=false`.
- Main-agent review found one lifecycle issue: daemon self-exit did not cancel the run context, which could leave the writer goroutine waiting forever. Volta fixed it by having `waitForDaemon` capture the matching cancel/stdin, clear state, cancel the run context, and close stdin; a regression test now covers stale writer behavior after self-exit.
- No CoreApp registration, C# edit, frontend edit, installer/package change, WMI call, real hardware write, `0x2E`, `0x1A`, `SetFanLevel`, or `SetFanMode` was added.

Main-agent validation:
- Passed `gofmt -l internal/plugins/omenfan/plugin.go internal/plugins/omenfan/plugin_test.go`.
- Passed targeted lifecycle tests: `TestStartDetectSupportedMockLaunchesDaemonAndMarksRunning`, `TestMockDaemonOutputUpdatesFanStatusAndCachesErrors`, `TestMockDaemonTargetWriterEncodesCommandsWithoutBlockingSetFanTargets`, `TestStopIdempotentTearsDownDaemonAndKeepsDetectedSupport`, `TestStartDetectSupportedNonMockReturnsSafeErrorWithoutLaunching`, and `TestDaemonSelfExitCancelsRunContextAndStopsWriter`.
- Passed `go test ./internal/plugins/omenfan/... -count=1`.
- Passed `go build ./internal/plugins/omenfan/...`.
- Passed `go vet ./internal/plugins/omenfan/...`.
- Passed boundary search: no `WMI`, `0x2E`, `0x1A`, `SetFanLevel`, or `SetFanMode` in `internal/plugins/omenfan`.
- Passed daemon-argument boundary check: `--daemon` appears only in `daemonCommand(runCtx, driverPath, "--daemon", "--mock")` and tests asserting `--daemon --mock`.
- Passed `git diff --check -- internal/plugins/omenfan/plugin.go internal/plugins/omenfan/plugin_test.go docs/codex-handoffs/omen-plugin-harness.md`.
- Passed trailing-whitespace check on E1b3c files and handoff.
- Passed `codegraph sync`; index refreshed.

Current progress: P7 Slice E2-B released by main-agent verification. Next action is P8 separate OMEN plugin installer planning, unless the user explicitly approves a future real-hardware write gate.

## P7 Slice E2-A Task Card - CoreApp Discovery Runtime Registration

Date: 2026-07-05
Implementation subagent: Fermat (`019f30fc-b753-7093-8369-d1f19de32ed4`)
Main-agent status: released by main-agent verification

Goal:
- Register the Go `omen-fan` runtime plugin when plugin discovery sees a valid `omen-fan` manifest and the discovered executable path exists.
- Populate discovered plugin info with the executable path so runtime/status/UI layers can surface where the separate OMEN plugin is installed.
- Keep this slice registration-only: no config-driven auto-start, no frontend work, no C# driver change, no installer/package change, and no real hardware write path.

Allowed files:
- `internal/coreapp/plugin_discovery.go`
- `internal/coreapp/plugin_discovery_test.go`

Forbidden files:
- `internal/coreapp/plugins_hotkeys.go` unless the main agent writes a separate E2-B task card
- `internal/plugins/omenfan/**`
- `internal/plugins/manager.go`
- `plugins/omen-fan/src/**`
- `frontend/**`
- `themes/**`
- `exported-themes/**`
- `build.bat`
- `scripts/package_portable.ps1`
- `bridge/**`
- `Cache/**`
- `THRM-reference-git/**`
- default main installer or portable packaging paths

Implementation constraints:
- The implementation subagent must use `gpt-5.5` with `xhigh` reasoning. Do not use `gpt-5.4`, `gpt-5.4-mini`, or any downgrade.
- Keep the slice lightweight and registration-only.
- Use current CodeGraph-backed structure, especially `updatePluginDiscoverySnapshot`, `pluginInfoFromDiscovered`, `pluginpkg.DiscoveredPlugin.ExecutablePath`, and `pluginManager.Plugin`.
- Import and instantiate `internal/plugins/omenfan` only where needed, using `omenfan.New(omenfan.Options{DriverPath: plugin.ExecutablePath, Logger: a.logger})`.
- Avoid duplicate registration on repeated discovery refreshes. If an `omen-fan` runtime plugin already exists, leave replacement/path-change behavior for a later slice unless a focused test proves a safe no-op.
- Do not start the plugin during discovery. Runtime start/stop remains behind explicit enable/disable handling and E2-B/apply-config wiring if needed.
- If the executable path is empty or missing on disk, keep discovery/status safe and do not register a runtime plugin.
- Do not add WMI calls, real `0x2E`, `0x1A`, `SetFanLevel`, `SetFanMode`, plain `--daemon`, installer logic, or packaging logic.

Checklist:
- [x] `PluginInfo.ExePath` is populated from discovered executable path / Validation: focused CoreApp test or existing discovery test assertion
- [x] `omen-fan` runtime plugin registers only when discovered manifest and executable path exist / Validation: focused CoreApp test proves runtime plugin exists but is not running
- [x] Missing executable path does not register runtime plugin and remains safe / Validation: focused CoreApp test
- [x] Repeated discovery refresh does not duplicate-register `omen-fan` / Validation: focused CoreApp test or direct source review plus status count assertion
- [x] Worker validation passed / Validation: `gofmt -l internal/coreapp/plugin_discovery.go internal/coreapp/plugin_discovery_test.go`; `go test ./internal/coreapp/... -count=1`; `go test ./internal/plugins/... -count=1`; `go build ./internal/coreapp/... ./internal/plugins/...`; scoped boundary search
- [x] Main-agent verification passed / Validation: source review; CodeGraph review; `gofmt -l`; `go test ./internal/coreapp/... -count=1`; `go test ./internal/plugins/... -count=1`; `go build ./internal/coreapp/... ./internal/plugins/...`; `go vet ./internal/coreapp/... ./internal/plugins/...`; scoped forbidden-token search; `git diff --check`; trailing-whitespace check; `codegraph sync`; `codegraph status`

Actual changes:
- `internal/coreapp/plugin_discovery.go`
- `internal/coreapp/plugin_discovery_test.go`

Main-agent review:
- E2-A stayed inside the allowed CoreApp discovery files. It did not touch `internal/coreapp/plugins_hotkeys.go`, `internal/plugins/omenfan/**`, `plugins/omen-fan/src/**`, frontend, themes, default installer, portable packaging, bridge, Cache, or THRM reference files.
- `pluginInfoFromDiscovered` now preserves `DiscoveredPlugin.ExecutablePath` in `PluginInfo.ExePath`.
- Discovery registers `omenfan.New` only for a valid `omen-fan` device manifest with a non-empty existing executable path, and it skips registration when a runtime `omen-fan` plugin already exists.
- Discovery does not start the plugin. Runtime start remains behind explicit enable/disable handling or a later E2-B config-lifecycle task.
- No WMI call, real hardware write, `0x2E`, `0x1A`, `SetFanLevel`, `SetFanMode`, installer logic, packaging logic, or new daemon path was added.

Main-agent validation:
- Passed `gofmt -l internal/coreapp/plugin_discovery.go internal/coreapp/plugin_discovery_test.go`.
- Passed `go test ./internal/coreapp/... -count=1`.
- Passed `go test ./internal/plugins/... -count=1`.
- Passed `go build ./internal/coreapp/... ./internal/plugins/...`.
- Passed `go vet ./internal/coreapp/... ./internal/plugins/...`.
- Passed scoped forbidden-token search in E2-A files: no `0x2E`, `0x1A`, `SetFanLevel`, `SetFanMode`, `WMI`, or `--daemon`.
- Confirmed `--daemon` remains only in the previously released mock-only `internal/plugins/omenfan` path.
- Passed `git diff --check -- internal/coreapp/plugin_discovery.go internal/coreapp/plugin_discovery_test.go docs/codex-handoffs/omen-plugin-harness.md`; because E2 files are untracked, also passed direct trailing-whitespace search.
- Passed `codegraph sync`; `codegraph status` reports index up to date.

## P7 Slice E2-B Task Card - OMEN Config Lifecycle Wiring

Date: 2026-07-05
Implementation subagent: Kierkegaard (`019f3116-eb3d-7f63-9936-1a45d054c48e`)
Main-agent status: released by main-agent verification

Goal:
- Extend CoreApp plugin config application so the registered `omen-fan` runtime follows explicit `cfg.Omen.Enabled`.
- Start `omen-fan` only when `cfg.Omen.Enabled == true` and a runtime `omen-fan` plugin is already registered.
- Stop `omen-fan` when `cfg.Omen.Enabled == false` and a runtime `omen-fan` plugin is registered.
- Keep this slice config-lifecycle only: no discovery changes, no frontend work, no C# driver change, no installer/package change, and no real hardware write path.

Allowed files:
- `internal/coreapp/plugins_hotkeys.go`
- `internal/coreapp/plugins_hotkeys_test.go`

Forbidden files:
- `internal/coreapp/plugin_discovery.go`
- `internal/coreapp/plugin_discovery_test.go`
- `internal/coreapp/plugin_request_handlers.go`
- `internal/plugins/omenfan/**`
- `internal/plugins/manager.go`
- `plugins/omen-fan/src/**`
- `frontend/**`
- `themes/**`
- `exported-themes/**`
- `build.bat`
- `scripts/package_portable.ps1`
- `bridge/**`
- `Cache/**`
- `THRM-reference-git/**`
- default main installer or portable packaging paths

Implementation constraints:
- The implementation subagent must use `gpt-5.5`; assign reasoning effort by slice risk per the top Subagent Model And Effort Rule. Do not use `gpt-5.4`, `gpt-5.4-mini`, or any downgrade.
- Keep the slice lightweight and lifecycle-only.
- Use current CodeGraph-backed structure, especially `applyPluginConfig`, `pluginManager.Plugin`, and `omenFanPluginID`.
- Do not require Legion Fn+Q support for OMEN lifecycle application. Existing Legion behavior must remain unchanged.
- Do not register `omen-fan` here; E2-A owns discovery registration.
- Because `UpdateConfig` calls `applyPluginConfig` while holding the CoreApp mutex, OMEN start/stop work must not run a potentially slow detect/process lifecycle synchronously on that path.
- Do not start OMEN when the runtime plugin is missing.
- Do not add WMI calls, real `0x2E`, `0x1A`, `SetFanLevel`, `SetFanMode`, plain `--daemon`, installer logic, or packaging logic.

Checklist:
- [x] OMEN enabled config starts registered `omen-fan` runtime / Validation: focused CoreApp fake-plugin test
- [x] OMEN disabled config stops registered `omen-fan` runtime / Validation: focused CoreApp fake-plugin test
- [x] Missing `omen-fan` runtime is a safe no-op / Validation: focused CoreApp fake-plugin test
- [x] Existing Legion Fn+Q apply behavior remains unchanged / Validation: focused CoreApp regression test
- [x] OMEN lifecycle application does not block config update callers / Validation: focused CoreApp tests with blocking fake `Start` and `Stop`
- [x] Rapid enable -> disable converges to disabled final config / Validation: focused CoreApp test rechecks current config after blocking `Start`
- [x] Worker validation passed / Validation: `gofmt -l internal/coreapp/plugins_hotkeys.go internal/coreapp/plugins_hotkeys_test.go`; `go test ./internal/coreapp/... -count=1`; `go build ./internal/coreapp/...`; `go vet ./internal/coreapp/...`; scoped boundary search
- [x] Main-agent verification passed / Validation: source review; `gofmt -l`; `go test ./internal/coreapp/... -count=1`; `go build ./internal/coreapp/...`; `go vet ./internal/coreapp/...`; scoped forbidden-token search; `git diff --check`; trailing-whitespace check; `codegraph sync`; `codegraph status`

Actual changes:
- `internal/coreapp/plugins_hotkeys.go`
- `internal/coreapp/plugins_hotkeys_test.go`

Main-agent review:
- E2-B stayed inside the two allowed CoreApp lifecycle files. It did not touch discovery registration, `internal/plugins/omenfan/**`, C# driver files, frontend, themes, default installer, portable packaging, bridge, Cache, or THRM reference files.
- `applyPluginConfig` now separates Legion Fn+Q and OMEN lifecycle application, so OMEN no longer depends on Legion support detection.
- OMEN lifecycle work runs through `safeGo`, so `UpdateConfig` does not synchronously wait on detect/process start or stop while holding the CoreApp mutex.
- OMEN start/stop only runs when a runtime `omen-fan` plugin is already registered. Missing runtime remains a safe no-op.
- The async lifecycle rechecks current config before and after lifecycle application, so a rapid enable then disable converges back to stopped/disabled.
- Existing Legion Fn+Q behavior is covered by regression test and remains gated by `legionFnQSupported`.
- No WMI call, real hardware write, `0x2E`, `0x1A`, `SetFanLevel`, `SetFanMode`, installer logic, packaging logic, or new daemon path was added.

Main-agent validation:
- Passed `gofmt -l internal/coreapp/plugins_hotkeys.go internal/coreapp/plugins_hotkeys_test.go`.
- Passed `go test ./internal/coreapp/... -count=1`.
- Passed `go build ./internal/coreapp/...`.
- Passed `go vet ./internal/coreapp/...`.
- Passed scoped forbidden-token search in E2-B files: no `0x2E`, `0x1A`, `SetFanLevel`, `SetFanMode`, `WMI`, or `--daemon`.
- Passed `git diff --check -- internal/coreapp/plugins_hotkeys.go internal/coreapp/plugins_hotkeys_test.go docs/codex-handoffs/omen-plugin-harness.md`.
- Passed trailing-whitespace check on E2-B files and handoff.
- Passed `codegraph sync`; `codegraph status` reports index up to date.

Current progress: P7 Slice E2-B released by main-agent verification. Next action is P8 separate OMEN plugin installer planning; any real non-mock hardware write slice remains gated on explicit user approval and hardware validation.

## P8 Task Card - Separate OMEN Plugin Installer

Date: 2026-07-05
Main-agent status: passed; P8 released for local preview packaging

Goal:
- Add and verify a separate `omen-fan-setup.exe` route for the optional OMEN plugin.
- Keep OMEN plugin files out of the default FanControl main installer and portable package.
- Install only into an existing user-selected or detected FanControl root under `plugins\omen-fan`.
- Uninstall only OMEN-owned plugin files and never remove FanControl root, user configs, themes, bridge, or other plugins.
- Build the main app as local `2.5.0-preview` installer/portable artifacts without publishing.
- Keep plugin versioning independent from the main app; current OMEN plugin version is `0.1.0`.

Allowed files for P8:
- `plugins/omen-fan/installer/omen-fan-setup.nsi`
- `plugins/omen-fan/installer/README.md` if needed for a tiny build note
- `docs/codex-handoffs/omen-plugin-harness.md`
- root `../SKILL_ROUTER_MEMORY.md` for main-agent write-back

Forbidden files for P8 unless the user gives a later explicit approval:
- `build.bat`
- `scripts/package_portable.ps1`
- `build/windows/installer/project.nsi` except the later user-approved preview-version and plugin-preservation uninstaller boundary changes
- `build/windows/installer/project_strings.nsh`
- default main installer or portable packaging paths that would include OMEN plugin files
- `frontend/**`
- `themes/**`
- `exported-themes/**`
- `internal/**`
- `plugins/omen-fan/src/**` except read-only driver build/use
- `bridge/**`
- `Cache/**`
- `THRM-reference-git/**`

Implementation adaptation notes:
- `plugins/omen-fan/src/plugin.json` already declares `id: omen-fan` and `executable: omen-fan-driver.exe`.
- `dotnet build plugins/omen-fan/src/OmenFanDriver.csproj -c Release` currently outputs `plugins/omen-fan/src/bin/Release/net472/omen-fan-driver.exe`.
- Local NSIS exists at `C:\Program Files (x86)\NSIS\makensis.exe` and `C:\Program Files (x86)\NSIS\Bin\makensis.exe`.
- The standalone installer may read existing FanControl uninstall registry keys and common install directories, but P8 must not modify the main installer to create new locator keys.
- The target directory must be validated before install by checking for `FanControl.exe` or `FanControl Core.exe`; legacy executable names may be accepted only as compatibility candidates.
- Copy only OMEN plugin-owned files: `omen-fan-driver.exe`, `plugin.json`, and direct driver dependency files such as `Newtonsoft.Json.dll` if present in the driver output directory.
- Main app installer/portable must not intentionally include, create, delete, or manage `plugins\omen-fan`; existing separate plugin installs should be inherited when users reuse the same install root.
- Main app full uninstall now removes the install root only if empty, so independently installed plugin directories can remain for plugin-market style installs.

Lightweight slice plan:
- Slice A - Standalone NSIS script: one worker owns only `plugins/omen-fan/installer/omen-fan-setup.nsi` and optional `README.md`.
- Slice B - Main-agent integration/verification: main agent reviews Slice A, compiles the driver and installer, checks default-package boundaries, and records P8 release or fix tasks.
- Slice C - Fix-only worker if main-agent verification finds a scoped installer issue.

Phase checklist:
- [x] Slice A standalone installer script added / Validation: Singer ran driver build and `makensis`; Sartre later fixed parent plugin-dir cleanup
- [x] Installer copies only plugin-owned files into `plugins\omen-fan` / Validation: main-agent source review and NSIS compile
- [x] Installer target validation prevents installing into non-FanControl directories / Validation: main-agent source review and NSIS compile
- [x] Uninstaller removes only OMEN-owned files and removes plugin dir only when empty / Validation: Sartre worker fix plus main-agent source review and NSIS compile
- [x] Default main installer/portable do not include OMEN plugin / Validation: `rg` boundary search; portable ZIP entry inspection returned no `omen-fan` or `plugins/` entries
- [x] Main installer preserves independently installed plugins on full uninstall / Validation: Bacon replaced recursive root delete with non-recursive `RMDir $INSTDIR`; `rg` found no `RMDir /r $INSTDIR`
- [x] Main app preview version set to `2.5.0-preview` / Validation: `cmd /c build.bat` produced preview installer and portable ZIP
- [x] Plugin version remains independent at `0.1.0` / Validation: `plugin.json` and installer `DisplayVersion`/VERSIONINFO checked
- [x] Driver build passed / Validation: `dotnet build plugins/omen-fan/src/OmenFanDriver.csproj -c Release`
- [x] Standalone installer compile passed / Validation: `& "C:\Program Files (x86)\NSIS\makensis.exe" plugins/omen-fan/installer/omen-fan-setup.nsi`
- [x] Main-agent verification passed / Validation: see P8 main-agent verification below

Current progress: P8 released by main-agent verification. Local preview artifacts were built, but nothing was published or uploaded.

## P8 Main-Agent Verification

Date: 2026-07-05
Main-agent status: passed; P8 released for local preview artifacts

Implementation and verification subagents:
- Singer (`019f3164-548e-73f3-b619-012a4bb03aee`) added the first standalone OMEN NSIS installer.
- Banach (`019f316b-a472-75f3-bb45-7b09993513d2`) verified the standalone installer safety boundary and found parent `plugins` cleanup risk.
- Plato (`019f316b-dbf7-7ea2-a5ee-bef6c65a3322`) verified the default package routes did not include OMEN plugin files.
- Sartre (`019f316d-098f-7fa2-98bf-b550aa60b7f1`) removed parent `plugins` directory cleanup from the OMEN uninstaller.
- Archimedes (`019f3170-9e85-7890-98ab-b6e9ebd274f0`) updated `OMEN_PLUGIN_PLAN.md` with plugin-market and main-package non-management rules.
- Hegel (`019f3173-a0f0-7e82-946b-c6a82abe7c16`) set main app preview version metadata to `2.5.0-preview` while preserving numeric NSIS VERSIONINFO as `2.5.0.0`.
- Hume (`019f3178-3032-71a3-b3fb-2782f67189ba`) added plugin-specific `0.1.0` installer version metadata.
- Bacon (`019f317f-abd7-7761-9ad6-ca4f2cc301e2`) changed the main uninstaller to preserve independently installed plugin directories by using non-recursive root removal.

Main-agent validation commands:
- `dotnet build plugins/omen-fan/src/OmenFanDriver.csproj -c Release` / passed
- `cmd /c build.bat` / passed, producing `FanControl-2.5.0-preview-amd64-installer.exe` and `FanControl-2.5.0-preview-portable.zip`
- `& "C:\Program Files (x86)\NSIS\makensis.exe" plugins/omen-fan/installer/omen-fan-setup.nsi` / passed, producing `omen-fan-setup.exe`
- Portable ZIP inspection for `omen-fan|OMEN|Omen|plugins/` / no matches
- Build output inspection confirmed `omen-fan-setup.exe` exists as a separate artifact, not inside `build\output` or the portable ZIP
- Default packaging boundary searches for OMEN/plugin paths / no matches in `build.bat`, `scripts/package_portable.ps1`, `build/windows/installer/project.nsi`, or `project_strings.nsh`
- OMEN installer forbidden-token search for `RMDir /r|taskkill|schtasks|GetParent` / no matches
- `go test ./internal/coreapp/... ./internal/plugins/... ./internal/smartcontrol/... -count=1` / passed
- `go build ./internal/coreapp/... ./internal/plugins/... ./internal/smartcontrol/...` / passed
- `go vet ./internal/coreapp/... ./internal/plugins/... ./internal/smartcontrol/...` / passed
- `cd frontend; npx tsc --noEmit` / passed
- `git diff --check` / passed with only LF-to-CRLF Git warnings
- `codegraph sync` and `codegraph status` / index up to date

Produced local artifacts:
- `build\bin\FanControl-2.5.0-preview-amd64-installer.exe`
- `build\bin\FanControl-2.5.0-preview-portable.zip`
- `build\bin\omen-fan-setup.exe`

Boundary notes:
- The main app version is now `2.5.0-preview`; this is a local preview build and has not been published.
- The OMEN plugin version remains `0.1.0` and is separate from the main app version.
- The OMEN plugin remains a separate installer route and is shaped for future standalone GitHub/plugin-market release.
- Main installer and portable packages do not include OMEN plugin files and do not intentionally manage `plugins\omen-fan`.
- Main full uninstall no longer recursively deletes the install root, so independently installed plugin directories can remain.
- Real non-mock OMEN 11 hardware writes remain gated on explicit user approval and hardware validation.

## P1 Task Card

Goal:
- Add plugin manifest parsing and safe plugin directory discovery.
- Start a CoreApp plugin discovery watcher after IPC startup and stop it during CoreApp shutdown.
- Broadcast plugin discovery lifecycle events to GUI clients.
- Keep discovery state in memory for P1; do not persist discovered plugin state into user config yet.

Allowed files:
- `go.mod`
- `go.sum`
- `internal/plugins/manifest.go`
- `internal/plugins/manifest_test.go`
- `internal/plugins/discovery.go`
- `internal/plugins/discovery_test.go`
- `internal/ipc/ipc.go`
- `internal/types/types.go` for minimal shared `PluginInfo` only
- `internal/coreapp/app.go`
- `internal/coreapp/lifecycle.go`
- `internal/coreapp/plugin_discovery.go`
- `internal/coreapp/plugin_discovery_test.go` if useful
- `docs/codex-handoffs/omen-plugin-harness.md`

Forbidden files:
- `themes/`
- `exported-themes/`
- `build.bat`
- `scripts/package_portable.ps1`
- `internal/plugins/fnqpowermode/`
- existing files under `internal/device/`
- Wails generated files such as `frontend/wailsjs/go/models.ts`

Implementation adaptation notes:
- Use current repo structure from CodeGraph, not stale P1 pseudocode verbatim.
- `internal/plugins/discovery.go` must not contain the invalid `entry.IsDir(event.Name)` pseudocode.
- P1 must not call future functions such as `initOmenPlugin` or `detectPluginHardwareSupport`.
- P1 may add plugin/OMEN IPC constants, but must not add handlers or UI.
- P1 may add `types.PluginInfo` for event payloads, but must not add `OmenConfig`, `AvailablePlugins` to `AppConfig`, WMI code, or OMEN control APIs.
- If `github.com/fsnotify/fsnotify` is added, run `go mod tidy` and keep the dependency minimal.

Phase checklist:
- [x] Manifest parsing and validation implemented / Validation: `go test ./internal/plugins/...` passed
- [x] Directory scan and watcher implemented with debounce and safe path handling / Validation: `go test ./internal/plugins/...` passed, including fsnotify `plugin.json` change test
- [x] IPC event/request constants appended without renaming existing constants / Validation: `go test ./internal/ipc ./internal/coreapp` passed
- [x] CoreApp in-memory discovery snapshot integrated into startup/shutdown / Validation: `go test ./internal/coreapp` passed
- [x] Forbidden files unchanged / Validation: `git diff --name-only` showed only allowed tracked files; `git status --short` showed new files only under allowed P1 paths plus pre-existing `Cache/`
- [x] P1 full validation passed / Validation: `go build ./internal/plugins/...`; `go build ./internal/coreapp/...`; `go test ./internal/plugins/...`; `go test ./internal/coreapp`; `go vet ./internal/plugins/... ./internal/coreapp/...` passed

## P1 Subagent Delivery

Harness Step: P1 - Plugin discovery infrastructure
Implementation subagent: P1 implementation subagent
Allowed scope: `go.mod`, `go.sum`, `internal/plugins/manifest.go`, `internal/plugins/manifest_test.go`, `internal/plugins/discovery.go`, `internal/plugins/discovery_test.go`, `internal/ipc/ipc.go`, `internal/types/types.go`, `internal/coreapp/app.go`, `internal/coreapp/lifecycle.go`, `internal/coreapp/plugin_discovery.go`, `internal/coreapp/plugin_discovery_test.go`, `docs/codex-handoffs/omen-plugin-harness.md`

Phase checklist:
- [x] Manifest parsing and validation implemented / Validation: `go test ./internal/plugins/...` passed
- [x] Directory scan and fsnotify watcher with debounce implemented / Validation: `go test ./internal/plugins/...` passed
- [x] Plugin/OMEN IPC constants appended only / Validation: `go test ./internal/ipc ./internal/coreapp` passed
- [x] CoreApp in-memory discovery snapshot starts after IPC and stops during shutdown / Validation: `go test ./internal/coreapp` passed
- [x] Forbidden files untouched / Validation: `git diff --name-only` and `git status --short`

Actual changes: `go.mod`, `go.sum`, `internal/plugins/manifest.go`, `internal/plugins/manifest_test.go`, `internal/plugins/discovery.go`, `internal/plugins/discovery_test.go`, `internal/ipc/ipc.go`, `internal/types/types.go`, `internal/coreapp/app.go`, `internal/coreapp/lifecycle.go`, `internal/coreapp/plugin_discovery.go`, `internal/coreapp/plugin_discovery_test.go`, `docs/codex-handoffs/omen-plugin-harness.md`

Validation commands: `go mod tidy`; `go test ./internal/plugins/...`; `go test ./internal/ipc ./internal/coreapp`; `go build ./internal/plugins/...`; `go build ./internal/coreapp/...`; `go vet ./internal/plugins/... ./internal/coreapp/...`; `git diff --name-only`; `git status --short`

Validation result: passed for Go tests/build/vet; boundary check showed no forbidden tracked files changed.

Risks/remaining: P1 exposes discovery snapshot/event infrastructure only. No IPC handlers, UI, OMEN WMI driver, plugin enable/disable behavior, or config persistence was added. Main-agent verification is still pending.

Current progress: P1 implementation subagent validation passed; main-agent verification pending.

Next step: Main agent reviews P1 diff and reruns key validation before releasing P1.

Main-agent verification: passed after the P1 IPC request string fix and main-agent validation rerun

Context record: updated `docs/codex-handoffs/omen-plugin-harness.md`

## P1 Fix - IPC Joint Learning Request String

Date: 2026-07-04
Implementation subagent: P1 fix implementation subagent
Allowed scope: `internal/ipc/ipc.go`, `docs/codex-handoffs/omen-plugin-harness.md`

Issue: Main-agent review found the Go constant `ReqSetOmenJointLearn` had string value `SetOmenJointLearn`, while `OMEN_PLUGIN_PLAN.md` defines the IPC request string as `SetOmenJointLearning`.

Fix: Kept the Go constant name `ReqSetOmenJointLearn` unchanged and changed only its string value to `SetOmenJointLearning`.

Validation commands:
- `go test ./internal/ipc ./internal/coreapp`
- `go test ./internal/plugins/...`
- `go build ./internal/coreapp/...`
- `go vet ./internal/plugins/... ./internal/coreapp/...`
- `git diff --name-only`

Validation result: passed. `go test ./internal/ipc ./internal/coreapp`, `go test ./internal/plugins/...`, `go build ./internal/coreapp/...`, and `go vet ./internal/plugins/... ./internal/coreapp/...` all completed successfully.

Forbidden files: no forbidden files touched by this fix.

## P1 Main-Agent Verification

Date: 2026-07-04
Main-agent status: passed; P1 released

Main-agent review notes:
- CodeGraph preflight was used before broad code inspection because `.codegraph/` exists.
- `ReqSetOmenJointLearn` now keeps the Go constant name but uses the planned IPC string `SetOmenJointLearning`.
- Plugin discovery remains P1-only: manifest parsing, safe directory scan/watch, in-memory CoreApp snapshot, and IPC event/request constants.
- No OMEN WMI driver, UI, config persistence, plugin enable/disable handler, or hardware control path was added in P1.
- Event payload shape for `EventPluginsDiscovered` is currently a raw `[]types.PluginInfo` snapshot; P5 should either reuse this shape or explicitly standardize it before frontend wiring.

Main-agent validation commands:
- `rg -n "ReqSetOmenJointLearn|SetOmenJoint" internal/ipc/ipc.go ../OMEN_PLUGIN_PLAN.md` / confirmed `SetOmenJointLearning` alignment
- `git diff --check` / passed with only LF-to-CRLF Git warnings
- `go test ./internal/ipc ./internal/coreapp` / passed
- `go test ./internal/plugins/...` / passed
- `go build ./internal/plugins/...` / passed
- `go build ./internal/coreapp/...` / passed
- `go vet ./internal/plugins/... ./internal/coreapp/...` / passed
- `git diff --name-only` / allowed tracked files only
- `git status --short` / allowed P1 files plus pre-existing untracked `Cache/`

Control-path feasibility notes:
- Current plan and local references agree that OMEN control is fan-level based, not raw RPM.
- Local reference paths checked: `D:\Desktop\风扇控制便携版\Cache\omen-control-refs\OmenSuperHub\OmenHardware.cs`, `D:\Desktop\风扇控制便携版\Cache\omen-control-refs\OmenMon\Hardware\BiosCtl.cs`, `D:\Desktop\风扇控制便携版\Cache\omen-control-refs\OmenMon\Hardware\BiosData.cs`, and `D:\Desktop\风扇控制便携版\Cache\omen-control-refs\OmenHubLighter\HPShimLibrary\Hp.Omen.OmenCommonLib\OMENHsaClient.cs`.
- P7 should keep 暗影精灵 11 first, require `0x28` SwFanControl support, read `0x10` protection state, parse `0x2C` fan type/capability, read `0x2D` current levels, infer safe level bounds from `0x2F`, write with `0x2E` only through a non-blocking worker, and use `0x1A` only for mode switching/restoration.
- Main risk for P7 remains EC/OMEN Hub initialization state on newer machines, not the basic command mapping.

Next step:
- Start P2 only after writing a new Harness task card and assigning implementation to a subagent.

## P2 Task Card

Goal:
- Add the OMEN device-plugin interface surface for future fan-control workers.
- Add OMEN status/config/support types without removing or renaming existing types or JSON fields.
- Add OMEN persistent config defaults and old-config backfill so existing user configs stay compatible.
- Keep plugin discovery state in CoreApp memory for now; do not persist discovered plugin state into user config in P2.

Allowed files:
- `internal/plugins/plugin.go`
- `internal/plugins/plugin_test.go` if useful
- `internal/types/types.go`
- `internal/types/*_test.go` if useful
- `internal/config/config.go`
- `internal/config/config_test.go`
- `docs/codex-handoffs/omen-plugin-harness.md`

Forbidden files:
- `themes/`
- `exported-themes/`
- `build.bat`
- `scripts/package_portable.ps1`
- `internal/plugins/fnqpowermode/`
- existing files under `internal/device/`
- generated Wails files such as `frontend/wailsjs/go/models.ts`
- frontend application files

Implementation adaptation notes:
- P2 must adapt to the current P1 implementation and current repo structure, not paste stale plan snippets verbatim.
- `types.PluginInfo` already exists from P1; extend it additively for P2 fields such as `Supported`, `Running`, `ExePath`, and `LastError` while keeping existing P1 fields.
- Do not add `AvailablePlugins` to `types.AppConfig` in P2. P1 released discovery as an in-memory CoreApp snapshot; persisting discovered plugin state would contradict the current memory-only boundary.
- Add `OmenSupportCache` and `OmenConfig` to `internal/types/types.go`, and append `Omen` / `OmenSupport` to `AppConfig` without deleting or renaming existing fields.
- Add default OMEN config via `GetDefaultConfig`: disabled, joint learning false, fan bias 0, target temp 70, and fan curve copied from `types.GetDefaultRPMFanCurve()`. OMEN must not inherit the root percent fan curve.
- Add old-config backfill in `internal/config/config.go` so configs lacking `omen` get a safe RPM default fan curve and target temp, while configs with partial `omen` keep explicit values and backfill only missing/invalid zero fields.
- Do not manually edit Wails generated model files. P2 is Go-only.
- `plugins.DevicePlugin.SetFanTargets(cpuRPM, gpuRPM)` is a non-blocking target API for later phases; P2 only declares the interface and status shape, no hardware control.

Phase checklist:
- [x] `plugins.DevicePlugin` and `plugins.OmenFanStatus` added additively / Validation: `go test ./internal/plugins/...`
- [x] `types.PluginInfo` extended and OMEN config/support types added / Validation: `go test ./internal/types/...`
- [x] `AppConfig` defaults and old-config backfill implemented without removing existing fields / Validation: `go test ./internal/config/...`
- [x] P2 full validation passed / Validation: `go build ./internal/types/...`; `go build ./internal/plugins/...`; `go build ./internal/config/...`; `go vet ./internal/types/... ./internal/plugins/... ./internal/config/...`
- [x] Forbidden files unchanged / Validation: `git diff --name-only`, `git status --short`, and forbidden-path check

Current progress: P2 released by main-agent verification.

Next step: Start P3 only after writing a new Harness task card and assigning lightweight implementation slices to subagents.

## P2 Interrupted Broad Dispatch

Date: 2026-07-05
Implementation subagent: Gauss (`019f2e04-73fe-7a32-b904-002fbaac89d3`)

Reason:
- User clarified that subagents should not receive overly broad tasks; prefer multiple lightweight subagents with small, narrow tasks.

Subagent-reported changed files before interruption:
- `internal/plugins/plugin.go`
- `internal/types/types.go`
- `internal/config/config.go`

Subagent-reported validation:
- None run.

Main-agent immediate observation:
- The broad subagent stopped without reverting changes.
- Current changes need review and smaller follow-up slices before P2 can be considered validated.
- `internal/config/config.go` has a formatting/indentation issue in the P2 insertion area and needs a narrow config-slice subagent to clean up and validate.

Current progress: P2 interrupted; split into lightweight subagent slices before continuing.

## P2 Slice A - Plugin Device Interface

Date: 2026-07-05
Implementation subagent: Turing (`019f2e12-04b7-7df2-a1ce-4f0212e63897`)
Allowed scope: `internal/plugins/plugin.go`

Checklist:
- [x] Additive `DevicePlugin` interface added, embedding existing `Plugin` / Validation: subagent ran `go test ./internal/plugins/...`, `go build ./internal/plugins/...`, `go vet ./internal/plugins/...`
- [x] `OmenFanStatus` status snapshot type added / Validation: subagent ran `go test ./internal/plugins/...`
- [x] Main-agent verification passed / Validation: `go test ./internal/plugins/...`; `go build ./internal/plugins/...`; `go vet ./internal/plugins/...`

Actual changes: `internal/plugins/plugin.go`

Main-agent review:
- Existing `Plugin` and `Status` APIs were preserved.
- `SetFanTargets` contract is documented as non-blocking.
- No forbidden files changed in this slice.

Current progress: P2 Slice A released by main-agent verification.

## P2 Slice B - OMEN Types And Defaults

Date: 2026-07-05
Implementation subagent: Franklin (`019f2e18-83e5-7541-9872-8c89c0a153dd`)
Allowed scope: `internal/types/types.go`

Checklist:
- [x] Extended existing `PluginInfo` additively while preserving P1 fields / Validation: subagent ran `go test ./internal/types/...`
- [x] Added `DefaultOmenTargetTemp`, `OmenSupportCache`, `OmenConfig`, and OMEN config helpers / Validation: subagent ran `go build ./internal/types/...`
- [x] Added `Omen` and `OmenSupport` to `AppConfig` without `AvailablePlugins` / Validation: main-agent `rg -n "AvailablePlugins|Omen|DefaultOmenTargetTemp|PluginInfo" internal/types/types.go`
- [x] Main-agent verification passed / Validation: `go test ./internal/types/...`; `go build ./internal/types/...`; `go vet ./internal/types/...`

Actual changes: `internal/types/types.go`

Main-agent review:
- `AvailablePlugins` was not added to `AppConfig`.
- Initial OMEN helpers were additive and default invalid target temp to 70.
- Slice D later corrected the OMEN default/backfill curve from root/default percent curve to RPM default curve.
- No forbidden files changed in this slice.

Current progress: P2 Slice B released by main-agent verification.

## P2 Slice C - Config Backfill

Date: 2026-07-05
Implementation subagent: Ohm (`019f2e20-1dd0-7d23-844e-de1dcb0aa9dd`)
Allowed scope: `internal/config/config.go`, `internal/config/config_test.go`

Checklist:
- [x] Fixed interrupted broad-dispatch formatting/indentation in config load path / Validation: subagent ran `gofmt -w internal/config/config.go internal/config/config_test.go`
- [x] Added `applyMissingOmenDefaults` config backfill / Validation: subagent ran `go test ./internal/config/...`
- [x] Added old-config and partial-OMEN load tests / Validation: subagent ran `go build ./internal/config/...` and `go vet ./internal/config/...`
- [x] Main-agent verification passed before Slice D refinement / Validation: `go test ./internal/config/...`; `go build ./internal/config/...`; `go vet ./internal/config/...`

Actual changes: `internal/config/config.go`, `internal/config/config_test.go`

Main-agent review:
- Config backfill preserves explicit OMEN booleans/fan bias and target temp behavior.
- Follow-up review found one control-semantics issue: OMEN default/backfill curve must be RPM, not root percent curve. Slice D fixed that before P2 release.

Current progress: P2 Slice C accepted with Slice D refinement required before final release.

## P2 Slice D - OMEN RPM Default Curve Fix

Date: 2026-07-05
Implementation subagent: McClintock (`019f2e30-fd49-71b3-b73d-60fc593b2f37`)
Allowed scope: `internal/types/types.go`, `internal/config/config.go`, `internal/config/config_test.go`

Checklist:
- [x] Changed OMEN defaults to clone `types.GetDefaultRPMFanCurve()` / Validation: subagent ran `go test ./internal/types/...`
- [x] Changed old-config and invalid partial-OMEN backfill to use RPM default curve, not root `cfg.FanCurve` / Validation: subagent ran `go test ./internal/config/...`
- [x] Added regressions proving OMEN does not inherit the root percent curve / Validation: subagent ran `go build ./internal/types/...`, `go build ./internal/config/...`, and `go vet ./internal/types/... ./internal/config/...`
- [x] Main-agent verification passed / Validation: `go test ./internal/types/...`; `go test ./internal/config/...`; `go build ./internal/types/...`; `go build ./internal/config/...`; `go vet ./internal/types/... ./internal/config/...`

Actual changes: `internal/types/types.go`, `internal/config/config.go`, `internal/config/config_test.go`

Main-agent review:
- This fix aligns P2 with the control-path rule: OMEN UI/Core curve targets are RPM, and the later C# worker converts RPM to fan level.
- It prevents the root 0-100 percent curve from being misread later as 0-100 RPM.
- No generated Wails models or forbidden files were changed.

Current progress: P2 Slice D released by main-agent verification.

## P2 Main-Agent Verification

Date: 2026-07-05
Main-agent status: passed; P2 released

Main-agent validation commands:
- `go test ./internal/plugins/...` / passed
- `go test ./internal/types/...` / passed
- `go test ./internal/config/...` / passed
- `go test ./internal/ipc ./internal/coreapp` / passed
- `go build ./internal/types/...`; `go build ./internal/plugins/...`; `go build ./internal/config/...`; `go build ./internal/coreapp/...` / passed
- `go vet ./internal/types/... ./internal/plugins/... ./internal/config/... ./internal/coreapp/...` / passed
- `git diff --check` / passed with only LF-to-CRLF Git warnings
- `git diff --name-only`, `git status --short`, and forbidden-path check / no forbidden tracked files changed
- `codegraph sync` / passed

Control-path review:
- Local reference checks still support the plan direction: OMEN direct fan writes are fan-level writes (`0x2E`), current levels are read with `0x2D`, fan table with `0x2F`, SystemDesignData with `0x28`, fan/protection state with `0x10`, fan type/capability with `0x2C`, and `0x1A` is mode switching/restoration rather than raw RPM control.
- P2 now keeps RPM only as the Core/UI target unit and avoids percentage-curve inheritance.
- P7 remains high risk until tested on 暗影精灵 11 hardware because EC/OMEN Hub initialization may still be required before WMI writes are accepted.

Boundary notes:
- P2 is Go-only; no frontend or generated Wails models were edited.
- Default main installer/portable route remains forbidden for OMEN plugin files; P8 must remain a separate `omen-fan-setup.exe` route only.
- Current `git status --short` still includes pre-existing/untracked `Cache/`, untracked handoff docs, and P1/P2 changed files.

Next step:
- Start P3 only after a fresh context recovery pass and a new Harness task card that splits the joint optimizer into lightweight subagent slices.

## P3 Task Card

Goal:
- Add the OMEN/FlyDigi joint noise optimizer as a pure `internal/smartcontrol` module.
- Keep this phase limited to algorithm and tests; do not wire it into CoreApp monitoring yet.
- Preserve the P2 control contract: OMEN targets are RPM values at the Core/UI layer, and later P7 converts RPM to OMEN fan levels.

Allowed files:
- `internal/smartcontrol/joint_optimizer.go`
- `internal/smartcontrol/joint_optimizer_test.go`
- `docs/codex-handoffs/omen-plugin-harness.md`

Forbidden files:
- `themes/`
- `exported-themes/`
- `build.bat`
- `scripts/package_portable.ps1`
- `internal/plugins/fnqpowermode/`
- `internal/device/`
- `internal/coreapp/`
- `internal/ipc/`
- `internal/types/`
- `internal/config/`
- generated Wails files such as `frontend/wailsjs/go/models.ts`
- frontend application files

Implementation adaptation notes:
- P3 must adapt to the existing `internal/smartcontrol` package and current unit split.
- New optimizer must be pure and deterministic: no config writes, no IPC, no device calls, no WMI, no goroutines, no timers.
- `Optimize` must clamp invalid max RPM values to conservative defaults and clamp `Bias` to `[-1.0, 1.0]`.
- If either device base target is negative, skip joint optimization and pass through the connected device target while returning zero for the disconnected side.
- Outputs must never be negative and must not exceed effective max RPM values.
- For the thermal constraint in the plan, avoid reducing either connected device below about 85% of its own base target when that base target is already high; this keeps high-load safety before P4/P7 hardware integration.
- Do not add `CalculateTargetRPMForCurve` in P3; that belongs to P4 unless a later task card explicitly moves it.

Lightweight slice plan:
- Slice A - optimizer implementation: one worker owns only `internal/smartcontrol/joint_optimizer.go`.
- Slice B - optimizer tests: after Slice A is main-agent-verified, a second worker owns only `internal/smartcontrol/joint_optimizer_test.go`.
- These are serial because Slice B depends on the concrete Slice A API compiling.

Phase checklist:
- [x] Slice A optimizer implementation added / Validation: worker ran `gofmt -w internal/smartcontrol/joint_optimizer.go`, `go test ./internal/smartcontrol/...`, `go build ./internal/smartcontrol/...`, `go vet ./internal/smartcontrol/...`
- [x] Slice A main-agent verification passed / Validation: main agent reran `go test ./internal/smartcontrol/...`, `go build ./internal/smartcontrol/...`, `go vet ./internal/smartcontrol/...`
- [x] Slice B optimizer tests added / Validation: worker ran `gofmt -w internal/smartcontrol/joint_optimizer_test.go`, `go test ./internal/smartcontrol/... -run TestOptimize -v`
- [x] P3 full validation passed / Validation: `go test ./internal/smartcontrol/...`; `go test ./internal/smartcontrol/... -run TestOptimize -v`; `go build ./internal/smartcontrol/...`; `go vet ./internal/smartcontrol/...`
- [x] Forbidden files unchanged / Validation: P3 status shows only `internal/smartcontrol/joint_optimizer.go`, `internal/smartcontrol/joint_optimizer_test.go`, and this handoff; global status still contains prior P1/P2 files

Current progress: P3 released by main-agent verification.

Next step: Start P4 only after a fresh context recovery pass and a new Harness task card that splits monitoring integration into lightweight subagent slices.

## P3 Slice A - Joint Optimizer Implementation

Date: 2026-07-05
Implementation subagent: Huygens (`019f2e47-001a-72a2-835b-e93d57a12560`)
Allowed scope: `internal/smartcontrol/joint_optimizer.go`

Checklist:
- [x] Added pure `JointInput`, `JointTarget`, `Optimize`, and default max-RPM constants / Validation: subagent ran `go test ./internal/smartcontrol/...`
- [x] Added local helper logic for max fallback, disconnected pass-through, RPM/bias clamps, load splitting, output caps, and thermal guard / Validation: subagent ran `go build ./internal/smartcontrol/...`
- [x] Kept implementation isolated from config, IPC, CoreApp, devices, WMI, goroutines, timers, and mutable global state / Validation: main-agent source review
- [x] Main-agent verification passed / Validation: `go test ./internal/smartcontrol/...`; `go build ./internal/smartcontrol/...`; `go vet ./internal/smartcontrol/...`; `git status --short -- internal/smartcontrol/joint_optimizer.go`

Actual changes: `internal/smartcontrol/joint_optimizer.go`

Main-agent review:
- Slice A only added the allowed implementation file.
- The optimizer treats OMEN and FlyDigi values as RPM targets, balances normalized load, clamps bias and outputs, passes through disconnected sides, and prevents high base targets from being reduced below roughly 85%.
- No P3 integration into monitoring or hardware control was done.

Current progress: P3 Slice A released by main-agent verification.

## P3 Slice B - Joint Optimizer Tests

Date: 2026-07-05
Implementation subagent: Goodall (`019f2e53-9ba4-7fc0-acfa-0bceeceba52a`)
Allowed scope: `internal/smartcontrol/joint_optimizer_test.go`

Checklist:
- [x] Added optimizer tests for balanced bias, positive/negative bias, disconnected pass-through, invalid max fallback, output caps, and high-base thermal guard / Validation: subagent ran `go test ./internal/smartcontrol/... -run TestOptimize -v`
- [x] Corrected an initial wrong-path test file placement outside the repo and removed the empty stray directories it created / Validation: `Test-Path "D:\Desktop\风扇控制便携版\internal\smartcontrol"` and `Test-Path "D:\Desktop\风扇控制便携版\internal"` returned `False`
- [x] Main-agent verification passed / Validation: `go test ./internal/smartcontrol/... -run TestOptimize -v`; `go test ./internal/smartcontrol/...`; `go build ./internal/smartcontrol/...`; `go vet ./internal/smartcontrol/...`

Actual changes: `internal/smartcontrol/joint_optimizer_test.go`

Main-agent review:
- Tests cover all P3 task-card cases and avoid editing the implementation file.
- The temporary misplaced outside-repo file and empty directories were cleaned by the same subagent before release.
- No P3 integration into monitoring or hardware control was done.

Current progress: P3 Slice B released by main-agent verification.

## P3 Main-Agent Verification

Date: 2026-07-05
Main-agent status: passed; P3 released

Main-agent validation commands:
- `gofmt -l internal/smartcontrol/joint_optimizer.go internal/smartcontrol/joint_optimizer_test.go` / no output, formatted
- `go test ./internal/smartcontrol/... -run TestOptimize -v` / passed all 8 `TestOptimize...` cases
- `go test ./internal/smartcontrol/...` / passed
- `go build ./internal/smartcontrol/...` / passed
- `go vet ./internal/smartcontrol/...` / passed
- `codegraph sync` and `codegraph status` / index up to date
- `Test-Path "D:\Desktop\风扇控制便携版\internal\smartcontrol"` and `Test-Path "D:\Desktop\风扇控制便携版\internal"` / both `False`

Boundary notes:
- P3 product changes are limited to `internal/smartcontrol/joint_optimizer.go` and `internal/smartcontrol/joint_optimizer_test.go`.
- The handoff file was updated by the main agent.
- Global `git status --short` still contains prior P1/P2 changes and untracked `Cache/`; P3 did not touch those areas.

Next step:
- Start P4 only after a fresh context recovery pass and a new Harness task card. P4 must stay split because monitoring integration touches latency-sensitive CoreApp behavior.

## P4 Task Card

Goal:
- Add Go-side OMEN monitoring integration without implementing the OMEN hardware driver yet.
- Let CoreApp find a registered `plugins.DevicePlugin` for `omen-fan` and send target RPMs through the non-blocking `SetFanTargets` interface only.
- Add a simple RPM curve interpolation helper for OMEN's independent curve.
- Keep the monitoring loop latency-sensitive: no WMI, no C# process I/O, no sleeps, no synchronous hardware detection, and no new long-running work inside a tick.

Allowed files:
- `internal/plugins/manager.go`
- `internal/plugins/manager_test.go`
- `internal/smartcontrol/target.go`
- `internal/smartcontrol/target_test.go`
- `internal/coreapp/omen_plugin.go`
- `internal/coreapp/omen_plugin_test.go`
- `internal/coreapp/monitoring.go`
- `docs/codex-handoffs/omen-plugin-harness.md`

Forbidden files:
- `themes/`
- `exported-themes/`
- `build.bat`
- `scripts/package_portable.ps1`
- `internal/plugins/fnqpowermode/`
- existing files under `internal/device/`
- `internal/ipc/ipc.go`
- `internal/types/types.go`
- `internal/config/config.go`
- generated Wails files such as `frontend/wailsjs/go/models.ts`
- frontend application files
- C# driver or packaging files

Implementation adaptation notes:
- Use current repo structure from CodeGraph. The released P1/P2/P3 state is authoritative, not old pseudocode in `OMEN_PLUGIN_PLAN.md`.
- Because P7 has not added the actual OMEN driver, P4 must be safe when no `omen-fan` device plugin is registered: return no-op and do not log noisy per-tick errors.
- Add a read-only plugin lookup surface instead of reaching into `plugins.Manager` internals from CoreApp.
- `CalculateTargetRPMForCurve` must treat `types.FanCurvePoint.RPM` as RPM, ignore learning offsets, handle empty/short curves safely, clamp outside the curve endpoints, and interpolate without importing hardware or config packages.
- OMEN joint optimization may only use FlyDigi target values when the active control unit is RPM. If the current active device uses percent/tick units, send OMEN's own RPM curve target and leave FlyDigi target unchanged.
- `SetFanTargets` errors may be logged, but the monitoring tick must continue and FlyDigi control must remain isolated.
- Broadcasting `ipc.EventOmenFanDataUpdate` is allowed only from cached `FanStatus()` and only when a client exists; do not introduce a high-frequency blocking event path.
- Do not add IPC handlers, Wails APIs, frontend UI, C# WMI calls, installer changes, or OMEN plugin persistence in P4.

Lightweight slice plan:
- Slice A - plugin manager lookup: one worker owns only `internal/plugins/manager.go` and `internal/plugins/manager_test.go`.
- Slice B - smartcontrol curve helper: one worker owns only `internal/smartcontrol/target.go` and `internal/smartcontrol/target_test.go`.
- Slice C - CoreApp monitoring integration: after Slice A and Slice B are main-agent-verified, one worker owns `internal/coreapp/omen_plugin.go`, `internal/coreapp/omen_plugin_test.go`, and `internal/coreapp/monitoring.go`.
- Optional Slice D - read-only review: a reviewer subagent may inspect P4 after Slice C, but must not edit code.

Phase checklist:
- [x] Slice A plugin manager lookup added / Validation: worker ran `go test ./internal/plugins/...`; `go build ./internal/plugins/...`; `go vet ./internal/plugins/...`
- [x] Slice A main-agent verification passed / Validation: `go test ./internal/plugins/...`; `go build ./internal/plugins/...`; `go vet ./internal/plugins/...`; `gofmt -l internal/plugins/manager.go internal/plugins/manager_test.go`
- [x] Slice B RPM curve helper added / Validation: worker ran `go test ./internal/smartcontrol/...`; `go build ./internal/smartcontrol/...`; `go vet ./internal/smartcontrol/...`
- [x] Slice B main-agent verification passed / Validation: `go test ./internal/smartcontrol/...`; `go build ./internal/smartcontrol/...`; `go vet ./internal/smartcontrol/...`; `gofmt -l internal/smartcontrol/target.go internal/smartcontrol/target_test.go`
- [x] Slice C CoreApp OMEN monitoring integration added / Validation: worker ran `go test ./internal/coreapp/...`; `go build ./internal/coreapp/...`; `go vet ./internal/coreapp/...`
- [x] P4 full validation passed / Validation: `go test ./internal/coreapp/... -count=1`; `go test ./internal/plugins/... -count=1`; `go test ./internal/smartcontrol/... -count=1`; `go build ./internal/coreapp/... ./internal/plugins/... ./internal/smartcontrol/...`; `go vet ./internal/coreapp/... ./internal/plugins/... ./internal/smartcontrol/...`
- [x] Forbidden files unchanged for P4 / Validation: scoped boundary check passed with only P4 allowed paths plus known pre-existing P1/P2/P3/Cache paths; full global status still contains earlier phase files
- [x] CodeGraph updated / Validation: `codegraph sync`; `codegraph status`

Current progress: P4 released by main-agent verification.

Next step: Start P5 only after a fresh context recovery pass and a new Harness task card.

## P4 Slice A - Plugin Manager Lookup

Date: 2026-07-05
Implementation subagent: Kepler (`019f2e75-181e-7191-a109-b6e8386cb273`)
Allowed scope: `internal/plugins/manager.go`, `internal/plugins/manager_test.go`

Checklist:
- [x] Added `Manager.Plugin(pluginID string) Plugin` read-only lookup / Validation: worker ran `go test ./internal/plugins/...`
- [x] Added `Manager.DevicePlugin(pluginID string) DevicePlugin` with nil for missing/non-device plugins / Validation: worker ran `go build ./internal/plugins/...`
- [x] Added focused fake plugin tests proving lookup behavior does not call lifecycle methods / Validation: worker ran `go vet ./internal/plugins/...`
- [x] Main-agent verification passed / Validation: `go test ./internal/plugins/...`; `go build ./internal/plugins/...`; `go vet ./internal/plugins/...`; `gofmt -l internal/plugins/manager.go internal/plugins/manager_test.go`

Actual changes: `internal/plugins/manager.go`, `internal/plugins/manager_test.go`

Main-agent review:
- Lookup methods are read-only and do not start, stop, spawn, or time work.
- Existing lifecycle behavior is preserved.
- No forbidden files changed in this slice.

Current progress: P4 Slice A released by main-agent verification.

## P4 Slice B - SmartControl OMEN RPM Curve Helper

Date: 2026-07-05
Implementation subagent: Averroes (`019f2e75-87e8-7af0-b7a2-fc20042c6bc5`)
Allowed scope: `internal/smartcontrol/target.go`, `internal/smartcontrol/target_test.go`

Checklist:
- [x] Added pure `CalculateTargetRPMForCurve` helper for OMEN RPM curves / Validation: worker ran `go test ./internal/smartcontrol/...`
- [x] Added endpoint clamp and rounded linear interpolation behavior / Validation: worker ran `go build ./internal/smartcontrol/...`
- [x] Added tests for empty curve, endpoint clamp, midpoint rounding, and RPM-scale values / Validation: worker ran `go vet ./internal/smartcontrol/...`
- [x] Main-agent verification passed / Validation: `go test ./internal/smartcontrol/...`; `go build ./internal/smartcontrol/...`; `go vet ./internal/smartcontrol/...`; `gofmt -l internal/smartcontrol/target.go internal/smartcontrol/target_test.go`

Actual changes: `internal/smartcontrol/target.go`, `internal/smartcontrol/target_test.go`

Main-agent review:
- Helper treats `FanCurvePoint.RPM` as RPM and does not apply learning offsets or touch config/device code.
- No forbidden files changed in this slice.

Current progress: P4 Slice B released by main-agent verification.

## P4 Slice C - CoreApp OMEN Monitoring Integration

Date: 2026-07-05
Implementation subagent: Lagrange (`019f2e7d-e027-7330-afcc-6dc5874092ad`)
Allowed scope: `internal/coreapp/omen_plugin.go`, `internal/coreapp/omen_plugin_test.go`, `internal/coreapp/monitoring.go`

Checklist:
- [x] Added nil-safe `omen-fan` device-plugin lookup helper / Validation: worker ran `go test ./internal/coreapp/...`
- [x] Added testable OMEN auto-target planning and dispatch helpers / Validation: worker ran `go build ./internal/coreapp/...`
- [x] Wired monitoring tick to dispatch OMEN RPM targets through non-blocking `SetFanTargets` only / Validation: worker ran `go vet ./internal/coreapp/...`
- [x] Added early no-op gating before plugin lookup/status calls for disabled OMEN, invalid control temperature, empty curve, or zero target / Validation: worker reran `go test ./internal/coreapp/...`
- [x] Preserved FlyDigi isolation: unsupported/missing/failed OMEN dispatch does not override FlyDigi, and joint learning overrides FlyDigi only in RPM mode after successful OMEN dispatch / Validation: `omen_plugin_test.go` helper tests
- [x] Main-agent verification passed / Validation: `go test ./internal/coreapp/... -count=1`; `go build ./internal/coreapp/...`; `go vet ./internal/coreapp/...`; `gofmt -l internal/coreapp/omen_plugin.go internal/coreapp/omen_plugin_test.go internal/coreapp/monitoring.go`

Actual changes: `internal/coreapp/omen_plugin.go`, `internal/coreapp/omen_plugin_test.go`, `internal/coreapp/monitoring.go`

Main-agent review:
- `monitoring.go` adds one narrow hook after FlyDigi runtime capability clamping and before the existing FlyDigi send decision, so joint mode can adjust `targetRPM` without a second FlyDigi send in the same tick.
- `dispatchOmenAutoTarget` performs early no-op gating before any plugin lookup or status call.
- No WMI, C# driver, IPC handler, GUI API, frontend, config, types, installer, or theme files were touched by Slice C.

Current progress: P4 Slice C released by main-agent verification.

## P4 Main-Agent Verification

Date: 2026-07-05
Main-agent status: passed; P4 released

Main-agent validation commands:
- `go test ./internal/coreapp/... -count=1` / passed
- `go test ./internal/plugins/... -count=1` / passed
- `go test ./internal/smartcontrol/... -count=1` / passed
- `go build ./internal/coreapp/... ./internal/plugins/... ./internal/smartcontrol/...` / passed
- `go vet ./internal/coreapp/... ./internal/plugins/... ./internal/smartcontrol/...` / passed
- `gofmt -l internal/coreapp/omen_plugin.go internal/coreapp/omen_plugin_test.go internal/coreapp/monitoring.go internal/plugins/manager.go internal/plugins/manager_test.go internal/smartcontrol/target.go internal/smartcontrol/target_test.go` / no output, formatted
- `git diff --check` / passed with only LF-to-CRLF Git warnings
- Scoped P4 boundary check / passed: P4 paths are `internal/plugins/manager.go`, `internal/plugins/manager_test.go`, `internal/smartcontrol/target.go`, `internal/smartcontrol/target_test.go`, `internal/coreapp/monitoring.go`, `internal/coreapp/omen_plugin.go`, `internal/coreapp/omen_plugin_test.go`, and this handoff; global status still includes known pre-existing P1/P2/P3/Cache paths
- `codegraph sync` and `codegraph status` / index up to date

Boundary notes:
- P4 is Go-side integration only. It does not implement OMEN WMI, C# driver, IPC handlers, Wails API, frontend UI, or installer packaging.
- OMEN direct control remains fan-level based for later P7; P4 continues to treat RPM as Core/UI target units only.
- Default main installer/portable route remains forbidden for OMEN plugin files; P8 must remain a separate installer route.

Next step:
- Start P5 only after a fresh context recovery pass and a new Harness task card. P5 must adapt to the current split IPC/CoreApp structure and must not persist discovered plugin state into user config unless its task card explicitly changes that boundary.

## P5 Task Card

Goal:
- Add the Go-side Wails/API layer and CoreApp IPC handlers for plugin discovery and OMEN config/status requests.
- Adapt to the released P1-P4 design: plugin discovery state remains an in-memory CoreApp snapshot, and OMEN hardware writes still happen only through the P4 non-blocking device-plugin interface.
- Do not add frontend UI, generated Wails bindings, C# driver code, WMI code, or installer/package changes in P5.

Allowed files:
- `internal/coreapp/ipc.go`
- `internal/coreapp/plugin_request_handlers.go`
- `internal/coreapp/plugin_request_handlers_test.go`
- `internal/guiapp/plugin_api.go`
- `internal/guiapp/omen_api.go`
- `docs/codex-handoffs/omen-plugin-harness.md`

Forbidden files:
- `themes/`
- `exported-themes/`
- `build.bat`
- `scripts/package_portable.ps1`
- `internal/plugins/fnqpowermode/`
- existing files under `internal/device/`
- `internal/types/types.go`
- `internal/config/config.go`
- `internal/ipc/ipc.go`
- generated Wails files such as `frontend/wailsjs/go/models.ts`
- frontend application files
- C# driver or packaging files

Implementation adaptation notes:
- Use current repo structure from CodeGraph, not stale P5 pseudocode verbatim.
- CoreApp IPC currently dispatches through a route array in `internal/coreapp/ipc.go`; P5 should add `handlePluginIPCRequest` in a new file and wire that route into the array.
- Do not add `AvailablePlugins` to `types.AppConfig` and do not read plugin discovery state from config. Use `availablePluginsSnapshot()` from P1.
- Plugin enable/disable in P5 may persist `cfg.Omen.Enabled` for `omen-fan` and call `pluginManager.Start/Stop` only if a runtime plugin is registered. Missing plugin should return a clear error for enable/disable rather than silently claiming success.
- `RefreshPluginDiscovery` may rescan installed plugin manifests and update the in-memory snapshot; it must not write discovered plugins into config.
- OMEN config requests should update only `cfg.Omen` fields, save config, and broadcast `ipc.EventConfigUpdate`. `SetOmenFanCurve` must not call hardware or `SetFanTargets`.
- `SetOmenFanBias` must clamp bias to `[-1.0, 1.0]`; GUI API may accept an integer `-100..100` and convert to float.
- `SetOmenJointLearning` must use the released IPC request string `SetOmenJointLearning` through `ipc.ReqSetOmenJointLearn`.
- `GetOmenFanStatus` should return cached `plugins.OmenFanStatus{}` when no OMEN device plugin is registered; it must not do hardware detection.
- Do not manually edit Wails generated files. Wails generation belongs to a later explicit build/generate step.

Lightweight slice plan:
- Slice A - CoreApp plugin/OMEN IPC handlers: one worker owns only `internal/coreapp/ipc.go`, `internal/coreapp/plugin_request_handlers.go`, and `internal/coreapp/plugin_request_handlers_test.go`.
- Slice B - GUI plugin management API: after Slice A is main-agent-verified, one worker owns only `internal/guiapp/plugin_api.go`.
- Slice C - GUI OMEN API: after Slice A is main-agent-verified, one worker owns only `internal/guiapp/omen_api.go`.
- Slices B and C can run in parallel after Slice A because their write scopes are disjoint.

Phase checklist:
- [x] Slice A CoreApp IPC handlers added / Validation: worker ran `gofmt -l internal/coreapp/ipc.go internal/coreapp/plugin_request_handlers.go internal/coreapp/plugin_request_handlers_test.go`; `go test ./internal/coreapp/...`; `go build ./internal/coreapp/...`; `go vet ./internal/coreapp/...`
- [x] Slice A main-agent verification passed / Validation: `gofmt -l internal/coreapp/ipc.go internal/coreapp/plugin_request_handlers.go internal/coreapp/plugin_request_handlers_test.go`; `go test ./internal/coreapp/... -count=1`; `go build ./internal/coreapp/...`; `go vet ./internal/coreapp/...`; `git diff --check -- internal/coreapp/ipc.go internal/coreapp/plugin_request_handlers.go internal/coreapp/plugin_request_handlers_test.go`
- [x] Slice B GUI plugin API added / Validation: worker ran `gofmt -l internal/guiapp/plugin_api.go`; `go build ./internal/guiapp/...`; `go vet ./internal/guiapp/...`
- [x] Slice C GUI OMEN API added / Validation: worker ran `gofmt -l internal/guiapp/omen_api.go`; `go build ./internal/guiapp/...`; `go vet ./internal/guiapp/...`
- [x] P5 full validation passed / Validation: `gofmt -l internal/coreapp/ipc.go internal/coreapp/plugin_request_handlers.go internal/coreapp/plugin_request_handlers_test.go internal/guiapp/plugin_api.go internal/guiapp/omen_api.go`; `go test ./internal/coreapp/... -count=1`; `go build ./internal/coreapp/... ./internal/guiapp/...`; `go vet ./internal/coreapp/... ./internal/guiapp/...`
- [x] Forbidden files unchanged for P5 / Validation: scoped status/diff check showed P5 changes only in `internal/coreapp/ipc.go`, `internal/coreapp/plugin_request_handlers.go`, `internal/coreapp/plugin_request_handlers_test.go`, `internal/guiapp/plugin_api.go`, `internal/guiapp/omen_api.go`, and this handoff; known P1/P2 dirty files remain unrelated
- [x] CodeGraph updated / Validation: `codegraph sync`; `codegraph status`

Current progress: P5 released by main-agent verification.

Next step: Start P6 only after a fresh context recovery pass and a new Harness task card. P6 must remain frontend-only plus later explicit Wails binding generation if that step is approved by the task card.

## P5 Slice A - CoreApp Plugin/OMEN IPC Handlers

Date: 2026-07-05
Implementation subagent: Hypatia (`019f2ea2-9357-72d2-ac1b-66264b5c7392`)
Allowed scope: `internal/coreapp/ipc.go`, `internal/coreapp/plugin_request_handlers.go`, `internal/coreapp/plugin_request_handlers_test.go`

Checklist:
- [x] Wired `handlePluginIPCRequest` into the current CoreApp IPC route array / Validation: worker ran `go test ./internal/coreapp/...`
- [x] Added plugin discovery/status/enable/disable handlers using in-memory `availablePluginsSnapshot()` rather than config persistence / Validation: focused `plugin_request_handlers_test.go` coverage
- [x] Added OMEN config/status handlers without hardware calls; `SetOmenFanCurve` accepts raw array plus `{fanCurve}` / `{curve}` payloads and does not call `SetFanTargets` / Validation: focused `plugin_request_handlers_test.go` coverage
- [x] Added clear missing-runtime errors for enable/disable and support only for `omen-fan` in P5 / Validation: focused `plugin_request_handlers_test.go` coverage
- [x] Main-agent verification passed / Validation: `gofmt -l internal/coreapp/ipc.go internal/coreapp/plugin_request_handlers.go internal/coreapp/plugin_request_handlers_test.go`; `go test ./internal/coreapp/... -count=1`; `go build ./internal/coreapp/...`; `go vet ./internal/coreapp/...`; `git diff --check -- internal/coreapp/ipc.go internal/coreapp/plugin_request_handlers.go internal/coreapp/plugin_request_handlers_test.go`

Actual changes: `internal/coreapp/ipc.go`, `internal/coreapp/plugin_request_handlers.go`, `internal/coreapp/plugin_request_handlers_test.go`

Main-agent review:
- Slice A stayed inside CoreApp allowed files and did not touch GUI, frontend, generated Wails bindings, config/type schema, IPC constants, installer, themes, devices, or C# driver code.
- `ReqSetOmenFanCurve` is protocol-compatible with planned GUI usage by accepting a raw `[]types.FanCurvePoint` payload while preserving object payload compatibility.
- OMEN config requests only save config and broadcast `ipc.EventConfigUpdate`; no hardware detection or OMEN driver call was introduced.

Current progress: P5 Slice A released by main-agent verification.

## P5 Slice B - GUI Plugin Management API

Date: 2026-07-05
Implementation subagent: Godel (`019f2ec4-615b-79f3-8d56-e0489746e2a8`)
Allowed scope: `internal/guiapp/plugin_api.go`

Checklist:
- [x] Added `GetAvailablePlugins`, `GetPluginStatus`, `EnablePlugin`, `DisablePlugin`, and `RefreshPluginDiscovery` Wails methods / Validation: worker ran `go build ./internal/guiapp/...`
- [x] Used existing `sendRequest` IPC style and existing plugin IPC constants / Validation: main-agent source review
- [x] Returned nil list fallbacks for list getters and errors for status/enable/disable failures / Validation: main-agent source review
- [x] Main-agent verification passed / Validation: `go build ./internal/guiapp/...`; `go vet ./internal/guiapp/...`; `gofmt -l internal/guiapp/plugin_api.go`

Actual changes: `internal/guiapp/plugin_api.go`

Main-agent review:
- Slice B stayed inside its single allowed file and did not touch OMEN API, CoreApp, IPC constants, config/types, frontend, generated Wails bindings, installer, themes, devices, or C# driver code.
- Plugin ID requests use `map[string]string{"id": pluginID}`, which is compatible with the Slice A CoreApp parser.

Current progress: P5 Slice B released by main-agent verification.

## P5 Slice C - GUI OMEN API

Date: 2026-07-05
Implementation subagent: Newton (`019f2ec4-dd38-7402-967b-727dd81aa561`)
Allowed scope: `internal/guiapp/omen_api.go`

Checklist:
- [x] Added `GetOmenFanStatus`, `GetOmenFanCurve`, `SetOmenFanCurve`, `SetOmenFanBias`, and `SetOmenJointLearning` Wails methods / Validation: worker ran `go build ./internal/guiapp/...`
- [x] Sent `SetOmenFanCurve` as raw `[]types.FanCurvePoint`, matching the Slice A compatibility fix / Validation: main-agent source review
- [x] Clamped GUI bias integer to `[-100,100]` and converted to float `[-1.0,1.0]` before IPC / Validation: main-agent source review
- [x] Main-agent verification passed / Validation: `go build ./internal/guiapp/...`; `go vet ./internal/guiapp/...`; `gofmt -l internal/guiapp/omen_api.go`

Actual changes: `internal/guiapp/omen_api.go`

Main-agent review:
- Slice C stayed inside its single allowed file and did not touch plugin API, CoreApp, IPC constants, config/types, frontend, generated Wails bindings, installer, themes, devices, or C# driver code.
- OMEN getters return zero/nil fallbacks on IPC/decode failures; setters propagate errors, matching existing GUI API style.

Current progress: P5 Slice C released by main-agent verification.

## P5 Main-Agent Verification

Date: 2026-07-05
Main-agent status: passed; P5 released

Main-agent validation commands:
- `gofmt -l internal/coreapp/ipc.go internal/coreapp/plugin_request_handlers.go internal/coreapp/plugin_request_handlers_test.go internal/guiapp/plugin_api.go internal/guiapp/omen_api.go` / no output, formatted
- `go test ./internal/coreapp/... -count=1` / passed
- `go build ./internal/coreapp/... ./internal/guiapp/...` / passed
- `go vet ./internal/coreapp/... ./internal/guiapp/...` / passed
- `git diff --check -- internal/coreapp/ipc.go internal/coreapp/plugin_request_handlers.go internal/coreapp/plugin_request_handlers_test.go internal/guiapp/plugin_api.go internal/guiapp/omen_api.go docs/codex-handoffs/omen-plugin-harness.md` / passed with only LF-to-CRLF Git warning on `internal/coreapp/ipc.go`
- Scoped P5 boundary check / passed: P5 paths are `internal/coreapp/ipc.go`, `internal/coreapp/plugin_request_handlers.go`, `internal/coreapp/plugin_request_handlers_test.go`, `internal/guiapp/plugin_api.go`, `internal/guiapp/omen_api.go`, and this handoff; global status still includes known earlier P1/P2/P3/P4 files and `Cache/`
- `codegraph sync` and `codegraph status` / index up to date

Boundary notes:
- P5 is Go-side IPC/Wails API only. It does not implement frontend UI, generated Wails bindings, C# OMEN driver, WMI control, or installer packaging.
- Plugin discovery remains an in-memory CoreApp snapshot; no `AvailablePlugins` config persistence was added.
- Default main installer/portable route remains forbidden for OMEN plugin files; P8 is still separate installer only.

Next step:
- Start P6 only after a fresh context recovery pass and a new Harness task card. P6 should not start from stale context and should preserve the lightweight multi-subagent rule.
