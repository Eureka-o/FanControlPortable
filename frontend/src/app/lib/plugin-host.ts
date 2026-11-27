import * as React from 'react';
import {
  Activity,
  AlertTriangle,
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  Battery,
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Cpu,
  Download,
  Fan,
  Flame,
  Gauge,
  Info,
  Laptop,
  Link,
  MapPin,
  Monitor,
  MoreHorizontal,
  Pause,
  Play,
  Plug,
  Plus,
  Power,
  RefreshCw,
  RotateCcw,
  RotateCw,
  Settings,
  Shield,
  SlidersHorizontal,
  Sparkles,
  Terminal,
  Thermometer,
  Trash2,
  Unlink,
  Wind,
  Wrench,
  Zap,
  type LucideIcon,
} from 'lucide-react';
import { toast } from 'sonner';
import * as FanControlUI from '../components/ui';
import { i18n } from './i18n';
import { apiService } from '../services/api';
import type { PluginInfo } from '../types/app';

export const PLUGIN_HOST_API_VERSION = 1;

export const PLUGIN_HOST_ICONS: Readonly<Record<string, LucideIcon>> = Object.freeze({
  activity: Activity,
  alertTriangle: AlertTriangle,
  arrowDown: ArrowDown,
  arrowLeft: ArrowLeft,
  arrowRight: ArrowRight,
  arrowUp: ArrowUp,
  battery: Battery,
  check: Check,
  checkCircle: CheckCircle2,
  chevronDown: ChevronDown,
  chevronRight: ChevronRight,
  cpu: Cpu,
  download: Download,
  fan: Fan,
  flame: Flame,
  gauge: Gauge,
  info: Info,
  laptop: Laptop,
  link: Link,
  mapPin: MapPin,
  monitor: Monitor,
  moreHorizontal: MoreHorizontal,
  pause: Pause,
  play: Play,
  plug: Plug,
  plus: Plus,
  power: Power,
  refreshCw: RefreshCw,
  rotateCcw: RotateCcw,
  rotateCw: RotateCw,
  settings: Settings,
  shield: Shield,
  sliders: SlidersHorizontal,
  sparkles: Sparkles,
  terminal: Terminal,
  thermometer: Thermometer,
  trash: Trash2,
  unlink: Unlink,
  wind: Wind,
  wrench: Wrench,
  zap: Zap,
});

export interface PluginThemeSnapshot {
  mode: string | null;
  isDark: boolean;
  layer: string;
}

export interface PluginThemeAPI {
  current: () => PluginThemeSnapshot;
  subscribe: (callback: (theme: PluginThemeSnapshot) => void) => () => void;
}

export interface PluginLocaleAPI {
  current: () => string;
  t: (key: string, options?: Record<string, unknown>) => string;
  subscribe: (callback: (locale: string) => void) => () => void;
}

export interface PluginToastAPI {
  success: (message: string, description?: string) => void;
  error: (message: string, description?: string) => void;
  info: (message: string, description?: string) => void;
}

export interface PluginScopedHost {
  pluginId: string;
  React: typeof React;
  ui: typeof FanControlUI;
  icons: Readonly<Record<string, LucideIcon>>;
  invoke: <T = unknown>(method: string, payload?: unknown) => Promise<T>;
  subscribe: (event: string, handler: (payload: unknown) => void) => () => void;
  getSettings: () => Promise<Record<string, unknown>>;
  saveSettings: (settings: Record<string, unknown>) => Promise<void>;
  getLogs: () => Promise<string[]>;
  assetURL: (relPath: string) => string;
  theme: PluginThemeAPI;
  locale: PluginLocaleAPI;
  toast: PluginToastAPI;
}

export interface PluginMountProps {
  plugin: PluginInfo;
  host: FanControlPluginHostV1;
  scopedHost?: PluginScopedHost;
}

export interface PluginComponentProps {
  plugin: PluginInfo;
  host: PluginScopedHost;
}

