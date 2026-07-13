'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Clock3, Languages, Monitor, Sparkles } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { toast } from 'sonner';
import { types } from '../../../../wailsjs/go/models';
import { type ThemeMeta } from '../../types/app';
import { type AppLocale, useLocale } from '../../lib/i18n';
import { apiService } from '../../services/api';
import { Button, Select, ToggleSwitch } from '../ui';
import HotkeySettingsSection from './HotkeySettingsSection';
import { Section, SettingRow } from './SettingLayout';

interface SystemSettingsSectionProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
}

const THEME_MODE_OPTIONS = [
  { value: 'light', labelKey: 'controlPanel.options.themeMode.light' },
  { value: 'dark', labelKey: 'controlPanel.options.themeMode.dark' },
  { value: 'system', labelKey: 'controlPanel.options.themeMode.system' },
];

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
  const [customThemes, setCustomThemes] = useState<ThemeMeta[]>([]);

  const setLoading = (key: string, value: boolean) => setLoadingStates((prev) => ({ ...prev, [key]: value }));
  const reportSettingsError = useCallback((error: unknown) => {
    toast.error(t('controlPanel.alerts.settingsOperationFailed', { error: getErrorMessage(error) }));
  }, [t]);

  const themeModeOptions = useMemo(
    () => [
      ...THEME_MODE_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey) })),
      ...customThemes.map((theme) => ({ value: theme.id, label: theme.name })),
    ],
    [customThemes, locale, t],
  );
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

  useEffect(() => {
    let cancelled = false;
    apiService
      .listThemes()
      .then((themes) => {
        if (!cancelled) setCustomThemes(themes);
      })
      .catch(() => {
        /* Keep built-in themes when runtime discovery fails. */
      });
    return () => {
      cancelled = true;
    };
  }, []);

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

  const handleThemeModeChange = useCallback(async (mode: string) => {
    const isBuiltin = mode === 'light' || mode === 'dark' || mode === 'system';
    const isKnownCustom = customThemes.some((theme) => theme.id === mode);
    const nextMode = isBuiltin || isKnownCustom ? mode : 'system';
    const nextThemeIsDark = nextMode === 'dark' || (nextMode === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
    const previousThemeIsDark = document.documentElement.classList.contains('dark');
    document.documentElement.classList.toggle('dark', nextThemeIsDark);
    try {
      const newCfg = types.AppConfig.createFrom({
        ...config,
        themeMode: nextMode,
      });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch (error) {
      document.documentElement.classList.toggle('dark', previousThemeIsDark);
      reportSettingsError(error);
    }
  }, [config, customThemes, onConfigChange, reportSettingsError]);

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

  const handleOpenThemesFolder = useCallback(async () => {
    try {
      await apiService.openThemesFolder();
    } catch (error) {
      reportSettingsError(error);
    }
  }, [reportSettingsError]);

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

  return (
    <Section title={t('controlPanel.system.sectionTitle')} icon={Monitor}>
      <SettingRow
        icon={<Monitor className="h-4 w-4" />}
        title={t('controlPanel.system.themeTitle')}
        description={t('controlPanel.system.themeDescription')}
      >
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={handleOpenThemesFolder}
            title={t('controlPanel.system.themeOpenFolder')}
          >
            {t('controlPanel.system.themeOpenFolder')}
          </Button>
          <div className="w-36">
            <Select
              value={((config as any).themeMode || 'system') as string}
              onChange={(value: string | number) => handleThemeModeChange(String(value))}
              options={themeModeOptions}
              size="sm"
            />
          </div>
        </div>
      </SettingRow>

      <SettingRow
        icon={<Sparkles className={clsx('h-4 w-4', ((config as any).windowBlur || 'acrylic') !== 'off' ? 'text-primary' : '')} />}
        title={t('controlPanel.system.windowBlurTitle')}
        description={t('controlPanel.system.windowBlurDescription')}
        tip={t('controlPanel.system.windowBlurTip')}
      >
        <div className="w-36">
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
        title={t('controlPanel.system.languageTitle')}
        description={t('controlPanel.system.languageDescription')}
      >
        <div className="w-36">
          <Select
            value={locale}
            onChange={(value: string | number) => setLocale(String(value) as AppLocale)}
            options={languageOptions}
            size="sm"
          />
        </div>
      </SettingRow>

      <HotkeySettingsSection config={config} onConfigChange={onConfigChange} />

      <SettingRow
        icon={<Monitor className={clsx('h-4 w-4', config.windowsAutoStart ? 'text-emerald-500' : '')} />}
        title={t('controlPanel.system.autoStartTitle')}
        description={t('controlPanel.system.autoStartDescription')}
        tip={t('controlPanel.system.autoStartTip')}
      >
        <ToggleSwitch
          enabled={config.windowsAutoStart}
          onChange={handleWindowsAutoStartChange}
          loading={loadingStates.windowsAutoStart}
          size="sm"
          color="green"
        />
      </SettingRow>

      <SettingRow
        icon={<Clock3 className={clsx('h-4 w-4', (config as any).ignoreDeviceOnReconnect ? 'text-emerald-500' : '')} />}
        title={t('controlPanel.system.reconnectTitle')}
        description={t('controlPanel.system.reconnectDescription')}
        tip={t('controlPanel.system.reconnectTip')}
      >
        <ToggleSwitch
          enabled={(config as any).ignoreDeviceOnReconnect ?? true}
          onChange={handleIgnoreDeviceOnReconnectChange}
          size="sm"
          color="green"
        />
      </SettingRow>
    </Section>
  );
}
