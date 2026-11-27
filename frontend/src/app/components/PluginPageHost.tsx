'use client';

import { Component, type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { AlertTriangle, Puzzle, RefreshCw, Terminal } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  createScopedPluginHost,
  ensurePluginHost,
  getRegisteredPluginPage,
  loadPluginAssets,
  subscribePluginRegistry,
} from '../lib/plugin-host';
import { apiService } from '../services/api';
import type { PluginInfo } from '../types/app';
import ActionMenu from './settings/ActionMenu';
import { Badge, Button, Card } from './ui';

interface PluginPageHostProps {
  plugin: PluginInfo;
}

interface ErrorBoundaryState {
  error: Error | null;
}

class PluginRenderErrorBoundary extends Component<
  { pluginKey: string; onError: (msg: string) => void; children: ReactNode },
  ErrorBoundaryState
> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error) {
    this.props.onError(error.message || String(error));
  }

  componentDidUpdate(prevProps: { pluginKey: string }) {
    if (prevProps.pluginKey !== this.props.pluginKey && this.state.error) {
      this.setState({ error: null });
    }
  }

  render() {
    if (this.state.error) return null;
    return this.props.children;
  }
}

export default function PluginPageHost({ plugin }: PluginPageHostProps) {
  const { t } = useTranslation();
  const mountRef = useRef<HTMLDivElement | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loadingAsset, setLoadingAsset] = useState(false);
  const [registryTick, setRegistryTick] = useState(0);
  const [showLogs, setShowLogs] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);

  useEffect(() => subscribePluginRegistry(() => setRegistryTick((n) => n + 1)), []);

  const triggerLoad = useCallback(async () => {
    if (!plugin.frontend) return;
    setLoadingAsset(true);
    setLoadError(null);
    try {
      await loadPluginAssets(plugin);
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : String(error));
    } finally {
      setLoadingAsset(false);
    }
  }, [plugin]);

  useEffect(() => {
    void triggerLoad();
  }, [triggerLoad]);

  const registered = getRegisteredPluginPage(plugin.id);
  const scopedHost = useMemo(() => createScopedPluginHost(plugin), [plugin]);

  useEffect(() => {
    const container = mountRef.current;
    if (!container || !registered || registered.component || typeof registered.mount !== 'function') {
      return;
    }
    container.innerHTML = '';
    const host = ensurePluginHost();
    let unmount: void | (() => void);
    try {
      unmount = registered.mount(container, { plugin, host, scopedHost });
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : String(error));
    }
    return () => {
      if (typeof unmount === 'function') {
        try {
          unmount();
        } catch {
          // Keep host teardown resilient.
        }
      }
      container.innerHTML = '';
    };
  }, [plugin, registered, registryTick, scopedHost]);

  const fetchLogs = useCallback(async () => {
    try {
      const lines = await apiService.getPluginLogs(plugin.id);
      setLogs(lines);
    } catch {
      setLogs([]);
    }
  }, [plugin.id]);

  const statusBanner =
    plugin.state === 'restarting'
      ? t('controlPanel.plugins.host.reconnecting')
      : plugin.state === 'suspended' || plugin.state === 'suspending'
        ? t('controlPanel.plugins.host.suspended')
        : null;

  const RegisteredComponent = registered?.component;
  const hideHeader = Boolean(registered?.hideDefaultHeader && !loadError);

  return (
    <div data-plugin-id={plugin.id} className="space-y-4">
      {!hideHeader && (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary/12 text-primary">
              <Puzzle className="h-4.5 w-4.5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-lg font-semibold tracking-tight text-foreground">{plugin.name}</h2>
                {plugin.devMode && plugin.version && <Badge variant="default">v{plugin.version}</Badge>}
                <Badge variant={plugin.state === 'ready' ? 'success' : 'warning'}>
                  {t(`controlPanel.plugins.states.${plugin.state}`, { defaultValue: plugin.state })}
                </Badge>
              </div>
              {plugin.description && (
                <p className="text-xs text-muted-foreground">{plugin.description}</p>
              )}
            </div>
          </div>
          <ActionMenu
            variant="outline"
            size="sm"
            ariaLabel={t('controlPanel.plugins.moreActions')}
            items={[
              {
                id: 'logs',
                label: t('controlPanel.plugins.actions.logs'),
                icon: <Terminal className="h-3.5 w-3.5" />,
                onSelect: () => {
                  const next = !showLogs;
                  setShowLogs(next);
                  if (next) void fetchLogs();
                },
              },
              {
                id: 'reload',
                label: t('controlPanel.plugins.host.reload'),
                icon: <RefreshCw className="h-3.5 w-3.5" />,
                loading: loadingAsset,
                onSelect: () => { void triggerLoad(); },
              },
            ]}
          />
        </div>
      )}

      {statusBanner && (
        <div className="flex items-center gap-2 rounded-xl border border-amber-500/30 bg-amber-500/10 px-4 py-2.5 text-sm text-amber-700 dark:text-amber-300">
          <AlertTriangle className="h-4 w-4 shrink-0" />
          <span>{statusBanner}</span>
        </div>
      )}

      {!hideHeader && showLogs && (
        <Card className="p-3">
          <div className="mb-2 flex items-center justify-between text-xs font-medium text-muted-foreground">
            <span>{t('controlPanel.plugins.logsTitle', { name: plugin.name })}</span>
            <button
              type="button"
              onClick={() => { void fetchLogs(); }}
              className="text-primary hover:underline"
            >
              {t('controlPanel.plugins.refreshButton')}
            </button>
          </div>
          <pre className="max-h-44 overflow-auto rounded-lg bg-muted/60 p-2.5 font-mono text-xs text-foreground/85">
            {logs.length > 0 ? logs.join('\n') : t('controlPanel.plugins.logsEmpty')}
          </pre>
        </Card>
      )}

      {loadError ? (
        <Card className="p-6 text-center">
          <AlertTriangle className="mx-auto mb-2 h-6 w-6 text-destructive" />
          <div className="text-sm font-medium text-foreground">
            {t('controlPanel.plugins.host.loadFailed')}
          </div>
          <div className="mt-1 text-xs text-muted-foreground">{loadError}</div>
          <div className="mt-4">
            <Button size="sm" onClick={() => { void triggerLoad(); }}>
              {t('controlPanel.plugins.host.retry')}
            </Button>
          </div>
        </Card>
      ) : !registered ? (
        <Card className="p-6 text-center text-sm text-muted-foreground">
          {loadingAsset
            ? t('controlPanel.plugins.host.loading')
            : t('controlPanel.plugins.host.waitingMount')}
        </Card>
      ) : null}

      {RegisteredComponent && !loadError ? (
        <PluginRenderErrorBoundary
          pluginKey={`${plugin.id}:${plugin.version}:${registryTick}`}
          onError={(msg) => setLoadError(msg)}
        >
          <RegisteredComponent plugin={plugin} host={scopedHost} />
        </PluginRenderErrorBoundary>
      ) : (
        <div
          ref={mountRef}
          className={registered && !RegisteredComponent && !loadError ? 'min-h-[120px]' : 'hidden'}
        />
      )}
    </div>
  );
}
