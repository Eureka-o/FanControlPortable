# OMEN Realtime Overview Layout

## Goal

Replace the OMEN plugin's six independent metric tiles with the same two-column realtime overview structure used at the top of the main settings page.

## Approved Layout

- The overview has a shared section heading and two panels.
- The left panel contains one CPU row and one GPU row.
- Each hardware row shows the hardware label, optional model, temperature, power, and fan speed on aligned baselines.
- The right panel shows the OMEN icon, device name, connection state, current performance mode, and joint-learning state.
- The four plugin page tabs remain directly below the overview.
- Charge protection, GPU/MUX, screen overdrive, and other controls remain in their existing quick-control or device sections.

## Shared Component

Extract the settings page's current inline overview markup into a generic host UI component exported from `frontend/src/app/components/ui/index.tsx`.

The component accepts:

- A section title and title icon.
- A list of hardware rows with icon, label, optional model, and metric values.
- A device summary with icon, name, connection state, and detail values.

The main settings page keeps its existing data mapping. The OMEN IIFE consumes the same component through `plugin.ui` and supplies OMEN status data. This remains an additive Host API v1 surface and does not require another React runtime or plugin dependency.

## Responsive Behavior

- Desktop uses the existing weighted two-column layout.
- Narrow layouts stack the hardware and device panels.
- Hardware metrics wrap without horizontal scrolling while retaining aligned labels and tabular numbers.
- Long CPU, GPU, and device names wrap or truncate only inside explicit model/name containers.
- Light mode, dark mode, focus states, and reduced-motion behavior remain controlled by the host design system.

## State And Safety

- The component is presentational and performs no hardware actions.
- Values continue to come from backend readback status.
- Missing telemetry renders as `--` without changing capability visibility.
- OMEN capabilities and commands remain plugin-owned.

## Verification

- Add a focused test for the shared component export and both consumers.
- Keep the OMEN capability/readback contract tests passing.
- Run the frontend test suite, TypeScript check, and production build.
- Verify settings and OMEN overview layouts in light, dark, desktop, and 390 px browser views with no horizontal overflow or console warnings.
