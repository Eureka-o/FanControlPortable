(function () {
  "use strict";

  const ns = window.FanControlOmenFanPlugin = window.FanControlOmenFanPlugin || {};

    const CSS = `
      .omen-page {
        --omen-line: color-mix(in srgb, var(--border) 78%, transparent);
        color: var(--foreground);
        display: grid;
        gap: 12px;
        letter-spacing: 0;
        min-width: 0;
      }
      .omen-page, .omen-page * { box-sizing: border-box; }
      .omen-page button, .omen-page input, .omen-page select { font-family: inherit; letter-spacing: 0; }
      @keyframes omen-tab-panel-in {
        from { opacity: 0; transform: translate3d(0, 6px, 0); }
        to { opacity: 1; transform: translate3d(0, 0, 0); }
      }
      @keyframes omen-card-rise-in {
        from { opacity: 0; transform: translate3d(0, 8px, 0); }
        to { opacity: 1; transform: translate3d(0, 0, 0); }
      }
      .omen-tab-panel {
        animation: omen-tab-panel-in .24s ease both;
        display: grid;
        gap: 12px;
        min-width: 0;
      }
      .omen-tab-panel > * {
        animation: omen-card-rise-in .26s ease both;
      }
      .omen-tab-panel > :nth-child(2) { animation-delay: 18ms; }
      .omen-tab-panel > :nth-child(3) { animation-delay: 36ms; }
      .omen-tab-panel > :nth-child(4) { animation-delay: 54ms; }
      .omen-tab-panel > :nth-child(5) { animation-delay: 72ms; }
      .omen-card-button {
        appearance: none;
        background: transparent;
        border: 0;
        color: inherit;
        cursor: pointer;
        display: block;
        font: inherit;
        padding: 0;
        text-align: left;
        width: 100%;
      }
      .omen-card-button:focus-visible {
        border-radius: 12px;
        outline: 2px solid var(--ring);
        outline-offset: 2px;
      }
      .omen-topbar {
        align-items: center;
        display: grid;
        gap: 12px;
        grid-template-columns: minmax(0, 1fr) auto;
        padding: 2px 2px 0;
      }
      .omen-title-line, .omen-toolbar, .omen-pill-row, .omen-inline, .omen-chip-row,
      .omen-section-head, .omen-curve-title, .omen-curve-toolbar, .omen-legend {
        align-items: center;
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
        min-width: 0;
      }
      .omen-title-line { gap: 10px; }
      .omen-toolbar, .omen-chip-row, .omen-curve-toolbar { justify-content: flex-end; }
      .omen-title {
        font-size: 18px;
        font-weight: 700;
        line-height: 1.2;
        margin: 0;
      }
      .omen-subtle, .omen-page p {
        color: var(--muted-foreground);
        font-size: 12px;
        line-height: 1.45;
        margin: 0;
      }
      .omen-diamond-mark {
        aspect-ratio: 1;
        background: linear-gradient(90deg, #ff4dff 0%, #ff004f 34%, #ff4200 66%, #ffc400 100%);
        clip-path: polygon(50% 0, 100% 50%, 50% 100%, 0 50%);
        display: inline-block;
        height: 22px;
        width: 22px;
      }
      .omen-mark, .omen-hero-mark, .omen-icon, .omen-card-icon {
        align-items: center;
        background: color-mix(in srgb, var(--primary) 10%, transparent);
        border-radius: 12px;
        color: var(--primary);
        display: inline-flex;
        flex: 0 0 auto;
        justify-content: center;
      }
      .omen-mark, .omen-icon, .omen-card-icon { height: 36px; width: 36px; }
      .omen-hero-mark { height: 56px; width: 56px; }
      .omen-hero-mark .omen-diamond-mark { height: 28px; width: 28px; }
      .omen-hero {
        align-items: center;
        display: flex;
        flex-wrap: wrap;
        gap: 12px;
        justify-content: space-between;
        min-width: 0;
      }
      .omen-hero h2, .omen-section-title, .omen-card-title {
        font-size: 15px;
        font-weight: 700;
        line-height: 1.25;
        margin: 0;
      }
      .omen-grid {
        display: grid;
        gap: 12px;
        min-width: 0;
      }
      .omen-grid.metrics, .omen-grid.tiles, .omen-grid.modes {
        grid-template-columns: repeat(4, minmax(0, 1fr));
      }
      .omen-grid.system {
        grid-template-columns: repeat(3, minmax(0, 1fr));
      }
      .omen-grid.two {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
      .omen-grid.curves { grid-template-columns: minmax(0, 1fr); }
      .omen-section-head, .omen-curve-title {
        align-items: flex-start;
        justify-content: space-between;
        margin-bottom: 14px;
      }
      .omen-stat-value {
        font-size: 15px;
        font-variant-numeric: tabular-nums;
        font-weight: 700;
        line-height: 1.3;
        min-width: 0;
      }
      .omen-stat-detail {
        color: var(--muted-foreground);
        font-size: 11px;
        line-height: 1.35;
        margin-top: 6px;
        min-width: 0;
      }
      .omen-metric-card {
        min-height: 92px;
      }
      .omen-metric-head {
        align-items: center;
        color: var(--muted-foreground);
        display: flex;
        font-size: 13px;
        font-weight: 650;
        gap: 8px;
        min-width: 0;
      }
      .omen-metric-value {
        color: var(--foreground);
        font-size: 20px;
        font-variant-numeric: tabular-nums;
        font-weight: 760;
        line-height: 1.2;
        margin-top: 12px;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .omen-metric-detail {
        color: var(--muted-foreground);
        font-size: 12px;
        line-height: 1.35;
        margin-top: 6px;
        min-width: 0;
      }
      .omen-mode-card {
        min-height: 132px;
        position: relative;
      }
      .omen-mode-card.active {
        border-color: color-mix(in srgb, var(--primary) 50%, var(--border));
        background: color-mix(in srgb, var(--primary) 9%, var(--background));
      }
      .omen-mode-check {
        align-items: center;
        background: color-mix(in srgb, var(--primary) 14%, transparent);
        border-radius: 999px;
        color: var(--primary);
        display: inline-flex;
        height: 28px;
        justify-content: center;
        position: absolute;
        right: 14px;
        top: 14px;
        width: 28px;
      }
      .omen-mode-title {
        color: var(--foreground);
        font-size: 17px;
        font-weight: 760;
        line-height: 1.2;
        margin-top: 18px;
      }
      .omen-mode-detail {
        color: var(--muted-foreground);
        font-size: 13px;
        line-height: 1.35;
        margin-top: 8px;
      }
      .omen-system-card {
        min-height: 158px;
      }
      .omen-system-title {
        align-items: center;
        color: var(--muted-foreground);
        display: flex;
        font-size: 13px;
        font-weight: 650;
        gap: 8px;
        min-width: 0;
      }
      .omen-system-value {
        color: var(--foreground);
        font-size: 15px;
        font-weight: 720;
        line-height: 1.35;
        margin-top: 12px;
      }
      .omen-system-note {
        color: var(--muted-foreground);
        font-size: 12px;
        line-height: 1.35;
        margin-top: 8px;
      }
      .omen-system-actions {
        align-items: center;
        display: grid;
        gap: 8px;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        margin-top: 16px;
      }
      .omen-system-actions.single {
        display: flex;
        justify-content: flex-start;
      }
      .omen-system-button {
        appearance: none;
        background: color-mix(in srgb, var(--background) 72%, transparent);
        border: 1px solid var(--omen-line);
        border-radius: 11px;
        color: var(--muted-foreground);
        cursor: pointer;
        font: inherit;
        font-size: 13px;
        font-weight: 700;
        min-height: 36px;
        padding: 7px 12px;
      }
      .omen-system-button.active {
        background: color-mix(in srgb, var(--primary) 12%, transparent);
        border-color: color-mix(in srgb, var(--primary) 60%, var(--border));
        color: var(--primary);
      }
      .omen-control-tile {
        min-height: 82px;
      }
      .omen-control-tile.clickable {
        cursor: pointer;
      }
      .omen-control-tile.active {
        border-color: color-mix(in srgb, var(--primary) 40%, var(--border));
        background: color-mix(in srgb, var(--primary) 9%, var(--background));
      }
      .omen-control-label {
        align-items: center;
        color: var(--muted-foreground);
        display: flex;
        font-size: 12px;
        font-weight: 600;
        gap: 6px;
        min-width: 0;
      }
      .omen-control-value {
        color: var(--foreground);
        font-size: 15px;
        font-variant-numeric: tabular-nums;
        font-weight: 700;
        line-height: 1.3;
        margin-top: 8px;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .omen-control-detail {
        color: var(--muted-foreground);
        font-size: 11px;
        line-height: 1.35;
        margin-top: 4px;
        min-width: 0;
      }
      .omen-alert {
        align-items: flex-start;
        border: 1px solid color-mix(in srgb, #eab308 34%, var(--border));
        border-radius: 12px;
        background: color-mix(in srgb, #eab308 10%, transparent);
        color: var(--foreground);
        display: flex;
        gap: 10px;
        padding: 12px 14px;
      }
      .omen-alert.info {
        border-color: color-mix(in srgb, var(--primary) 30%, var(--border));
        background: color-mix(in srgb, var(--primary) 8%, transparent);
      }
      .omen-legend {
        color: var(--muted-foreground);
        font-size: 11px;
        font-weight: 600;
        gap: 14px;
        margin-bottom: 10px;
      }
      .omen-legend-line {
        background: var(--chart-primary, var(--primary));
        display: inline-flex;
        height: 2px;
        margin-right: 6px;
        vertical-align: middle;
        width: 28px;
      }
      .omen-legend-line.dashed {
        background: repeating-linear-gradient(90deg, var(--chart-primary, var(--primary)), var(--chart-primary, var(--primary)) 5px, transparent 5px, transparent 10px);
      }
      .omen-chart-host {
        min-width: 0;
      }
      .omen-chart-height {
        height: 360px;
        min-height: 360px;
      }
      .omen-pop-card {
        animation: omen-card-rise-in .24s ease both;
      }
      .omen-pop-grid > :nth-child(2) {
        animation-delay: 36ms;
      }
      .omen-toast {
        align-items: center;
        background: var(--theme-toast-background, var(--card));
        border: 1px solid color-mix(in srgb, #22c55e 35%, var(--border));
        border-radius: calc(var(--radius) + 2px);
        box-shadow: var(--theme-toast-shadow, 0 18px 44px rgba(0, 0, 0, .18));
        display: flex;
        font-size: 13px;
        font-weight: 650;
        gap: 8px;
        max-width: 420px;
        padding: 12px 14px;
        position: fixed;
        right: 28px;
        top: 24px;
        z-index: 20;
      }
      .omen-native-button {
        align-items: center;
        border: 1px solid var(--omen-line);
        border-radius: 10px;
        background: var(--secondary);
        color: var(--secondary-foreground);
        cursor: pointer;
        display: inline-flex;
        font-size: 13px;
        font-weight: 600;
        gap: 7px;
        justify-content: center;
        min-height: 34px;
        padding: 7px 12px;
        white-space: nowrap;
      }
      .omen-native-button.primary {
        background: var(--primary);
        border-color: var(--primary);
        color: var(--primary-foreground);
      }
      .omen-native-button:disabled { cursor: not-allowed; opacity: .55; }
      .omen-pill, .omen-segment, .omen-tab {
        align-items: center;
        border: 1px solid var(--omen-line);
        border-radius: 999px;
        color: var(--muted-foreground);
        display: inline-flex;
        font-size: 11px;
        font-weight: 600;
        gap: 5px;
        min-height: 24px;
        padding: 3px 8px;
        white-space: nowrap;
      }
      .omen-segment, .omen-tab {
        border-radius: 9px;
        cursor: pointer;
        font-size: 13px;
        min-height: 32px;
        padding: 6px 11px;
      }
      .omen-segment.active, .omen-tab.active, .omen-pill.info {
        background: color-mix(in srgb, var(--primary) 10%, transparent);
        border-color: color-mix(in srgb, var(--primary) 34%, var(--border));
        color: var(--primary);
      }
      .omen-pill.success { color: #16a34a; }
      .omen-pill.warning { color: #a16207; }
      .omen-tab-list {
        align-items: center;
        display: flex;
        gap: 4px;
      }
      .omen-slider-fallback {
        accent-color: var(--primary);
        cursor: pointer;
        width: 100%;
      }
      .omen-select-fallback {
        background: var(--background);
        border: 1px solid var(--omen-line);
        border-radius: 10px;
        color: var(--foreground);
        min-height: 36px;
        padding: 6px 10px;
        width: 100%;
      }
      @media (min-width: 1180px) {
        .omen-chart-height {
          height: 420px;
          min-height: 420px;
        }
      }
      @media (max-width: 1180px) {
        .omen-topbar { grid-template-columns: 1fr; }
        .omen-toolbar { justify-content: flex-start; }
      }
      @media (max-width: 900px) {
        .omen-grid.metrics, .omen-grid.tiles, .omen-grid.modes, .omen-grid.system {
          grid-template-columns: repeat(2, minmax(0, 1fr));
        }
      }
      @media (max-width: 760px) {
        .omen-toolbar { align-items: stretch; flex-direction: column; }
        .omen-toolbar > *, .omen-tab-list { width: 100%; }
        .omen-tab { flex: 1; justify-content: center; }
  	      .omen-grid.metrics, .omen-grid.tiles, .omen-grid.modes, .omen-grid.system, .omen-grid.two { grid-template-columns: 1fr; }
  	      .omen-section-head, .omen-curve-title { flex-direction: column; }
  	      .omen-chip-row, .omen-curve-toolbar { justify-content: flex-start; }
  	    }
      @media (prefers-reduced-motion: reduce) {
        .omen-tab-panel, .omen-tab-panel > *, .omen-pop-card {
          animation: none;
        }
        .omen-page *, .omen-page *::before, .omen-page *::after {
          transition-duration: .01ms !important;
        }
      }
    `;

    function installStyle() {
      if (document.getElementById("omen-fan-plugin-style")) return;
      const style = document.createElement("style");
      style.id = "omen-fan-plugin-style";
      style.textContent = CSS;
      document.head.appendChild(style);
    }

  Object.assign(ns, { installStyle });
})();
