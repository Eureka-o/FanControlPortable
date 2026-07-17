'use client';

import { useState, useCallback, useEffect, useMemo, type ComponentType, type ReactNode } from 'react';
import { motion, useReducedMotion, type Variants } from 'framer-motion';
import {
  Cpu,
  Blocks,
  Fan,
  Gpu,
  Radio,
  Settings,
  Thermometer,
  TriangleAlert,
  Zap,
} from 'lucide-react';
import { apiService } from '../services/api';
import { types } from '../../../wailsjs/go/models';
import { toast } from 'sonner';
import { useLocale } from '../lib/i18n';
import {
  clampFanSpeedToRange,
  fanSpeedUnitLabel,
  getActiveDeviceProfile,
  getFanSpeedRange,
  getFanSpeedUnit,
  readCurrentFanSpeed,
} from '../lib/fan-speed';
import DeviceDebugPanel from './DeviceDebugPanel';
import { normalizeTransport } from './devices/profile-utils';
import {
  activeProfileIdsByTransportFromConfig,
  configuredDeviceProfiles,
  normalizeWiFiSmartStartStopStandbySpeed,
  profileConnection,
  profileLabel,
} from './settings/device-connection-utils';
import FanControlSection from './settings/FanControlSection';
import DeviceFeaturePanel from './settings/DeviceFeaturePanel';
import DeviceLightingControls from './settings/DeviceLightingControls';
import DeviceConnectionSection from './settings/DeviceConnectionSection';
import SystemSettingsSection from './settings/SystemSettingsSection';
import PluginManagementSection from './settings/PluginManagementSection';
import { RealtimeOverview } from './ui';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';

interface ControlPanelProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  runtimeDeviceProfile?: types.DeviceProfile | null;
  runtimeDeviceCapabilities?: types.DeviceCapabilities | null;
  onDeviceContextRefresh?: () => Promise<unknown>;
}

type SettingsTab = 'device' | 'fan' | 'system' | 'plugins';

const SMART_START_STOP_OPTIONS = [
  { value: 'off', labelKey: 'controlPanel.options.smartStartStop.off.label', descriptionKey: 'controlPanel.options.smartStartStop.off.description' },
  { value: 'immediate', labelKey: 'controlPanel.options.smartStartStop.immediate.label', descriptionKey: 'controlPanel.options.smartStartStop.immediate.description' },
  { value: 'delayed', labelKey: 'controlPanel.options.smartStartStop.delayed.label', descriptionKey: 'controlPanel.options.smartStartStop.delayed.description' },
];
const WIFI_SMART_START_STOP_STANDBY_SPEED_OPTIONS = [1, 2, 5, 10, 15, 20];
const SETTINGS_PANEL_ENTER_DURATION = 0.17;
const SETTINGS_PANEL_EXIT_DURATION = 0.13;

function createSettingsPanelVariants(reduceMotion: boolean): Variants {
  return {
    active: {
      opacity: reduceMotion ? 1 : [0, 1],
      y: reduceMotion ? 0 : [8, 0],
      transition: {
        duration: reduceMotion ? 0 : SETTINGS_PANEL_ENTER_DURATION,
        ease: [0.22, 1, 0.36, 1],
      },
    },
    inactive: {
      opacity: reduceMotion ? 0 : [1, 0],
      y: reduceMotion ? 0 : [0, -6],
      transition: {
        duration: reduceMotion ? 0 : SETTINGS_PANEL_EXIT_DURATION,
        ease: [0.22, 1, 0.36, 1],
      },
    },
  };
}

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function formatOverviewTemperature(value?: number, unavailable = false) {
  return !unavailable && Number.isFinite(value) && Number(value) > 0
    ? `${Math.round(Number(value))}\u00b0C`
    : '--';
}

function formatOverviewPower(value?: number, unavailable = false) {
  const watts = Number(value || 0);
  if (unavailable || !Number.isFinite(watts) || watts <= 0) return '--';
  return `${watts < 10 ? Math.round(watts * 10) / 10 : Math.round(watts)} W`;
}