export interface PluginSurfaceDefinition {
  id: string;
  title?: string;
  hideDefaultHeader?: boolean;
  component?: React.ComponentType<PluginComponentProps>;
  mount?: (container: HTMLElement, props: PluginMountProps) => void | (() => void);
  onDispose?: () => void;
}

export interface FanControlPluginHostV1 {
  version: number;
  React: typeof React;
  ui: typeof FanControlUI;
  icons: Readonly<Record<string, LucideIcon>>;
  registerPage: (definition: PluginSurfaceDefinition) => void;
  registerSettingsPanel: (definition: PluginSurfaceDefinition) => void;
  invoke: <T = unknown>(pluginId: string, method: string, payload?: unknown) => Promise<T>;
  subscribe: (
    pluginId: string,
    event: string,
    handler: (payload: unknown) => void,
  ) => () => void;
  getSettings: (pluginId: string) => Promise<Record<string, unknown>>;
  saveSettings: (pluginId: string, settings: Record<string, unknown>) => Promise<void>;
  getAssetURL: (pluginId: string, relPath: string, version?: string) => string;
  getTheme: () => { mode: string | null; isDark: boolean };
  getLocale: () => string;
  theme: PluginThemeAPI;
  locale: PluginLocaleAPI;
  toast: PluginToastAPI;
}

const registeredPages = new Map<string, PluginSurfaceDefinition>();
const registeredPanels = new Map<string, PluginSurfaceDefinition>();
const listeners = new Set<() => void>();
const loadedAssetVersions = new Map<string, string>();
const loadingAssetPromises = new Map<string, Promise<void>>();
const pluginEventSubscribers = new Map<string, Set<(payload: unknown) => void>>();

let hostEventBound = false;

function notifyRegistryListeners() {
  for (const listener of listeners) {
    listener();
  }
}

export function subscribePluginRegistry(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function getRegisteredPluginPage(pluginId: string): PluginSurfaceDefinition | undefined {
  return registeredPages.get(pluginId);
}

export function getRegisteredPluginPanel(pluginId: string): PluginSurfaceDefinition | undefined {
  return registeredPanels.get(pluginId);
}

function eventSubscriptionKey(pluginId: string, event: string): string {
  return `${pluginId}:${event}`;
}

function isValidSurfaceDefinition(definition: PluginSurfaceDefinition | null | undefined): boolean {
  if (!definition || !definition.id) return false;
  if (typeof definition.mount === 'function') return true;
  const comp = definition.component as unknown;
  return (
    typeof comp === 'function' ||
    (typeof comp === 'object' && comp !== null && '$$typeof' in comp)
  );
}

export function buildPluginAssetURL(pluginId: string, relPath: string, version?: string): string {
  const cleanRel = String(relPath || '').replace(/^[/\\]+/, '');
  const encodedRel = cleanRel
    .split('/')
    .map((segment) => encodeURIComponent(segment))
    .join('/');
  const query = version ? `?v=${encodeURIComponent(version)}` : '';
  return `/plugin-assets/${encodeURIComponent(pluginId)}/${encodedRel}${query}`;
}

export function isSidebarPluginVisible(plugin: PluginInfo): boolean {
  if (!plugin || plugin.uiSlot !== 'sidebar-page') return false;
  if (!plugin.enabled || !plugin.supported) return false;
  return (
    plugin.state === 'ready' ||
    plugin.state === 'restarting' ||
    plugin.state === 'suspended' ||
    plugin.state === 'suspending'
  );
}

function readCurrentThemeSnapshot(): PluginThemeSnapshot {
  if (typeof document === 'undefined') {
    return { mode: null, isDark: false, layer: 'basic' };
  }
  const root = document.documentElement;
  return {
    mode: root.dataset.theme || null,
    isDark: root.classList.contains('dark'),
    layer: root.dataset.themeLayer || 'basic',
  };
}

const themeAPIRaw: PluginThemeAPI = {
  current: readCurrentThemeSnapshot,
  subscribe(callback) {
    callback(readCurrentThemeSnapshot());
    if (typeof document === 'undefined' || typeof MutationObserver === 'undefined') {
      return () => {};
    }
    const observer = new MutationObserver(() => callback(readCurrentThemeSnapshot()));
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class', 'data-theme', 'data-theme-layer'],
    });
    return () => observer.disconnect();
  },
};
const themeAPI: PluginThemeAPI = Object.freeze(themeAPIRaw);

