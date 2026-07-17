# Bundled Theme Seed Implementation Plan

**Goal:** Ship every official theme in the installer and portable package, create an install-root `themes/` folder for management, and never overwrite an existing theme directory.

**Architecture:** `build.bat` stages source themes under `build/bin/themes`. NSIS temporarily extracts that payload and runs `migrate-themes.ps1`, which copies only missing theme IDs into `$INSTDIR/themes`; existing official or user-created directories are untouched. Portable packages keep the complete `themes/` directory. The four small embedded themes remain an executable fallback.

## Completed Work

- [x] Preserve existing official themes regardless of bundled version.
- [x] Preserve user-created theme IDs and files.
- [x] Copy missing official themes into the install-root `themes/` directory.
- [x] Keep legacy user-profile theme migration compatibility.
- [x] Add `migrate-themes_test.ps1` covering existing official, custom, and missing official themes.
- [x] Keep all eight official themes in the portable ZIP.
- [x] Run Go tests, Go vet, TypeScript checks, frontend build, and `build.bat`.

## Boundary

- Do not update or merge files inside any existing install-root theme directory.
- Do not remove the install-root `themes/` directory from installer or portable packages.
- Do not expand the executable fallback set unless a separate memory/performance decision is made.
