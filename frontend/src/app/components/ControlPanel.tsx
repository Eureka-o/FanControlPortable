'use client';

import { useState, useCallback, useEffect, useMemo, memo, type ReactNode } from 'react';
import { m, useReducedMotion, type Variants } from 'framer-motion';
import {
  Cpu,
  Gpu,
  Monitor,
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
import { selectFanData, selectTemperature, useAppStore } from '../store/app-store';
import { applyPowerSpoofToTemperature } from '../lib/power-spoof';
import {
  clampFanSpeedToRange,
  fanSpeedUnitLabel,
  getActiveDeviceProfile,
  getFanSpeedRange,
  getFanSpeedUnit,
  readCurrentFanSpeed,
} from '../lib/fan-speed';
import { BlackSharkPanel, ScenePanel, ScreenPanel } from './BlackSharkPanels';
import DeviceDebugPanelBase from './DeviceDebugPanel';
import { normalizeTransport } from './devices/profile-utils';
import {
  activeProfileIdsByTransportFromConfig,
  configuredDeviceProfiles,
  normalizeWiFiSmartStartStopStandbySpeed,
  profileConnection,
  profileLabel,
} from './settings/device-connection-utils';
import FanControlSection from './settings/FanControlSection';
import DeviceFeaturePanelBase from './settings/DeviceFeaturePanel';
import DeviceLightingControls from './settings/DeviceLightingControls';
import DeviceImageTransferControls from './settings/DeviceImageTransferControls';
import { Section } from './settings/SettingLayout';
import DeviceConnectionSection from './settings/DeviceConnectionSection';
import PluginsSection from './settings/PluginsSection';
import SystemSettingsSectionBase from './settings/SystemSettingsSection';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';

interface ControlPanelProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  monitorOnlyActive?: boolean;
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
const SETTINGS_TAB_ORDER: Record<SettingsTab, number> = {
  device: 0,
  fan: 1,
  system: 2,
  plugins: 3,
};
const SETTINGS_PANEL_ENTER_DURATION = 0.36;
const SETTINGS_PANEL_ENTER_DELAY = 0.04;
const SETTINGS_PANEL_EXIT_DURATION = 0.18;
const SETTINGS_PANEL_SLIDE_OFFSET = 20;
const SETTINGS_PANEL_EXIT_OFFSET = 12;
const SETTINGS_PANEL_EASE: [number, number, number, number] = [0.32, 0.72, 0, 1];
const SETTINGS_PANEL_EXIT_EASE: [number, number, number, number] = [0.4, 0, 0.2, 1];

function createSettingsPanelVariants(reduceMotion: boolean): Variants {
  return {
    active: (direction: number) => ({
      opacity: 1,
      x: reduceMotion ? 0 : [direction * SETTINGS_PANEL_SLIDE_OFFSET, 0],
      transition: {
        opacity: {
          duration: reduceMotion ? 0 : SETTINGS_PANEL_ENTER_DURATION - SETTINGS_PANEL_ENTER_DELAY,
          delay: reduceMotion ? 0 : SETTINGS_PANEL_ENTER_DELAY,
          ease: SETTINGS_PANEL_EASE,
        },
        x: {
          duration: reduceMotion ? 0 : SETTINGS_PANEL_ENTER_DURATION,
          ease: SETTINGS_PANEL_EASE,
        },
      },
    }),
    exiting: (direction: number) => ({
      opacity: 0,
      x: reduceMotion ? 0 : direction * -SETTINGS_PANEL_EXIT_OFFSET,
      transition: {
        duration: reduceMotion ? 0 : SETTINGS_PANEL_EXIT_DURATION,
        ease: SETTINGS_PANEL_EXIT_EASE,
      },
    }),
    hidden: {
      opacity: 0,
      x: 0,
      transition: {
        duration: 0,
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
  if (unavailable || !Number.isFinite(watts) || watts < 0.05) return '--';
  const rounded = watts < 10 ? Math.round(watts * 10) / 10 : Math.round(watts);
  return rounded > 0 ? `${rounded} W` : '--';
}

// 设置页里几块不随遥测变化的大子树 memo 化：父级因 loading/tab 切换重渲时整块跳过。
const DeviceFeaturePanel = memo(DeviceFeaturePanelBase);
const SystemSettingsSection = memo(SystemSettingsSectionBase);
const DeviceDebugPanel = memo(DeviceDebugPanelBase);

// 「风扇」设置页的温度基线区展示实时传感器读数，但它是设置面板里唯一需要遥测的部分：
// 用自订阅包装承载 fanData/temperature，ControlPanel 本体不再按遥测节拍重渲。
const LiveFanControlSection = memo(function LiveFanControlSection({
  config,
  onConfigChange,
  isConnected,
  runtimeDeviceProfile,
  supportsCustomSpeed,
}: {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  runtimeDeviceProfile?: types.DeviceProfile | null;
  supportsCustomSpeed: boolean;
}) {
  const fanData = useAppStore(selectFanData);
  const rawTemperature = useAppStore(selectTemperature);
  const temperature = useMemo(
    () => applyPowerSpoofToTemperature(rawTemperature, config),
    [config, rawTemperature],
  );
  return (
    <FanControlSection
      config={config}
      onConfigChange={onConfigChange}
      isConnected={isConnected}
      fanData={fanData}
      temperature={temperature}
      runtimeDeviceProfile={runtimeDeviceProfile}
      supportsCustomSpeed={supportsCustomSpeed}
    />
  );
});

// 设置页顶部的遥测概览条（温度/功率 + 当前转速/连接状态）：唯一逐拍刷新的区块。
// 自己用精确 selector 订阅 store 并在本地做电源伪装换算，ControlPanel 不再接收遥测 props。
const ControlPanelTelemetryOverview = memo(function ControlPanelTelemetryOverview({
  config,
  isConnected,
  runtimeDeviceProfile,
  overviewConnectionName,
  overviewConnectionDetail,
}: {
  config: types.AppConfig;
  isConnected: boolean;
  runtimeDeviceProfile?: types.DeviceProfile | null;
  overviewConnectionName: string;
  overviewConnectionDetail: string;
}) {
  const { t } = useTranslation();
  const fanData = useAppStore(selectFanData);
  const rawTemperature = useAppStore(selectTemperature);
  const temperature = useMemo(
    () => applyPowerSpoofToTemperature(rawTemperature, config),
    [config, rawTemperature],
  );
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

  return (
    <section data-theme-card="settings-overview" className="rounded-[20px] border border-border/75 bg-card p-5 shadow-sm">
      <div className="mb-4 flex items-center gap-2.5">
        <div data-theme-ui="setting-section-icon" data-icon-tone="blue" className="flex h-7 w-7 shrink-0 items-center justify-center rounded-[9px] bg-primary/10 text-primary">
          <Settings className="h-4 w-4" />
        </div>
        <h3 className="text-base font-semibold tracking-[-0.015em] text-foreground">{t('controlPanel.overview.title')}</h3>
      </div>
      <div className="grid grid-cols-1 gap-3.5 md:grid-cols-[minmax(0,1.2fr)_minmax(220px,0.8fr)]">
        <div data-theme-card="settings-overview-temperature" className="grid min-h-[8.5rem] grid-rows-2 divide-y divide-border/55 rounded-2xl border border-border/70 bg-muted/30 px-4">
          {overviewThermals.map(({ id, label, model, Icon, temperatureValue, powerValue }) => (
            <div key={id} className="flex min-w-0 items-center gap-3 py-3">
              <div
                data-theme-ui="setting-row-icon"
                data-icon-tone={id === 'cpu' ? 'blue' : 'purple'}
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[11px] bg-background/65 text-muted-foreground shadow-inner shadow-white/15"
              >
                <Icon className="h-4.5 w-4.5" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex min-h-5 min-w-0 items-center gap-2">
                  <div className="shrink-0 text-sm font-semibold text-foreground">{label}</div>
                  {model && (
                    <span
                      data-theme-ui="settings-overview-model"
                      title={model}
                      className="ml-auto min-w-0 max-w-[min(68%,22rem)] truncate rounded-full border border-primary/20 bg-background/80 px-2 py-0.5 text-[10px] font-medium leading-4 text-foreground/75 shadow-sm shadow-black/15 backdrop-blur-md"
                    >
                      {model}
                    </span>
                  )}
                </div>
                <div
                  data-theme-ui="settings-overview-metrics"
                  className="mt-1.5 grid min-w-0 grid-cols-[1rem_2.25rem_minmax(3.25rem,1fr)_1rem_2.25rem_minmax(3.25rem,1fr)] items-center gap-x-1.5 text-[11px] leading-none text-muted-foreground"
                >
                  <Thermometer className="h-3.5 w-3.5" />
                  <span className="whitespace-nowrap">{t('controlPanel.overview.temperatureMetric')}</span>
                  <span className="whitespace-nowrap text-sm font-semibold tabular-nums text-foreground">{temperatureValue}</span>
                  <Zap className="h-3.5 w-3.5" />
                  <span className="whitespace-nowrap">{t('controlPanel.overview.powerMetric')}</span>
                  <span className="whitespace-nowrap text-sm font-semibold tabular-nums text-foreground">{powerValue}</span>
                </div>
              </div>
            </div>
          ))}
        </div>

        <div data-theme-card="settings-overview-device" className="grid min-h-[8.5rem] grid-rows-2 divide-y divide-border/55 rounded-2xl border border-border/70 bg-muted/30 px-4">
          <div className="flex min-w-0 items-center gap-3 py-3">
            <div
              data-theme-ui="setting-row-icon"
              data-icon-tone="emerald"
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[11px] bg-background/65 text-muted-foreground shadow-inner shadow-white/15"
            >
              <Radio className="h-4.5 w-4.5" />
            </div>
            <div className="min-w-0 flex-1">
              <div title={overviewConnectionName} className="line-clamp-2 break-words text-sm font-semibold leading-snug text-foreground">{overviewConnectionName}</div>
              <div className="mt-1.5 flex min-w-0 items-center gap-2 text-[11px] text-muted-foreground">
                <span className={clsx('h-2 w-2 shrink-0 rounded-full', isConnected ? 'bg-emerald-500 ring-4 ring-emerald-500/15' : 'bg-muted-foreground/45')} />
                <span className="truncate">{overviewConnectionDetail}</span>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-2 items-center gap-4 py-2.5">
            <div className="min-w-0">
              <div className="text-[11px] text-muted-foreground">{t('controlPanel.overview.currentRpm')}</div>
              <div className="mt-0.5 truncate text-base font-semibold tabular-nums text-foreground">{overviewFanSpeed ?? '--'}{overviewSpeedLabel}</div>
            </div>
            <div className="min-w-0">
              <div className="text-[11px] text-muted-foreground">{t('controlPanel.overview.controlModeMetric')}</div>
              <div className="mt-0.5 truncate text-sm font-semibold text-foreground">
                {config.autoControl ? t('appShell.status.smartControl') : t('appShell.status.manualMode')}
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
});

export default function ControlPanel({
  config,
  onConfigChange,
  isConnected,
  monitorOnlyActive = false,
  runtimeDeviceProfile,
  runtimeDeviceCapabilities,
  onDeviceContextRefresh,
}: ControlPanelProps) {
  const { t } = useTranslation();
  const { locale } = useLocale();
  const reduceMotion = useReducedMotion();
  const [loadingStates, setLoadingStates] = useState<Record<string, boolean>>({});
  const [activeSettingsTab, setActiveSettingsTab] = useState<SettingsTab>('device');
  const [previousSettingsTab, setPreviousSettingsTab] = useState<SettingsTab | null>(null);
  const [settingsTabDirection, setSettingsTabDirection] = useState<1 | -1>(1);
  const [isSettingsTabAnimating, setIsSettingsTabAnimating] = useState(false);
  const [mountedSettingsTabs, setMountedSettingsTabs] = useState<Record<SettingsTab, boolean>>({
    device: true,
    fan: false,
    system: false,
    plugins: false,
  });
  useEffect(() => {
    if (!isSettingsTabAnimating) return;
    const timer = window.setTimeout(
      () => setIsSettingsTabAnimating(false),
      Math.round(SETTINGS_PANEL_ENTER_DURATION * 1000) + 40,
    );
    return () => window.clearTimeout(timer);
  }, [activeSettingsTab, isSettingsTabAnimating]);
  // 遥测不再作为 props 进入 ControlPanel；只有 fanData.transport 仍被读取（连接回退与上下文键），
  // 用窄 selector 只订阅这一个字段，字段不变就不重渲。
  const fanDataTransport = useAppStore((state) => state.fanData?.transport ?? null);
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
        || fanDataTransport
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
  // 黑鲨专属控制区的判据取设备档案 id 前缀 `builtin.blackshark.brb02.`（isBlackSharkDevice）：
  // 黑鲨有 BLE 与 USB 两条档案，都带此前缀；钉死具体档案 id 则另一条命中不到。

  const currentDeviceSupportsLighting = isConnected && !monitorOnlyActive && !!currentDeviceCapabilities?.supportsLighting;
  const currentDeviceSupportsGearLight = isConnected && !monitorOnlyActive && !!(currentDeviceCapabilities as any)?.supportsGearLight;
  const currentDeviceSupportsBrightness = isConnected && !monitorOnlyActive && !!((currentDeviceCapabilities as any)?.supportsBrightness || currentDeviceSupportsLighting);
  const currentDeviceSupportsPowerOnStart = !!currentDeviceCapabilities?.supportsPowerOnStart;
  const currentDeviceSupportsSmartStartStop = !!currentDeviceCapabilities?.supportsSmartStartStop;
  const currentDeviceSupportsWiFiSmartStartStopStandbySpeed = currentDeviceSupportsSmartStartStop && connectedDeviceTransport === 'wifi';
  const currentDeviceSupportsScreenImageTransfer = isConnected && (
    !!(runtimeDeviceCapabilities as any)?.supportsScreenImageTransfer
    || !!(effectiveDeviceProfile?.capabilities as any)?.supportsScreenImageTransfer
  );
  const isBlackSharkDevice = isConnected
    && (runtimeDeviceProfile?.id || effectiveDeviceProfile?.id || '').startsWith('builtin.blackshark.brb02.');

  // 黑鲨的屏幕图片由它自己的「屏幕」页提供；上游那个通用图传入口对黑鲨不渲染
  // （灯效同理，已由档案的能力位关掉：黑鲨不声明灯效/亮度能力）。
  const showDeviceImageTransfer = isConnected && !isBlackSharkDevice && currentDeviceSupportsScreenImageTransfer;
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
    fanDataTransport || '',
  ].join(':');
  const settingsTabs: Array<{ id: SettingsTab; label: string }> = [
    { id: 'device', label: t('controlPanel.device.sectionTitle') },
    { id: 'fan', label: t('controlPanel.fan.sectionTitle') },
    { id: 'system', label: t('controlPanel.system.sectionTitle') },
    { id: 'plugins', label: t('controlPanel.plugins.sectionTitle') },
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

  // 重设置区的 JSX 元素用 useMemo 稳住：ControlPanel 因 tab 切换/loading 翻转重渲时，
  // memo 化的 DeviceFeaturePanel / SystemSettingsSection / DeviceDebugPanel 依 props 浅比较整块跳过。
  const deviceFeaturePanelElement = useMemo(() => (
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
  ), [
    activeDeviceProfileId,
    activeDeviceProfileIdsByTransport,
    availableDeviceProfiles,
    config,
    connectedDeviceProfile,
    connectedDeviceTransport,
    currentDeviceSupportsBrightness,
    currentDeviceSupportsGearLight,
    currentDeviceSupportsLighting,
    currentDeviceSupportsPowerOnStart,
    currentDeviceSupportsSmartStartStop,
    currentDeviceSupportsWiFiSmartStartStopStandbySpeed,
    deviceContextRefreshing,
    effectiveDeviceProfile,
    handleGearLightChange,
    handlePowerOnStartChange,
    handleSmartStartStopChange,
    handleWiFiSmartStartStopStandbySpeedChange,
    isConnected,
    loadDeviceProfiles,
    loadingStates,
    onConfigChange,
    refreshConnectedDeviceContext,
    refreshDeviceConfig,
    setActiveDeviceProfileId,
    smartStartStopOptions,
    wifiSmartStartStopStandbySpeedOptions,
  ]);

  const settingsPanelContent: Record<SettingsTab, ReactNode> = {
    device: (
      <div className="space-y-5">
        {deviceFeaturePanelElement}
        {showDeviceImageTransfer && (
          <Section
            title={t('controlPanel.device.groups.imageTransfer')}
            icon={Monitor}
            iconTone="blue"
          >
            <div className="px-5 py-4">
              <DeviceImageTransferControls
                isConnected={isConnected}
                transport={connectedDeviceTransport}
                canTransfer={currentDeviceSupportsScreenImageTransfer}
              />
            </div>
          </Section>
        )}
        {isBlackSharkDevice && <BlackSharkPanel />}
        {isBlackSharkDevice && <ScenePanel />}
        {isBlackSharkDevice && <ScreenPanel />}
      </div>
    ),
    fan: (
      // 黑鲨也走同一套 FanControlSection；四档散热曲线与「曲线」页重复，不再单列。
      // 遥测由 LiveFanControlSection 自行订阅，只在这一个挂载的设置页内逐拍更新。
      <LiveFanControlSection
        config={config}
        onConfigChange={onConfigChange}
        isConnected={isConnected}
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
    plugins: (
      <PluginsSection
        config={config}
        onConfigChange={onConfigChange}
      />
    ),
  };

  return (
    <>
      <div data-theme-section="settings-page" data-page-reveal="cards" className="space-y-5">
        <ControlPanelTelemetryOverview
          config={config}
          isConnected={isConnected}
          runtimeDeviceProfile={runtimeDeviceProfile}
          overviewConnectionName={overviewConnectionName}
          overviewConnectionDetail={overviewConnectionDetail}
        />

        <div
          data-theme-ui="settings-tabs"
          role="tablist"
          aria-label={t('controlPanel.system.sectionTitle')}
          className="grid grid-cols-4 gap-1 rounded-[18px] relative border border-border/70 bg-card/92 p-1.5 shadow-sm shadow-black/5"
        >
          <span
            aria-hidden="true"
            className={clsx(
              'settings-tab-active-pill pointer-events-none absolute bottom-1.5 left-1.5 top-1.5 w-[calc((100%-1.5rem)/4)] rounded-[12px]',
              reduceMotion ? 'transition-none' : 'transition-transform duration-[360ms] ease-[cubic-bezier(0.32,0.72,0,1)]',
            )}
            style={{
              transform: `translateX(calc(${Math.max(0, settingsTabs.findIndex((tab) => tab.id === activeSettingsTab))} * (100% + 0.25rem)))`,
            }}
          />
          {settingsTabs.map(({ id, label }) => (
            <button
              key={id}
              id={`settings-tab-${id}`}
              data-theme-ui="settings-tab"
              type="button"
              role="tab"
              aria-selected={activeSettingsTab === id}
              aria-controls={`settings-panel-${id}`}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => {
                if (activeSettingsTab === id) return;
                const nextDirection: 1 | -1 = SETTINGS_TAB_ORDER[id] > SETTINGS_TAB_ORDER[activeSettingsTab] ? 1 : -1;
                setSettingsTabDirection(nextDirection);
                setPreviousSettingsTab(activeSettingsTab);
                setIsSettingsTabAnimating(!reduceMotion);
                setMountedSettingsTabs((previous) => (previous[id] ? previous : { ...previous, [id]: true }));
                setActiveSettingsTab(id);
              }}
              className={clsx(
                'relative z-10 min-w-0 rounded-xl px-3 py-2 text-[14px] font-semibold tracking-[-0.01em] outline-none transition-colors duration-300 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring/50',
                activeSettingsTab === id
                  ? 'text-primary'
                  : 'text-sidebar-foreground/62 hover:bg-sidebar-accent hover:text-sidebar-foreground',
              )}
            >
              <span className="block truncate">{label}</span>
            </button>
          ))}
        </div>

        <div data-theme-ui="settings-panels" className="relative">
          <div className="-mx-3 -my-2 overflow-clip px-3 py-2">
            <div className="relative">
              {settingsTabs.filter(({ id }) => mountedSettingsTabs[id]).map(({ id }) => {
                const active = activeSettingsTab === id;
                const panelState = active ? 'active' : previousSettingsTab === id ? 'exiting' : 'hidden';
                return (
                  <m.div
                    key={id}
                    id={`settings-panel-${id}`}
                    data-theme-ui="settings-panel"
                    data-state={active ? 'active' : 'inactive'}
                    role="tabpanel"
                    aria-labelledby={`settings-tab-${id}`}
                    aria-hidden={activeSettingsTab !== id}
                    tabIndex={-1}
                    inert={active ? undefined : true}
                    custom={settingsTabDirection}
                    initial={id === 'device' ? false : { opacity: 0, x: reduceMotion ? 0 : settingsTabDirection * SETTINGS_PANEL_SLIDE_OFFSET }}
                    animate={panelState}
                    variants={settingsPanelVariants}
                    className={clsx(
                      'w-full outline-none',
                      isSettingsTabAnimating && panelState !== 'hidden' && 'will-change-[opacity,transform]',
                      active ? 'relative' : 'pointer-events-none absolute inset-x-0 top-0',
                      active ? 'z-10' : 'z-0',
                    )}
                  >
                    {settingsPanelContent[id]}
                  </m.div>
                );
              })}
            </div>
          </div>
        </div>
        {!isConnected && (
          <div data-theme-card="settings-offline-tip" className="flex items-center gap-2.5 rounded-2xl border border-amber-500/25 bg-amber-500/8 px-4 py-3 text-[13px] leading-[1.45] text-amber-700 dark:text-amber-300">
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
