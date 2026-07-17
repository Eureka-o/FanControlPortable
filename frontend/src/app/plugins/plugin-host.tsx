'use client';

import * as React from 'react';
import { AlertTriangle, LoaderCircle, RotateCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import * as FanControlUI from '../components/ui';
import { i18n } from '../lib/i18n';
import { apiService } from '../services/api';
import type { PluginCatalogEntry, PluginCatalogSnapshot } from '../types/app';
import { PLUGIN_ICONS } from './plugin-icons';
import {
  isVisiblePluginPage,
  pluginAssetURL,
  pluginFrontendFingerprint,
  sortVisiblePluginPages,
} from './plugin-host-logic.mts';
import type {
  FanControlPluginHostV1,
  PluginPageProps,
  PluginPageRegistration,
  PluginScopedHost,
  PluginThemeSnapshot,
  RegisteredPluginPage,
} from './plugin-host-types';

type PluginResources = {
  fingerprint: string;
  style?: HTMLLinkElement;
  script?: HTMLScriptElement;
  unsubscribers: Set<() => void>;
};

const expectedPlugins = new Map<string, PluginCatalogEntry>();
const registeredPages = new Map<string, RegisteredPluginPage>();
const pluginResources = new Map<string, PluginResources>();
const pendingLoads = new Map<string, Promise<RegisteredPluginPage>>();

const pluginToast = {
  success: (message: string, description?: string) => toast.success(message, { description }),
  error: (message: string, description?: string) => toast.error(message, { description }),
  info: (message: string, description?: string) => toast.info(message, { description }),
};

function currentTheme(): PluginThemeSnapshot {
  const root = document.documentElement;
  return {
    mode: root.dataset.theme || (root.classList.contains('dark') ? 'dark' : 'light'),
    layer: root.dataset.themeLayer || 'basic',
    dark: root.classList.contains('dark'),
  };
}

function trackPluginCleanup(pluginId: string, cleanup: () => void): () => void {
  const resources = pluginResources.get(pluginId);
  if (!resources) return cleanup;
  let active = true;
  const tracked = () => {
    if (!active) return;
    active = false;
    resources.unsubscribers.delete(tracked);
    cleanup();
  };
  resources.unsubscribers.add(tracked);
  return tracked;
}

function subscribePluginEvent(pluginId: string, eventName: string, callback: (payload: unknown) => void) {
  if (!expectedPlugins.has(pluginId)) throw new Error(`Plugin is not active: ${pluginId}`);
  return trackPluginCleanup(pluginId, apiService.onPluginEvent((event) => {
    if (event.pluginId !== pluginId || (eventName !== '*' && event.event !== eventName)) return;
    callback(event.payload);
  }));
}

function subscribeTheme(pluginId: string, callback: (theme: PluginThemeSnapshot) => void) {
  callback(currentTheme());
  const observer = new MutationObserver(() => callback(currentTheme()));
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['class', 'data-theme', 'data-theme-layer'],
  });
  return trackPluginCleanup(pluginId, () => observer.disconnect());
}

function subscribeLocale(pluginId: string, callback: (locale: string) => void) {
  callback(i18n.language);
  const onLanguageChanged = (locale: string) => callback(locale);
  i18n.on('languageChanged', onLanguageChanged);
  return trackPluginCleanup(pluginId, () => i18n.off('languageChanged', onLanguageChanged));
}

function validPageComponent(component: PluginPageRegistration['component']) {
  return typeof component === 'function' || (typeof component === 'object' && component !== null && '$$typeof' in component);
}

function registerPage(registration: PluginPageRegistration) {
  const script = document.currentScript as HTMLScriptElement | null;
  const pluginId = script?.dataset.pluginId?.trim() || '';
  const pageId = script?.dataset.pageId?.trim() || '';
  const expected = expectedPlugins.get(pluginId);
  if (
    !expected ||
    expected.page.id !== pageId ||
    registration.id !== pageId ||
    script?.dataset.pluginFingerprint !== pluginFrontendFingerprint(expected)
  ) {
    throw new Error('Plugin page registration does not match its manifest');
  }
  if (!validPageComponent(registration.component)) {
    throw new Error('Plugin page must be a React component');
  }
  if (registeredPages.has(pluginId)) {
    throw new Error(`Plugin page is already registered: ${pluginId}`);
  }
  registeredPages.set(pluginId, { ...registration, pluginId });
}

