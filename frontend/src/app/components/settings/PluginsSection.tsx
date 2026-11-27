'use client';

import { Component, type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { AnimatePresence, m } from 'framer-motion';
import { BrowserOpenURL } from '../../../../wailsjs/runtime/runtime';
import {
  AlertTriangle,
  ChevronDown,
  Code2,
  Cpu,
  ExternalLink,
  FolderArchive,
  FolderOpen,
  Laptop,
  Plus,
  Puzzle,
  RefreshCw,
  RotateCw,
  Search,
  SlidersHorizontal,
  Store,
  Terminal,
  Trash2,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { types } from '../../../../wailsjs/go/models';
import { BRAND } from '../../lib/brand';
import {
  buildPluginAssetURL,
  createScopedPluginHost,
  ensurePluginHost,
  getRegisteredPluginPanel,
  loadPluginAssets,
  subscribePluginRegistry,
  unloadPluginAssets,
} from '../../lib/plugin-host';
import { apiService } from '../../services/api';
import { useAppStore } from '../../store/app-store';
import type { PluginInfo } from '../../types/app';
import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  ToggleSwitch,
} from '../ui';
import ActionMenu, { type ActionMenuItem } from './ActionMenu';
import MarketplacePanel from './MarketplacePanel';
import PluginDeclarativeSettings from './PluginDeclarativeSettings';
import { Section } from './SettingLayout';

interface PluginsSectionProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
}

type ExternalPluginFilter = 'all' | 'enabled' | 'disabled' | 'sidecar' | 'inproc';

const LEGION_FNQ_PLUGIN_ID = 'lenovo-legion-fnq-power-mode';

function getErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function openExternalUrl(url: string) {
  try {
    BrowserOpenURL(url);
  } catch {
    window.open(url, '_blank', 'noopener,noreferrer');
  }
}

function pluginStateBadgeVariant(state: string): 'success' | 'warning' | 'error' | 'info' | 'default' {
  switch (state) {
    case 'ready':
      return 'success';
    case 'discovered':
    case 'starting':
      return 'info';
    case 'incompatible':
    case 'failed':
      return 'error';
    case 'unsupported':
    case 'restarting':
    case 'suspended':
    case 'suspending':
    case 'pending_restart':
      return 'warning';
    default:
      return 'default';
  }
}

class PanelErrorBoundary extends Component<
  { pluginKey: string; onError: (msg: string) => void; children: ReactNode },
  { error: Error | null }
