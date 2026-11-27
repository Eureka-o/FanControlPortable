'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { BrowserOpenURL } from '../../../../wailsjs/runtime/runtime';
import {
  Check,
  Download,
  ExternalLink,
  FolderOpen,
  Monitor,
  Palette,
  RefreshCw,
  Trash2,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { types } from '../../../../wailsjs/go/models';
import { BRAND } from '../../lib/brand';
import { apiService } from '../../services/api';
import type {
  MarketplaceCatalogTheme,
  MarketplaceCatalogView,
  MarketplaceProgressEvent,
  ThemeMeta,
} from '../../types/app';
import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Select,
} from '../ui';
import ActionMenu from './ActionMenu';
import MarketplacePanel, { type ThemeMarketFilter } from './MarketplacePanel';
import { SettingRow } from './SettingLayout';

interface ThemeManagementPanelProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
}

type ThemeManagerTab = 'installed' | 'download';

const THEME_MODE_OPTIONS = [
  { value: 'light', labelKey: 'controlPanel.options.themeMode.light' },
  { value: 'dark', labelKey: 'controlPanel.options.themeMode.dark' },
  { value: 'system', labelKey: 'controlPanel.options.themeMode.system' },
];

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function openExternalUrl(url: string) {
  try {
    BrowserOpenURL(url);
  } catch {
    window.open(url, '_blank', 'noopener,noreferrer');
  }
}