const localeAPIRaw: PluginLocaleAPI = {
  current: () => i18n.language || 'zh-CN',
  t: (key, options) => String(i18n.t(key, options)),
  subscribe(callback) {
    callback(i18n.language || 'zh-CN');
    const listener = (lng: string) => callback(lng || 'zh-CN');
    i18n.on('languageChanged', listener);
    return () => {
      i18n.off('languageChanged', listener);
    };
  },
};
const localeAPI: PluginLocaleAPI = Object.freeze(localeAPIRaw);

const toastAPIRaw: PluginToastAPI = {
  success: (message, description) => toast.success(message, description ? { description } : undefined),
  error: (message, description) => toast.error(message, description ? { description } : undefined),
  info: (message, description) => toast.info(message, description ? { description } : undefined),
};
const toastAPI: PluginToastAPI = Object.freeze(toastAPIRaw);

export function ensurePluginHost(): FanControlPluginHostV1 {
  if (typeof window === 'undefined') {
    throw new Error('Plugin host requires browser environment');
  }
  const win = window as unknown as Record<string, unknown>;
  const existing = win.FanControlPluginHost as FanControlPluginHostV1 | undefined;
  if (existing && existing.version === PLUGIN_HOST_API_VERSION && existing.ui && existing.React) {
    return existing;
  }

  if (!hostEventBound) {
    hostEventBound = true;
    apiService.onPluginEvent((evt) => {
      if (!evt || !evt.pluginId || !evt.event) return;
      const key = eventSubscriptionKey(evt.pluginId, evt.event);
      const handlers = pluginEventSubscribers.get(key);
      if (!handlers) return;
      for (const handler of handlers) {
        try {
          handler(evt.payload);
        } catch {
          // Keep plugin subscriber errors isolated from host dispatch.
        }
      }
    });
  }

  const hostRaw: FanControlPluginHostV1 = {
    version: PLUGIN_HOST_API_VERSION,
    React,
    ui: FanControlUI,
    icons: PLUGIN_HOST_ICONS,
    registerPage(definition) {
      if (!isValidSurfaceDefinition(definition)) {
        return;
      }
      registeredPages.set(definition.id, definition);
      notifyRegistryListeners();
    },
    registerSettingsPanel(definition) {
      if (!isValidSurfaceDefinition(definition)) {
        return;
      }
      registeredPanels.set(definition.id, definition);
      notifyRegistryListeners();
    },
    invoke<T = unknown>(pluginId: string, method: string, payload?: unknown): Promise<T> {
      return apiService.invokePlugin(pluginId, method, payload) as Promise<T>;
    },
    subscribe(pluginId, event, handler) {
      const key = eventSubscriptionKey(pluginId, event);
      let set = pluginEventSubscribers.get(key);
      if (!set) {
        set = new Set();
        pluginEventSubscribers.set(key, set);
      }
      set.add(handler);
      return () => {
        const current = pluginEventSubscribers.get(key);
        if (!current) return;
        current.delete(handler);
        if (current.size === 0) {
          pluginEventSubscribers.delete(key);
        }
      };
    },
    getSettings(pluginId) {
      return apiService.getPluginSettings(pluginId);
    },
    saveSettings(pluginId, settings) {
      return apiService.savePluginSettings(pluginId, settings);
    },
    getAssetURL(pluginId, relPath, version) {
      return buildPluginAssetURL(pluginId, relPath, version);
    },
    getTheme() {
      const snap = readCurrentThemeSnapshot();
      return {
        mode: snap.mode,
        isDark: snap.isDark,
      };
    },
    getLocale() {
      return i18n.language || 'zh-CN';
    },
    theme: themeAPI,
    locale: localeAPI,
    toast: toastAPI,
  };
  const host = Object.freeze(hostRaw);

  win.FanControlPluginHost = host;
  return host;
}

