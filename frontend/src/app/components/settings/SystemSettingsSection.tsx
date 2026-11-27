'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Activity, AlertTriangle, Clock3, History, Languages, Monitor, Palette, Power, Sparkles } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { toast } from 'sonner';
import { types } from '../../../../wailsjs/go/models';
import { type AppLocale, useLocale } from '../../lib/i18n';
import { HISTORY_RETENTION_HOUR_OPTIONS } from '../../lib/temperature-history';
import { useTemperatureHistory } from '../../hooks/useTemperatureHistory';
import { apiService } from '../../services/api';
import { Button, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, Select, ToggleSwitch } from '../ui';
import HotkeySettingsSection from './HotkeySettingsSection';
import { Section, SettingRow } from './SettingLayout';
import ThemeManagementPanel from './ThemeManagementPanel';

interface SystemSettingsSectionProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
}

const WINDOW_MATERIAL_OPTIONS = [
  { value: 'acrylic', labelKey: 'controlPanel.options.windowMaterial.acrylic' },
  { value: 'mica', labelKey: 'controlPanel.options.windowMaterial.mica' },
  { value: 'tabbed', labelKey: 'controlPanel.options.windowMaterial.tabbed' },
  { value: 'off', labelKey: 'controlPanel.options.windowMaterial.off' },
];

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export default function SystemSettingsSection({
  config,
  onConfigChange,
}: SystemSettingsSectionProps) {
  const { t } = useTranslation();
  const { locale, setLocale } = useLocale();
  const [loadingStates, setLoadingStates] = useState<Record<string, boolean>>({});
  const [monitorOnlyDialogOpen, setMonitorOnlyDialogOpen] = useState(false);
  const {
    enabled: temperatureHistoryEnabled,
    retentionHours: historyRetentionHours,
    saving: temperatureHistorySaving,
    setEnabled: setTemperatureHistoryEnabled,
    setRetentionHours: setHistoryRetentionHours,
  } = useTemperatureHistory();

  const setLoading = (key: string, value: boolean) => setLoadingStates((prev) => ({ ...prev, [key]: value }));
  const reportSettingsError = useCallback((error: unknown) => {
    toast.error(t('controlPanel.alerts.settingsOperationFailed', { error: getErrorMessage(error) }));
  }, [t]);

  const languageOptions = useMemo(
    () => ([
      { value: 'zh-CN', label: t('common.languages.zh-CN') },
      { value: 'en-US', label: t('common.languages.en-US') },
      { value: 'ja-JP', label: t('common.languages.ja-JP') },
    ]),
    [locale, t],
  );
  const windowMaterialOptions = useMemo(
    () => WINDOW_MATERIAL_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey) })),
    [locale, t],
  );
  const historyRetentionOptions = useMemo(
    () => HISTORY_RETENTION_HOUR_OPTIONS.map((hours) => ({
      value: hours,
      label: t('fanCurve.history.retention.hours', { count: hours }),
    })),
    [locale, t],
  );

  useEffect(() => {
    let cancelled = false;
    setLoading('windowsAutoStart', true);
    apiService
      .checkWindowsAutoStart()
      .then((enabled) => {
        if (!cancelled && enabled !== config.windowsAutoStart) {
          onConfigChange(types.AppConfig.createFrom({ ...config, windowsAutoStart: enabled }));
        }
      })
      .catch(() => {
        /* Keep the persisted value if the system check is unavailable. */
      })
      .finally(() => {
        if (!cancelled) setLoading('windowsAutoStart', false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const handleWindowBlurChange = useCallback(async (mode: string) => {
    const previousConfig = config;
    const optimisticCfg = types.AppConfig.createFrom({
      ...config,
      windowBlur: mode,
    });
    onConfigChange(optimisticCfg);
    try {
      await apiService.updateConfig(optimisticCfg);
      onConfigChange(types.AppConfig.createFrom(await apiService.getConfig()));
    } catch (error) {
      onConfigChange(previousConfig);
      reportSettingsError(error);
    }
  }, [config, onConfigChange, reportSettingsError]);

  const handleWindowsAutoStartChange = useCallback(async (enabled: boolean) => {
    setLoading('windowsAutoStart', true);
    try {
      const isAdmin = await apiService.isRunningAsAdmin();
      if (enabled) await apiService.setAutoStartWithMethod(true, isAdmin ? 'task_scheduler' : 'registry');
      else await apiService.setAutoStartWithMethod(false, '');
      onConfigChange(types.AppConfig.createFrom({ ...config, windowsAutoStart: enabled }));
    } catch (error) {
      reportSettingsError(error);
    } finally {
      setLoading('windowsAutoStart', false);
    }
  }, [config, onConfigChange, reportSettingsError]);

  const handleIgnoreDeviceOnReconnectChange = useCallback(async (enabled: boolean) => {
    try {
      const newCfg = types.AppConfig.createFrom({ ...config, ignoreDeviceOnReconnect: enabled });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch (error) {
      reportSettingsError(error);
    }
  }, [config, onConfigChange, reportSettingsError]);

  const applyMonitorOnlyChange = useCallback(async (enabled: boolean) => {
    setLoading('monitorOnly', true);
    const nextConfig = types.AppConfig.createFrom({ ...config, monitorOnly: enabled });
    try {
      await apiService.updateConfig(nextConfig);
      onConfigChange(nextConfig);
      await apiService.restartCore(false);
    } catch (error) {
      reportSettingsError(error);
    } finally {
      setLoading('monitorOnly', false);
    }
  }, [config, onConfigChange, reportSettingsError]);

  const handleMonitorOnlyChange = useCallback((enabled: boolean) => {
    if (enabled) {
      setMonitorOnlyDialogOpen(true);
      return;
    }
    void applyMonitorOnlyChange(false);
  }, [applyMonitorOnlyChange]);

  const confirmMonitorOnlyChange = useCallback(() => {
    setMonitorOnlyDialogOpen(false);
    void applyMonitorOnlyChange(true);
  }, [applyMonitorOnlyChange]);

  return (
    <>
    <div className="space-y-5">
      <Section title={t('controlPanel.system.groups.appearance')} icon={Palette} iconTone="purple">
        <ThemeManagementPanel config={config} onConfigChange={onConfigChange} />

        <SettingRow
          icon={<Sparkles className={clsx('h-4 w-4', ((config as any).windowBlur || 'acrylic') !== 'off' ? 'text-primary' : '')} />}
          iconTone="blue"
          title={t('controlPanel.system.windowBlurTitle')}
          description={t('controlPanel.system.windowBlurDescription')}
          tip={t('controlPanel.system.windowBlurTip')}
        >
          <div className="w-40">
            <Select
              value={String((config as any).windowBlur || 'acrylic')}
              onChange={(value: string | number) => handleWindowBlurChange(String(value))}
              options={windowMaterialOptions}
              size="sm"
            />
          </div>
        </SettingRow>

        <SettingRow
          icon={<Languages className="h-4 w-4" />}
          iconTone="emerald"
          title={t('controlPanel.system.languageTitle')}
          description={t('controlPanel.system.languageDescription')}
        >
          <div className="w-40">
            <Select
              value={locale}
              onChange={(value: string | number) => setLocale(String(value) as AppLocale)}
              options={languageOptions}
              size="sm"
            />
          </div>
        </SettingRow>
      </Section>

      <Section title={t('controlPanel.system.groups.monitoringAndHotkeys')} icon={Activity} iconTone="blue">
        <SettingRow
          icon={<Activity className={clsx('h-4 w-4', config.monitorOnly ? 'text-primary' : '')} />}
          iconTone="amber"
          title={t('controlPanel.system.monitorOnlyTitle')}
          description={t('controlPanel.system.monitorOnlyDescription')}
          tip={t('controlPanel.system.monitorOnlyTip')}
        >
          <ToggleSwitch
            enabled={config.monitorOnly === true}
            onChange={handleMonitorOnlyChange}
            loading={loadingStates.monitorOnly}
            size="sm"
            color="blue"
          />
        </SettingRow>

        <SettingRow
          icon={<History className={clsx('h-4 w-4', temperatureHistoryEnabled ? 'text-primary' : '')} />}
          iconTone="blue"
          title={t('controlPanel.system.temperatureHistoryTitle')}
          description={t('controlPanel.system.temperatureHistoryDescription')}
        >
          <ToggleSwitch
            enabled={temperatureHistoryEnabled}
            onChange={(enabled) => { void setTemperatureHistoryEnabled(enabled); }}
            loading={temperatureHistorySaving}
            size="sm"
            color="blue"
          />
        </SettingRow>

        <SettingRow
          icon={<Clock3 className={clsx('h-4 w-4', temperatureHistoryEnabled ? 'text-primary' : '')} />}
          iconTone="default"
          title={t('controlPanel.system.temperatureHistoryRetentionTitle')}
          description={t('controlPanel.system.temperatureHistoryRetentionDescription')}
          disabled={!temperatureHistoryEnabled}
        >
          <div className="w-40">
            <Select
              value={historyRetentionHours}
              onChange={(value: string | number) => { void setHistoryRetentionHours(Number(value)); }}
              disabled={temperatureHistorySaving || !temperatureHistoryEnabled}
              size="sm"
              options={historyRetentionOptions}
            />
          </div>
        </SettingRow>

        <HotkeySettingsSection config={config} onConfigChange={onConfigChange} />
      </Section>

      <Section title={t('controlPanel.system.groups.startupAndConnection')} icon={Power} iconTone="emerald">
        <SettingRow
          icon={<Monitor className={clsx('h-4 w-4', config.windowsAutoStart ? 'text-primary' : '')} />}
          iconTone="emerald"
          title={t('controlPanel.system.autoStartTitle')}
          description={t('controlPanel.system.autoStartDescription')}
          tip={t('controlPanel.system.autoStartTip')}
        >
          <ToggleSwitch
            enabled={config.windowsAutoStart}
            onChange={handleWindowsAutoStartChange}
            loading={loadingStates.windowsAutoStart}
            size="sm"
            color="blue"
          />
        </SettingRow>

        <SettingRow
          icon={<Clock3 className={clsx('h-4 w-4', (config as any).ignoreDeviceOnReconnect ? 'text-primary' : '')} />}
          iconTone="blue"
          title={t('controlPanel.system.reconnectTitle')}
          description={t('controlPanel.system.reconnectDescription')}
          tip={t('controlPanel.system.reconnectTip')}
        >
          <ToggleSwitch
            enabled={(config as any).ignoreDeviceOnReconnect ?? true}
            onChange={handleIgnoreDeviceOnReconnectChange}
            size="sm"
            color="blue"
          />
        </SettingRow>
      </Section>
    </div>
    <Dialog open={monitorOnlyDialogOpen} onOpenChange={(open) => !loadingStates.monitorOnly && setMonitorOnlyDialogOpen(open)}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-amber-600" />
            {t('controlPanel.system.monitorOnlyDialogTitle')}
          </DialogTitle>
          <DialogDescription>{t('controlPanel.system.monitorOnlyDialogDescription')}</DialogDescription>
        </DialogHeader>
        <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-3 text-sm leading-relaxed text-amber-800 dark:text-amber-200">
          {t('controlPanel.system.monitorOnlyDialogWarning')}
        </div>
        <DialogFooter>
          <Button variant="secondary" onClick={() => setMonitorOnlyDialogOpen(false)} disabled={loadingStates.monitorOnly}>
            {t('controlPanel.system.monitorOnlyDialogCancel')}
          </Button>
          <Button onClick={confirmMonitorOnlyChange} loading={loadingStates.monitorOnly}>
            {t('controlPanel.system.monitorOnlyDialogConfirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
    </>
  );
}
