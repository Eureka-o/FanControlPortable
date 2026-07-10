# OMEN Plugin Injection Handoff - 2026-07-07

## Direction

- Main app owns only generic plugin hooks and tab rendering.
- OMEN UI/backend payload stays under `plugins/omen-fan`.
- Installed plugin manifests `frontend: "ui/omen-fan.plugin.js"` and registers a React page through `window.FanControlPluginHost.registerPage`.
- Removing the plugin removes the OMEN page from discovery; no built-in OMEN page exists under `frontend/src/app`.

## Implemented

- Added generic frontend host: `frontend/src/app/plugins/plugin-host.tsx`.
- Restored `frontend/src/app/components/PluginPage.tsx` as a JS-injection loader.
- Added generic backend asset reader: `internal/guiapp/plugin_api.go` `GetPluginFrontendAsset`, preserving `GetPluginFrontendHTML`.
- Added optional plugin `icon` metadata through manifest/types and sidebar rendering.
- Rewrote `plugins/omen-fan/ui/omen-fan.plugin.js` as a React injected page with overview, CPU/GPU curve, and settings tabs.
- OMEN plugin manifest now points to `ui/omen-fan.plugin.js`, version remains `0.1.0`.
- OMEN installer installs/uninstalls `ui/omen-fan.plugin.js` and keeps plugin-owned cleanup only.
- Backed up recovered old built-in page under `docs/codex-handoffs/recovered-omen/`.

## Validation

- `node --check plugins/omen-fan/ui/omen-fan.plugin.js` passed.
- `cd frontend; npx tsc --noEmit` passed after Wails rebuild.
- `cd frontend; npm run build` passed.
- `go test ./internal/plugins/... ./internal/coreapp/... ./internal/guiapp/... ./internal/types -count=1` passed.
- `go build` and `go vet` for the same Go package set passed.
- `dotnet build plugins/omen-fan/src/OmenFanDriver.csproj -c Release` passed.
- `cmd /c build.bat` rebuilt `2.5.0-preview` main installer and portable zip.
- NSIS rebuilt `build/bin/omen-fan-setup.exe`.
- Portable zip inspection found no `plugins/` or OMEN payload entries.
- Local preview payload copied to `build/bin/plugins/omen-fan`; `--detect-only --mock` passed.
- `git diff --check` reported only existing LF-to-CRLF warnings.
- `codegraph sync; codegraph status` passed.

## Preview

- Launch: `build/bin/FanControl.exe`
- Optional mock backend: `build/bin/plugins/omen-fan/omen-fan-driver.exe --mock-http --port 8787`
- Plugin installer: `build/bin/omen-fan-setup.exe`

## Remaining

- Real OMEN hardware writes are still gated and not enabled.
- The injected UI is a first React plugin version matching the previous three-page shape; further screenshot polish should happen inside `plugins/omen-fan/ui/omen-fan.plugin.js`.
