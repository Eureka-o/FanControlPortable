'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import {
  Blocks,
  ExternalLink,
  FolderOpen,
  Loader2,
  PackageOpen,
  RefreshCw,
  RotateCcw,
  Trash2,
  TriangleAlert,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { BrowserOpenURL } from '../../../../wailsjs/runtime/runtime';
import { apiService } from '../../services/api';
import type { PluginCatalogEntry, PluginCatalogSnapshot } from '../../types/app';
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
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Section, SettingRow } from './SettingLayout';

const PLUGIN_RELEASES_URL = 'https://github.com/Eureka-o/FanControlPortable/releases';

type ConfirmAction = 'delete' | 'reset';

interface PendingConfirmation {
  action: ConfirmAction;
  plugin: PluginCatalogEntry;
}

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export default function PluginManagementSection() {
  const { t } = useTranslation();
  const [snapshot, setSnapshot] = useState<PluginCatalogSnapshot | null>(null);
  const [loadError, setLoadError] = useState('');
  const [initialLoading, setInitialLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [busyAction, setBusyAction] = useState('');
  const [pendingConfirmation, setPendingConfirmation] = useState<PendingConfirmation | null>(null);
  const revisionRef = useRef(0);

  const applySnapshot = useCallback((next: PluginCatalogSnapshot) => {
    if (!next || !Array.isArray(next.plugins)) {
      throw new Error(t('controlPanel.plugins.errors.apiUnavailable'));
    }
    if (next.revision < revisionRef.current) {
      return;
    }
    revisionRef.current = next.revision;
    setSnapshot(next);
    setLoadError('');
  }, [t]);

  const loadPlugins = useCallback(async () => {
    setInitialLoading(true);
    try {
      applySnapshot(await apiService.getPluginSnapshot());
    } catch (error) {
      setLoadError(getErrorMessage(error));
    } finally {
      setInitialLoading(false);
    }
  }, [applySnapshot]);

  useEffect(() => {
    const unsubscribeStatus = apiService.onPluginStatusUpdate(applySnapshot);
    const unsubscribeCore = apiService.onCoreServiceOK(() => {
      revisionRef.current = 0;
      void loadPlugins();
    });
    void loadPlugins();
    return () => {
      unsubscribeStatus();
      unsubscribeCore();
    };
  }, [applySnapshot, loadPlugins]);

  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      applySnapshot(await apiService.refreshPlugins());
      toast.success(t('controlPanel.plugins.toasts.refreshed'));
    } catch (error) {
      toast.error(t('controlPanel.plugins.toasts.refreshFailed', { error: getErrorMessage(error) }));
    } finally {
      setRefreshing(false);
    }
  };

  const handleOpenFolder = async () => {
    setBusyAction('open-folder');
    try {
      await apiService.openPluginsFolder();
    } catch (error) {
      toast.error(t('controlPanel.plugins.toasts.openFolderFailed', { error: getErrorMessage(error) }));
    } finally {
      setBusyAction('');
    }
  };

  const handleEnabledChange = async (plugin: PluginCatalogEntry, enabled: boolean) => {
    const actionKey = `toggle:${plugin.id}`;
    setBusyAction(actionKey);
    try {
      const next = await apiService.setPluginEnabled(plugin.id, enabled);
      applySnapshot(next);
      const updated = next.plugins.find((entry) => entry.id === plugin.id);
      if (enabled && updated?.state === 'failed') {
        toast.error(t('controlPanel.plugins.toasts.startFailed', { error: updated.lastError || t('controlPanel.plugins.states.failed') }));
      } else {
        toast.success(t(enabled
          ? 'controlPanel.plugins.toasts.enabled'
          : 'controlPanel.plugins.toasts.disabled', { name: plugin.name || plugin.id }));
      }
    } catch (error) {
      toast.error(t('controlPanel.plugins.toasts.toggleFailed', { error: getErrorMessage(error) }));
    } finally {
      setBusyAction('');
    }
  };

  const handleConfirmedAction = async () => {
    if (!pendingConfirmation) return;
    const { action, plugin } = pendingConfirmation;
    const actionKey = `${action}:${plugin.id}`;
    setBusyAction(actionKey);
    try {
      const next = action === 'delete'
        ? await apiService.deletePlugin(plugin.id)
        : await apiService.resetPlugin(plugin.id);
      applySnapshot(next);
      toast.success(t(action === 'delete'
        ? 'controlPanel.plugins.toasts.deleted'
        : 'controlPanel.plugins.toasts.reset', { name: plugin.name || plugin.id }));
      setPendingConfirmation(null);
    } catch (error) {
      toast.error(t(action === 'delete'
        ? 'controlPanel.plugins.toasts.deleteFailed'
        : 'controlPanel.plugins.toasts.resetFailed', { error: getErrorMessage(error) }));
    } finally {
      setBusyAction('');
    }
  };

  const plugins = snapshot?.plugins || [];
  const catalogError = loadError || snapshot?.error || '';
  const anyBusy = !!busyAction || refreshing;
  const confirmationBusy = !!pendingConfirmation
    && busyAction === `${pendingConfirmation.action}:${pendingConfirmation.plugin.id}`;

  const statusBadge = (plugin: PluginCatalogEntry) => {
    if (plugin.state === 'invalid') {
      return <Badge variant="error">{t('controlPanel.plugins.states.invalid')}</Badge>;
    }
    if (plugin.state === 'incompatible') {
      return <Badge variant="warning">{t('controlPanel.plugins.states.incompatible')}</Badge>;
    }
    switch (plugin.state) {
      case 'ready':
        return <Badge variant="success">{t('controlPanel.plugins.states.ready')}</Badge>;
      case 'starting':
        return <Badge variant="info">{t('controlPanel.plugins.states.starting')}</Badge>;
      case 'restarting':
        return <Badge variant="warning">{t('controlPanel.plugins.states.restarting')}</Badge>;
      case 'suspending':
        return <Badge variant="warning">{t('controlPanel.plugins.states.suspending')}</Badge>;
      case 'suspended':
        return <Badge>{t('controlPanel.plugins.states.suspended')}</Badge>;
      case 'unsupported':
        return <Badge variant="warning">{t('controlPanel.plugins.states.unsupported')}</Badge>;
      case 'failed':
        return <Badge variant="error">{t('controlPanel.plugins.states.failed')}</Badge>;
      default:
        if (plugin.enabled) {
          return <Badge variant="info">{t('controlPanel.plugins.states.enabledConfigured')}</Badge>;
        }
        return <Badge>{t('controlPanel.plugins.states.disabled')}</Badge>;
    }
  };

  return (
    <div className="space-y-4">
      <Section title={t('controlPanel.plugins.get.title')} icon={Blocks}>
        <SettingRow
          icon={<PackageOpen className="h-4.5 w-4.5" />}
          title={t('controlPanel.plugins.get.officialTitle')}
          description={t('controlPanel.plugins.get.description')}
        >
          <div className="flex w-full flex-wrap gap-2 sm:w-auto sm:justify-end">
            <Button
              size="sm"
              icon={<ExternalLink className="h-4 w-4" />}
              onClick={() => BrowserOpenURL(PLUGIN_RELEASES_URL)}
            >
              {t('controlPanel.plugins.get.openReleases')}
            </Button>
            <Button
              size="sm"
              variant="outline"
              icon={<FolderOpen className="h-4 w-4" />}
              loading={busyAction === 'open-folder'}
              disabled={anyBusy}
              onClick={() => void handleOpenFolder()}
            >
              {t('controlPanel.plugins.get.openFolder')}
            </Button>
          </div>
        </SettingRow>
      </Section>

      <Section
        title={t('controlPanel.plugins.management.title')}
        icon={PackageOpen}
        action={(
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                aria-label={t('controlPanel.plugins.management.refresh')}
                disabled={anyBusy}
                onClick={() => void handleRefresh()}
                className="inline-flex h-9 w-9 cursor-pointer items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
              >
                {refreshing
                  ? <Loader2 className="h-4 w-4 animate-spin" />
                  : <RefreshCw className="h-4 w-4" />}
              </button>
            </TooltipTrigger>
            <TooltipContent>{t('controlPanel.plugins.management.refresh')}</TooltipContent>
          </Tooltip>
        )}
      >
        {catalogError && (
          <div role="alert" className="flex items-start gap-2 px-5 py-3 text-sm text-destructive">
            <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" />
            <span className="min-w-0 break-words">{catalogError}</span>
          </div>
        )}

        {initialLoading && !snapshot ? (
          <div className="flex min-h-36 items-center justify-center gap-2 px-5 py-10 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            {t('controlPanel.plugins.management.loading')}
          </div>
        ) : plugins.length === 0 ? (
          <div className="flex min-h-40 flex-col items-center justify-center px-5 py-10 text-center">
            <PackageOpen className="h-8 w-8 text-muted-foreground/55" />
            <div className="mt-3 text-sm font-medium text-foreground">{t('controlPanel.plugins.management.emptyTitle')}</div>
            <div className="mt-1 max-w-md text-xs leading-relaxed text-muted-foreground">
              {t('controlPanel.plugins.management.emptyDescription')}
            </div>
          </div>
        ) : plugins.map((plugin) => {
          const toggleBusy = busyAction === `toggle:${plugin.id}`;
          const actionDisabled = anyBusy || plugin.enabled;
          const actionDisabledTip = plugin.enabled
            ? t('controlPanel.plugins.management.disableBeforeAction')
            : '';
          return (
            <div
              key={plugin.id}
              className="flex flex-col gap-4 px-5 py-4 transition-colors duration-200 hover:bg-muted/18 lg:flex-row lg:items-center lg:justify-between"
            >
              <div className="flex min-w-0 flex-1 items-start gap-3">
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-muted/70 text-muted-foreground shadow-inner shadow-white/20">
                  <Blocks className="h-4.5 w-4.5" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 flex-wrap items-center gap-2">
                    <div className="min-w-0 break-words text-base font-medium text-foreground">
                      {plugin.name || plugin.id}
                    </div>
                    {statusBadge(plugin)}
                    {plugin.version && <Badge>{`v${plugin.version}`}</Badge>}
                  </div>
                  <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                    <span className="break-all font-mono">{plugin.id}</span>
                    {plugin.platform && <span>{plugin.platform}</span>}
                  </div>
                  {plugin.description && (
                    <div className="mt-1.5 text-sm leading-relaxed text-muted-foreground">{plugin.description}</div>
                  )}
                  {plugin.lastError && (
                    <div
                      role="alert"
                      className={`mt-2 flex items-start gap-1.5 text-xs leading-relaxed ${plugin.state === 'unsupported' ? 'text-amber-700 dark:text-amber-300' : 'text-destructive'}`}
                    >
                      <TriangleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                      <span className="min-w-0 break-words">{plugin.lastError}</span>
                    </div>
                  )}
                </div>
              </div>

              <div className="flex w-full flex-wrap items-center justify-between gap-3 sm:justify-end lg:w-auto lg:shrink-0">
                <ToggleSwitch
                  enabled={plugin.enabled}
                  loading={toggleBusy}
                  disabled={anyBusy || plugin.state !== 'discovered'}
                  srLabel={t('controlPanel.plugins.management.toggleAria', { name: plugin.name || plugin.id })}
                  onChange={(enabled) => void handleEnabledChange(plugin, enabled)}
                />

                <div className="flex items-center gap-1">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span className="inline-flex">
                        <button
                          type="button"
                          aria-label={t('controlPanel.plugins.management.reset')}
                          disabled={actionDisabled}
                          onClick={() => setPendingConfirmation({ action: 'reset', plugin })}
                          className="inline-flex h-9 w-9 cursor-pointer items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-40"
                        >
                          <RotateCcw className="h-4 w-4" />
                        </button>
                      </span>
                    </TooltipTrigger>
                    <TooltipContent>{actionDisabledTip || t('controlPanel.plugins.management.reset')}</TooltipContent>
                  </Tooltip>

                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span className="inline-flex">
                        <button
                          type="button"
                          aria-label={t('controlPanel.plugins.management.delete')}
                          disabled={actionDisabled}
                          onClick={() => setPendingConfirmation({ action: 'delete', plugin })}
                          className="inline-flex h-9 w-9 cursor-pointer items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-40"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </span>
                    </TooltipTrigger>
                    <TooltipContent>{actionDisabledTip || t('controlPanel.plugins.management.delete')}</TooltipContent>
                  </Tooltip>
                </div>
              </div>
            </div>
          );
        })}
      </Section>

      <Dialog
        open={!!pendingConfirmation}
        onOpenChange={(open) => {
          if (!open && !confirmationBusy) setPendingConfirmation(null);
        }}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {pendingConfirmation && t(
                `controlPanel.plugins.confirm.${pendingConfirmation.action}.title`,
                { name: pendingConfirmation.plugin.name || pendingConfirmation.plugin.id },
              )}
            </DialogTitle>
            <DialogDescription>
              {pendingConfirmation && t(`controlPanel.plugins.confirm.${pendingConfirmation.action}.description`)}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="secondary"
              disabled={confirmationBusy}
              onClick={() => setPendingConfirmation(null)}
            >
              {t('common.actions.cancel')}
            </Button>
            <Button
              variant={pendingConfirmation?.action === 'delete' ? 'danger' : 'primary'}
              loading={confirmationBusy}
              onClick={() => void handleConfirmedAction()}
            >
              {pendingConfirmation
                ? t(`controlPanel.plugins.confirm.${pendingConfirmation.action}.confirm`)
                : t('common.actions.confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
