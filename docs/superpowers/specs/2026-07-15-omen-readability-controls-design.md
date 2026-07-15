# OMEN Readability And Direct Controls Design

## Goal

Reduce overview density, remove duplicated controls, and make performance values easier to scan and adjust without changing plugin protocol or hardware behavior.

## Overview Page

- Keep the realtime CPU/GPU and OMEN summary unchanged.
- Make the performance-mode card the dominant overview control with more vertical space, a clearer current-mode badge, and larger four-way mode buttons.
- Display the highest mode as `大师`, not `大师模式`.
- Remove the CPU tuning summary and custom-curve summary from the overview. Their dedicated tabs already own those workflows.
- Keep only four compact direct controls in a balanced two-column grid:
  - GPU Dynamic Boost toggle.
  - Screen overdrive toggle.
  - Charge protection 80% / 100% segmented buttons.
  - GPU / MUX mode select with the existing restart confirmation.
- Do not use overview buttons that only navigate to another tab.

## Performance Page

- Reuse the host `ui.Slider` and `ui.NumberInput`; add no new control component.
- Each numeric power or temperature limit uses one labeled slider with a compact numeric input beside it. Both edit the same existing draft value and keep the current explicit Apply action.
- CPU power and CPU strategy remain the first row. GPU power spans the full grid width below them to eliminate the uneven empty column.
- Keep boost policy, Windows performance bias, Dynamic Boost, capability gates, pending states, backend readback, and validation ranges unchanged.

## Device Page

- Remove GPU / MUX mode, screen overdrive, and charge protection because the overview now owns them directly.
- Keep OMEN-key behavior and the full-width diagnostics row.
- Hide the section entirely when neither remaining capability is available.

## Responsive And Visual Rules

- Desktop: two equal overview control columns and a two-column performance grid.
- Narrow screens: all controls stack to one column without horizontal scrolling.
- Increase secondary-text contrast and line height; keep the existing host colors, Lucide icons, radii, and card primitives.
- Preserve fixed control heights and visible keyboard focus states.

## Verification

- Extend the OMEN frontend contract test first, confirming the old navigation summaries and duplicate device controls are gone and `ui.Slider` is used for numeric tuning.
- Run the focused plugin test, full frontend tests, TypeScript, production frontend build, and plugin packaging.
- Check overview and performance pages at desktop and narrow widths in both light and dark themes.