export function createScopedPluginHost(plugin: PluginInfo): PluginScopedHost {
  const host = ensurePluginHost();
  return {
    pluginId: plugin.id,
    React,
    ui: FanControlUI,
    icons: PLUGIN_HOST_ICONS,
    invoke: (method, payload) => host.invoke(plugin.id, method, payload),
    subscribe: (event, handler) => host.subscribe(plugin.id, event, handler),
    getSettings: () => host.getSettings(plugin.id),
    saveSettings: (settings) => host.saveSettings(plugin.id, settings),
    getLogs: () => apiService.getPluginLogs(plugin.id),
    assetURL: (relPath) => buildPluginAssetURL(plugin.id, relPath, plugin.version),
    theme: themeAPI,
    locale: localeAPI,
    toast: toastAPI,
  };
}

export async function loadPluginAssets(plugin: PluginInfo): Promise<void> {
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return;
  }
  if (!plugin || !plugin.frontend) {
    return;
  }

  ensurePluginHost();
  const versionKey = `${plugin.version || '0'}:${plugin.frontend}:${plugin.style || ''}`;
  if (loadedAssetVersions.get(plugin.id) === versionKey) {
    return;
  }

  const inflightKey = `${plugin.id}@${versionKey}`;
  const existingPromise = loadingAssetPromises.get(inflightKey);
  if (existingPromise) {
    return existingPromise;
  }

  const task = (async () => {
    unloadPluginAssets(plugin.id, false);

    if (plugin.style) {
      const href = buildPluginAssetURL(plugin.id, plugin.style, plugin.version);
      await new Promise<void>((resolve, reject) => {
        const link = document.createElement('link');
        link.rel = 'stylesheet';
        link.href = href;
        link.dataset.pluginStyleId = plugin.id;
        link.onload = () => resolve();
        link.onerror = () => reject(new Error(`加载插件样式失败: ${plugin.style}`));
        document.head.appendChild(link);
      });
    }

    const src = buildPluginAssetURL(plugin.id, plugin.frontend!, plugin.version);
    await new Promise<void>((resolve, reject) => {
      const script = document.createElement('script');
      script.src = src;
      script.async = true;
      script.dataset.pluginScriptId = plugin.id;
      script.onload = () => resolve();
      script.onerror = () => reject(new Error(`加载插件脚本失败: ${plugin.frontend}`));
      document.head.appendChild(script);
    });

    loadedAssetVersions.set(plugin.id, versionKey);
  })().finally(() => {
    loadingAssetPromises.delete(inflightKey);
  });

  loadingAssetPromises.set(inflightKey, task);
  return task;
}

export function unloadPluginAssets(pluginId: string, clearRegistration = true): void {
  if (typeof document !== 'undefined') {
    document
      .querySelectorAll(`[data-plugin-style-id="${pluginId}"], [data-plugin-script-id="${pluginId}"]`)
      .forEach((node) => node.remove());
  }
  loadedAssetVersions.delete(pluginId);

  if (clearRegistration) {
    const page = registeredPages.get(pluginId);
    if (page?.onDispose) {
      try {
        page.onDispose();
      } catch {
        // Ignore plugin cleanup errors.
      }
    }
    const panel = registeredPanels.get(pluginId);
    if (panel?.onDispose) {
      try {
        panel.onDispose();
      } catch {
        // Ignore plugin cleanup errors.
      }
    }
    registeredPages.delete(pluginId);
    registeredPanels.delete(pluginId);
    notifyRegistryListeners();
  }
}
