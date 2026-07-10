(function () {
  "use strict";

  const PLUGIN_ID = "omen-fan";
  const MOCK_BASE_URL = "http://127.0.0.1:8787";
  const STORAGE_KEY = "fancontrol.omen-fan.preview.v3";
  const RPM_MIN = 800;
  const RPM_MAX = 6000;
  const RPM_STEP = 100;
  const TEMP_MIN = 20;
  const CPU_TEMP_MAX = 105;
  const GPU_TEMP_MAX = 90;

  const CPU_CURVE = curveFromPairs([
    [20, 1000], [25, 1000], [30, 1000], [35, 1200], [40, 1400], [45, 1600],
    [50, 1800], [55, 2000], [60, 2300], [65, 2600], [70, 2900], [75, 3200],
    [80, 3500], [85, 3800], [90, 4000], [95, 4200], [100, 4400], [105, 4600],
  ]);

  const GPU_CURVE = curveFromPairs([
    [20, 1000], [25, 1000], [30, 1000], [35, 1200], [40, 1400], [45, 1600],
    [50, 1800], [55, 2000], [60, 2300], [65, 2600], [70, 2900], [75, 3200],
    [80, 3500], [85, 3800], [90, 4000],
  ]);

  const QUIET_CPU_CURVE = curveFromPairs([
    [20, 900], [25, 900], [30, 900], [35, 1000], [40, 1100], [45, 1200],
    [50, 1400], [55, 1600], [60, 1800], [65, 2100], [70, 2500], [75, 2900],
    [80, 3400], [85, 3900], [90, 4300], [95, 4700], [100, 5200], [105, 5600],
  ]);

  const QUIET_GPU_CURVE = curveFromPairs([
    [20, 900], [25, 900], [30, 900], [35, 1000], [40, 1100], [45, 1300],
    [50, 1500], [55, 1800], [60, 2100], [65, 2500], [70, 3000], [75, 3500],
    [80, 4100], [85, 4700], [90, 5200],
  ]);

  const COOL_CPU_CURVE = curveFromPairs([
    [20, 1200], [25, 1200], [30, 1300], [35, 1500], [40, 1700], [45, 2000],
    [50, 2300], [55, 2700], [60, 3100], [65, 3500], [70, 3900], [75, 4300],
    [80, 4700], [85, 5100], [90, 5400], [95, 5700], [100, 6000], [105, 6000],
  ]);

  const COOL_GPU_CURVE = curveFromPairs([
    [20, 1200], [25, 1200], [30, 1300], [35, 1500], [40, 1700], [45, 2000],
    [50, 2300], [55, 2700], [60, 3100], [65, 3500], [70, 4000], [75, 4500],
    [80, 5000], [85, 5500], [90, 6000],
  ]);

  const MODE_CARDS = [
    { id: "balanced", title: "均衡", detail: "平衡性能与噪声，适合日常使用。", icon: "Gauge", cpu: 65 },
    { id: "performance", title: "性能", detail: "更高 CPU 功耗限制，优先性能释放。", icon: "Zap", cpu: 95 },
    { id: "quiet", title: "安静", detail: "降低响应强度，优先控制噪声。", icon: "Fan", cpu: 35 },
    { id: "custom", title: "大师", detail: "启用自定义 CPU/GPU 风扇曲线。", icon: "SlidersHorizontal", cpu: 95 },
  ];

  const TABS = [
    ["overview", "概览"],
    ["curve", "风扇曲线"],
    ["settings", "设置"],
  ];

  const CSS = `
    .omen-page {
      --omen-line: color-mix(in srgb, var(--border) 76%, transparent);
      --omen-soft: color-mix(in srgb, var(--muted) 34%, transparent);
      --omen-primary-soft: color-mix(in srgb, var(--primary) 11%, transparent);
      --omen-primary-line: color-mix(in srgb, var(--primary) 34%, transparent);
      color: var(--foreground);
      display: grid;
      gap: 12px;
      letter-spacing: 0;
      min-width: 0;
    }
    .omen-page, .omen-page * { box-sizing: border-box; }
    .omen-page button, .omen-page input, .omen-page select { font-family: inherit; letter-spacing: 0; }
    .omen-topbar {
      align-items: center;
      display: grid;
      gap: 12px;
      grid-template-columns: minmax(260px, 1fr) auto;
      padding: 2px 2px 0;
    }
    .omen-title-line, .omen-toolbar, .omen-tab-list, .omen-pill-row, .omen-inline {
      align-items: center;
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      min-width: 0;
    }
    .omen-title-line { gap: 10px; }
    .omen-mark, .omen-hero-icon, .omen-card-icon {
      align-items: center;
      background: var(--omen-primary-soft);
      border: 1px solid var(--omen-primary-line);
      border-radius: 10px;
      color: var(--primary);
      display: inline-flex;
      flex: 0 0 auto;
      justify-content: center;
    }
    .omen-mark { height: 32px; width: 32px; }
    .omen-hero-icon { height: 52px; width: 52px; }
    .omen-card-icon { height: 36px; width: 36px; }
    .omen-diamond-mark {
      aspect-ratio: 1;
      background: linear-gradient(90deg, #ff4dff 0%, #ff004f 34%, #ff4200 66%, #ffc400 100%);
      clip-path: polygon(50% 0, 100% 50%, 50% 100%, 0 50%);
      display: inline-block;
      height: 18px;
      width: 18px;
    }
    .omen-hero-icon .omen-diamond-mark { height: 28px; width: 28px; }
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
    .omen-pill {
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
    .omen-pill.success { background: color-mix(in srgb, #22c55e 11%, transparent); border-color: color-mix(in srgb, #22c55e 36%, transparent); color: #16a34a; }
    .omen-pill.warning { background: color-mix(in srgb, #eab308 12%, transparent); border-color: color-mix(in srgb, #eab308 36%, transparent); color: #a16207; }
    .omen-pill.info { background: var(--omen-primary-soft); border-color: var(--omen-primary-line); color: var(--primary); }
    .omen-toolbar { justify-content: flex-end; }
    .omen-tab-list {
      background: color-mix(in srgb, var(--muted) 34%, transparent);
      border: 1px solid var(--omen-line);
      border-radius: 12px;
      flex-wrap: nowrap;
      padding: 3px;
    }
    .omen-tab, .omen-chip, .omen-segment {
      align-items: center;
      background: transparent;
      border: 1px solid transparent;
      border-radius: 9px;
      color: var(--muted-foreground);
      cursor: pointer;
      display: inline-flex;
      font-size: 13px;
      font-weight: 600;
      gap: 7px;
      justify-content: center;
      min-height: 32px;
      padding: 6px 11px;
      transition: background .18s ease, border-color .18s ease, color .18s ease, box-shadow .18s ease;
      white-space: nowrap;
    }
    .omen-tab.active, .omen-chip.active, .omen-segment.active {
      background: var(--omen-primary-soft);
      border-color: var(--omen-primary-line);
      color: var(--primary);
      box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary) 8%, transparent);
    }
    .omen-native-button {
      align-items: center;
      border: 1px solid var(--omen-line);
      border-radius: 10px;
      background: color-mix(in srgb, var(--card) 88%, var(--muted));
      color: var(--foreground);
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
    .omen-hero {
      align-items: center;
      display: grid;
      gap: 14px;
      grid-template-columns: auto minmax(0, 1fr) auto;
      min-width: 0;
      padding: 16px;
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
    .omen-grid.metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .omen-grid.modes { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .omen-grid.system { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .omen-grid.curves { grid-template-columns: minmax(0, 1fr); }
    .omen-grid.two { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .omen-metric {
      align-items: flex-start;
      display: flex;
      flex-direction: column;
      min-height: 112px;
      padding: 14px;
    }
    .omen-metric-label {
      align-items: center;
      color: var(--muted-foreground);
      display: flex;
      font-size: 12px;
      font-weight: 600;
      gap: 7px;
    }
    .omen-metric-value {
      font-size: 28px;
      font-variant-numeric: tabular-nums;
      font-weight: 760;
      line-height: 1.05;
      margin-top: 12px;
    }
    .omen-metric-detail {
      color: var(--muted-foreground);
      font-size: 12px;
      line-height: 1.4;
      margin-top: auto;
      padding-top: 10px;
    }
    .omen-section {
      min-width: 0;
      padding: 16px;
    }
    .omen-section-head {
      align-items: flex-start;
      display: flex;
      gap: 12px;
      justify-content: space-between;
      margin-bottom: 14px;
      min-width: 0;
    }
    .omen-chip-row {
      align-items: center;
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      justify-content: flex-end;
    }
    .omen-chip, .omen-segment {
      border-color: var(--omen-line);
      background: color-mix(in srgb, var(--background) 72%, transparent);
      min-width: 76px;
    }
    .omen-stat-card {
      border: 1px solid var(--omen-line);
      border-radius: 12px;
      background: color-mix(in srgb, var(--background) 48%, transparent);
      cursor: pointer;
      min-height: 112px;
      padding: 14px;
      position: relative;
      transition: background .18s ease, border-color .18s ease, transform .18s ease;
    }
    .omen-stat-card:hover {
      border-color: var(--theme-card-hover-border, var(--omen-primary-line));
      transform: translateY(-1px);
    }
    .omen-stat-card.active {
      background: var(--omen-primary-soft);
      border-color: var(--omen-primary-line);
    }
    .omen-card-check {
      align-items: center;
      background: var(--omen-primary-soft);
      border-radius: 999px;
      color: var(--primary);
      display: inline-flex;
      height: 24px;
      justify-content: center;
      position: absolute;
      right: 14px;
      top: 14px;
      width: 24px;
    }
    .omen-mode-title {
      font-size: 16px;
      font-weight: 750;
      margin-top: 14px;
    }
    .omen-system-card {
      cursor: default;
      min-height: 118px;
    }
    .omen-alert {
      align-items: flex-start;
      border: 1px solid var(--omen-primary-line);
      border-radius: 12px;
      background: var(--omen-primary-soft);
      color: var(--foreground);
      display: flex;
      gap: 10px;
      padding: 12px 14px;
    }
    .omen-alert.warning {
      background: color-mix(in srgb, #eab308 10%, transparent);
      border-color: color-mix(in srgb, #eab308 34%, transparent);
    }
    .omen-curve-toolbar {
      align-items: center;
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      justify-content: flex-end;
    }
    .omen-curve-card {
      padding: 16px;
    }
    .omen-curve-title {
      align-items: flex-start;
      display: flex;
      gap: 12px;
      justify-content: space-between;
      margin-bottom: 12px;
      min-width: 0;
    }
    .omen-legend {
      align-items: center;
      color: var(--muted-foreground);
      display: flex;
      flex-wrap: wrap;
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
    .omen-chart-host [data-theme-card="curve-editor"] > .relative.rounded-3xl {
      border-radius: 18px;
    }
    .omen-toast {
      align-items: center;
      background: var(--theme-card-background, var(--card));
      border: 1px solid color-mix(in srgb, #22c55e 35%, var(--border));
      border-radius: 12px;
      box-shadow: var(--theme-card-shadow, 0 18px 44px rgba(0, 0, 0, .18));
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
    .omen-settings-section { padding: 0; }
    .omen-settings-header {
      align-items: center;
      border-bottom: 1px solid var(--omen-line);
      display: flex;
      gap: 10px;
      padding: 14px 16px;
    }
    .omen-setting-row {
      align-items: center;
      border-top: 1px solid var(--omen-line);
      display: grid;
      gap: 14px;
      grid-template-columns: minmax(0, 1fr) minmax(220px, 360px);
      padding: 14px 16px;
    }
    .omen-setting-row:first-of-type { border-top: 0; }
    .omen-setting-copy {
      align-items: flex-start;
      display: flex;
      gap: 12px;
      min-width: 0;
    }
    .omen-setting-control {
      display: flex;
      justify-content: flex-end;
      min-width: 0;
    }
    .omen-subcard {
      border: 1px solid var(--omen-line);
      border-radius: 12px;
      background: color-mix(in srgb, var(--background) 42%, transparent);
      margin: 0 16px 16px;
      padding: 14px;
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
    @media (min-width: 1700px) {
      .omen-grid.metrics { grid-template-columns: repeat(4, minmax(0, 1fr)); }
      .omen-grid.modes { grid-template-columns: repeat(4, minmax(0, 1fr)); }
      .omen-grid.system { grid-template-columns: repeat(3, minmax(0, 1fr)); }
    }
    @media (max-width: 1180px) {
      .omen-topbar { grid-template-columns: 1fr; }
      .omen-toolbar { justify-content: flex-start; }
    }
    @media (max-width: 760px) {
      .omen-toolbar { align-items: stretch; flex-direction: column; }
      .omen-toolbar > *, .omen-tab-list { width: 100%; }
      .omen-tab { flex: 1; }
      .omen-hero, .omen-setting-row { grid-template-columns: 1fr; }
      .omen-grid.metrics, .omen-grid.modes, .omen-grid.system, .omen-grid.two { grid-template-columns: 1fr; }
      .omen-section-head, .omen-curve-title { flex-direction: column; }
      .omen-chip-row, .omen-curve-toolbar, .omen-setting-control { justify-content: flex-start; }
    }
  `;

  function curveFromPairs(pairs) {
    return pairs.map(([temperature, rpm]) => ({ temperature, rpm }));
  }

  function cloneCurve(curve) {
    return curve.map((point) => ({ temperature: point.temperature, rpm: point.rpm }));
  }

  function createDefaultSettings() {
    return {
      tab: "overview",
      mode: "custom",
      cpuPowerLimit: 95,
      gpuBoost: true,
      gpuMode: "hybrid",
      screenOverdrive: false,
      batteryCap: 100,
      coolerConnected: false,
      responseSpeed: 5,
      jointLearning: false,
      learningBias: "quiet",
      cpuTarget: 70,
      gpuTarget: 70,
      quietCpuLimit: 95,
      quietGpuLimit: 85,
      risePrediction: false,
      debugMode: false,
      cpuCurve: cloneCurve(CPU_CURVE),
      gpuCurve: cloneCurve(GPU_CURVE),
    };
  }

  function readStoredSettings() {
    try {
      const raw = window.localStorage.getItem(STORAGE_KEY)
        || window.localStorage.getItem("fancontrol.omen-fan.preview.v2")
        || window.localStorage.getItem("fancontrol.omen-fan.preview.v1");
      if (!raw) return createDefaultSettings();
      const parsed = JSON.parse(raw);
      return {
        ...createDefaultSettings(),
        ...parsed,
        cpuCurve: normalizeCurve(parsed.cpuCurve, CPU_CURVE, CPU_TEMP_MAX),
        gpuCurve: normalizeCurve(parsed.gpuCurve, GPU_CURVE, GPU_TEMP_MAX),
      };
    } catch {
      return createDefaultSettings();
    }
  }

  function writeStoredSettings(settings) {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
    } catch {
      /* ignore storage failures in restricted WebView contexts */
    }
  }

  function clamp(value, min, max) {
    const number = Number(value);
    if (!Number.isFinite(number)) return min;
    return Math.min(max, Math.max(min, number));
  }

  function roundRpm(value) {
    return Math.round(clamp(value, RPM_MIN, RPM_MAX) / RPM_STEP) * RPM_STEP;
  }

  function normalizeCurve(curve, fallback, maxTemp) {
    const source = Array.isArray(curve) && curve.length ? curve : fallback;
    const byTemp = new Map();
    for (let temp = TEMP_MIN; temp <= maxTemp; temp += 5) {
      const found = source.find((point) => Number(point.temperature) === temp);
      byTemp.set(temp, roundRpm(found ? found.rpm : rpmAtTemp(source, temp)));
    }
    return Array.from(byTemp, ([temperature, rpm]) => ({ temperature, rpm }));
  }

  function rpmAtTemp(curve, temp) {
    const points = [...curve].sort((left, right) => left.temperature - right.temperature);
    if (!points.length) return RPM_MIN;
    if (temp <= points[0].temperature) return points[0].rpm;
    if (temp >= points[points.length - 1].temperature) return points[points.length - 1].rpm;
    for (let index = 1; index < points.length; index += 1) {
      const right = points[index];
      const left = points[index - 1];
      if (temp <= right.temperature) {
        const span = right.temperature - left.temperature || 1;
        const ratio = (temp - left.temperature) / span;
        return roundRpm(left.rpm + (right.rpm - left.rpm) * ratio);
      }
    }
    return roundRpm(points[points.length - 1].rpm);
  }

  function learnedCurveFrom(curve, bias) {
    const direction = bias === "quiet" ? -1 : 1;
    return curve.map((point, index) => ({
      ...point,
      rpm: roundRpm(point.rpm + direction * (index > curve.length * 0.55 ? 180 : 80)),
    }));
  }

  function formatRpm(value) {
    const number = Number(value);
    return Number.isFinite(number) && number > 0 ? `${Math.round(number).toLocaleString()} RPM` : "0 RPM";
  }

  function formatTemp(value) {
    const number = Number(value);
    return Number.isFinite(number) && number > 0 ? `${Math.round(number)}°C` : "--°C";
  }

  function formatWatts(value) {
    const number = Number(value);
    return Number.isFinite(number) && number > 0 ? `${Math.round(number)}W` : "--";
  }

  function getStatusValue(status, key, fallback) {
    const value = status && status[key];
    return value === undefined || value === null ? fallback : value;
  }

  function installStyle() {
    if (document.getElementById("omen-fan-plugin-style")) return;
    const style = document.createElement("style");
    style.id = "omen-fan-plugin-style";
    style.textContent = CSS;
    document.head.appendChild(style);
  }

  function usePersistentSettings(React) {
    const [settings, setSettings] = React.useState(readStoredSettings);
    const update = React.useCallback((patch) => {
      setSettings((current) => {
        const next = typeof patch === "function" ? patch(current) : { ...current, ...patch };
        writeStoredSettings(next);
        return next;
      });
    }, []);
    return [settings, update];
  }

  async function fetchMockStatus(signal) {
    const response = await fetch(`${MOCK_BASE_URL}/status`, { cache: "no-store", signal });
    if (!response.ok) throw new Error(`${response.status} ${response.statusText}`);
    const payload = await response.json();
    return payload && payload.status ? payload.status : payload;
  }

  async function postMock(path, body) {
    const response = await fetch(`${MOCK_BASE_URL}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body || {}),
    });
    if (!response.ok) throw new Error(`${response.status} ${response.statusText}`);
    const payload = await response.json();
    return payload && payload.status ? payload.status : payload;
  }

  function registerWithHost(host) {
    if (!host || typeof host.registerPage !== "function") {
      window.setTimeout(() => registerWithHost(window.FanControlPluginHost), 60);
      return;
    }

    const React = host.React;
    const components = host.components || {};
    const icons = host.icons || {};
    const e = React.createElement;
    const HostButton = components.Button;
    const HostToggleSwitch = components.ToggleSwitch;
    const HostSelect = components.Select;
    const HostSlider = components.Slider;
    const FanCurveEditor = components.FanCurveEditor;
    let toastTimer = 0;

    function Icon({ name, size = 18, className = "" }) {
      const Component = icons[name] || icons.Circle;
      return Component ? e(Component, { size, className }) : null;
    }

    function OmenMark() {
      return e("span", { className: "omen-diamond-mark", "aria-hidden": "true" });
    }

    function StatusPill({ children, tone = "muted", icon }) {
      return e("span", { className: `omen-pill ${tone}` }, icon || null, children);
    }

    function Button({ children, onClick, primary, disabled, icon }) {
      if (HostButton) {
        return e(HostButton, {
          variant: primary ? "primary" : "secondary",
          size: "sm",
          onClick,
          disabled,
          icon,
        }, children);
      }
      return e("button", {
        type: "button",
        className: `omen-native-button${primary ? " primary" : ""}`,
        onClick,
        disabled,
      }, icon ? [icon, children] : children);
    }

    function Toggle({ checked, onChange, label }) {
      if (HostToggleSwitch) {
        return e(HostToggleSwitch, { enabled: checked, onChange, label, size: "sm", color: "blue" });
      }
      return e("button", {
        type: "button",
        className: `omen-segment${checked ? " active" : ""}`,
        "aria-pressed": checked,
        onClick: () => onChange(!checked),
      }, label || (checked ? "开" : "关"));
    }

    function SelectControl({ value, onChange, options }) {
      if (HostSelect) {
        return e(HostSelect, { value, onChange, options, size: "sm", className: "w-full" });
      }
      return e("select", {
        className: "omen-select-fallback",
        value,
        onChange: (event) => onChange(event.target.value),
      }, options.map((option) => e("option", { key: option.value, value: option.value }, option.label)));
    }

    function SliderControl({ title, value, min, max, suffix, onChange }) {
      if (HostSlider) {
        return e(HostSlider, {
          value,
          min,
          max,
          step: 1,
          label: title,
          valueFormatter: (next) => `${next}${suffix || ""}`,
          onChange,
        });
      }
      return e("label", { className: "omen-subtle" },
        e("div", { className: "omen-inline", style: { justifyContent: "space-between", marginBottom: 8 } },
          e("span", null, title),
          e("strong", { style: { color: "var(--primary)" } }, `${value}${suffix || ""}`),
        ),
        e("input", {
          className: "omen-slider-fallback",
          type: "range",
          min,
          max,
          step: 1,
          value,
          onChange: (event) => onChange(Number(event.target.value)),
        }),
      );
    }

    function OmenFanPage({ plugin }) {
      installStyle();
      const [settings, updateSettings] = usePersistentSettings(React);
      const [status, setStatus] = React.useState(null);
      const [mockConnected, setMockConnected] = React.useState(false);
      const [loading, setLoading] = React.useState(false);
      const [toastText, setToastText] = React.useState("");
      const temperature = host.useAppStore((store) => store.temperature);

      const cpuTemp = Number(getStatusValue(status, "cpuTemp", temperature && temperature.cpuTemp)) || 70;
      const gpuTemp = Number(getStatusValue(status, "gpuTemp", temperature && temperature.gpuTemp)) || 55;
      const cpuPower = Number(getStatusValue(status, "cpuPowerWatts", temperature && temperature.cpuPowerWatts)) || 0;
      const gpuPower = Number(getStatusValue(status, "gpuPowerWatts", temperature && temperature.gpuPowerWatts)) || 0;
      const cpuTarget = roundRpm(rpmAtTemp(settings.cpuCurve, cpuTemp));
      const gpuTarget = roundRpm(rpmAtTemp(settings.gpuCurve, gpuTemp));
      const mode = MODE_CARDS.find((item) => item.id === settings.mode) || MODE_CARDS[3];
      const canEditCurve = settings.mode === "custom";

      const showToast = React.useCallback((message, kind) => {
        setToastText(message);
        if (host.toast) {
          const notify = kind === "error" ? host.toast.error : host.toast.success;
          if (typeof notify === "function") notify(message);
        }
        window.clearTimeout(toastTimer);
        toastTimer = window.setTimeout(() => setToastText(""), 1800);
      }, []);

      const updateWithToast = React.useCallback((patch, message) => {
        updateSettings(patch);
        if (message) showToast(message);
      }, [showToast, updateSettings]);

      const refresh = React.useCallback(async () => {
        const controller = new AbortController();
        const timeout = window.setTimeout(() => controller.abort(), 1600);
        setLoading(true);
        try {
          const next = await fetchMockStatus(controller.signal);
          setStatus(next);
          setMockConnected(true);
        } catch {
          setMockConnected(false);
        } finally {
          window.clearTimeout(timeout);
          setLoading(false);
        }
      }, []);

      React.useEffect(() => {
        refresh();
        const timer = window.setInterval(refresh, 5000);
        return () => window.clearInterval(timer);
      }, [refresh]);

      const setMode = async (modeID) => {
        const nextMode = MODE_CARDS.find((item) => item.id === modeID) || MODE_CARDS[3];
        updateSettings({ mode: modeID, cpuPowerLimit: nextMode.cpu });
        if (mockConnected) {
          try {
            const next = await postMock("/mode", { mode: modeID });
            setStatus(next);
          } catch {
            setMockConnected(false);
          }
        }
      };

      const setPowerLimit = async (watts) => {
        updateSettings({ cpuPowerLimit: watts });
        if (mockConnected) {
          try {
            const next = await postMock("/power", { powerLimitWatts: watts, cpuPowerLimitWatts: watts });
            setStatus(next);
          } catch {
            setMockConnected(false);
          }
        }
      };

      const saveCurve = async () => {
        if (!canEditCurve) {
          showToast("当前模式不允许编辑曲线，请切换到大师模式。", "error");
          return;
        }
        const payload = { cpuRpm: cpuTarget, gpuRpm: gpuTarget };
        if (mockConnected) {
          try {
            const next = await postMock("/set-fan", payload);
            setStatus(next);
            showToast("曲线已应用到 Mock 后端");
            return;
          } catch {
            setMockConnected(false);
          }
        }
        showToast("曲线已保存为本地预览");
      };

      const enablePlugin = async () => {
        try {
          if (host.apiService && host.apiService.enablePlugin) {
            await host.apiService.enablePlugin(PLUGIN_ID);
          }
          if (host.apiService && host.apiService.refreshPluginDiscovery) {
            await host.apiService.refreshPluginDiscovery();
          }
          showToast("已请求启用插件");
        } catch (error) {
          const message = error instanceof Error ? error.message : String(error || "启用插件失败");
          showToast(`启用插件失败：${message}`, "error");
        }
      };

      return e("div", { className: "omen-page" },
        toastText ? e("div", { className: "omen-toast" }, e(Icon, { name: "CircleCheck", size: 17 }), toastText) : null,
        e("div", { className: "omen-topbar", "data-theme-card": "curve-header" },
          e("div", null,
            e("div", { className: "omen-title-line" },
              e("span", { className: "omen-mark" }, e(OmenMark)),
              e("h1", { className: "omen-title" }, "OMEN 笔记本风扇"),
              e("span", { className: "omen-pill-row" },
                e(StatusPill, { tone: plugin.running ? "success" : "warning" }, plugin.running ? "运行中" : "已停止"),
                e(StatusPill, { tone: mockConnected ? "success" : "warning" }, mockConnected ? "Mock:8787 已连接" : "Mock:8787 离线"),
                settings.debugMode ? e(StatusPill, { tone: "info" }, "调试模式") : null,
              ),
            ),
            e("div", { className: "omen-subtle", style: { marginTop: 4 } }, "温度与功耗优先读取 FanControl 主程序传感器设置；Mock 调试端可覆盖预览值。"),
          ),
          e("div", { className: "omen-toolbar" },
            e("div", { className: "omen-tab-list" }, TABS.map(([id, label]) => (
              e("button", {
                key: id,
                type: "button",
                className: `omen-tab${settings.tab === id ? " active" : ""}`,
                onClick: () => updateSettings({ tab: id }),
              }, label)
            ))),
            e(Button, { onClick: refresh, disabled: loading, icon: e(Icon, { name: "RefreshCw", size: 15 }) }, loading ? "刷新中" : "刷新"),
            e(Button, { onClick: () => updateWithToast({ debugMode: !settings.debugMode }, !settings.debugMode ? "调试模式已开启" : "调试模式已关闭"), icon: e(Icon, { name: "Bug", size: 15 }) }, settings.debugMode ? "关闭调试" : "调试模式"),
            e(Button, { onClick: enablePlugin, primary: true, icon: e(Icon, { name: "Power", size: 15 }) }, "启用插件"),
          ),
        ),
        settings.debugMode ? e("div", { className: "omen-alert warning" },
          e(Icon, { name: "AlertTriangle", size: 18 }),
          e("div", null,
            e("strong", null, "非 OMEN 机型调试"),
            e("p", null, "当前页面允许在不匹配机型上预览前端与 Mock 后端。真实硬件写入仍需通过驱动检测与机型验证。"),
          ),
        ) : null,
        settings.tab === "overview" ? e(OverviewView, {
          e, Icon, StatusPill, Toggle, settings, updateWithToast, setMode, setPowerLimit, mode,
          status, mockConnected, cpuTemp, gpuTemp, cpuPower, gpuPower, cpuTarget, gpuTarget,
        }) : null,
        settings.tab === "curve" ? e(CurveView, {
          e, Icon, StatusPill, Button, Toggle, FanCurveEditor, settings, updateSettings, updateWithToast, cpuTemp, gpuTemp,
          cpuTarget, gpuTarget, saveCurve, canEditCurve,
        }) : null,
        settings.tab === "settings" ? e(SettingsView, {
          e, Icon, Toggle, SelectControl, SliderControl, settings, updateWithToast,
        }) : null,
      );
    }

    function OverviewView(props) {
      const {
        e, Icon, StatusPill, Toggle, settings, updateWithToast, setMode, setPowerLimit,
        mode, mockConnected, cpuTemp, gpuTemp, cpuPower, gpuPower, cpuTarget, gpuTarget,
      } = props;

      return e(React.Fragment, null,
        e("section", { className: "glacier-hero-card omen-hero rounded-xl border border-border bg-card shadow-sm shadow-black/5", "data-theme-card": "omen-hero" },
          e("div", { className: "omen-hero-icon" }, e(OmenMark)),
          e("div", null,
            e("div", { className: "omen-inline" },
              e("h2", null, "HP OMEN 笔记本风扇"),
              e(StatusPill, { tone: "info" }, `${mode.title}模式`),
              e(StatusPill, null, `CPU ${settings.cpuPowerLimit}W`),
            ),
            e("p", { style: { marginTop: 5 } }, `${mode.title}模式：插件以 CPU/GPU 独立曲线进行软件闭环控制，未连接散热器时也可独立学习。`),
          ),
          e(Toggle, {
            checked: settings.gpuBoost,
            onChange: (gpuBoost) => updateWithToast({ gpuBoost }, gpuBoost ? "GPU 动态加速已开启" : "GPU 动态加速已关闭"),
            label: "GPU 动态加速",
          }),
        ),
        e("div", { className: "omen-grid metrics" },
          e(MetricCard, { e, Icon, icon: "Fan", label: "CPU 风扇", value: formatRpm(getStatusValue(props.status, "cpuRpm", 0)), detail: `软件目标 ${formatRpm(cpuTarget)} · CPU 曲线` }),
          e(MetricCard, { e, Icon, icon: "Fan", label: "GPU 风扇", value: formatRpm(getStatusValue(props.status, "gpuRpm", 0)), detail: `软件目标 ${formatRpm(gpuTarget)} · GPU 曲线` }),
          e(MetricCard, { e, Icon, icon: "Cpu", label: "CPU 温度", value: formatTemp(cpuTemp), detail: `CPU 功耗 ${formatWatts(cpuPower)}` }),
          e(MetricCard, { e, Icon, icon: "Monitor", label: "GPU 温度", value: formatTemp(gpuTemp), detail: `GPU 功耗 ${formatWatts(gpuPower)}` }),
        ),
        e("section", { className: "glacier-control-card omen-section rounded-xl border border-border bg-card shadow-sm shadow-black/5", "data-theme-card": "omen-mode" },
          e("div", { className: "omen-section-head" },
            e("div", null,
              e("div", { className: "omen-inline" }, e(Icon, { name: "Sparkles", size: 18, className: "text-primary" }), e("h2", { className: "omen-section-title" }, "性能档位")),
              e("p", { style: { marginTop: 4 } }, mockConnected ? "Mock 后端已连接，模式和 CPU 功耗限制会同步到本地调试端。" : "Mock 离线时仅保存本地预览。"),
            ),
            e("div", { className: "omen-chip-row" },
              e("span", { className: "omen-subtle" }, "CPU 功耗限制"),
              [35, 65, 95].map((watts) => e("button", {
                key: watts,
                type: "button",
                className: `omen-chip${settings.cpuPowerLimit === watts ? " active" : ""}`,
                onClick: () => setPowerLimit(watts),
              }, `CPU ${watts}W`)),
            ),
          ),
          e("div", { className: "omen-grid modes" }, MODE_CARDS.map((card) => (
            e("article", {
              key: card.id,
              className: `glacier-stat-tile omen-stat-card${settings.mode === card.id ? " active" : ""}`,
              onClick: () => setMode(card.id),
            },
              e("span", { className: "omen-card-icon" }, e(Icon, { name: card.icon, size: 18 })),
              settings.mode === card.id ? e("span", { className: "omen-card-check" }, e(Icon, { name: "Check", size: 14 })) : null,
              e("div", { className: "omen-mode-title" }, card.title),
              e("p", { style: { marginTop: 4 } }, card.detail),
            )
          ))),
        ),
        e("section", { className: "glacier-control-card omen-section rounded-xl border border-border bg-card shadow-sm shadow-black/5", "data-theme-card": "omen-system" },
          e("div", { className: "omen-section-head" },
            e("div", null,
              e("div", { className: "omen-inline" }, e(Icon, { name: "Settings2", size: 18, className: "text-primary" }), e("h2", { className: "omen-section-title" }, "系统设置")),
              e("p", { style: { marginTop: 4 } }, "这些项目先作为插件前端状态预览，真实硬件写入仍需后端能力逐步接入。"),
            ),
          ),
          e("div", { className: "omen-grid system" },
            e(SystemCard, {
              e, Icon, title: "GPU 模式（MUX）", icon: "Zap",
              detail: `当前：${settings.gpuMode === "hybrid" ? "混合（Optimus）" : "独显"}；切换后通常需要重启。`,
              action: e("div", { className: "omen-chip-row" }, ["hybrid", "discrete"].map((item) => e("button", {
                key: item,
                type: "button",
                className: `omen-chip${settings.gpuMode === item ? " active" : ""}`,
                onClick: () => updateWithToast({ gpuMode: item }, "GPU 模式设置已保存"),
              }, item === "hybrid" ? "混合" : "独显"))),
            }),
            e(SystemCard, {
              e, Icon, title: "屏幕过驱", icon: "Monitor",
              detail: "高刷屏减少拖影，具体生效依赖机型支持。",
              action: e(Toggle, {
                checked: settings.screenOverdrive,
                onChange: (screenOverdrive) => updateWithToast({ screenOverdrive }, "屏幕过驱设置已保存"),
              }),
            }),
            e(SystemCard, {
              e, Icon, title: "电池充电上限", icon: "BatteryCharging",
              detail: `当前限制 ${settings.batteryCap}% ，用于延长电池寿命。`,
              action: e("div", { className: "omen-chip-row" }, [100, 80].map((cap) => e("button", {
                key: cap,
                type: "button",
                className: `omen-chip${settings.batteryCap === cap ? " active" : ""}`,
                onClick: () => updateWithToast({ batteryCap: cap }, "电池充电上限已保存"),
              }, `${cap}%`))),
            }),
          ),
        ),
      );
    }

    function MetricCard({ e, Icon, icon, label, value, detail }) {
      return e("article", { className: "glacier-metric-card omen-metric rounded-xl border border-border bg-card shadow-sm shadow-black/5", "data-theme-card": `omen-${label}` },
        e("div", { className: "omen-metric-label" }, e(Icon, { name: icon, size: 16 }), label),
        e("div", { className: "omen-metric-value" }, value),
        e("div", { className: "omen-metric-detail" }, detail),
      );
    }

    function SystemCard({ e, Icon, title, icon, detail, action }) {
      return e("article", { className: "glacier-stat-tile omen-stat-card omen-system-card" },
        e("span", { className: "omen-card-icon" }, e(Icon, { name: icon, size: 18 })),
        e("div", { className: "omen-mode-title" }, title),
        e("p", { style: { marginTop: 4 } }, detail),
        e("div", { style: { marginTop: 12 } }, action),
      );
    }

    function CurveView(props) {
      const { e, Icon, StatusPill, Button, Toggle, FanCurveEditor, settings, updateSettings, updateWithToast, cpuTemp, gpuTemp, cpuTarget, gpuTarget, saveCurve, canEditCurve } = props;

      const applyTemplate = (name) => {
        if (name === "quiet") {
          updateWithToast({ mode: "custom", cpuCurve: cloneCurve(QUIET_CPU_CURVE), gpuCurve: cloneCurve(QUIET_GPU_CURVE) }, "静音模板已载入");
          return;
        }
        if (name === "cool") {
          updateWithToast({ mode: "custom", cpuCurve: cloneCurve(COOL_CPU_CURVE), gpuCurve: cloneCurve(COOL_GPU_CURVE) }, "强散热模板已载入");
          return;
        }
        updateWithToast({ mode: "custom", cpuCurve: cloneCurve(CPU_CURVE), gpuCurve: cloneCurve(GPU_CURVE) }, "默认模板已载入");
      };

      return e(React.Fragment, null,
        e("div", { className: "omen-section-head", "data-theme-card": "curve-header" },
          e("div", null,
            e("div", { className: "omen-inline" },
              e(Icon, { name: "Spline", size: 18, className: "text-primary" }),
              e("h2", { className: "omen-section-title" }, "OMEN 风扇曲线"),
              e(StatusPill, { tone: canEditCurve ? "info" : "warning" }, canEditCurve ? "大师模式可编辑" : "非自定义模式只读"),
            ),
            e("p", { className: "omen-subtle", style: { marginTop: 4 } }, "CPU/GPU 独立控制，5°C 节点；拖动节点预览，应用时按 100 RPM 取整。"),
          ),
          e("div", { className: "omen-curve-toolbar" },
            e("button", { type: "button", className: `omen-segment${!settings.coolerConnected ? " active" : ""}`, onClick: () => updateWithToast({ coolerConnected: false }, "已切换为散热器未连接方案") }, "散热器未连接"),
            e("button", { type: "button", className: `omen-segment${settings.coolerConnected ? " active" : ""}`, onClick: () => updateWithToast({ coolerConnected: true }, "已切换为散热器已连接方案") }, "散热器已连接"),
            e(Button, { onClick: () => applyTemplate("default"), icon: e(Icon, { name: "RotateCcw", size: 15 }) }, "还原"),
            e(Button, { onClick: saveCurve, primary: true, disabled: !canEditCurve, icon: e(Icon, { name: "Save", size: 15 }) }, "保存曲线"),
          ),
        ),
        !canEditCurve ? e("div", { className: "omen-alert warning" },
          e(Icon, { name: "AlertTriangle", size: 18 }),
          e("div", null,
            e("strong", null, "当前模式不会应用自定义风扇曲线"),
            e("p", { style: { marginTop: 3 } }, "切换到大师模式后，CPU 与 GPU 曲线才会进入软件闭环控制。"),
          ),
        ) : null,
        e("section", { className: "glacier-control-card omen-section rounded-xl border border-border bg-card shadow-sm shadow-black/5" },
          e("div", { className: "omen-section-head", style: { marginBottom: 0 } },
            e("div", null,
              e("div", { className: "omen-inline" }, e(Icon, { name: "Sparkles", size: 18, className: "text-primary" }), e("h2", { className: "omen-section-title" }, "快速模板")),
              e("p", { style: { marginTop: 4 } }, "模板会同时覆盖 CPU/GPU 草稿，保存后生效。"),
            ),
            e("div", { className: "omen-chip-row" },
              e("button", { type: "button", className: "omen-chip", onClick: () => applyTemplate("default") }, "默认"),
              e("button", { type: "button", className: "omen-chip", onClick: () => applyTemplate("quiet") }, "静音"),
              e("button", { type: "button", className: "omen-chip", onClick: () => applyTemplate("cool") }, "强散热"),
            ),
          ),
        ),
        e("div", { className: "omen-grid curves" },
          e(CurveCard, {
            e, Icon, FanCurveEditor,
            title: "CPU 风扇曲线",
            icon: "Cpu",
            maxTemp: CPU_TEMP_MAX,
            curve: settings.cpuCurve,
            fallbackCurve: CPU_CURVE,
            currentTemp: cpuTemp,
            targetRpm: cpuTarget,
            detail: `CPU ${Math.round(cpuTemp)}°C → 草稿预览 ${cpuTarget} RPM · 上限 105°C`,
            learnedCurve: settings.jointLearning ? learnedCurveFrom(settings.cpuCurve, settings.learningBias) : null,
            editable: canEditCurve,
            onCurve: (cpuCurve) => updateSettings({ cpuCurve }),
          }),
          e(CurveCard, {
            e, Icon, FanCurveEditor,
            title: "GPU 风扇曲线",
            icon: "Monitor",
            maxTemp: GPU_TEMP_MAX,
            curve: settings.gpuCurve,
            fallbackCurve: GPU_CURVE,
            currentTemp: gpuTemp,
            targetRpm: gpuTarget,
            detail: `GPU ${Math.round(gpuTemp)}°C → 草稿预览 ${gpuTarget} RPM · 上限 90°C`,
            learnedCurve: settings.jointLearning ? learnedCurveFrom(settings.gpuCurve, settings.learningBias) : null,
            editable: canEditCurve,
            onCurve: (gpuCurve) => updateSettings({ gpuCurve }),
          }),
        ),
        e("section", { className: "glacier-control-card omen-section rounded-xl border border-border bg-card shadow-sm shadow-black/5" },
          e("div", { className: "omen-section-head", style: { marginBottom: 0 } },
            e("div", null,
              e("div", { className: "omen-inline" }, e(Icon, { name: "Sparkles", size: 18, className: "text-primary" }), e("h2", { className: "omen-section-title" }, "学习曲线显示")),
              e("p", { style: { marginTop: 4 } }, "开启后，图中虚线表示学习偏移后的建议曲线；基础曲线仍由手动节点控制。"),
            ),
            e(Toggle, { checked: settings.jointLearning, onChange: (jointLearning) => updateWithToast({ jointLearning }, "联合学习设置已保存"), label: "联合学习" }),
          ),
        ),
      );
    }

    function CurveCard({ e, Icon, FanCurveEditor, title, icon, detail, curve, fallbackCurve, maxTemp, currentTemp, learnedCurve, editable, onCurve }) {
      return e("article", { className: "glacier-chart-card omen-curve-card rounded-xl border border-border bg-card shadow-sm shadow-black/5", "data-theme-card": "omen-curve-card" },
        e("div", { className: "omen-curve-title" },
          e("div", { className: "omen-inline", style: { alignItems: "flex-start" } },
            e("span", { className: "omen-card-icon" }, e(Icon, { name: icon, size: 18 })),
            e("div", null,
              e("h3", { className: "omen-card-title" }, title),
              e("p", { className: "omen-subtle", style: { marginTop: 4 } }, detail),
            ),
          ),
          e("span", { className: `omen-pill ${editable ? "info" : "warning"}` }, editable ? "软件闭环" : "只读预览"),
        ),
        e("div", { className: "omen-legend" },
          e("span", null, e("span", { className: "omen-legend-line" }), "基础曲线"),
          learnedCurve ? e("span", null, e("span", { className: "omen-legend-line dashed" }), "学习后曲线") : null,
          e("span", null, "当前温度"),
        ),
        FanCurveEditor
          ? e("div", { className: "omen-chart-host" }, e(FanCurveEditor, {
            curve,
            fallbackCurve,
            learnedCurve,
            currentTemp,
            minTemp: TEMP_MIN,
            maxTemp,
            tempStep: 5,
            minSpeed: RPM_MIN,
            maxSpeed: RPM_MAX,
            speedStep: RPM_STEP,
            speedTicks: [800, 1500, 2500, 3500, 4500, 5500, 6000],
            speedUnit: " RPM",
            editable,
            onCurveChange: onCurve,
            heightClassName: "h-[360px] md:h-[420px]",
            labels: {
              temperatureAxis: "温度",
              speedAxis: "速度（RPM）",
              baseCurve: "基础曲线",
              learnedCurve: "学习曲线",
              currentTemperature: "当前 {{temperature}}°C",
            },
          }))
          : e("div", { className: "omen-alert warning" }, "当前主程序未暴露曲线组件，请更新 FanControl 主程序。"),
      );
    }

    function SettingsView({ e, Icon, Toggle, SelectControl, SliderControl, settings, updateWithToast }) {
      const responseOptions = Array.from({ length: 10 }, (_, index) => {
        const value = index + 1;
        const label = value <= 3 ? `${value} · 更平滑` : value >= 7 ? `${value} · 响应更快` : `${value} · 较强平滑`;
        return { value: String(value), label };
      });
      const biasOptions = [
        { value: "cooling", label: "偏散热" },
        { value: "balanced", label: "均衡" },
        { value: "quiet", label: "偏静音" },
      ];

      return e("section", { className: "omen-settings-section rounded-xl border border-border bg-card shadow-sm shadow-black/5", "data-theme-ui": "setting-section" },
        e("div", { className: "omen-settings-header", "data-theme-ui": "setting-section-header" },
          e("span", { className: "omen-card-icon" }, e(Icon, { name: "SlidersHorizontal", size: 18 })),
          e("div", null,
            e("h2", { className: "omen-section-title" }, "电脑风扇设置"),
            e("div", { className: "omen-subtle", style: { marginTop: 3 } }, settings.coolerConnected ? "散热器已连接方案" : "散热器未连接方案"),
          ),
        ),
        e(SettingRow, {
          e, Icon, icon: "Gauge", title: "控制响应速度", detail: "控制温度采样平滑度，数值越小越平滑，数值越大响应越快。",
          control: e(SelectControl, {
            value: String(settings.responseSpeed),
            onChange: (value) => updateWithToast({ responseSpeed: Number(value) }, "响应速度已保存"),
            options: responseOptions,
          }),
        }),
        e(SettingRow, {
          e, Icon, icon: "Sparkles", title: "联合学习", detail: settings.coolerConnected ? "连接散热器时使用联合曲线方案。" : "未检测到散热器，电脑风扇按独立方案学习。",
          control: e(Toggle, {
            checked: settings.jointLearning,
            onChange: (jointLearning) => updateWithToast({ jointLearning }, "联合学习设置已保存"),
          }),
        }),
        e(SettingRow, {
          e, Icon, icon: "Target", title: "学习倾向", detail: "偏散热会保守增速，偏静音会在忍受上限内优先降噪。",
          control: e(SelectControl, {
            value: settings.learningBias,
            onChange: (learningBias) => updateWithToast({ learningBias }, "学习倾向已保存"),
            options: biasOptions,
          }),
        }),
        e("div", { className: "omen-subcard" },
          e("div", { className: "omen-grid two" },
            e(SliderControl, { title: "CPU 目标温度", value: settings.cpuTarget, min: 55, max: 90, suffix: "°C", onChange: (cpuTarget) => updateWithToast({ cpuTarget }) }),
            e(SliderControl, { title: "GPU 目标温度", value: settings.gpuTarget, min: 50, max: 85, suffix: "°C", onChange: (gpuTarget) => updateWithToast({ gpuTarget }) }),
          ),
        ),
        settings.learningBias === "quiet" ? e("div", { className: "omen-subcard" },
          e("div", { className: "omen-inline", style: { marginBottom: 12 } },
            e(Icon, { name: "Volume2", size: 17, className: "text-primary" }),
            e("h3", { className: "omen-card-title" }, "偏静音温度忍受上限"),
          ),
          e("p", { className: "omen-subtle", style: { marginTop: -6, marginBottom: 14 } }, "达到上限后会优先恢复散热，不继续为了降噪压低转速。"),
          e("div", { className: "omen-grid two" },
            e(SliderControl, { title: "CPU 上限", value: settings.quietCpuLimit, min: 75, max: 105, suffix: "°C", onChange: (quietCpuLimit) => updateWithToast({ quietCpuLimit }) }),
            e(SliderControl, { title: "GPU 上限", value: settings.quietGpuLimit, min: 70, max: 90, suffix: "°C", onChange: (quietGpuLimit) => updateWithToast({ quietGpuLimit }) }),
          ),
        ) : null,
        e(SettingRow, {
          e, Icon, icon: "Radar", title: "温升预判", detail: "提前响应温度上升趋势，减少散热滞后。",
          control: e(Toggle, {
            checked: settings.risePrediction,
            onChange: (risePrediction) => updateWithToast({ risePrediction }, "温升预判设置已保存"),
          }),
        }),
      );
    }

    function SettingRow({ e, Icon, icon, title, detail, control }) {
      return e("div", { className: "omen-setting-row", "data-theme-ui": "setting-row" },
        e("div", { className: "omen-setting-copy" },
          e("span", { className: "omen-card-icon", "data-theme-ui": "setting-row-icon" }, e(Icon, { name: icon, size: 18 })),
          e("div", null,
            e("h3", { className: "omen-card-title" }, title),
            e("p", { style: { marginTop: 3 } }, detail),
          ),
        ),
        e("div", { className: "omen-setting-control", "data-theme-ui": "setting-row-control" }, control),
      );
    }

    host.registerPage(PLUGIN_ID, {
      title: "OMEN 笔记本风扇",
      component: OmenFanPage,
    });
  }

  registerWithHost(window.FanControlPluginHost);
})();