export default function ThemeManagementPanel({
  config,
  onConfigChange,
}: ThemeManagementPanelProps) {
  const { t } = useTranslation();
  const [customThemes, setCustomThemes] = useState<ThemeMeta[]>([]);
  const [catalog, setCatalog] = useState<MarketplaceCatalogView | null>(null);
  const [checkingUpdates, setCheckingUpdates] = useState(false);
  const [batchUpdating, setBatchUpdating] = useState(false);
  const [busyThemeId, setBusyThemeId] = useState<string | null>(null);
  const [progressMap, setProgressMap] = useState<Record<string, MarketplaceProgressEvent>>({});
  const [managerOpen, setManagerOpen] = useState(false);
  const [managerTab, setManagerTab] = useState<ThemeManagerTab>('installed');
  const [marketFilter, setMarketFilter] = useState<ThemeMarketFilter>('all');
  const [marketRefreshNonce, setMarketRefreshNonce] = useState(0);
  const [uninstallTarget, setUninstallTarget] = useState<ThemeMeta | null>(null);
  const [uninstalling, setUninstalling] = useState(false);

  const reloadLocalThemes = useCallback(async () => {
    try {
      const themes = await apiService.listThemes();
      setCustomThemes(themes);
    } catch {
      /* Keep existing theme list when runtime discovery temporarily fails. */
    }
  }, []);

  const checkThemeCatalog = useCallback(async (showToast: boolean) => {
    setCheckingUpdates(true);
    try {
      const view = await apiService.fetchMarketplaceCatalog('');
      setCatalog(view);
      const count = (view?.themes || []).filter(
        (item) => item.installStatus === 'update_available' && item.compatible,
      ).length;
      if (showToast) {
        if (count > 0) {
          toast.info(t('controlPanel.system.themeManagement.themeUpdatesFound', { count }));
        } else {
          toast.success(t('controlPanel.system.themeManagement.themesUpToDate'));
        }
      }
    } catch (error) {
      if (showToast) {
        toast.error(t('controlPanel.alerts.settingsOperationFailed', { error: getErrorMessage(error) }));
      }
    } finally {
      setCheckingUpdates(false);
    }
  }, [t]);

  useEffect(() => {
    void reloadLocalThemes();
    const onUpdated = () => {
      void reloadLocalThemes();
    };
    window.addEventListener('fancontrol:themes-updated', onUpdated);
    return () => {
      window.removeEventListener('fancontrol:themes-updated', onUpdated);
    };
  }, [reloadLocalThemes]);

  useEffect(() => {
    return apiService.onMarketplaceProgress((evt) => {
      if (!evt || evt.kind !== 'theme' || !evt.id) return;
      setProgressMap((prev) => ({ ...prev, [evt.id]: evt }));
    });
  }, []);

  const themeModeOptions = useMemo(
    () => [
      ...THEME_MODE_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey) })),
      ...customThemes.map((theme) => ({ value: theme.id, label: theme.name })),
    ],
    [customThemes, t],
  );

  const catalogThemeMap = useMemo(() => {
    const map = new Map<string, MarketplaceCatalogTheme>();
    for (const item of catalog?.themes || []) {
      map.set(item.id, item);
    }
    return map;
  }, [catalog]);

  const updatableThemes = useMemo(
    () => (catalog?.themes || []).filter(
      (item) => item.installStatus === 'update_available' && item.compatible,
    ),
    [catalog],
  );

  const handleThemeModeChange = useCallback(async (mode: string) => {
    const isBuiltin = mode === 'light' || mode === 'dark' || mode === 'system';
    const isKnownCustom = customThemes.some((theme) => theme.id === mode);
    const nextMode = isBuiltin || isKnownCustom ? mode : 'system';
    const previousThemeIsDark = document.documentElement.classList.contains('dark');
    if (isBuiltin) {
      const nextThemeIsDark = nextMode === 'dark' || (nextMode === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
      document.documentElement.classList.toggle('dark', nextThemeIsDark);
    }
    try {
      const newCfg = types.AppConfig.createFrom({
        ...config,
        themeMode: nextMode,
      });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch (error) {
      if (isBuiltin) {
        document.documentElement.classList.toggle('dark', previousThemeIsDark);
      }
      toast.error(t('controlPanel.alerts.settingsOperationFailed', { error: getErrorMessage(error) }));
    }
  }, [config, customThemes, onConfigChange, t]);

  const handleOpenThemesFolder = useCallback(async () => {
    try {
      await apiService.openThemesFolder();
    } catch (error) {
      toast.error(t('controlPanel.alerts.settingsOperationFailed', { error: getErrorMessage(error) }));
    }
  }, [t]);

  const handlePullSingleUpdate = useCallback(async (entry: MarketplaceCatalogTheme) => {
    setBusyThemeId(entry.id);
    try {
      await apiService.installMarketplaceTheme(entry.id, entry.downloadUrl, entry.sha256);
      window.dispatchEvent(new CustomEvent('fancontrol:themes-updated'));
      await reloadLocalThemes();
      await checkThemeCatalog(false);
      toast.success(t('controlPanel.plugins.marketplace.themeInstallSuccess', { name: entry.name }));
    } catch (error) {
      toast.error(t('controlPanel.plugins.marketplace.installFailedToast', { error: getErrorMessage(error) }));
    } finally {
      setBusyThemeId(null);
    }
  }, [checkThemeCatalog, reloadLocalThemes, t]);

  const handleUpdateAllThemes = useCallback(async () => {
    if (updatableThemes.length === 0) return;
    setBatchUpdating(true);
    let updatedCount = 0;
    try {
      for (const entry of updatableThemes) {
        setBusyThemeId(entry.id);
        await apiService.installMarketplaceTheme(entry.id, entry.downloadUrl, entry.sha256);
        updatedCount++;
      }
      window.dispatchEvent(new CustomEvent('fancontrol:themes-updated'));
      await reloadLocalThemes();
      await checkThemeCatalog(false);
      toast.success(t('controlPanel.plugins.marketplace.updateAllSuccess', { count: updatedCount }));
    } catch (error) {
      toast.error(t('controlPanel.plugins.marketplace.installFailedToast', { error: getErrorMessage(error) }));
    } finally {
      setBusyThemeId(null);
      setBatchUpdating(false);
    }
  }, [checkThemeCatalog, reloadLocalThemes, t, updatableThemes]);

  const confirmUninstallTheme = useCallback(async () => {
    if (!uninstallTarget) return;
    setUninstalling(true);
    try {
      await apiService.uninstallTheme(uninstallTarget.id);
      if ((config as any)?.themeMode === uninstallTarget.id) {
        const fallback = types.AppConfig.createFrom({
          ...config,
          themeMode: 'system',
        });
        await apiService.updateConfig(fallback);
        onConfigChange(fallback);
      }
      window.dispatchEvent(new CustomEvent('fancontrol:themes-updated'));
      await reloadLocalThemes();
      if (catalog) {
        await checkThemeCatalog(false);
      }
      toast.success(t('controlPanel.plugins.marketplace.themeUninstalled', { name: uninstallTarget.name }));
      setUninstallTarget(null);
    } catch (error) {
      toast.error(t('controlPanel.alerts.settingsOperationFailed', { error: getErrorMessage(error) }));
    } finally {
      setUninstalling(false);
    }
  }, [catalog, checkThemeCatalog, config, onConfigChange, reloadLocalThemes, t, uninstallTarget]);

  const openManagerDialog = useCallback(() => {
    setManagerTab('installed');
    setMarketFilter('all');
    setManagerOpen(true);
    if (!catalog && !checkingUpdates) {
      void checkThemeCatalog(false);
    }
  }, [catalog, checkThemeCatalog, checkingUpdates]);

  const handleRefreshManager = useCallback(() => {
    void reloadLocalThemes();
    if (managerTab === 'download') {
      setMarketRefreshNonce((n) => n + 1);
    } else {
      void checkThemeCatalog(true);
    }
  }, [checkThemeCatalog, managerTab, reloadLocalThemes]);

  const activeThemeMode = String((config as any).themeMode || 'system');

  return (
    <>
      <SettingRow
        icon={<Monitor className="h-4 w-4" />}
        iconTone="purple"
        title={t('controlPanel.system.themeTitle')}
        description={t('controlPanel.system.themeDescription')}
      >
        <div className="flex items-center gap-2">
          <div className="w-40">
            <Select
              value={activeThemeMode}
              onChange={(value: string | number) => { void handleThemeModeChange(String(value)); }}
              options={themeModeOptions}
              size="sm"
            />
          </div>
          <Button
            variant="outline"
            size="sm"
            icon={<Palette className="h-3.5 w-3.5" />}
            onClick={openManagerDialog}
          >
            <span className="inline-flex items-center gap-1.5">
              <span>{t('controlPanel.system.themeManagement.manageButton')}</span>
              {updatableThemes.length > 0 && (
                <span className="inline-flex min-w-[1.125rem] items-center justify-center rounded-full bg-amber-500/18 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-amber-600 dark:text-amber-300">
                  {updatableThemes.length}
                </span>
              )}
            </span>
          </Button>
        </div>
      </SettingRow>

      <Dialog open={managerOpen} onOpenChange={setManagerOpen}>
        <DialogContent className="flex max-h-[85vh] max-w-3xl flex-col overflow-hidden p-0 sm:max-w-[780px]">
          <div className="border-b border-border/70 px-6 pb-4 pt-5 pr-12">
            <DialogHeader className="text-left">
              <DialogTitle>{t('controlPanel.system.themeManagement.dialogTitle')}</DialogTitle>
              <DialogDescription>{t('controlPanel.system.themeManagement.dialogDescription')}</DialogDescription>
            </DialogHeader>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border/60 bg-background/35 px-6 py-3">
            <div className="inline-flex rounded-xl border border-border/70 bg-muted/40 p-1" role="tablist">
              <button
                type="button"
                role="tab"
                aria-selected={managerTab === 'installed'}
                onClick={() => setManagerTab('installed')}
                className={`cursor-pointer rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
                  managerTab === 'installed'
                    ? 'bg-card text-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                {t('controlPanel.system.themeManagement.tabs.installed', { count: customThemes.length })}
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={managerTab === 'download'}
                onClick={() => {
                  setMarketFilter(updatableThemes.length > 0 ? 'updates' : 'all');
                  setManagerTab('download');
                }}
                className={`inline-flex cursor-pointer items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
                  managerTab === 'download'
                    ? 'bg-card text-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                <span>{t('controlPanel.system.themeManagement.tabs.download')}</span>
                {updatableThemes.length > 0 && (
                  <span className="inline-flex min-w-[1.125rem] items-center justify-center rounded-full bg-amber-500/18 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-amber-600 dark:text-amber-300">
                    {updatableThemes.length}
                  </span>
                )}
              </button>
            </div>

            <div className="flex items-center gap-2">
              {managerTab === 'installed' && (
                <Button
                  variant="ghost"
                  size="sm"
                  icon={<RefreshCw className="h-3.5 w-3.5" />}
                  loading={checkingUpdates}
                  onClick={handleRefreshManager}
                >
                  {t('controlPanel.system.themeManagement.checkUpdatesButton')}
                </Button>
              )}
              <Button
                variant="outline"
                size="sm"
                icon={<FolderOpen className="h-3.5 w-3.5" />}
                onClick={() => { void handleOpenThemesFolder(); }}
              >
                {t('controlPanel.system.themeManagement.openFolderShort')}
              </Button>
              {managerTab === 'installed' && (
                <ActionMenu
                  variant="outline"
                  size="sm"
                  ariaLabel={t('controlPanel.plugins.marketplace.moreOptions')}
                  items={[
                    {
                      id: 'open-repo',
                      label: t('controlPanel.plugins.marketplace.openRepository'),
                      icon: <ExternalLink className="h-3.5 w-3.5" />,
                      onSelect: () => openExternalUrl(BRAND.pluginsRepositoryUrl),
                    },
                  ]}
                />
              )}
            </div>
          </div>

          <div className="flex-1 overflow-y-auto">
            {managerTab === 'installed' ? (
              <div>
                {updatableThemes.length > 0 && (
                  <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border/60 bg-amber-500/8 px-6 py-3">
                    <div className="flex min-w-0 items-center gap-2 text-xs text-foreground">
                      <Download className="h-4 w-4 shrink-0 text-amber-500" />
                      <span className="font-medium">
                        {t('controlPanel.system.themeManagement.updatesBannerText', {
                          count: updatableThemes.length,
                        })}
                      </span>
                    </div>
                    <Button
                      size="sm"
                      loading={batchUpdating}
                      disabled={Boolean(busyThemeId) || checkingUpdates}
                      icon={<Download className="h-3.5 w-3.5" />}
                      onClick={() => { void handleUpdateAllThemes(); }}
                    >
                      {t('controlPanel.plugins.marketplace.updateAllThemes', { count: updatableThemes.length })}
                    </Button>
                  </div>
                )}

                {customThemes.length === 0 ? (
                  <div className="px-6 py-12 text-center">
                    <Palette className="mx-auto mb-2.5 h-6 w-6 text-muted-foreground/70" />
                    <div className="text-sm font-medium text-foreground">
                      {t('controlPanel.system.themeManagement.emptyInstalledTitle')}
                    </div>
                    <div className="mt-3">
                      <Button
                        size="sm"
                        variant="outline"
                        icon={<Download className="h-3.5 w-3.5" />}
                        onClick={() => {
                          setMarketFilter('all');
                          setManagerTab('download');
                        }}
                      >
                        {t('controlPanel.system.themeManagement.goToDownload')}
                      </Button>
                    </div>
                  </div>
                ) : (
                  <div className="grid grid-cols-1 gap-3 p-5 sm:grid-cols-2">
                    {customThemes.map((theme) => {
                      const catalogEntry = catalogThemeMap.get(theme.id);
                      const hasUpdate = catalogEntry?.installStatus === 'update_available' && catalogEntry.compatible;
                      const isActive = activeThemeMode === theme.id;
                      const isBusy = busyThemeId === theme.id;
                      const isBuiltin = Boolean(theme.builtin) || theme.source === 'builtin';
                      const progress = progressMap[theme.id];

                      return (
                        <div
                          key={theme.id}
                          className={`flex flex-col justify-between rounded-xl border p-3.5 transition-colors ${
                            isActive
                              ? 'border-primary/50 bg-primary/5'
                              : 'border-border/70 bg-card/75'
                          }`}
                        >
                          <div>
                            <div className="flex flex-wrap items-center gap-1.5">
                              <span className="truncate text-sm font-semibold text-foreground">
                                {theme.name}
                              </span>
                              <Badge variant={theme.base === 'dark' ? 'info' : 'default'}>
                                {theme.base === 'dark'
                                  ? t('controlPanel.plugins.marketplace.themeDark')
                                  : t('controlPanel.plugins.marketplace.themeLight')}
                              </Badge>
                              {isBuiltin && (
                                <Badge variant="info">
                                  {t('controlPanel.system.themeManagement.builtinBadge')}
                                </Badge>
                              )}
                              {hasUpdate && (
                                <Badge variant="warning">
                                  {t('controlPanel.system.themeManagement.updateAvailableShort')}
                                </Badge>
                              )}
                            </div>
                            {theme.description && (
                              <p className="mt-1.5 line-clamp-2 text-xs leading-relaxed text-muted-foreground">
                                {theme.description}
                              </p>
                            )}
                            {isBusy && progress && progress.percent >= 0 && (
                              <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-muted">
                                <div
                                  className="h-full bg-primary transition-all duration-200"
                                  style={{ width: `${Math.min(100, progress.percent)}%` }}
                                />
                              </div>
                            )}
                          </div>

                          <div className="mt-3 flex flex-wrap items-center justify-end gap-1.5">
                            {hasUpdate && catalogEntry && (
                              <Button
                                size="sm"
                                loading={isBusy}
                                disabled={batchUpdating}
                                icon={<Download className="h-3.5 w-3.5" />}
                                onClick={() => { void handlePullSingleUpdate(catalogEntry); }}
                              >
                                {t('controlPanel.plugins.marketplace.updateButton')}
                              </Button>
                            )}
                            <Button
                              size="sm"
                              variant={isActive ? 'secondary' : 'outline'}
                              disabled={isActive}
                              icon={isActive ? <Check className="h-3.5 w-3.5" /> : undefined}
                              onClick={() => { void handleThemeModeChange(theme.id); }}
                            >
                              {isActive
                                ? t('controlPanel.plugins.marketplace.activeThemeBadge')
                                : t('controlPanel.plugins.marketplace.applyThemeButton')}
                            </Button>
                            {!isBuiltin && (
                              <Button
                                size="sm"
                                variant="ghost"
                                disabled={isBusy || batchUpdating}
                                icon={<Trash2 className="h-3.5 w-3.5" />}
                                onClick={() => setUninstallTarget(theme)}
                              >
                                {t('controlPanel.system.themeManagement.deleteButton')}
                              </Button>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            ) : (
              <MarketplacePanel
                embedded
                mode="themes"
                config={config}
                onConfigChange={onConfigChange}
                initialThemeFilter={marketFilter}
                refreshNonce={marketRefreshNonce}
                onCatalogLoaded={setCatalog}
                onThemesChanged={() => { void reloadLocalThemes(); }}
              />
            )}
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(uninstallTarget)} onOpenChange={(open) => !uninstalling && !open && setUninstallTarget(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {uninstallTarget
                ? t('controlPanel.system.themeManagement.uninstallConfirmTitle', { name: uninstallTarget.name })
                : ''}
            </DialogTitle>
            <DialogDescription>
              {t('controlPanel.system.themeManagement.uninstallConfirmDesc')}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setUninstallTarget(null)} disabled={uninstalling}>
              {t('appShell.titleBar.close')}
            </Button>
            <Button variant="danger" onClick={() => { void confirmUninstallTheme(); }} loading={uninstalling}>
              {t('controlPanel.system.themeManagement.confirmDeleteButton')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
