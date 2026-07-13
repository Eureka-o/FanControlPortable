export const CUSTOM_STYLE_ID = 'thrm-custom-theme-style';
export const THEME_BOOTSTRAP_STORAGE_KEY = 'thrm.theme-bootstrap';

// Bump this when custom theme CSS resource semantics change, so stale cached
// CSS cannot point at assets unavailable to the current executable.
const THEME_BOOTSTRAP_VERSION = 5;

// 缓存的 CSS 最大体积（字节），超过此值截断并添加注释标记。
// 防止 localStorage QuotaExceededError，以及重型 advanced 主题导致的前端内存压力。
const MAX_CACHED_CSS_BYTES = 512 * 1024;
const BUILTIN_THEME_MODES = ['system', 'light', 'dark'] as const;
const CUSTOM_THEME_LAYERS = ['basic', 'advanced'] as const;

export type BuiltinThemeMode = (typeof BUILTIN_THEME_MODES)[number];
export type CustomThemeBase = 'light' | 'dark';
export type CustomThemeLayer = (typeof CUSTOM_THEME_LAYERS)[number];

export type ThemeBootstrapSnapshot = {
  version: typeof THEME_BOOTSTRAP_VERSION;
  mode: string;
  base?: CustomThemeBase;
  layer?: CustomThemeLayer;
  css?: string;
  /** CSS 被截断时标记，避免重放超长旧缓存 */
  cssTruncated?: boolean;
};

export function isBuiltinMode(mode: string): mode is BuiltinThemeMode {
  return (BUILTIN_THEME_MODES as readonly string[]).includes(mode);
}

function normalizeMode(value: unknown): string | null {
  if (typeof value !== 'string') {
    return null;
  }

  const trimmed = value.trim();
  return trimmed ? trimmed : null;
}

export function normalizeCustomThemeLayer(value: unknown): CustomThemeLayer {
  return value === 'advanced' ? 'advanced' : 'basic';
}

export function parseThemeBootstrapSnapshot(raw: string | null | undefined): ThemeBootstrapSnapshot | null {
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw) as Partial<ThemeBootstrapSnapshot> | null;
    const mode = normalizeMode(parsed?.mode);
    if (!parsed || parsed.version !== THEME_BOOTSTRAP_VERSION || !mode) {
      return null;
    }

    if (isBuiltinMode(mode)) {
      return {
        version: THEME_BOOTSTRAP_VERSION,
        mode,
      };
    }

    if (parsed.cssTruncated) {
      return null;
    }

    return {
      version: THEME_BOOTSTRAP_VERSION,
      mode,
      base: parsed.base === 'dark' ? 'dark' : 'light',
      layer: normalizeCustomThemeLayer(parsed.layer),
      css: typeof parsed.css === 'string' ? parsed.css : '',
      cssTruncated: false,
    };
  } catch {
    return null;
  }
}

export function serializeThemeBootstrapSnapshot(snapshot: ThemeBootstrapSnapshot): string {
  return JSON.stringify(snapshot);
}

export function createBuiltinThemeSnapshot(mode: BuiltinThemeMode): ThemeBootstrapSnapshot {
  return {
    version: THEME_BOOTSTRAP_VERSION,
    mode,
  };
}

export function createCustomThemeSnapshot(mode: string, base: CustomThemeBase, css: string, layer: CustomThemeLayer = 'basic'): ThemeBootstrapSnapshot {
  const cssTruncated = css.length > MAX_CACHED_CSS_BYTES;
  return {
    version: THEME_BOOTSTRAP_VERSION,
    mode,
    base,
    layer,
    css: cssTruncated ? css.slice(0, MAX_CACHED_CSS_BYTES) : css,
    cssTruncated,
  };
}

export function getThemeBootstrapScript(): string {
  return `
(() => {
  const STORAGE_KEY = ${JSON.stringify(THEME_BOOTSTRAP_STORAGE_KEY)};
  const STYLE_ID = ${JSON.stringify(CUSTOM_STYLE_ID)};
  const BUILTIN_MODES = new Set(${JSON.stringify([...BUILTIN_THEME_MODES])});
  const root = document.documentElement;
  root.dataset.windowBlur = 'on';

  const applyBaseTheme = (isDark) => {
    const styleEl = document.getElementById(STYLE_ID);
    if (styleEl) styleEl.remove();
    delete root.dataset.theme;
    delete root.dataset.themeLayer;
    root.classList.toggle('dark', !!isDark);
  };

  const detectOs = () => {
    const ua = navigator.userAgent || '';
    const platform = (navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || '';
    const probe = (ua + ' ' + platform).toLowerCase();
    if (probe.includes('windows') || probe.includes('win32') || probe.includes('win64')) return 'win';
    if (probe.includes('mac') || probe.includes('darwin')) return 'mac';
    if (probe.includes('linux')) return 'linux';
    return 'other';
  };

  try {
    root.dataset.os = detectOs();
  } catch {
    // noop
  }

  let snapshot = null;
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    snapshot = raw ? JSON.parse(raw) : null;
  } catch {
    snapshot = null;
  }

  const prefersDark = !!(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);
  if (!snapshot || snapshot.version !== ${JSON.stringify(THEME_BOOTSTRAP_VERSION)} || typeof snapshot.mode !== 'string' || !snapshot.mode) {
    applyBaseTheme(prefersDark);
    return;
  }

  if (BUILTIN_MODES.has(snapshot.mode)) {
    applyBaseTheme(snapshot.mode === 'dark' || (snapshot.mode === 'system' && prefersDark));
    return;
  }

  const base = snapshot.base === 'dark' ? 'dark' : 'light';
  if (snapshot.cssTruncated) {
    applyBaseTheme(base === 'dark');
    return;
  }

  const css = typeof snapshot.css === 'string' ? snapshot.css : '';
  if (!css) {
    applyBaseTheme(base === 'dark');
    return;
  }

  delete root.dataset.windowBlur;
  root.dataset.themeLayer = snapshot.layer === 'advanced' ? 'advanced' : 'basic';
  let styleEl = document.getElementById(STYLE_ID);
  if (!styleEl) {
    styleEl = document.createElement('style');
    styleEl.id = STYLE_ID;
    document.head.appendChild(styleEl);
  }
  styleEl.textContent = css;
  root.classList.toggle('dark', base === 'dark');
  root.dataset.theme = snapshot.mode;
})();
`.trim();
}