> {
  state: { error: Error | null } = { error: null };

  static getDerivedStateFromError(error: Error) {
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

function PluginCustomSettingsPanel({ plugin }: { plugin: PluginInfo }) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const [tick, setTick] = useState(0);
  const [panelError, setPanelError] = useState<string | null>(null);

  useEffect(() => subscribePluginRegistry(() => setTick((n) => n + 1)), []);

  useEffect(() => {
    setPanelError(null);
    if (!plugin.frontend || plugin.uiSlot !== 'settings-panel' || !plugin.enabled) {
      return;
    }
    void loadPluginAssets(plugin).catch((error) => {
      setPanelError(error instanceof Error ? error.message : String(error));
    });
  }, [plugin]);

  const panel = getRegisteredPluginPanel(plugin.id);
  const PanelComponent = panel?.component;
  const scopedHost = useMemo(() => createScopedPluginHost(plugin), [plugin]);

  useEffect(() => {
    const el = hostRef.current;
    if (!el || !panel || PanelComponent || typeof panel.mount !== 'function' || !plugin.enabled) return;
    el.innerHTML = '';
    const host = ensurePluginHost();
    let dispose: void | (() => void);
    try {
      dispose = panel.mount(el, { plugin, host, scopedHost });
    } catch (error) {
      setPanelError(error instanceof Error ? error.message : String(error));
    }
    return () => {
      if (typeof dispose === 'function') {
        try {
          dispose();
        } catch {
          // Ignore plugin panel cleanup failure.
        }
      }
      el.innerHTML = '';
    };
  }, [PanelComponent, panel, plugin, scopedHost, tick]);

  if (!plugin.enabled || plugin.uiSlot !== 'settings-panel' || !plugin.frontend) return null;
  if (panelError) {
    return (
      <div className="mt-3 rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-2.5 text-xs text-destructive">
        {panelError}
      </div>
    );
  }
  if (!panel) return null;
  if (PanelComponent) {
    return (
      <div data-plugin-id={plugin.id} className="mt-3">
        <PanelErrorBoundary
          pluginKey={`${plugin.id}:${plugin.version}:${tick}`}
          onError={(msg) => setPanelError(msg)}
        >
          <PanelComponent plugin={plugin} host={scopedHost} />
        </PanelErrorBoundary>
      </div>
    );
  }
  return <div ref={hostRef} data-plugin-id={plugin.id} className="mt-3" />;
}

export default function PluginsSection({ config, onConfigChange }: PluginsSectionProps) {
  const { t } = useTranslation();
  const legionFnQSupported = useAppStore((state) => state.legionFnQSupported);
  const setActiveTab = useAppStore((state) => state.setActiveTab);
  const [marketOpen, setMarketOpen] = useState(false);
  const [marketRefreshNonce, setMarketRefreshNonce] = useState(0);
  const [plugins, setPlugins] = useState<PluginInfo[]>([]);
  const [refreshing, setRefreshing] = useState(false);
  const [openingFolder, setOpeningFolder] = useState(false);
  const [importingZip, setImportingZip] = useState(false);
  const [restartingCore, setRestartingCore] = useState(false);
  const [togglingMap, setTogglingMap] = useState<Record<string, boolean>>({});
  const [searchQuery, setSearchQuery] = useState('');
  const [runtimeFilter, setRuntimeFilter] = useState<ExternalPluginFilter>('all');
  const [collapsedSettingsMap, setCollapsedSettingsMap] = useState<Record<string, boolean>>({});
  const [highlightedPluginId, setHighlightedPluginId] = useState<string | null>(null);
  const [devPathInput, setDevPathInput] = useState('');
  const [showDevBox, setShowDevBox] = useState(false);
  const [logsPlugin, setLogsPlugin] = useState<PluginInfo | null>(null);
  const [logsLines, setLogsLines] = useState<string[]>([]);
  const [uninstallTarget, setUninstallTarget] = useState<PluginInfo | null>(null);
  const [removeUninstallData, setRemoveUninstallData] = useState(false);
  const [uninstalling, setUninstalling] = useState(false);

  useEffect(() => {
    if (!highlightedPluginId) return undefined;
    const timer = window.setTimeout(() => setHighlightedPluginId(null), 2200);
    return () => window.clearTimeout(timer);
  }, [highlightedPluginId]);

  const reloadPlugins = useCallback(async () => {
    try {
      const list = await apiService.getPlugins();
      setPlugins(list);
    } catch {
      // Preserve existing state on transient read failures.
    }
  }, []);

  useEffect(() => {
    let active = true;
    void apiService.getPlugins()
      .then((list) => {
        if (active) setPlugins(list);
      })
      .catch(() => {});
    const unsubscribe = apiService.onPluginsUpdate((list) => {
      if (active && Array.isArray(list)) {
        setPlugins(list);
      }
    });
    return () => {
      active = false;
      unsubscribe();
    };
  }, []);

  useEffect(() => {
    for (const plugin of plugins) {
      if (plugin.enabled && plugin.uiSlot === 'settings-panel' && plugin.frontend) {
        void loadPluginAssets(plugin).catch(() => {});
      }
    }
  }, [plugins]);

  const handleRefresh = useCallback(async () => {
    setRefreshing(true);
    try {
      const list = await apiService.refreshPlugins();
      setPlugins(list);
    } catch (error) {
      toast.error(t('controlPanel.alerts.settingsOperationFailed', { error: getErrorMessage(error) }));
    } finally {
      setRefreshing(false);
    }
  }, [t]);

  const handleOpenFolder = useCallback(async () => {
    setOpeningFolder(true);
    try {
      await apiService.openPluginsFolder();
    } catch (error) {
      toast.error(t('controlPanel.alerts.settingsOperationFailed', { error: getErrorMessage(error) }));
    } finally {
      setOpeningFolder(false);
    }
  }, [t]);

  const handleImportLocalZip = useCallback(async () => {
    setImportingZip(true);
    try {
      const res = await apiService.importMarketplacePackage('');
      if (!res) return;
      if (res.kind === 'theme') {
        window.dispatchEvent(new CustomEvent('fancontrol:themes-updated'));
        toast.success(t('controlPanel.plugins.marketplace.themeInstallSuccess', { name: res.name }));
      } else {
        if (res.restartRequired) {
          toast.info(t('controlPanel.plugins.marketplace.restartNeededToast', { name: res.name }));
        } else {
          toast.success(t('controlPanel.plugins.marketplace.installSuccessToast', { name: res.name }));
        }
        await reloadPlugins();
      }
    } catch (error) {
      toast.error(t('controlPanel.plugins.marketplace.installFailedToast', {
        error: getErrorMessage(error),
      }));
    } finally {
      setImportingZip(false);
    }
  }, [reloadPlugins, t]);

  const handleManageInstalled = useCallback((pluginId: string) => {
    setSearchQuery('');
    setRuntimeFilter('all');
    setMarketOpen(false);
    setHighlightedPluginId(pluginId);
    setCollapsedSettingsMap((prev) => ({ ...prev, [pluginId]: false }));
    void reloadPlugins();
  }, [reloadPlugins]);

  const handleRequestUninstallById = useCallback(async (pluginId: string) => {
    let target = plugins.find((item) => item.id === pluginId && item.kind === 'external')
      || plugins.find((item) => item.id === pluginId);
    if (!target) {
      try {
        const fresh = await apiService.getPlugins();
        setPlugins(fresh);
        target = fresh.find((item) => item.id === pluginId && item.kind === 'external')
          || fresh.find((item) => item.id === pluginId);
      } catch {
        // Preserve existing state on transient read failures.
      }
    }
    if (!target || target.kind === 'builtin') return;
    setRemoveUninstallData(false);
    setUninstallTarget(target);
  }, [plugins]);

  const handleToggleExternalPlugin = useCallback(async (plugin: PluginInfo, enabled: boolean) => {
    setTogglingMap((prev) => ({ ...prev, [plugin.id]: true }));
    try {
      const list = await apiService.setPluginEnabled(plugin.id, enabled);
      setPlugins(list);
      if (!enabled) {
        unloadPluginAssets(plugin.id);
      }
      if (plugin.runtime === 'inproc-dll') {
        toast.info(t('controlPanel.plugins.pendingRestartToast'));
      }
    } catch (error) {
      toast.error(t('controlPanel.alerts.settingsOperationFailed', { error: getErrorMessage(error) }));
    } finally {
      setTogglingMap((prev) => ({ ...prev, [plugin.id]: false }));
    }
  }, [t]);

  const handleRestartPlugin = useCallback(async (pluginId: string) => {
    setTogglingMap((prev) => ({ ...prev, [pluginId]: true }));
    try {
      const list = await apiService.restartPlugin(pluginId);
      setPlugins(list);
    } catch (error) {
      toast.error(t('controlPanel.alerts.settingsOperationFailed', { error: getErrorMessage(error) }));
    } finally {
      setTogglingMap((prev) => ({ ...prev, [pluginId]: false }));
    }
  }, [t]);

  const handleOpenLogs = useCallback(async (plugin: PluginInfo) => {
    setLogsPlugin(plugin);
    try {
      const lines = await apiService.getPluginLogs(plugin.id);
      setLogsLines(lines);
    } catch {
      setLogsLines([]);
    }
  }, []);

  const handleConfirmUninstall = useCallback(async () => {
    if (!uninstallTarget) return;
    setUninstalling(true);
    try {
      const res = await apiService.uninstallPlugin(uninstallTarget.id, removeUninstallData);
      unloadPluginAssets(uninstallTarget.id);
      if (res?.restartRequired) {
        toast.info(t('controlPanel.plugins.pendingRestartToast'));
      } else {
        toast.success(t('controlPanel.plugins.uninstallSuccess', { name: uninstallTarget.name }));
      }
      setUninstallTarget(null);
      setRemoveUninstallData(false);
      await reloadPlugins();
      setMarketRefreshNonce((n) => n + 1);
    } catch (error) {
      toast.error(t('controlPanel.alerts.settingsOperationFailed', { error: getErrorMessage(error) }));
    } finally {
      setUninstalling(false);
    }
  }, [reloadPlugins, removeUninstallData, t, uninstallTarget]);

  const handleRestartCore = useCallback(async () => {
    setRestartingCore(true);
    try {
      await apiService.restartCore(false);
    } catch (error) {
      toast.error(t('controlPanel.alerts.settingsOperationFailed', { error: getErrorMessage(error) }));
      setRestartingCore(false);
    }
  }, [t]);

  const devPaths = useMemo(
    () => (Array.isArray((config as any)?.pluginDevPaths) ? ((config as any).pluginDevPaths as string[]) : []),
    [config],
  );

  const handleUpdateDevPaths = useCallback(async (nextPaths: string[]) => {
    try {
      const list = await apiService.setPluginDevPaths(nextPaths);
      setPlugins(list);
      onConfigChange(types.AppConfig.createFrom({
        ...config,
        pluginDevPaths: nextPaths,
      }));
    } catch (error) {
      toast.error(t('controlPanel.alerts.settingsOperationFailed', { error: getErrorMessage(error) }));
    }
  }, [config, onConfigChange, t]);

  const legionConfig = useMemo(() => ({
    enabled: Boolean(config?.legionFnQ?.enabled),
    takeOverFan: Boolean(config?.legionFnQ?.takeOverFan),
    modeMapping: config?.legionFnQ?.modeMapping || {},
  }), [config?.legionFnQ]);

  const handleLegionUpdate = useCallback(async (patch: { enabled?: boolean; takeOverFan?: boolean }) => {
    setTogglingMap((prev) => ({ ...prev, [LEGION_FNQ_PLUGIN_ID]: true }));
    try {
      const nextLegion = {
        ...legionConfig,
        ...patch,
      };
      const newCfg = types.AppConfig.createFrom({
        ...config,
        legionFnQ: nextLegion,
      });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch (error) {
      toast.error(t('controlPanel.legionFnQ.toasts.saveFailed', { error: getErrorMessage(error) }));
    } finally {
      setTogglingMap((prev) => ({ ...prev, [LEGION_FNQ_PLUGIN_ID]: false }));
    }
  }, [config, legionConfig, onConfigChange, t]);

  const fnqPlugin = useMemo(() => plugins.find((item) => item.id === LEGION_FNQ_PLUGIN_ID), [plugins]);
  const externalPlugins = useMemo(() => plugins.filter((item) => item.kind === 'external'), [plugins]);
  const runningExternalCount = useMemo(
    () => externalPlugins.filter((item) => item.running || item.state === 'ready').length,
    [externalPlugins],
  );
  const filteredExternalPlugins = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    return externalPlugins.filter((plugin) => {
      if (runtimeFilter === 'enabled' && !plugin.enabled) return false;
      if (runtimeFilter === 'disabled' && plugin.enabled) return false;
      if (runtimeFilter === 'sidecar' && plugin.runtime !== 'sidecar') return false;
      if (runtimeFilter === 'inproc' && plugin.runtime !== 'inproc-dll') return false;
      if (!q) return true;
      return [plugin.name, plugin.id, plugin.description]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(q);
    });
  }, [externalPlugins, runtimeFilter, searchQuery]);
  const hasPendingRestart = useMemo(() => plugins.some((item) => item.pendingRestart), [plugins]);

  const fnqAvailable = fnqPlugin ? fnqPlugin.supported : legionFnQSupported;
  const fnqState = fnqPlugin?.state || (fnqAvailable ? (legionConfig.enabled ? 'ready' : 'disabled') : 'unsupported');

  const filterOptions: Array<{ id: ExternalPluginFilter; label: string }> = useMemo(() => {
    const base: Array<{ id: ExternalPluginFilter; label: string }> = [
      { id: 'all', label: t('controlPanel.plugins.filters.all') },
      { id: 'enabled', label: t('controlPanel.plugins.filters.enabled') },
      { id: 'disabled', label: t('controlPanel.plugins.filters.disabled') },
    ];
    if (showDevBox) {
      base.push(
        { id: 'sidecar', label: t('controlPanel.plugins.filters.sidecar') },
        { id: 'inproc', label: t('controlPanel.plugins.filters.inproc') },
      );
    }
    return base;
  }, [showDevBox, t]);

  const showToolbar = externalPlugins.length > 3 || searchQuery.trim() !== '' || runtimeFilter !== 'all';

  const headerMenuItems: ActionMenuItem[] = useMemo(() => [
    { type: 'label', label: t('controlPanel.plugins.menuGroups.store') },
    {
      id: 'market',
      label: t('controlPanel.plugins.browseMarketButton'),
      icon: <Store className="h-3.5 w-3.5" />,
      onSelect: () => {
        setMarketRefreshNonce((n) => n + 1);
        setMarketOpen(true);
      },
    },
    {
      id: 'import-zip',
      label: t('controlPanel.plugins.marketplace.importZip'),
      icon: <FolderArchive className="h-3.5 w-3.5" />,
      loading: importingZip,
      onSelect: () => { void handleImportLocalZip(); },
    },
    {
      id: 'open-repo',
      label: t('controlPanel.plugins.marketplace.openRepository'),
      icon: <ExternalLink className="h-3.5 w-3.5" />,
      onSelect: () => openExternalUrl(BRAND.pluginsRepositoryUrl),
    },
    { type: 'separator' },
    { type: 'label', label: t('controlPanel.plugins.menuGroups.manage') },
    {
      id: 'refresh',
      label: t('controlPanel.plugins.refreshButton'),
      icon: <RefreshCw className="h-3.5 w-3.5" />,
      loading: refreshing,
      onSelect: () => { void handleRefresh(); },
    },
    {
      id: 'open-folder',
      label: t('controlPanel.plugins.openFolderButton'),
      icon: <FolderOpen className="h-3.5 w-3.5" />,
      loading: openingFolder,
      onSelect: () => { void handleOpenFolder(); },
    },
    {
      id: 'dev-mode',
      label: t('controlPanel.plugins.devModeButton'),
      icon: <Code2 className="h-3.5 w-3.5" />,
      active: showDevBox,
      onSelect: () => setShowDevBox((v) => !v),
    },
  ], [handleImportLocalZip, handleOpenFolder, handleRefresh, importingZip, openingFolder, refreshing, showDevBox, t]);

  return (
    <div className="space-y-5">
      {hasPendingRestart && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-amber-500/35 bg-amber-500/10 px-4 py-3 text-sm text-amber-800 dark:text-amber-200">
          <div className="flex items-center gap-2">
            <AlertTriangle className="h-4 w-4 shrink-0" />
            <span>{t('controlPanel.plugins.pendingRestartBanner')}</span>
          </div>
          <Button
            size="sm"
            loading={restartingCore}
            onClick={() => { void handleRestartCore(); }}
          >
            {t('controlPanel.plugins.restartCoreNow')}
          </Button>
        </div>
      )}

      <Section
        title={t('controlPanel.plugins.centerTitle')}
        description={t('controlPanel.plugins.centerDescription')}
        icon={Puzzle}
        iconTone="purple"
        action={(
          <div className="flex items-center gap-2">
            {externalPlugins.length > 0 && (
              <Badge variant={runningExternalCount > 0 ? 'success' : 'default'}>
                {t('controlPanel.plugins.summaryBadge', {
                  running: runningExternalCount,
                  total: externalPlugins.length,
                })}
              </Badge>
            )}
            <ActionMenu
              label={t('controlPanel.plugins.menuButton')}
              icon={<SlidersHorizontal className="h-3.5 w-3.5" />}
              showChevron
              ariaLabel={t('controlPanel.plugins.menuButton')}
              items={headerMenuItems}
            />
          </div>
        )}
      >
        <AnimatePresence initial={false}>
          {showDevBox && (
            <m.div
              key="dev-box"
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              transition={{ duration: 0.2, ease: [0.22, 1, 0.36, 1] }}
              className="overflow-hidden"
            >
              <div className="bg-background/40 px-5 py-3">
                <div className="mb-2 text-xs font-medium text-foreground">
                  {t('controlPanel.plugins.devModeTitle')}
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <input
                    type="text"
                    value={devPathInput}
                    onChange={(e) => setDevPathInput(e.target.value)}
                    placeholder={t('controlPanel.plugins.devModePlaceholder')}
                    className="h-8 min-w-[240px] flex-1 rounded-lg border border-border bg-card px-2.5 text-xs text-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
                  />
                  <Button
                    size="sm"
                    variant="outline"
                    icon={<Plus className="h-3.5 w-3.5" />}
                    onClick={() => {
                      const trimmed = devPathInput.trim();
                      if (!trimmed || devPaths.includes(trimmed)) return;
                      setDevPathInput('');
                      void handleUpdateDevPaths([...devPaths, trimmed]);
                    }}
                  >
                    {t('controlPanel.plugins.devModeAdd')}
                  </Button>
                </div>
                {devPaths.length > 0 && (
                  <div className="mt-2 space-y-1.5">
                    {devPaths.map((p) => (
                      <div key={p} className="flex items-center justify-between gap-2 rounded-lg border border-border/60 bg-card/70 px-2.5 py-1.5 text-xs">
                        <span className="truncate font-mono text-muted-foreground">{p}</span>
                        <button
                          type="button"
                          onClick={() => { void handleUpdateDevPaths(devPaths.filter((item) => item !== p)); }}
                          className="cursor-pointer text-destructive hover:underline"
                        >
                          {t('controlPanel.plugins.actions.uninstall')}
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </m.div>
          )}
        </AnimatePresence>

        {externalPlugins.length === 0 ? (
          <div className="px-5 py-6 text-center text-sm text-muted-foreground">
            <div>{t('controlPanel.plugins.externalEmptyTitle')}</div>
            <div className="mt-1 text-xs text-muted-foreground/80">
              {t('controlPanel.plugins.externalEmptyHint')}
            </div>
            <div className="mt-3">
              <Button
                size="sm"
                variant="outline"
                icon={<Store className="h-3.5 w-3.5" />}
                onClick={() => {
                  setMarketRefreshNonce((n) => n + 1);
                  setMarketOpen(true);
                }}
              >
                {t('controlPanel.plugins.browseMarketButton')}
              </Button>
            </div>
          </div>
        ) : (
          <>
            {showToolbar && (
              <div className="flex flex-wrap items-center justify-between gap-2 bg-background/30 px-5 py-2.5">
                <div className="relative min-w-[200px] flex-1 max-w-xs">
                  <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
                  <input
                    type="text"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    placeholder={t('controlPanel.plugins.searchPlaceholder')}
                    className="h-8 w-full rounded-lg border border-border bg-card pl-8 pr-2.5 text-xs text-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
                  />
                </div>
                <div className="flex flex-wrap items-center gap-1">
                  {filterOptions.map((opt) => {
                    const active = runtimeFilter === opt.id;
                    return (
                      <button
                        key={opt.id}
                        type="button"
                        onClick={() => setRuntimeFilter(opt.id)}
                        className={`h-7 cursor-pointer rounded-lg px-2.5 text-xs font-medium transition-colors ${
                          active
                            ? 'bg-primary text-primary-foreground shadow-sm'
                            : 'bg-card/80 text-muted-foreground hover:bg-muted/70 hover:text-foreground'
                        }`}
                      >
                        {opt.label}
                      </button>
                    );
                  })}
                </div>
              </div>
            )}

            {filteredExternalPlugins.length === 0 ? (
              <div className="px-5 py-6 text-center text-sm text-muted-foreground">
                {t('controlPanel.plugins.noFilterMatches')}
              </div>
            ) : (
              filteredExternalPlugins.map((plugin) => {
                const incompatible = plugin.state === 'incompatible';
                const isSidecar = plugin.runtime === 'sidecar';
                const showTechMeta = showDevBox || Boolean(plugin.devMode);
                const hasAssetIcon = Boolean(
                  plugin.icon && /\.(png|svg|webp|jpe?g)$/i.test(plugin.icon.trim()),
                );
                const hasCustomSettings =
                  plugin.uiSlot === 'settings-panel' && Boolean(plugin.frontend) && plugin.enabled;
                const hasDeclarativeSettings =
                  Array.isArray(plugin.settings) && plugin.settings.length > 0 && !hasCustomSettings;
                const hasExpandableSettings = hasCustomSettings || hasDeclarativeSettings;
                const isSettingsExpanded = hasCustomSettings
                  ? collapsedSettingsMap[plugin.id] !== true
                  : collapsedSettingsMap[plugin.id] === false;
                const canOpenSidebarPage =
                  plugin.uiSlot === 'sidebar-page' && plugin.enabled && !incompatible;
                const isHighlighted = highlightedPluginId === plugin.id;

                const menuItems: ActionMenuItem[] = [];
                if (canOpenSidebarPage && hasExpandableSettings) {
                  menuItems.push({
                    id: 'settings',
                    label: isSettingsExpanded
                      ? t('controlPanel.plugins.actions.collapseSettings')
                      : t('controlPanel.plugins.actions.settings'),
                    icon: <SlidersHorizontal className="h-3.5 w-3.5" />,
                    onSelect: () => {
                      setCollapsedSettingsMap((prev) => ({
                        ...prev,
                        [plugin.id]: isSettingsExpanded,
                      }));
                    },
                  });
                }
                if (isSidecar) {
                  menuItems.push(
                    {
                      id: 'restart',
                      label: t('controlPanel.plugins.actions.restart'),
                      icon: <RotateCw className="h-3.5 w-3.5" />,
                      onSelect: () => { void handleRestartPlugin(plugin.id); },
                    },
                    {
                      id: 'logs',
                      label: t('controlPanel.plugins.actions.logs'),
                      icon: <Terminal className="h-3.5 w-3.5" />,
                      onSelect: () => { void handleOpenLogs(plugin); },
                    },
                  );
                }
                if (!plugin.devMode) {
                  if (menuItems.length > 0) {
                    menuItems.push({ type: 'separator' });
                  }
                  menuItems.push({
                    id: 'uninstall',
                    label: t('controlPanel.plugins.actions.uninstall'),
                    icon: <Trash2 className="h-3.5 w-3.5" />,
                    danger: true,
                    onSelect: () => {
                      setRemoveUninstallData(false);
                      setUninstallTarget(plugin);
                    },
                  });
                }

                return (
                  <div
                    key={plugin.id}
                    className={`px-5 py-4 transition-colors ${
                      isHighlighted ? 'bg-primary/5 ring-1 ring-inset ring-primary/35' : ''
                    }`}
                  >
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <div className="flex min-w-0 flex-1 items-start gap-3">
                        <div
                          data-theme-ui="setting-row-icon"
                          data-icon-tone={isSidecar ? 'blue' : 'amber'}
                          className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-[11px] bg-muted/70 text-muted-foreground shadow-inner shadow-white/20"
                        >
                          {hasAssetIcon ? (
                            <img
                              src={buildPluginAssetURL(plugin.id, plugin.icon!, plugin.version)}
                              alt={plugin.name}
                              className="h-6 w-6 object-contain"
                            />
                          ) : (
                            <Puzzle className="h-4 w-4" />
                          )}
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="text-[15px] font-semibold tracking-[-0.01em] text-foreground">
                              {plugin.name}
                            </span>
                            {showTechMeta && plugin.version && <Badge variant="default">v{plugin.version}</Badge>}
                            {showTechMeta && plugin.runtime && (
                              <Badge variant={plugin.runtime === 'inproc-dll' ? 'warning' : 'info'}>
                                {t(`controlPanel.plugins.runtimes.${plugin.runtime}`, { defaultValue: plugin.runtime })}
                              </Badge>
                            )}
                            {plugin.devMode && (
                              <Badge variant="info">{t('controlPanel.plugins.devModeBadge')}</Badge>
                            )}
                            <Badge variant={pluginStateBadgeVariant(plugin.state)}>
                              {t(`controlPanel.plugins.states.${plugin.state}`, { defaultValue: plugin.state })}
                            </Badge>
                          </div>
                          <div className="mt-0.5 text-[13px] leading-[1.45] text-muted-foreground">
                            {plugin.description || plugin.id}
                          </div>
                          {plugin.reason && (
                            <div className="mt-1 text-xs text-amber-600 dark:text-amber-400">
                              {plugin.reason}
                            </div>
                          )}
                          {plugin.lastError && (
                            <div className="mt-1 text-xs text-destructive">
                              {plugin.lastError}
                            </div>
                          )}
                        </div>
                      </div>

                      <div className="flex shrink-0 items-center gap-2">
                        {canOpenSidebarPage ? (
                          <Button
                            variant="outline"
                            size="sm"
                            icon={<ExternalLink className="h-3.5 w-3.5" />}
                            onClick={() => setActiveTab(`plugin:${plugin.id}`)}
                          >
                            {t('controlPanel.plugins.actions.openPage')}
                          </Button>
                        ) : hasExpandableSettings ? (
                          <Button
                            variant="outline"
                            size="sm"
                            icon={
                              <ChevronDown
                                className={`h-3.5 w-3.5 transition-transform duration-200 ${
                                  isSettingsExpanded ? 'rotate-180' : ''
                                }`}
                              />
                            }
                            onClick={() => {
                              setCollapsedSettingsMap((prev) => ({
                                ...prev,
                                [plugin.id]: isSettingsExpanded,
                              }));
                            }}
                          >
                            {isSettingsExpanded
                              ? t('controlPanel.plugins.actions.collapseSettings')
                              : t('controlPanel.plugins.actions.settings')}
                          </Button>
                        ) : null}

                        {menuItems.length > 0 && (
                          <ActionMenu
                            variant="ghost"
                            size="sm"
                            ariaLabel={t('controlPanel.plugins.moreActions')}
                            items={menuItems}
                          />
                        )}

                        <ToggleSwitch
                          enabled={plugin.enabled}
                          onChange={(enabled) => { void handleToggleExternalPlugin(plugin, enabled); }}
                          disabled={incompatible}
                          loading={togglingMap[plugin.id]}
                          size="sm"
                          color="green"
                        />
                      </div>
                    </div>

                    <AnimatePresence initial={false}>
                      {isSettingsExpanded && (hasDeclarativeSettings || hasCustomSettings) && (
                        <m.div
                          key="plugin-settings"
                          initial={{ opacity: 0, height: 0 }}
                          animate={{ opacity: 1, height: 'auto' }}
                          exit={{ opacity: 0, height: 0 }}
                          transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
                          className="overflow-hidden"
                        >
                          <div className="flow-root">
                            {hasDeclarativeSettings && (
                              <PluginDeclarativeSettings pluginId={plugin.id} fields={plugin.settings!} />
                            )}

                            {hasCustomSettings && (
                              <PluginCustomSettingsPanel plugin={plugin} />
                            )}
                          </div>
                        </m.div>
                      )}
                    </AnimatePresence>
                  </div>
                );
              })
            )}
          </>
        )}
      </Section>

      <Section
        title={t('controlPanel.plugins.builtinGroupTitle')}
        description={t('controlPanel.plugins.builtinGroupDescription')}
        icon={Cpu}
        iconTone="blue"
      >
        <div className="px-5 py-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <div
                data-theme-ui="setting-row-icon"
                data-icon-tone="amber"
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[11px] bg-muted/70 text-muted-foreground shadow-inner shadow-white/20"
              >
                <Laptop className="h-4 w-4" />
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-[15px] font-semibold tracking-[-0.01em] text-foreground">
                    {t('controlPanel.legionFnQ.sectionTitle')}
                  </span>
                  <Badge variant={pluginStateBadgeVariant(fnqState)}>
                    {t(`controlPanel.plugins.states.${fnqState}`, { defaultValue: fnqState })}
                  </Badge>
                </div>
                <div className="mt-0.5 text-[13px] leading-[1.45] text-muted-foreground">
                  {fnqAvailable
                    ? t('controlPanel.legionFnQ.enableDescription')
                    : (fnqPlugin?.reason || t('controlPanel.plugins.unsupportedHost'))}
                </div>
              </div>
            </div>
            <ToggleSwitch
              enabled={fnqAvailable && legionConfig.enabled}
              onChange={(enabled) => { void handleLegionUpdate({ enabled }); }}
              disabled={!fnqAvailable}
              loading={togglingMap[LEGION_FNQ_PLUGIN_ID]}
              size="sm"
              color="green"
            />
          </div>

          {fnqAvailable && (
            <div className="mt-3 rounded-xl border border-border/70 bg-background/45 px-4 py-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium text-foreground">
                    {t('controlPanel.legionFnQ.takeOverTitle')}
                  </div>
                  <div className="mt-0.5 text-xs text-muted-foreground">
                    {t('controlPanel.legionFnQ.takeOverDescription')}
                  </div>
                </div>
                <ToggleSwitch
                  enabled={legionConfig.takeOverFan}
                  onChange={(takeOverFan) => { void handleLegionUpdate({ takeOverFan }); }}
                  disabled={!legionConfig.enabled}
                  loading={togglingMap[LEGION_FNQ_PLUGIN_ID]}
                  size="sm"
                  color="blue"
                />
              </div>
            </div>
          )}
        </div>
      </Section>

      <Dialog open={marketOpen} onOpenChange={setMarketOpen}>
        <DialogContent className="flex max-h-[85vh] max-w-3xl flex-col overflow-hidden p-0 sm:max-w-[780px]">
          <div className="border-b border-border/70 px-6 pb-4 pt-5 pr-12">
            <DialogHeader className="text-left">
              <DialogTitle>{t('controlPanel.plugins.marketplace.pluginDialogTitle')}</DialogTitle>
              <DialogDescription>{t('controlPanel.plugins.marketplace.pluginDialogDescription')}</DialogDescription>
            </DialogHeader>
          </div>
          <div className="flex-1 overflow-y-auto">
            <MarketplacePanel
              embedded
              mode="plugins"
              config={config}
              onConfigChange={onConfigChange}
              refreshNonce={marketRefreshNonce}
              onPluginsChanged={() => { void reloadPlugins(); }}
              onManageInstalled={handleManageInstalled}
              onRequestUninstall={(pluginId) => { void handleRequestUninstallById(pluginId); }}
            />
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(logsPlugin)} onOpenChange={(open) => !open && setLogsPlugin(null)}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {logsPlugin ? t('controlPanel.plugins.logsTitle', { name: logsPlugin.name }) : ''}
            </DialogTitle>
          </DialogHeader>
          <pre className="max-h-72 overflow-auto rounded-lg bg-muted p-3 font-mono text-xs text-foreground/85">
            {logsLines.length > 0 ? logsLines.join('\n') : t('controlPanel.plugins.logsEmpty')}
          </pre>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setLogsPlugin(null)}>
              {t('appShell.titleBar.close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(uninstallTarget)} onOpenChange={(open) => !uninstalling && !open && setUninstallTarget(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {uninstallTarget ? t('controlPanel.plugins.uninstallDialogTitle', { name: uninstallTarget.name }) : ''}
            </DialogTitle>
            <DialogDescription>
              {uninstallTarget?.runtime === 'inproc-dll'
                ? t('controlPanel.plugins.uninstallInprocHint')
                : t('controlPanel.plugins.uninstallSidecarHint')}
            </DialogDescription>
          </DialogHeader>
          <label className="flex cursor-pointer items-center gap-2 text-sm text-foreground">
            <input
              type="checkbox"
              checked={removeUninstallData}
              onChange={(e) => setRemoveUninstallData(e.target.checked)}
              className="h-4 w-4 rounded border-border"
            />
            <span>{t('controlPanel.plugins.removeDataOption')}</span>
          </label>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setUninstallTarget(null)} disabled={uninstalling}>
              {t('deviceStatus.monitorOnly.cancel')}
            </Button>
            <Button loading={uninstalling} onClick={() => { void handleConfirmUninstall(); }}>
              {t('controlPanel.plugins.actions.uninstall')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
