# Noise Benefit Learning And Diagnostic Guidance Design

## Goal

Use a connected device's saved noise diagnostic to improve quiet-learning value without weakening thermal safety, and make both noise dialogs clear about duration, automatic speed changes, cancellation, and result ownership.

## Backend Design

- Keep `NoiseDiagnosticsByDevice` as the only persisted noise source. Results never cross device keys or speed units.
- Derive a bounded gain from the local measured noise slope divided by the diagnostic's overall average slope.
- Apply the gain only when long-term steady learning is already allowed and the calculated learning step is negative.
- Require at least four valid points, medium or high confidence, matching units, a positive speed span, at least 2 dB measured rise, and `NoiseWeight > 0`.
- Leave over-temperature learning, prediction, hardware limiting, manual/custom speed, active diagnostic leases, and axis-noise-adjusted samples unchanged.
- Keep axis-noise avoidance separate. Microphone peaks remain diagnostic information and never create avoidance zones automatically.

## Frontend Design

- Reuse the current dialogs, cards, controls, icons, and theme variables.
- Show the planned base-point count and an approximate duration derived from the selected range.
- Explain that the fan changes speed automatically, the user should keep airflow and microphone conditions stable, cancellation discards only the current run, and normal control is restored.
- Tell users that a reliable noise result guides later quiet learning; low-confidence data remains visible but is ignored by learning.
- For axis-noise scanning, state that the user must rate every stable point and that reported points may trigger confirmation and fine rescans, extending the duration.
- During active runs, expose a concise status message through `aria-live` and keep stop-and-discard available.
- Preserve the existing narrow-window one-column action layout.

## Validation

- Go tests prove steep local noise slopes increase only negative learning, low-confidence or mismatched results are neutral, and positive safety learning is unchanged.
- Frontend tests prove duration estimation, reminder copy wiring, live status, and responsive action layout.
- Run targeted tests first, then `go test ./...`, frontend typecheck/build, and `git diff --check`.

