'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Clock3, Languages, Monitor, Sparkles } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { types } from '../../../../wailsjs/go/models';
import { type ThemeMeta } from '../../types/app';
import { type AppLocale, useLocale } from '../../lib/i18n';
import { apiService } from '../../services/api';
import { Button, Select, ToggleSwitch } from '../ui';
import DeviceConnectionSection from './DeviceConnectionSection';
import HotkeySettingsSection from './HotkeySettingsSection';
import { Section, SettingRow } from './SettingLayout';

interface SystemSettingsSectionProps {
  config: types.AppConfig;
  availableDeviceProfiles: types.DeviceProfile[];
  activeDeviceProfileId: string;
  activeDeviceProfileIdsByTransport: Record<string, string>;
  connectedDeviceProfile: types.DeviceProfile | null;
  connectedDeviceTransport: string;
  onConfigChange: (config: types.AppConfig) => void;
  onActiveDeviceProfileIdChange: (profileId: string) => void;
  refreshDeviceConfig: () => Promise<types.AppConfig>;
  loadDeviceProfiles: () => Promise<types.DeviceProfile[]>;
  refreshConnectedDeviceContext: () => Promise<void>;
}

const THEME_MODE_OPTIONS = [
  { value: 'light', labelKey: 'controlPanel.options.themeMode.light' },
  { value: 'dark', labelKey: 'controlPanel.options.themeMode.dark' },
  { value: 'system', labelKey: 'controlPanel.options.themeMode.system' },
];

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export default function SystemSettingsSection({
  config,
  availableDeviceProfiles,
  activeDeviceProfileId,
  activeDeviceProfileIdsByTransport,
  connectedDeviceProfile,
  connectedDeviceTransport,
  onConfigChange,
  onActiveDeviceProfileIdChange,
  refreshDeviceConfig,
  loadDeviceProfiles,
  refreshConnectedDeviceContext,
}: SystemSettingsSectionProps) {
  const { t } = useTranslation();
  const { locale, setLocale } = useLocale();
  const [loadingStates, setLoadingStates] = useState<Record<string, boolean>>({});
  const [customThemes, setCustomThemes] = useState<ThemeMeta[]>([]);

  const setLoading = (key: string, value: boolean) => setLoadingStates((prev) => ({ ...prev, [key]: value }));

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
    document.documentElement.classList.toggle('dark', nextThemeIsDark);
    try {
      const newCfg = types.AppConfig.createFrom({
        ...config,
        themeMode: nextMode,
      });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch {
      /* noop */
    }
  }, [config, customThemes, onConfigChange]);

  const handleWindowBlurChange = useCallback(async (enabled: boolean) => {
    const previousConfig = config;
    const optimisticCfg = types.AppConfig.createFrom({
      ...config,
      windowBlur: enabled ? 'on' : 'off',
    });
    onConfigChange(optimisticCfg);
    try {
      await apiService.updateConfig(optimisticCfg);
      onConfigChange(types.AppConfig.createFrom(await apiService.getConfig()));
    } catch {
      onConfigChange(previousConfig);
    }
  }, [config, onConfigChange]);

  const handleOpenThemesFolder = useCallback(async () => {
    try {
      await apiService.openThemesFolder();
    } catch {
      /* noop */
    }
  }, []);

  const handleWindowsAutoStartChange = useCallback(async (enabled: boolean) => {
    setLoading('windowsAutoStart', true);
    try {
      const isAdmin = await apiService.isRunningAsAdmin();
      if (enabled) await apiService.setAutoStartWithMethod(true, isAdmin ? 'task_scheduler' : 'registry');
      else await apiService.setAutoStartWithMethod(false, '');
      onConfigChange(types.AppConfig.createFrom({ ...config, windowsAutoStart: enabled }));
    } catch (error) {
      alert(t('controlPanel.alerts.autoStartFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoading('windowsAutoStart', false);
    }
  }, [config, onConfigChange, t]);

  const handleIgnoreDeviceOnReconnectChange = useCallback(async (enabled: boolean) => {
    try {
      const newCfg = types.AppConfig.createFrom({ ...config, ignoreDeviceOnReconnect: enabled });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch {
      /* noop */
    }
  }, [config, onConfigChange]);

  return (
    <Section title={t('controlPanel.system.sectionTitle')} icon={Monitor}>
      <DeviceConnectionSection
        config={config}
        availableDeviceProfiles={availableDeviceProfiles}
        activeDeviceProfileId={activeDeviceProfileId}
        activeDeviceProfileIdsByTransport={activeDeviceProfileIdsByTransport}
        connectedDeviceProfile={connectedDeviceProfile}
        connectedDeviceTransport={connectedDeviceTransport}
        onConfigChange={onConfigChange}
        onActiveDeviceProfileIdChange={onActiveDeviceProfileIdChange}
        refreshDeviceConfig={refreshDeviceConfig}
        loadDeviceProfiles={loadDeviceProfiles}
        refreshConnectedDeviceContext={refreshConnectedDeviceContext}
      />

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
        icon={<Sparkles className={clsx('h-4 w-4', ((config as any).windowBlur || 'on') !== 'off' ? 'text-primary' : '')} />}
        title={t('controlPanel.system.windowBlurTitle')}
        description={t('controlPanel.system.windowBlurDescription')}
        tip={t('controlPanel.system.windowBlurTip')}
      >
        <ToggleSwitch
          enabled={((config as any).windowBlur || 'on') !== 'off'}
          onChange={handleWindowBlurChange}
          size="sm"
          color="blue"
        />
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