const hostAPI: FanControlPluginHostV1 = Object.freeze({
  version: 1 as const,
  React,
  registerPage,
  invoke: <T,>(pluginId: string, method: string, payload: unknown = {}) => {
    if (!expectedPlugins.has(pluginId)) return Promise.reject(new Error(`Plugin is not active: ${pluginId}`));
    return apiService.invokePlugin<T>(pluginId, method, payload);
  },
  subscribe: subscribePluginEvent,
  ui: FanControlUI,
  icons: PLUGIN_ICONS,
  theme: {
    current: currentTheme,
    subscribe: subscribeTheme,
  },
  locale: {
    current: () => i18n.language,
    t: (key: string, options?: Record<string, unknown>) => String(i18n.t(key, options)),
    subscribe: subscribeLocale,
  },
  toast: pluginToast,
});

function installPluginHost() {
  window.FanControlPluginHost = hostAPI;
}

function removePluginResources(pluginId: string, owner?: PluginResources) {
  if (owner && pluginResources.get(pluginId) !== owner) return;
  pendingLoads.delete(pluginId);
  const registration = registeredPages.get(pluginId);
  registeredPages.delete(pluginId);
  try {
    registration?.dispose?.();
  } catch (error) {
    console.error(`Plugin dispose failed (${pluginId}):`, error);
  }
  const resources = pluginResources.get(pluginId);
  pluginResources.delete(pluginId);
  resources?.unsubscribers.forEach((unsubscribe) => unsubscribe());
  resources?.script?.remove();
  resources?.style?.remove();
}

export function unloadPluginPage(pluginId: string) {
  expectedPlugins.delete(pluginId);
  removePluginResources(pluginId);
}

function reconcilePluginPages(plugins: PluginCatalogEntry[]) {
  const next = new Map(
    plugins.filter(isVisiblePluginPage).map((plugin) => [plugin.id, plugin] as const),
  );
  for (const [pluginId, current] of expectedPlugins) {
    const replacement = next.get(pluginId);
    if (!replacement || pluginFrontendFingerprint(replacement) !== pluginFrontendFingerprint(current)) {
      removePluginResources(pluginId);
    }
  }
  expectedPlugins.clear();
  next.forEach((plugin, pluginId) => expectedPlugins.set(pluginId, plugin));
}

function loadElement(element: HTMLLinkElement | HTMLScriptElement, label: string): Promise<void> {
  return new Promise((resolve, reject) => {
    element.onload = () => resolve();
    element.onerror = () => reject(new Error(`Failed to load plugin ${label}`));
    document.head.appendChild(element);
  });
}

export function loadPluginPage(plugin: PluginCatalogEntry): Promise<RegisteredPluginPage> {
  const current = registeredPages.get(plugin.id);
  const fingerprint = pluginFrontendFingerprint(plugin);
  if (current && pluginResources.get(plugin.id)?.fingerprint === fingerprint) return Promise.resolve(current);
  const pending = pendingLoads.get(plugin.id);
  if (pending) return pending;

  expectedPlugins.set(plugin.id, plugin);
  removePluginResources(plugin.id);
  const resources: PluginResources = { fingerprint, unsubscribers: new Set() };
  pluginResources.set(plugin.id, resources);

  const load = (async () => {
    if (plugin.style) {
      const style = document.createElement('link');
      style.rel = 'stylesheet';
      style.href = pluginAssetURL(plugin.id, plugin.style, plugin.version);
      style.dataset.pluginId = plugin.id;
      resources.style = style;
      await loadElement(style, 'styles');
    }

    const script = document.createElement('script');
    script.src = pluginAssetURL(plugin.id, plugin.frontend || '', plugin.version);
    script.async = false;
    script.dataset.pluginId = plugin.id;
    script.dataset.pageId = plugin.page.id;
    script.dataset.pluginFingerprint = fingerprint;
    resources.script = script;
    await loadElement(script, 'page');

    const registration = registeredPages.get(plugin.id);
    if (!registration) throw new Error('Plugin script did not register its page');
    return registration;
  })();

  pendingLoads.set(plugin.id, load);
  void load.catch(() => removePluginResources(plugin.id, resources)).finally(() => {
    if (pendingLoads.get(plugin.id) === load) pendingLoads.delete(plugin.id);
  });
  return load;
}

function scopedHost(pluginId: string): PluginScopedHost {
  return Object.freeze({
    pluginId,
    invoke: <T,>(method: string, payload: unknown = {}) => hostAPI.invoke<T>(pluginId, method, payload),
    subscribe: (event: string, callback: (payload: unknown) => void) => subscribePluginEvent(pluginId, event, callback),
    ui: FanControlUI,
    icons: PLUGIN_ICONS,
    theme: {
      current: currentTheme,
      subscribe: (callback: (theme: PluginThemeSnapshot) => void) => subscribeTheme(pluginId, callback),
    },
    locale: {
      current: () => i18n.language,
      t: (key: string, options?: Record<string, unknown>) => String(i18n.t(key, options)),
      subscribe: (callback: (locale: string) => void) => subscribeLocale(pluginId, callback),
    },
    toast: pluginToast,
  });
}

