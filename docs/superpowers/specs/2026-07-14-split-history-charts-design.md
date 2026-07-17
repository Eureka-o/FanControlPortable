# Split History Charts Design

## Goal

Separate power history from temperature and fan history while preserving the existing history data, display preferences, visual tokens, and summary cards.

## Layout

- Keep the existing history chart card and series controls.
- The upper chart renders CPU temperature, GPU temperature, and fan speed at the current height.
- When CPU power or GPU power is enabled and has data, render a divider, a Power Trend heading, and a shorter lower chart.
- When both power series are disabled, remove the lower chart, heading, and divider.
- Give both charts the same time domain, margins, left axis width, and right axis width so their plot boundaries align.

## Interaction

- Reuse one tooltip renderer in both charts.
- Hovering either chart shows the timestamp and every currently enabled series with a valid value, including values drawn in the other chart.
- Series visibility continues to use `useHistoryDisplayPreferences`; no preference schema or storage key changes.
- If only one power series remains enabled, the lower chart stays visible and renders that series only.

## Implementation

- Keep both charts in `FanCurve.tsx` and reuse the existing Recharts dependency, chart data, colors, axes, and formatting helpers.
- Build tooltip rows from the hovered data point instead of the chart-local Recharts payload.
- Reserve the unused opposite axis width in the lower chart so the shared X-axis plot area stays aligned.
- Do not add shared hover state or `syncId`; the requested behavior only requires each chart's tooltip to read the full point.

## Verification

- Add one focused frontend regression check covering the split charts, shared full-point tooltip, aligned axis widths, and conditional power chart.
- Run frontend tests, TypeScript checking, and the production build.
- Compare the rendered history detail against the supplied THRM screenshot in light mode and verify the no-power state.

## Out Of Scope

- Backend history format, sampling, retention, and summary calculations.
- Cross-chart cursor synchronization or zooming.
- Changes to the connection workflow, themes, or chart color tokens.
