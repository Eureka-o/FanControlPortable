'use client';

import { useState, useCallback, useEffect, useMemo } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import {
  Cpu,
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
import { supportsManualGearsFromCapabilities } from '../lib/manualGearPresets';
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
import SystemSettingsSection from './settings/SystemSettingsSection';
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

type SettingsTab = 'device' | 'fan' | 'system';

const SMART_START_STOP_OPTIONS = [
  { value: 'off', labelKey: 'controlPanel.options.smartStartStop.off.label', descriptionKey: 'controlPanel.options.smartStartStop.off.description' },
  { value: 'immediate', labelKey: 'controlPanel.options.smartStartStop.immediate.label', descriptionKey: 'controlPanel.options.smartStartStop.immediate.description' },
  { value: 'delayed', labelKey: 'controlPanel.options.smartStartStop.delayed.label', descriptionKey: 'controlPanel.options.smartStartStop.delayed.description' },
];
const WIFI_SMART_START_STOP_STANDBY_SPEED_OPTIONS = [1, 2, 5, 10, 15, 20];

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
  const [loadingStates, setLoadingStates] = useState<Record<string, boolean>>({});
  const [activeSettingsTab, setActiveSettingsTab] = useState<SettingsTab>('device');
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
  const currentDeviceSupportsManualGears = supportsManualGearsFromCapabilities(currentDeviceCapabilities);
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
  const configuredDeviceCurveKey = isConnected && effectiveDeviceProfile
    ? `${normalizeTransport(effectiveDeviceProfile.transport)}:${effectiveDeviceProfile.id || effectiveDeviceProfile.model || effectiveDeviceProfile.displayName || ''}`
    : `${((config as any).deviceTransport || '') as string}:${((config as any).activeDeviceProfileId || '') as string}`;

  const gpuReadState = (((temperature as any)?.gpuReadState as string) || 'unknown');
  const gpuNotPolled = gpuReadState === 'notPolled';
  const overviewThermals = [
    {
      id: 'cpu',
      label: 'CPU',
      Icon: Cpu,
      temperatureValue: formatOverviewTemperature(temperature?.cpuTemp),
      powerValue: formatOverviewPower(temperature?.cpuPowerWatts),
    },
    {
      id: 'gpu',
      label: 'GPU',
      Icon: Gpu,
      temperatureValue: formatOverviewTemperature(temperature?.gpuTemp, gpuNotPolled),
      powerValue: formatOverviewPower(temperature?.gpuPowerWatts, gpuNotPolled),
    },
  ];
  const settingsTabs: Array<{ id: SettingsTab; label: string }> = [
    { id: 'device', label: t('controlPanel.device.sectionTitle') },
    { id: 'fan', label: t('controlPanel.fan.sectionTitle') },
    { id: 'system', label: t('controlPanel.system.sectionTitle') },
  ];
  const smartStartStopOptions = useMemo(
    () => SMART_START_STOP_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey), description: t(item.descriptionKey) })),
    [locale, t],
  );
  const wifiSmartStartStopStandbySpeedOptions = useMemo(
    () => WIFI_SMART_START_STOP_STANDBY_SPEED_OPTIONS.map((percent) => ({ value: percent, label: `${percent}%` })),
    [],
  );
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

  return (
    <>
      <div data-theme-section="settings-page" data-page-reveal="cards" className="space-y-4">
        <section data-theme-card="settings-overview" className="rounded-2xl border border-border bg-card p-5 shadow-sm">
          <div className="mb-4 flex items-center gap-2">
            <Settings className="h-4 w-4 text-muted-foreground" />
            <h3 className="text-base font-semibold text-foreground">{t('controlPanel.overview.title')}</h3>
          </div>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-[minmax(0,1.2fr)_minmax(220px,0.8fr)]">
            <div data-theme-card="settings-overview-temperature" className="divide-y divide-border/55 rounded-xl border border-border/70 bg-muted/30 px-4">
              {overviewThermals.map(({ id, label, Icon, temperatureValue, powerValue }) => (
                <div key={id} className="flex min-w-0 items-center gap-3 py-4">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-background/65 text-muted-foreground shadow-inner shadow-white/15">
                    <Icon className="h-5 w-5" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="text-base font-semibold text-foreground">{label}</div>
                    <div className="mt-1 flex flex-wrap items-center gap-x-5 gap-y-1 text-xs text-muted-foreground">
                      <span className="inline-flex items-center gap-1.5">
                        <Thermometer className="h-3.5 w-3.5" />
                        {t('controlPanel.overview.temperatureMetric')}
                        <span className="font-semibold tabular-nums text-foreground">{temperatureValue}</span>
                      </span>
                      <span className="inline-flex items-center gap-1.5">
                        <Zap className="h-3.5 w-3.5" />
                        {t('controlPanel.overview.powerMetric')}
                        <span className="font-semibold tabular-nums text-foreground">{powerValue}</span>
                      </span>
                    </div>
                  </div>
                </div>
              ))}
            </div>

            <div data-theme-card="settings-overview-device" className="flex min-h-[11rem] flex-col rounded-xl border border-border/70 bg-muted/30 p-4">
              <div className="flex min-w-0 items-start gap-3">
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-background/65 text-muted-foreground shadow-inner shadow-white/15">
                  <Radio className="h-5 w-5" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-base font-semibold text-foreground">{overviewConnectionName}</div>
                  <div className="mt-1 flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
                    <span className={clsx('h-2 w-2 shrink-0 rounded-full', isConnected ? 'bg-emerald-500' : 'bg-muted-foreground/45')} />
                    <span className="truncate">{overviewConnectionDetail}</span>
                  </div>
                </div>
              </div>

              <div className="mt-auto grid grid-cols-2 gap-4 border-t border-border/55 pt-4">
                <div className="min-w-0">
                  <div className="text-xs text-muted-foreground">{t('controlPanel.overview.currentRpm')}</div>
                  <div className="mt-1 truncate text-lg font-semibold tabular-nums text-foreground">{overviewFanSpeed ?? '--'}{overviewSpeedLabel}</div>
                </div>
                <div className="min-w-0">
                  <div className="text-xs text-muted-foreground">{t('controlPanel.overview.controlModeMetric')}</div>
                  <div className="mt-1 truncate text-sm font-semibold text-foreground">
                    {config.autoControl ? t('appShell.status.smartControl') : t('appShell.status.manualMode')}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        <div
          data-theme-ui="settings-tabs"
          role="tablist"
          aria-label={t('controlPanel.system.sectionTitle')}
          className="grid grid-cols-3 gap-1 rounded-[18px] border border-border/70 bg-card/92 p-1.5 shadow-sm shadow-black/5 backdrop-blur-xl"
        >
          {settingsTabs.map(({ id, label }) => (
            <button
              key={id}
              id={`settings-tab-${id}`}
              type="button"
              role="tab"
              aria-selected={activeSettingsTab === id}
              aria-controls={`settings-panel-${id}`}
              onClick={() => setActiveSettingsTab(id)}
              className={clsx(
                'min-w-0 rounded-xl px-3 py-2.5 text-sm font-medium transition-colors duration-150',
                activeSettingsTab === id
                  ? 'bg-background text-foreground shadow-sm ring-1 ring-border/55'
                  : 'text-muted-foreground hover:bg-muted/45 hover:text-foreground',
              )}
            >
              <span className="block truncate">{label}</span>
            </button>
          ))}
        </div>

        <div
          id="settings-panel-device"
          role="tabpanel"
          aria-labelledby="settings-tab-device"
          hidden={activeSettingsTab !== 'device'}
        >
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
          />
        </div>

        <div
          id="settings-panel-fan"
          role="tabpanel"
          aria-labelledby="settings-tab-fan"
          hidden={activeSettingsTab !== 'fan'}
        >
          <FanControlSection
            config={config}
            onConfigChange={onConfigChange}
            isConnected={isConnected}
            fanData={fanData}
            temperature={temperature}
            runtimeDeviceProfile={effectiveDeviceProfile || null}
            supportsCustomSpeed={currentDeviceSupportsCustomSpeed}
            supportsManualGears={currentDeviceSupportsManualGears}
            configuredDeviceCurveKey={configuredDeviceCurveKey}
          />
        </div>

        <div
          id="settings-panel-system"
          role="tabpanel"
          aria-labelledby="settings-tab-system"
          hidden={activeSettingsTab !== 'system'}
        >
          <SystemSettingsSection
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