const EMPTY_SNAPSHOT: PluginCatalogSnapshot = { revision: 0, plugins: [] };

export function useOfficialPluginCatalog() {
  const [snapshot, setSnapshot] = React.useState<PluginCatalogSnapshot>(EMPTY_SNAPSHOT);

  React.useEffect(() => {
    installPluginHost();
    let active = true;
    let latestRevision = -1;
    const applySnapshot = (next?: PluginCatalogSnapshot) => {
      if (!active || !next || !Array.isArray(next.plugins) || next.revision < latestRevision) return;
      latestRevision = next.revision;
      const normalized = { ...next, plugins: next.plugins || [] };
      reconcilePluginPages(normalized.plugins);
      setSnapshot(normalized);
    };
    const refresh = async () => {
      try {
        applySnapshot(await apiService.getPluginSnapshot());
      } catch (error) {
        console.error('Failed to load plugin catalog:', error);
      }
    };

    const offStatus = apiService.onPluginStatusUpdate(applySnapshot);
    const offCoreOK = apiService.onCoreServiceOK(() => {
      latestRevision = -1;
      void refresh();
    });
    const offCoreError = apiService.onCoreServiceError(() => {
      latestRevision = -1;
      reconcilePluginPages([]);
      setSnapshot(EMPTY_SNAPSHOT);
    });
    void refresh();

    return () => {
      active = false;
      offStatus();
      offCoreOK();
      offCoreError();
      reconcilePluginPages([]);
      if (window.FanControlPluginHost === hostAPI) delete window.FanControlPluginHost;
    };
  }, []);

  const plugins = React.useMemo(() => sortVisiblePluginPages(snapshot.plugins), [snapshot.plugins]);
  return { snapshot, plugins };
}

function PluginFailure({ error, onRetry }: { error: Error; onRetry: () => void }) {
  const { t } = useTranslation();
  return (
    <section role="alert" className="mx-auto mt-8 max-w-xl rounded-lg border border-destructive/30 bg-destructive/5 p-5">
      <div className="flex items-start gap-3">
        <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-destructive" />
        <div className="min-w-0 flex-1">
          <h2 className="text-sm font-semibold text-foreground">{t('pluginHost.loadFailed')}</h2>
          <p className="mt-1 break-words text-sm text-muted-foreground">{error.message}</p>
          <FanControlUI.Button
            type="button"
            variant="outline"
            size="sm"
            icon={<RotateCw className="h-4 w-4" />}
            className="mt-4"
            onClick={onRetry}
          >
            {t('pluginHost.retry')}
          </FanControlUI.Button>
        </div>
      </div>
    </section>
  );
}

class PluginErrorBoundary extends React.Component<
  { resetKey: string; onRetry: () => void; children: React.ReactNode },
  { error: Error | null }
> {
  state = { error: null as Error | null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidUpdate(previous: { resetKey: string }) {
    if (previous.resetKey !== this.props.resetKey && this.state.error) this.setState({ error: null });
  }

  render() {
    return this.state.error
      ? <PluginFailure error={this.state.error} onRetry={this.props.onRetry} />
      : this.props.children;
  }
}

export function PluginPageOutlet({ plugin }: { plugin: PluginCatalogEntry }) {
  const { t } = useTranslation();
  const [attempt, setAttempt] = React.useState(0);
  const [page, setPage] = React.useState<RegisteredPluginPage | null>(null);
  const [error, setError] = React.useState<Error | null>(null);
  const fingerprint = pluginFrontendFingerprint(plugin);
  const host = React.useMemo(() => scopedHost(plugin.id), [plugin.id]);

  const retry = React.useCallback(() => {
    removePluginResources(plugin.id);
    setAttempt((value) => value + 1);
  }, [plugin.id]);

  React.useEffect(() => {
    let active = true;
    setPage(null);
    setError(null);
    void loadPluginPage(plugin).then((registration) => {
      if (active) setPage(registration);
    }).catch((reason) => {
      if (active) setError(reason instanceof Error ? reason : new Error(String(reason)));
    });
    return () => {
      active = false;
    };
  }, [attempt, fingerprint, plugin.id]);

  if (error) return <PluginFailure error={error} onRetry={retry} />;
  if (!page) {
    return (
      <div aria-live="polite" className="flex min-h-56 items-center justify-center gap-2 text-sm text-muted-foreground">
        <LoaderCircle className="h-4 w-4 animate-spin" />
        <span>{t('pluginHost.loading')}</span>
      </div>
    );
  }

  const Page = page.component;
  return (
    <PluginErrorBoundary resetKey={`${fingerprint}:${attempt}`} onRetry={retry}>
      <div data-plugin-id={plugin.id}>
        <Page host={host} />
      </div>
    </PluginErrorBoundary>
  );
}