export default function ControlPanel({
  config,
  onConfigChange,
  isConnected,
  fanData,
  temperature,
  runtimeDeviceProfile,
  runtimeDeviceCapabilities,
  onDeviceContextRefresh,
}: ControlPanelProps) {
  const { t } = useTranslation();
  const { locale } = useLocale();
  const reduceMotion = useReducedMotion();
  const [loadingStates, setLoadingStates] = useState<Record<string, boolean>>({});
  const [activeSettingsTab, setActiveSettingsTab] = useState<SettingsTab>('device');
  const [mountedSettingsTabs, setMountedSettingsTabs] = useState<Record<SettingsTab, boolean>>({
    device: true,
    fan: false,
    system: false,
    plugins: false,
  });
  const overviewRuntimeProfile = isConnected ? runtimeDeviceProfile : null;
  const overviewSpeedUnit = getFanSpeedUnit(fanData as any, config as any, overviewRuntimeProfile as any);
  const overviewSpeedLabel = fanSpeedUnitLabel(overviewSpeedUnit);
  const overviewSpeedRange = useMemo(() => getFanSpeedRange(config as any, overviewSpeedUnit, overviewRuntimeProfile as any), [config, overviewRuntimeProfile, overviewSpeedUnit]);
  const defaultCustomSpeed = useMemo(() => {
    const fallback = overviewSpeedUnit === 'rpm' ? 2000 : 45;
    return clampFanSpeedToRange(fallback, overviewSpeedRange, overviewSpeedRange.min) ?? overviewSpeedRange.min;
  }, [overviewSpeedRange, overviewSpeedUnit]);
  const configuredCustomSpeedValue = useMemo(
    () => clampFanSpeedToRange((config as any).customSpeedRPM, overviewSpeedRange, defaultCustomSpeed) ?? defaultCustomSpeed,
    [config, defaultCustomSpeed, overviewSpeedRange],
  );
  const overviewFanSpeed = clampFanSpeedToRange(readCurrentFanSpeed(fanData, overviewSpeedUnit, config as any, overviewRuntimeProfile as any), overviewSpeedRange)
    ?? ((config as any).customSpeedEnabled ? configuredCustomSpeedValue : undefined);
  const [deviceProfiles, setDeviceProfiles] = useState<types.DeviceProfile[]>(() => configuredDeviceProfiles(config));
  const [activeDeviceProfileId, setActiveDeviceProfileId] = useState<string>(((config as any).activeDeviceProfileId || '') as string);
  const [activeDeviceProfileIdsByTransport, setActiveDeviceProfileIdsByTransport] = useState<Record<string, string>>(
    () => activeProfileIdsByTransportFromConfig(config),
  );
  const [deviceContextRefreshing, setDeviceContextRefreshing] = useState(false);

  const configDeviceProfiles = useMemo(() => configuredDeviceProfiles(config), [config]);
  const availableDeviceProfiles = useMemo(
    () => (deviceProfiles.length > 0 ? deviceProfiles : configDeviceProfiles),
    [configDeviceProfiles, deviceProfiles],
  );
  const activeDeviceProfile = useMemo(
    () => availableDeviceProfiles.find((profile) => profile.id === activeDeviceProfileId)
      || availableDeviceProfiles.find((profile) => profile.id === ((config as any).activeDeviceProfileId || ''))
      || availableDeviceProfiles[0]
      || null,
    [activeDeviceProfileId, availableDeviceProfiles, config],
  );
  const currentDeviceProfile = useMemo(
    () => getActiveDeviceProfile(config as any) as types.DeviceProfile | undefined,
    [config],
  );
  const effectiveDeviceProfile = isConnected && runtimeDeviceProfile ? runtimeDeviceProfile : currentDeviceProfile;
  const connectedDeviceProfile = useMemo(() => {
    if (!isConnected) return null;
    return effectiveDeviceProfile || activeDeviceProfile || null;
  }, [activeDeviceProfile, effectiveDeviceProfile, isConnected]);
  const connectedDeviceTransport = isConnected
    ? normalizeTransport(
      connectedDeviceProfile?.transport
        || ((fanData as any)?.transport as string)
        || ((config as any).deviceTransport as string)
        || '',
    )
    : '';
  const connectedDeviceConnection = profileConnection(connectedDeviceProfile);
  const currentDeviceCapabilities = (isConnected && runtimeDeviceCapabilities)
    ? runtimeDeviceCapabilities
    : effectiveDeviceProfile?.capabilities;
  const currentDeviceSupportsCustomSpeed = currentDeviceCapabilities
    ? currentDeviceCapabilities.supportsCustomSpeed || currentDeviceCapabilities.supportsSetSpeed
    : true;
  const currentDeviceSupportsLighting = !!currentDeviceCapabilities?.supportsLighting;
  const currentDeviceSupportsGearLight = !!((currentDeviceCapabilities as any)?.supportsGearLight || currentDeviceSupportsLighting);
  const currentDeviceSupportsBrightness = !!((currentDeviceCapabilities as any)?.supportsBrightness || currentDeviceSupportsLighting);
  const currentDeviceSupportsPowerOnStart = !!currentDeviceCapabilities?.supportsPowerOnStart;
  const currentDeviceSupportsSmartStartStop = !!currentDeviceCapabilities?.supportsSmartStartStop;
  const currentDeviceSupportsWiFiSmartStartStopStandbySpeed = currentDeviceSupportsSmartStartStop && connectedDeviceTransport === 'wifi';
  const currentDeviceSupportsScreen = !!(currentDeviceCapabilities as any)?.supportsScreen;
  const overviewConnectionName = isConnected
    ? (connectedDeviceProfile ? profileLabel(connectedDeviceProfile) : connectedDeviceTransport.toUpperCase() || '--')
    : t('controlPanel.system.deviceConnection.connectedDevicesEmpty');
  const overviewConnectionDetail = isConnected
    ? [
      connectedDeviceTransport.toUpperCase(),
      connectedDeviceTransport === 'wifi' ? (connectedDeviceConnection.endpoint || (((config as any).fanControlDeviceIp || '') as string)) : '',
    ].filter(Boolean).join(' · ') || t('appShell.status.connected')
    : t('appShell.status.offline');
  const runtimeDeviceContextKey = [
    isConnected ? 'connected' : 'offline',
    runtimeDeviceProfile?.id || '',
    runtimeDeviceProfile?.transport || '',
    (fanData as any)?.transport || '',
  ].join(':');
  const gpuReadState = (((temperature as any)?.gpuReadState as string) || 'unknown');
  const gpuNotPolled = gpuReadState === 'notPolled';
  const overviewThermals = [
    {
      id: 'cpu',
      label: 'CPU',
      model: temperature?.cpuModel?.trim() || '',
      Icon: Cpu,
      temperatureValue: formatOverviewTemperature(temperature?.cpuTemp),
      powerValue: formatOverviewPower(temperature?.cpuPowerWatts),
    },
    {
      id: 'gpu',
      label: 'GPU',
      model: temperature?.gpuModel?.trim() || '',
      Icon: Gpu,
      temperatureValue: formatOverviewTemperature(temperature?.gpuTemp, gpuNotPolled),
      powerValue: formatOverviewPower(temperature?.gpuPowerWatts, gpuNotPolled),
    },
  ];
  const settingsTabs: Array<{ id: SettingsTab; label: string; Icon: ComponentType<{ className?: string }> }> = [
    { id: 'device', label: t('controlPanel.device.sectionTitle'), Icon: Radio },
    { id: 'fan', label: t('controlPanel.fan.sectionTitle'), Icon: Fan },
    { id: 'system', label: t('controlPanel.system.sectionTitle'), Icon: Settings },
    { id: 'plugins', label: t('controlPanel.plugins.sectionTitle'), Icon: Blocks },
  ];
  const smartStartStopOptions = useMemo(
    () => SMART_START_STOP_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey), description: t(item.descriptionKey) })),
    [locale, t],
  );
  const wifiSmartStartStopStandbySpeedOptions = useMemo(
    () => WIFI_SMART_START_STOP_STANDBY_SPEED_OPTIONS.map((percent) => ({ value: percent, label: `${percent}%` })),
    [],
  );
  const settingsPanelVariants = useMemo(() => createSettingsPanelVariants(!!reduceMotion), [reduceMotion]);
  const setLoading = (key: string, value: boolean) => setLoadingStates((prev) => ({ ...prev, [key]: value }));

  const refreshDeviceConfig = useCallback(async () => {
    const latest = await apiService.getConfig();
    const nextConfig = types.AppConfig.createFrom(latest);
    onConfigChange(nextConfig);
    return nextConfig;
  }, [onConfigChange]);

  const loadDeviceProfiles = useCallback(async () => {
    setLoading('deviceProfiles', true);
    try {
      const payload = await apiService.getDeviceProfiles();
      const profiles = Array.isArray(payload?.profiles) ? payload.profiles : [];
      setDeviceProfiles(profiles);
      setActiveDeviceProfileId(payload?.activeId || profiles[0]?.id || '');
      setActiveDeviceProfileIdsByTransport((payload?.activeIdsByTransport || {}) as Record<string, string>);
      return profiles;
    } catch (error) {
      toast.error(t('controlPanel.system.deviceConnection.toasts.loadFailed', { error: getErrorMessage(error) }));
      return [];
    } finally {
      setLoading('deviceProfiles', false);
    }
  }, [t]);

  const refreshConnectedDeviceContext = useCallback(async () => {
    setDeviceContextRefreshing(true);
    try {
      if (onDeviceContextRefresh) {
        await onDeviceContextRefresh();
      } else {
        await refreshDeviceConfig();
      }
      await loadDeviceProfiles();
    } finally {
      window.setTimeout(() => setDeviceContextRefreshing(false), 450);
    }
  }, [loadDeviceProfiles, onDeviceContextRefresh, refreshDeviceConfig]);

  const handleGearLightChange = useCallback(async (enabled: boolean) => {
    if (!isConnected) return;
    setLoading('gearLight', true);
    try {
      const ok = await apiService.setGearLight(enabled);
      if (ok) {
        onConfigChange(types.AppConfig.createFrom({ ...config, gearLight: enabled }));
      } else {
        toast.error(t('controlPanel.alerts.deviceCommandFailed'));
      }
    } catch (error) {
      toast.error(t('controlPanel.alerts.deviceCommandFailedWithError', { error: getErrorMessage(error) }));
    } finally { setLoading('gearLight', false); }
  }, [config, onConfigChange, isConnected, t]);

  const handlePowerOnStartChange = useCallback(async (enabled: boolean) => {
    if (!isConnected) return;
    setLoading('powerOnStart', true);
    try {
      const ok = await apiService.setPowerOnStart(enabled);
      if (ok) {
        onConfigChange(types.AppConfig.createFrom({ ...config, powerOnStart: enabled }));
      } else {
        toast.error(t('controlPanel.alerts.deviceCommandFailed'));
      }
    } catch (error) {
      toast.error(t('controlPanel.alerts.deviceCommandFailedWithError', { error: getErrorMessage(error) }));
    } finally { setLoading('powerOnStart', false); }
  }, [config, onConfigChange, isConnected, t]);

  const handleSmartStartStopChange = useCallback(async (mode: string) => {
    if (!isConnected) return;
    try {
      const ok = await apiService.setSmartStartStop(mode);
      if (ok) {
        const standbySpeed = normalizeWiFiSmartStartStopStandbySpeed((config as any).wifiSmartStartStopStandbySpeed);
        onConfigChange(types.AppConfig.createFrom({ ...config, smartStartStop: mode, wifiSmartStartStopStandbySpeed: standbySpeed }));
      } else {
        toast.error(t('controlPanel.alerts.deviceCommandFailed'));
      }
    } catch (error) {
      toast.error(t('controlPanel.alerts.deviceCommandFailedWithError', { error: getErrorMessage(error) }));
    }
  }, [config, onConfigChange, isConnected, t]);

  const handleWiFiSmartStartStopStandbySpeedChange = useCallback(async (value: string | number) => {
    if (!isConnected) return;
    const standbySpeed = normalizeWiFiSmartStartStopStandbySpeed(value);
    setLoading('wifiSmartStartStopStandbySpeed', true);
    try {
      const ok = await apiService.setWiFiSmartStartStopStandbySpeed(standbySpeed);
      if (ok) {
        onConfigChange(types.AppConfig.createFrom({ ...config, wifiSmartStartStopStandbySpeed: standbySpeed }));
      } else {
        toast.error(t('controlPanel.alerts.deviceCommandFailed'));
      }
    } catch (error) {
      toast.error(t('controlPanel.alerts.deviceCommandFailedWithError', { error: getErrorMessage(error) }));
    } finally {
      setLoading('wifiSmartStartStopStandbySpeed', false);
    }
  }, [config, onConfigChange, isConnected, t]);

  useEffect(() => { void loadDeviceProfiles(); }, [loadDeviceProfiles]);
  useEffect(() => {
    const profiles = configuredDeviceProfiles(config);
    if (profiles.length > 0) {
      setDeviceProfiles(profiles);
    }
    setActiveDeviceProfileId(((config as any).activeDeviceProfileId || activeDeviceProfile?.id || '') as string);
    setActiveDeviceProfileIdsByTransport(activeProfileIdsByTransportFromConfig(config));
  }, [
    activeDeviceProfile?.id,
    config,
  ]);
  useEffect(() => {
    if (!isConnected) {
      setDeviceContextRefreshing(false);
      return;
    }
    setDeviceContextRefreshing(true);
    const timer = window.setTimeout(() => setDeviceContextRefreshing(false), 700);
    return () => window.clearTimeout(timer);
  }, [isConnected, runtimeDeviceContextKey]);

  const settingsPanelContent: Record<SettingsTab, ReactNode> = {
    device: (
      <DeviceFeaturePanel
        config={config}
        isConnected={isConnected}
        refreshing={deviceContextRefreshing}
        deviceProfile={effectiveDeviceProfile || null}
        loadingStates={loadingStates}
        supportsGearLight={currentDeviceSupportsGearLight}
        supportsPowerOnStart={currentDeviceSupportsPowerOnStart}
        supportsSmartStartStop={currentDeviceSupportsSmartStartStop}
        supportsWiFiSmartStartStopStandbySpeed={currentDeviceSupportsWiFiSmartStartStopStandbySpeed}
        supportsScreen={currentDeviceSupportsScreen}
        smartStartStopOptions={smartStartStopOptions}
        wifiSmartStartStopStandbySpeedOptions={wifiSmartStartStopStandbySpeedOptions}
        onGearLightChange={handleGearLightChange}
        onPowerOnStartChange={handlePowerOnStartChange}
        onSmartStartStopChange={handleSmartStartStopChange}
        onWiFiSmartStartStopStandbySpeedChange={handleWiFiSmartStartStopStandbySpeedChange}
        lightingControls={currentDeviceSupportsLighting ? (
          <DeviceLightingControls
            config={config}
            onConfigChange={onConfigChange}
            isConnected={isConnected}
            supportsBrightness={currentDeviceSupportsBrightness}
          />
        ) : undefined}
      >
        <DeviceConnectionSection
          config={config}
          availableDeviceProfiles={availableDeviceProfiles}
          activeDeviceProfileId={activeDeviceProfileId}
          activeDeviceProfileIdsByTransport={activeDeviceProfileIdsByTransport}
          connectedDeviceProfile={connectedDeviceProfile}
          connectedDeviceTransport={connectedDeviceTransport}
          onConfigChange={onConfigChange}
          onActiveDeviceProfileIdChange={setActiveDeviceProfileId}
          refreshDeviceConfig={refreshDeviceConfig}
          loadDeviceProfiles={loadDeviceProfiles}
          refreshConnectedDeviceContext={refreshConnectedDeviceContext}
        />
      </DeviceFeaturePanel>
    ),
    fan: (
      <FanControlSection
        config={config}
        onConfigChange={onConfigChange}
        isConnected={isConnected}
        fanData={fanData}
        temperature={temperature}
        runtimeDeviceProfile={effectiveDeviceProfile || null}
        supportsCustomSpeed={currentDeviceSupportsCustomSpeed}
      />
    ),
    system: (
      <SystemSettingsSection
        config={config}
        onConfigChange={onConfigChange}
      />
    ),
    plugins: <PluginManagementSection />,
  };

  return (
    <>
      <div data-theme-section="settings-page" data-page-reveal="cards" className="space-y-4">
        <RealtimeOverview
          title={t('controlPanel.overview.title')}
          titleIcon={<Settings className="h-4 w-4" />}
          hardware={overviewThermals.map(({ id, label, model, Icon, temperatureValue, powerValue }) => ({
            id,
            label,
            model,
            icon: <Icon className="h-4.5 w-4.5" />,
            metrics: [
              {
                id: 'temperature',
                icon: <Thermometer className="h-3.5 w-3.5" />,
                label: t('controlPanel.overview.temperatureMetric'),
                value: temperatureValue,
              },
              {
                id: 'power',
                icon: <Zap className="h-3.5 w-3.5" />,
                label: t('controlPanel.overview.powerMetric'),
                value: powerValue,
              },
            ],
          }))}
          device={{
            icon: <Radio className="h-4.5 w-4.5" />,
            name: overviewConnectionName,
            connected: isConnected,
            connectionLabel: overviewConnectionDetail,
            details: [
              {
                id: 'speed',
                label: t('controlPanel.overview.currentRpm'),
                value: `${overviewFanSpeed ?? '--'}${overviewSpeedLabel}`,
              },
              {
                id: 'mode',
                label: t('controlPanel.overview.controlModeMetric'),
                value: config.autoControl ? t('appShell.status.smartControl') : t('appShell.status.manualMode'),
              },
            ],
          }}
        />

        <div
          data-theme-ui="settings-tabs"
          role="tablist"
          aria-label={t('controlPanel.system.sectionTitle')}
          className="grid grid-cols-2 gap-1 rounded-[18px] border border-border/70 bg-card/92 p-1.5 shadow-sm shadow-black/5 backdrop-blur-xl sm:grid-cols-4"
        >
          {settingsTabs.map(({ id, label, Icon }) => (
            <button
              key={id}
              id={`settings-tab-${id}`}
              data-theme-ui="settings-tab"
              type="button"
              role="tab"
              aria-selected={activeSettingsTab === id}
              aria-controls={`settings-panel-${id}`}
              onClick={() => {
                if (activeSettingsTab === id) return;
                setMountedSettingsTabs((previous) => previous[id] ? previous : { ...previous, [id]: true });
                setActiveSettingsTab(id);
              }}
              className={clsx(
                'relative min-w-0 rounded-xl px-3 py-2.5 text-sm font-medium',
                activeSettingsTab === id
                  ? 'text-primary'
                  : 'text-sidebar-foreground/62 hover:bg-sidebar-accent hover:text-sidebar-foreground',
              )}
            >
              <span className="flex min-w-0 items-center justify-center gap-2">
                <Icon className="h-4 w-4 shrink-0" />
                <span className="truncate">{label}</span>
              </span>
            </button>
          ))}
        </div>

        <div data-theme-ui="settings-panels" className="relative">
          {settingsTabs.filter(({ id }) => mountedSettingsTabs[id]).map(({ id }) => {
            const active = activeSettingsTab === id;
            return (
              <motion.div
                key={id}
                id={`settings-panel-${id}`}
                data-theme-ui="settings-panel"
                data-state={active ? 'active' : 'inactive'}
                role="tabpanel"
                aria-labelledby={`settings-tab-${id}`}
                aria-hidden={activeSettingsTab !== id}
                tabIndex={active ? 0 : -1}
                inert={active ? undefined : true}
                initial={id === 'device' ? false : { opacity: 0, y: reduceMotion ? 0 : 8 }}
                animate={active ? 'active' : 'inactive'}
                variants={settingsPanelVariants}
                className={clsx(
                  'w-full will-change-[opacity,transform]',
                  active ? 'relative' : 'pointer-events-none absolute inset-x-0 top-0',
                )}
              >
                {settingsPanelContent[id]}
              </motion.div>
            );
          })}
        </div>
        {!isConnected && (
          <div data-theme-card="settings-offline-tip" className="flex items-center gap-2 rounded-xl border border-border bg-muted/50 px-4 py-3 text-sm text-muted-foreground">
            <TriangleAlert className="h-4 w-4 shrink-0" />
            {t('controlPanel.offline.message')}
          </div>
        )}

        <DeviceDebugPanel
          config={config}
          isConnected={isConnected}
          onConfigChange={onConfigChange}
        />
      </div>
    </>
  );
}
