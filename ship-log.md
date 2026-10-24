# Ship Log

Artifact: Windows desktop app - FanControl
Version: 2.7.0
Deploy target: GitHub Releases (`Eureka-o/FanControlPortable`)
Started: 2026-07-31T00:00:00+08:00

## Phase 1 - Verify Completeness

- [x] Release source and required build files present
- [x] Intended release scope reviewed against the dirty worktree
- [x] Unrelated experiments and local caches excluded

## Phase 2 - Test Suite

- [x] `go test ./... -count=1`: all packages passed
- [x] `go vet ./...`: passed
- [x] Frontend tests: 97 passed, 0 failed
- [x] TypeScript `npx tsc --noEmit`: passed

## Phase 3 - Security Scan

- [x] `npm audit --omit=dev`: 0 vulnerabilities
- [x] Final release diff credential scan: no credential-like additions

## Phase 4 - Documentation

- [x] README updated for `2.7.0`
- [x] Bilingual release notes added for `v2.7.0`

## Phase 5 - Version

- [x] User selected `2.7.0`
- [x] Wails/NSIS metadata: `2.7.0`
- [x] Windows numeric resources: `2.7.0.0`

## Phase 6 - Build

- [x] `cmd /c build.bat`: exit 0
- [x] Local installer SHA-256: `07E71288B222B5085B963DCA2111D8F5C0E8935D8042EF58E85A430CB0157E97`
- [x] Local portable SHA-256: `B2DDD78775B9A29B8E15323863CB0621ACA0858438E79608279BA0FB75262C43`
- [x] Portable package contains app, core, temperature bridge, and 9 built-in themes

## Phase 7 - Confirmation

- [x] User explicitly authorized direct publication on 2026-07-31

## Phase 8 - Deploy

- [x] Commit and push `main`: `798aa69487055eee1b5c9583deece27504ab71ab`
- [x] Create and push annotated tag `v2.7.0`
- [x] GitHub Actions run `30571333731`: success
- [x] Published formal GitHub Release: https://github.com/Eureka-o/FanControlPortable/releases/tag/v2.7.0
- [x] Release body matches `docs/release-notes/fancontrol-2.7.0.md`
- [x] CI installer SHA-256: `8E8832C1AF7C0FA124BA8609F4DFC928CC800C4C5B0A6440A26C7A4459EA4C46`
- [x] CI portable SHA-256: `A2E5ACB4FA47FD7983EC952F4C0FEFD40C15BF60BFD5F4C70747AEA541C3C4BE`
