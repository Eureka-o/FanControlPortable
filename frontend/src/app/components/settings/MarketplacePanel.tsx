'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { AnimatePresence, m } from 'framer-motion';
import { BrowserOpenURL } from '../../../../wailsjs/runtime/runtime';
import {
  CheckCircle2,
  Download,
  ExternalLink,
  FolderArchive,
  Palette,
  RefreshCw,
  Search,
  ShieldAlert,
  SlidersHorizontal,
  Sparkles,
  Trash2,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { types } from '../../../../wailsjs/go/models';
import { BRAND } from '../../lib/brand';
import { apiService } from '../../services/api';
import type {
  MarketplaceCatalogPlugin,
  MarketplaceCatalogTheme,
  MarketplaceCatalogView,
  MarketplaceProgressEvent,
} from '../../types/app';
import { Badge, Button } from '../ui';
import ActionMenu from './ActionMenu';
import { Section } from './SettingLayout';

export type ThemeMarketFilter = 'all' | 'updates' | 'available' | 'installed';

interface MarketplacePanelProps {
  mode: 'plugins' | 'themes';
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  onPluginsChanged?: () => void;
  onThemesChanged?: () => void;
  onCatalogLoaded?: (view: MarketplaceCatalogView) => void;
  onManageInstalled?: (pluginId: string) => void;
  onRequestUninstall?: (pluginId: string) => void;
  initialThemeFilter?: ThemeMarketFilter;
  embedded?: boolean;
  refreshNonce?: number;
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '-';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function openExternalUrl(url: string) {
  try {
    BrowserOpenURL(url);
  } catch {
    window.open(url, '_blank', 'noopener,noreferrer');
  }
}

export default function MarketplacePanel({
  mode,
  config,
  onConfigChange,
  onPluginsChanged,
  onThemesChanged,
  onCatalogLoaded,
  onManageInstalled,
  onRequestUninstall,
  initialThemeFilter = 'all',
  embedded = false,
  refreshNonce = 0,
}: MarketplacePanelProps) {
  const { t } = useTranslation();
  const [catalogUrlInput, setCatalogUrlInput] = useState(
    () => String((config as any)?.pluginCatalogUrl || ''),
  );
  const [showSourceInput, setShowSourceInput] = useState(false);
  const [catalog, setCatalog] = useState<MarketplaceCatalogView | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [batchUpdating, setBatchUpdating] = useState(false);
  const [importingZip, setImportingZip] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [themeFilter, setThemeFilter] = useState<ThemeMarketFilter>(initialThemeFilter);
  const [progressMap, setProgressMap] = useState<Record<string, MarketplaceProgressEvent>>({});

  useEffect(() => {
    setSearchQuery('');
  }, [mode]);

  useEffect(() => {
    setThemeFilter(initialThemeFilter);
  }, [initialThemeFilter]);

  const loadCatalog = useCallback(async (overrideUrl?: string) => {
    setLoading(true);
    setLoadError(null);
    try {
      const view = await apiService.fetchMarketplaceCatalog(overrideUrl ?? catalogUrlInput);
      setCatalog(view);
      if (view) {
        onCatalogLoaded?.(view);
      }
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : String(error));
    } finally {
      setLoading(false);
    }
  }, [catalogUrlInput, onCatalogLoaded]);

  useEffect(() => {
    void loadCatalog();
  }, [loadCatalog, refreshNonce]);

  useEffect(() => {
    return apiService.onMarketplaceProgress((evt) => {
      if (!evt || !evt.id) return;
      setProgressMap((prev) => ({ ...prev, [`${evt.kind}:${evt.id}`]: evt }));
    });
  }, []);

  const handleInstallPlugin = useCallback(async (item: MarketplaceCatalogPlugin) => {
    setBusyId(item.id);
    try {
      const res = await apiService.installMarketplacePlugin(item.id, item.downloadUrl, item.sha256);
      if (res?.restartRequired) {
        toast.info(t('controlPanel.plugins.marketplace.restartNeededToast', { name: item.name }));
      } else {
        toast.success(t('controlPanel.plugins.marketplace.installSuccessToast', { name: item.name }));
      }
      onPluginsChanged?.();
      await loadCatalog();
    } catch (error) {
      toast.error(t('controlPanel.plugins.marketplace.installFailedToast', {
        error: error instanceof Error ? error.message : String(error),
      }));
    } finally {
      setBusyId(null);
    }
  }, [loadCatalog, onPluginsChanged, t]);

  const handleInstallTheme = useCallback(async (item: MarketplaceCatalogTheme) => {
    setBusyId(item.id);
    try {
      await apiService.installMarketplaceTheme(item.id, item.downloadUrl, item.sha256);
      window.dispatchEvent(new CustomEvent('fancontrol:themes-updated'));
      onThemesChanged?.();
      toast.success(t('controlPanel.plugins.marketplace.themeInstallSuccess', { name: item.name }));
      await loadCatalog();
    } catch (error) {
      toast.error(t('controlPanel.plugins.marketplace.installFailedToast', {
        error: error instanceof Error ? error.message : String(error),
      }));
    } finally {
      setBusyId(null);
    }
  }, [loadCatalog, onThemesChanged, t]);

  const updatableThemes = useMemo(
    () => (catalog?.themes || []).filter((item) => item.installStatus === 'update_available' && item.compatible),
    [catalog],
  );

  const handleUpdateAllThemes = useCallback(async () => {
    if (updatableThemes.length === 0) return;
    setBatchUpdating(true);
    let updatedCount = 0;
    try {
      for (const item of updatableThemes) {
        setBusyId(item.id);
        await apiService.installMarketplaceTheme(item.id, item.downloadUrl, item.sha256);
        updatedCount++;
      }
      window.dispatchEvent(new CustomEvent('fancontrol:themes-updated'));
      onThemesChanged?.();
      toast.success(t('controlPanel.plugins.marketplace.updateAllSuccess', { count: updatedCount }));
      await loadCatalog();
    } catch (error) {
      toast.error(t('controlPanel.plugins.marketplace.installFailedToast', {
        error: error instanceof Error ? error.message : String(error),
      }));
    } finally {
      setBusyId(null);
      setBatchUpdating(false);
    }
  }, [loadCatalog, onThemesChanged, t, updatableThemes]);

  const handleApplyTheme = useCallback(async (themeId: string) => {
    try {
      const newCfg = types.AppConfig.createFrom({
        ...config,
        themeMode: themeId,
      });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
      window.dispatchEvent(new CustomEvent('fancontrol:themes-updated'));
      toast.success(t('controlPanel.plugins.marketplace.themeApplied'));
    } catch (error) {
      toast.error(t('controlPanel.alerts.settingsOperationFailed', {
        error: error instanceof Error ? error.message : String(error),
      }));
    }
  }, [config, onConfigChange, t]);

  const handleUninstallTheme = useCallback(async (item: MarketplaceCatalogTheme) => {
    setBusyId(item.id);
    try {
      await apiService.uninstallTheme(item.id);
      if ((config as any)?.themeMode === item.id) {
        const fallback = types.AppConfig.createFrom({
          ...config,
          themeMode: 'system',
        });
        await apiService.updateConfig(fallback);
        onConfigChange(fallback);
      }
      window.dispatchEvent(new CustomEvent('fancontrol:themes-updated'));
      onThemesChanged?.();
      toast.success(t('controlPanel.plugins.marketplace.themeUninstalled', { name: item.name }));
      await loadCatalog();
    } catch (error) {
      toast.error(t('controlPanel.alerts.settingsOperationFailed', {
        error: error instanceof Error ? error.message : String(error),
      }));
    } finally {
      setBusyId(null);
    }
  }, [config, loadCatalog, onConfigChange, onThemesChanged, t]);

  const handleImportLocalZip = useCallback(async () => {
    setImportingZip(true);
    try {
      const res = await apiService.importMarketplacePackage('');
      if (!res) return;
      if (res.kind === 'theme') {
        window.dispatchEvent(new CustomEvent('fancontrol:themes-updated'));
        onThemesChanged?.();
        toast.success(t('controlPanel.plugins.marketplace.themeInstallSuccess', { name: res.name }));
      } else if (res.restartRequired) {
        toast.info(t('controlPanel.plugins.marketplace.restartNeededToast', { name: res.name }));
        onPluginsChanged?.();
      } else {
        toast.success(t('controlPanel.plugins.marketplace.installSuccessToast', { name: res.name }));
        onPluginsChanged?.();
      }
      await loadCatalog();
    } catch (error) {
      toast.error(t('controlPanel.plugins.marketplace.installFailedToast', {
        error: error instanceof Error ? error.message : String(error),
      }));
    } finally {
      setImportingZip(false);
    }
  }, [loadCatalog, onPluginsChanged, onThemesChanged, t]);

  const saveCatalogSource = useCallback(async (overrideUrl?: string) => {
    const targetUrl = (overrideUrl !== undefined ? overrideUrl : catalogUrlInput).trim();
    try {
      const nextCfg = types.AppConfig.createFrom({
        ...config,
        pluginCatalogUrl: targetUrl,
      });
      await apiService.updateConfig(nextCfg);
      onConfigChange(nextCfg);
      await loadCatalog(targetUrl);
    } catch (error) {
      toast.error(t('controlPanel.alerts.settingsOperationFailed', {
        error: error instanceof Error ? error.message : String(error),
      }));
    }
  }, [catalogUrlInput, config, loadCatalog, onConfigChange, t]);

  const filteredPlugins = useMemo(() => {
    const list = catalog?.plugins || [];
    const q = searchQuery.trim().toLowerCase();
    if (!q) return list;
    return list.filter((item) =>
      [item.name, item.id, item.description, item.category, ...(item.capabilities || [])]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(q),
    );
  }, [catalog, searchQuery]);

  const filteredThemes = useMemo(() => {
    let list = catalog?.themes || [];
    if (themeFilter === 'updates') {
      list = list.filter((item) => item.installStatus === 'update_available');
    } else if (themeFilter === 'available') {
      list = list.filter((item) => item.installStatus === 'not_installed');
    } else if (themeFilter === 'installed') {
      list = list.filter((item) => item.installStatus === 'installed' || item.installStatus === 'update_available');
    }
    const q = searchQuery.trim().toLowerCase();
    if (!q) return list;
    return list.filter((item) =>
      [item.name, item.id, item.description, item.author]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(q),
    );
  }, [catalog, searchQuery, themeFilter]);

  const totalCount = mode === 'plugins' ? (catalog?.plugins?.length || 0) : (catalog?.themes?.length || 0);
  const filteredCount = mode === 'plugins' ? filteredPlugins.length : filteredThemes.length;

  const sectionIcon = mode === 'plugins' ? Download : Palette;
  const sectionTitle =
    mode === 'plugins'
      ? t('controlPanel.plugins.marketplace.pluginMarketTitle')
      : t('controlPanel.plugins.marketplace.themeMarketTitle');
  const sectionDesc =
    mode === 'plugins'
      ? t('controlPanel.plugins.marketplace.pluginMarketDesc')
      : t('controlPanel.plugins.marketplace.themeMarketDesc');

  const toolbarButtons = (
    <div className="flex flex-wrap items-center gap-2">
      {mode === 'themes' && updatableThemes.length > 0 && (
        <Button
          size="sm"
          loading={batchUpdating}
          disabled={Boolean(busyId) || loading}
          icon={<Download className="h-3.5 w-3.5" />}
          onClick={() => { void handleUpdateAllThemes(); }}
        >
          {t('controlPanel.plugins.marketplace.updateAllThemes', { count: updatableThemes.length })}
        </Button>
      )}
      <Button
        variant="outline"
        size="sm"
        icon={<RefreshCw className="h-3.5 w-3.5" />}
        loading={loading}
        onClick={() => { void loadCatalog(); }}
      >
        {t('controlPanel.plugins.marketplace.refreshShort')}
      </Button>
      <ActionMenu
        variant="outline"
        size="sm"
        ariaLabel={t('controlPanel.plugins.marketplace.moreOptions')}
        items={[
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
          {
            id: 'source-toggle',
            label: t('controlPanel.plugins.marketplace.sourceToggle'),
            icon: <SlidersHorizontal className="h-3.5 w-3.5" />,
            active: showSourceInput,
            onSelect: () => setShowSourceInput((v) => !v),
          },
        ]}
      />
    </div>
  );

  const themeFilterTabs: { id: ThemeMarketFilter; label: string }[] = [
    { id: 'all', label: t('controlPanel.plugins.marketplace.themeFilters.all') },
    { id: 'updates', label: t('controlPanel.plugins.marketplace.themeFilters.updates', { count: updatableThemes.length }) },
    { id: 'available', label: t('controlPanel.plugins.marketplace.themeFilters.available') },
    { id: 'installed', label: t('controlPanel.plugins.marketplace.themeFilters.installed') },
  ];

  const searchInputControl = (
    <div className="relative min-w-[200px] flex-1 max-w-xs">
      <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
      <input
        type="text"
        value={searchQuery}
        onChange={(e) => setSearchQuery(e.target.value)}
        placeholder={t('controlPanel.plugins.marketplace.searchPlaceholder')}
        className="h-8 w-full rounded-lg border border-border bg-card pl-8 pr-2.5 text-xs text-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
      />
    </div>
  );

  const content = (
    <div className="divide-y divide-border/60">
      {embedded && (
        <div className="flex flex-wrap items-center justify-between gap-2 bg-background/35 px-5 py-3">
          {!loadError && totalCount > 0 && searchInputControl}
          <div className="ml-auto flex flex-wrap items-center gap-2">
            {toolbarButtons}
          </div>
        </div>
      )}
      <AnimatePresence initial={false}>
        {showSourceInput && (
          <m.div
            key="source-input"
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.2, ease: [0.22, 1, 0.36, 1] }}
            className="overflow-hidden"
          >
            <div className="border-b border-border/60 bg-background/40 px-5 py-3">
              <div className="mb-1.5 text-xs font-medium text-muted-foreground">
                {t('controlPanel.plugins.marketplace.sourceHint')}
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <input
                  type="text"
                  value={catalogUrlInput}
                  onChange={(e) => setCatalogUrlInput(e.target.value)}
                  placeholder="https://example.com/catalog.json"
                  className="h-8 min-w-[260px] flex-1 rounded-lg border border-border bg-card px-2.5 text-xs text-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
                />
                <Button size="sm" variant="outline" onClick={() => { void saveCatalogSource(); }}>
                  {t('controlPanel.plugins.marketplace.applySource')}
                </Button>
                {catalogUrlInput.trim() !== '' && (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => {
                      setCatalogUrlInput('');
                      void saveCatalogSource('');
                    }}
                  >
                    {t('controlPanel.plugins.marketplace.resetSource')}
                  </Button>
                )}
              </div>
            </div>
          </m.div>
        )}
      </AnimatePresence>

      {!loadError && totalCount > 0 && (!embedded || mode === 'themes') && (
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border/60 bg-background/25 px-5 py-2.5">
          {!embedded && searchInputControl}
          {mode === 'themes' && (
            <div className="flex flex-wrap items-center gap-1">
              {themeFilterTabs.map((tab) => {
                const active = themeFilter === tab.id;
                return (
                  <button
                    key={tab.id}
                    type="button"
                    onClick={() => setThemeFilter(tab.id)}
                    className={`h-7 cursor-pointer rounded-lg px-2.5 text-xs font-medium transition-colors ${
                      active
                        ? 'bg-primary text-primary-foreground shadow-sm'
                        : 'bg-card/80 text-muted-foreground hover:bg-muted/70 hover:text-foreground'
                    }`}
                  >
                    {tab.label}
                  </button>
                );
              })}
            </div>
          )}
        </div>
      )}

      {loadError ? (
        <div className="px-5 py-6 text-center">
          <ShieldAlert className="mx-auto mb-2 h-6 w-6 text-amber-500" />
          <div className="text-sm font-medium text-foreground">
            {t('controlPanel.plugins.marketplace.loadFailed')}
          </div>
          <div className="mt-1 text-xs text-muted-foreground">{loadError}</div>
        </div>
      ) : loading && !catalog ? (
        <div className="px-5 py-8 text-center text-sm text-muted-foreground">
          {t('controlPanel.plugins.marketplace.loading')}
        </div>
      ) : totalCount === 0 ? (
        <div className="px-5 py-8 text-center">
          <Sparkles className="mx-auto mb-2 h-5 w-5 text-muted-foreground/70" />
          <div className="text-sm text-muted-foreground">
            {mode === 'plugins'
              ? t('controlPanel.plugins.marketplace.emptyPlugins')
              : t('controlPanel.plugins.marketplace.emptyThemes')}
          </div>
        </div>
      ) : filteredCount === 0 ? (
        <div className="px-5 py-8 text-center text-sm text-muted-foreground">
          {t('controlPanel.plugins.noFilterMatches')}
        </div>
      ) : mode === 'plugins' ? (
        filteredPlugins.map((item) => {
          const progress = progressMap[`plugin:${item.id}`];
          const isBusy = busyId === item.id;
          const isManagedInstalled =
            item.installStatus === 'installed' || item.installStatus === 'update_available';
          return (
            <div key={item.id} className="px-5 py-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-[15px] font-semibold tracking-[-0.01em] text-foreground">
                      {item.name}
                    </span>
                    {item.installStatus === 'update_available' && (
                      <Badge variant="warning">
                        {t('controlPanel.plugins.marketplace.updateAvailableBadge')}
                      </Badge>
                    )}
                    <span className="text-xs text-muted-foreground">{formatBytes(item.size)}</span>
                  </div>
                  <div className="mt-0.5 text-[13px] leading-[1.45] text-muted-foreground">
                    {item.description || item.id}
                  </div>
                  {item.reason && (
                    <div className="mt-1 text-xs text-amber-600 dark:text-amber-400">
                      {item.reason}
                    </div>
                  )}
                  {isBusy && progress && progress.percent >= 0 && (
                    <div className="mt-2 h-1.5 w-full max-w-xs overflow-hidden rounded-full bg-muted">
                      <div
                        className="h-full bg-primary transition-all duration-200"
                        style={{ width: `${Math.min(100, progress.percent)}%` }}
                      />
                    </div>
                  )}
                </div>
                <div className="flex shrink-0 flex-wrap items-center gap-2">
                  {isManagedInstalled && onManageInstalled && (
                    <Button
                      size="sm"
                      variant="outline"
                      icon={<SlidersHorizontal className="h-3.5 w-3.5" />}
                      onClick={() => onManageInstalled(item.id)}
                    >
                      {t('controlPanel.plugins.actions.manage')}
                    </Button>
                  )}
                  {item.installStatus === 'installed' ? (
                    <Badge variant="success">
                      <CheckCircle2 className="mr-1 inline h-3 w-3" />
                      {t('controlPanel.plugins.marketplace.statusInstalled')}
                    </Badge>
                  ) : item.installStatus === 'incompatible' ? (
                    <Badge variant="error">
                      {t('controlPanel.plugins.states.incompatible')}
                    </Badge>
                  ) : (
                    <Button
                      size="sm"
                      loading={isBusy}
                      icon={<Download className="h-3.5 w-3.5" />}
                      onClick={() => { void handleInstallPlugin(item); }}
                    >
                      {item.installStatus === 'update_available'
                        ? t('controlPanel.plugins.marketplace.updateButton')
                        : t('controlPanel.plugins.marketplace.installButton')}
                    </Button>
                  )}
                  {isManagedInstalled && onRequestUninstall && (
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={isBusy}
                      icon={<Trash2 className="h-3.5 w-3.5" />}
                      onClick={() => onRequestUninstall(item.id)}
                    >
                      {t('controlPanel.plugins.actions.uninstall')}
                    </Button>
                  )}
                </div>
              </div>
            </div>
          );
        })
      ) : (
        filteredThemes.map((item) => {
          const progress = progressMap[`theme:${item.id}`];
          const isBusy = busyId === item.id;
          const isActiveTheme = (config as any)?.themeMode === item.id;
          const isInstalled =
            item.installStatus === 'installed' || item.installStatus === 'update_available';
          const isBuiltinTheme = Boolean(item.installedBuiltin) || item.installedSource === 'builtin';
          return (
            <div key={item.id} className="px-5 py-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-[15px] font-semibold tracking-[-0.01em] text-foreground">
                      {item.name}
                    </span>
                    {item.installStatus === 'update_available' && (
                      <Badge variant="warning">
                        {t('controlPanel.plugins.marketplace.updateAvailableBadge')}
                      </Badge>
                    )}
                    <Badge variant={item.base === 'dark' ? 'info' : 'default'}>
                      {item.base === 'dark'
                        ? t('controlPanel.plugins.marketplace.themeDark')
                        : t('controlPanel.plugins.marketplace.themeLight')}
                    </Badge>
                    {isBuiltinTheme && (
                      <Badge variant="info">{t('controlPanel.system.themeManagement.builtinBadge')}</Badge>
                    )}
                    <span className="text-xs text-muted-foreground">{formatBytes(item.size)}</span>
                    {item.author && (
                      <span className="text-xs text-muted-foreground">· {item.author}</span>
                    )}
                  </div>
                  <div className="mt-0.5 text-[13px] leading-[1.45] text-muted-foreground">
                    {item.description || item.id}
                  </div>
                  {isBusy && progress && progress.percent >= 0 && (
                    <div className="mt-2 h-1.5 w-full max-w-xs overflow-hidden rounded-full bg-muted">
                      <div
                        className="h-full bg-primary transition-all duration-200"
                        style={{ width: `${Math.min(100, progress.percent)}%` }}
                      />
                    </div>
                  )}
                </div>
                <div className="flex shrink-0 flex-wrap items-center gap-2">
                  {isInstalled && (
                    <Button
                      size="sm"
                      variant={isActiveTheme ? 'secondary' : 'outline'}
                      disabled={isActiveTheme}
                      icon={<ExternalLink className="h-3.5 w-3.5" />}
                      onClick={() => { void handleApplyTheme(item.id); }}
                    >
                      {isActiveTheme
                        ? t('controlPanel.plugins.marketplace.activeThemeBadge')
                        : t('controlPanel.plugins.marketplace.applyThemeButton')}
                    </Button>
                  )}
                  {(item.installStatus === 'not_installed' || item.installStatus === 'update_available') && (
                    <Button
                      size="sm"
                      loading={isBusy}
                      disabled={batchUpdating}
                      icon={<Download className="h-3.5 w-3.5" />}
                      onClick={() => { void handleInstallTheme(item); }}
                    >
                      {item.installStatus === 'update_available'
                        ? t('controlPanel.plugins.marketplace.updateButton')
                        : t('controlPanel.plugins.marketplace.installButton')}
                    </Button>
                  )}
                  {isInstalled && !isBuiltinTheme && (
                    <Button
                      size="sm"
                      variant="outline"
                      loading={isBusy}
                      disabled={batchUpdating}
                      icon={<Trash2 className="h-3.5 w-3.5" />}
                      onClick={() => { void handleUninstallTheme(item); }}
                    >
                      {t('controlPanel.system.themeManagement.deleteButton')}
                    </Button>
                  )}
                </div>
              </div>
            </div>
          );
        })
      )}
    </div>
  );

  if (embedded) {
    return content;
  }

  return (
    <Section
      title={sectionTitle}
      description={sectionDesc}
      icon={sectionIcon}
      iconTone={mode === 'plugins' ? 'blue' : 'purple'}
      action={(
        <div className="flex flex-wrap items-center gap-2">
          {toolbarButtons}
        </div>
      )}
    >
      {content}
    </Section>
  );
}
