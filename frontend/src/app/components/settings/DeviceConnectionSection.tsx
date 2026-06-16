'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { CheckCircle2, HardDrive, MapPin, Pause, Play, Plug, RadioTower, RotateCw, Search, Usb, Wifi, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { toast } from 'sonner';
import { types } from '../../../../wailsjs/go/models';
import { apiService } from '../../services/api';
import { Button, Select, ToggleSwitch } from '../ui';
import {
  CompatibilitySubmenu,
  CompatibilitySubmenuRow,
  ConnectionPanel,
  DeviceProfileInline,
  EmptyConnectionState,
  InlineHint,
  SettingRow,
} from './SettingLayout';
import {
  EMPTY_PROFILE_SELECT_VALUE,
  activeProfileForTransport,
  configuredDeviceProfiles,
  isEmptyProfileSelectValue,
  isSerialCompatibilityEnabled,
  isWiFiCompatibilityEnabled,
  isWiFiDynamicIPCompatibilityEnabled,
  profileConnection,
  profileLabel,
  profileSelectOptions,
  profilesForTransport,
  wifiDiscoveryElapsedLabel,
  wifiDiscoverySourceKey,
} from './device-connection-utils';
import { formatSpeedRange, normalizeTransport, summarizeConnection } from '../devices/profile-utils';
import { useWiFiDiscovery } from './useWiFiDiscovery';

interface DeviceConnectionSectionProps {
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

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function firstNonEmptyDeviceName(...values: Array<string | undefined | null>) {
  return values.map((value) => (value || '').trim()).find(Boolean) || '';
}

export default function DeviceConnectionSection({
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
}: DeviceConnectionSectionProps) {
  const { t } = useTranslation();
  const [loadingStates, setLoadingStates] = useState<Record<string, boolean>>({});
  const [wifiCompatibilityEnabled, setWiFiCompatibilityEnabled] = useState<boolean>(() => isWiFiCompatibilityEnabled(config));
  const [wifiDynamicIPCompatibilityEnabled, setWiFiDynamicIPCompatibilityEnabled] = useState<boolean>(() => isWiFiDynamicIPCompatibilityEnabled(config));
  const [serialCompatibilityEnabled, setSerialCompatibilityEnabled] = useState<boolean>(() => isSerialCompatibilityEnabled(config));

  const wifiProfiles = useMemo(
    () => profilesForTransport(availableDeviceProfiles, 'wifi'),
    [availableDeviceProfiles],
  );
  const serialProfiles = useMemo(
    () => profilesForTransport(availableDeviceProfiles, 'serial'),
    [availableDeviceProfiles],
  );
  const wifiProfile = useMemo(
    () => activeProfileForTransport(availableDeviceProfiles, activeDeviceProfileIdsByTransport, 'wifi'),
    [activeDeviceProfileIdsByTransport, availableDeviceProfiles],
  );
  const serialProfile = useMemo(
    () => activeProfileForTransport(availableDeviceProfiles, activeDeviceProfileIdsByTransport, 'serial'),
    [activeDeviceProfileIdsByTransport, availableDeviceProfiles],
  );
  const wifiConnection = profileConnection(wifiProfile);
  const [deviceIpInput, setDeviceIpInput] = useState<string>(
    () => wifiConnection.endpoint || (((config as any).fanControlDeviceIp || '') as string),
  );

  const wifiDiscovery = useWiFiDiscovery({
    profileAvailable: !!wifiProfile,
    resetKey: activeDeviceProfileId,
  });

  const wifiProfileOptions = useMemo(
    () => profileSelectOptions(wifiProfiles, t('controlPanel.system.deviceConnection.noEnabledDevice')),
    [t, wifiProfiles],
  );
  const serialProfileOptions = useMemo(
    () => profileSelectOptions(serialProfiles, t('controlPanel.system.deviceConnection.noEnabledDevice')),
    [serialProfiles, t],
  );
  const wifiConnectedProfile = connectedDeviceTransport === 'wifi' ? connectedDeviceProfile : null;
  const serialConnectedProfile = connectedDeviceTransport === 'serial' ? connectedDeviceProfile : null;

  const setLoading = (key: string, value: boolean) => setLoadingStates((prev) => ({ ...prev, [key]: value }));

  useEffect(() => {
    setWiFiCompatibilityEnabled(isWiFiCompatibilityEnabled(config));
    setWiFiDynamicIPCompatibilityEnabled(isWiFiDynamicIPCompatibilityEnabled(config));
    setSerialCompatibilityEnabled(isSerialCompatibilityEnabled(config));
    setDeviceIpInput(wifiConnection.endpoint || (((config as any).fanControlDeviceIp || '') as string));
  }, [config, wifiConnection.endpoint]);

  const handleCompatibilityModeChange = useCallback(async (transport: 'wifi' | 'serial', enabled: boolean) => {
    if (transport === 'wifi') {
      setWiFiCompatibilityEnabled(enabled);
    } else {
      setSerialCompatibilityEnabled(enabled);
    }
    setLoading(`${transport}Compatibility`, true);
    try {
      const newCfg = types.AppConfig.createFrom({
        ...config,
        wifiCompatibilityEnabled: transport === 'wifi' ? enabled : wifiCompatibilityEnabled,
        serialCompatibilityEnabled: transport === 'serial' ? enabled : serialCompatibilityEnabled,
      });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
      toast.success(t(enabled
        ? 'controlPanel.system.deviceConnection.toasts.compatibilityEnabled'
        : 'controlPanel.system.deviceConnection.toasts.compatibilityDisabled'));
    } catch (error) {
      if (transport === 'wifi') {
        setWiFiCompatibilityEnabled(!enabled);
      } else {
        setSerialCompatibilityEnabled(!enabled);
      }
      toast.error(t('controlPanel.system.deviceConnection.toasts.transportFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoading(`${transport}Compatibility`, false);
    }
  }, [config, onConfigChange, serialCompatibilityEnabled, t, wifiCompatibilityEnabled]);

  const handleCompatibilityDeviceSelect = useCallback(async (transport: 'wifi' | 'serial', value: string | number) => {
    if (isEmptyProfileSelectValue(value)) {
      toast.info(t('controlPanel.system.deviceConnection.toasts.profileRequired'));
      return;
    }
    const profileID = String(value || '').trim();
    const profile = availableDeviceProfiles.find(
      (item) => item.id === profileID && normalizeTransport(item.transport) === transport,
    );
    if (!profile) {
      toast.info(t('controlPanel.system.deviceConnection.toasts.profileRequired'));
      return;
    }

    setLoading(`${transport}Profile`, true);
    try {
      const selected = await apiService.setActiveDeviceProfile(profile.id);
      onActiveDeviceProfileIdChange(selected.id);
      if (transport === 'wifi') {
        setWiFiCompatibilityEnabled(true);
      } else {
        setSerialCompatibilityEnabled(true);
      }
      await refreshDeviceConfig();
      await loadDeviceProfiles();
      toast.success(t('controlPanel.system.deviceConnection.toasts.profileSaved'));
    } catch (error) {
      toast.error(t('controlPanel.system.deviceConnection.toasts.profileFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoading(`${transport}Profile`, false);
    }
  }, [availableDeviceProfiles, loadDeviceProfiles, onActiveDeviceProfileIdChange, refreshDeviceConfig, t]);

  const handleWiFiConnectionSave = useCallback(async (endpointOverride?: string) => {
    setLoading('deviceIp', true);
    try {
      const nextIp = (endpointOverride ?? deviceIpInput).trim();
      if (!wifiProfile) {
        toast.info(t('controlPanel.system.deviceConnection.toasts.profileRequired'));
        return;
      }
      const profileConn = profileConnection(wifiProfile);
      const stateEndpoint = profileConn.stateEndpoint || '/api/data';
      const speedEndpoint = profileConn.speedEndpoint || '/api/speed';
      const nextProfiles = wifiProfile
        ? availableDeviceProfiles.map((profile) => {
          if (profile.id !== wifiProfile.id) {
            return profile;
          }
          return types.DeviceProfile.createFrom({
            ...profile,
            connection: types.DeviceConnectionSettings.createFrom({
              ...(profile.connection || {}),
              endpoint: nextIp,
              stateEndpoint,
              speedEndpoint,
            }),
          });
        })
        : configuredDeviceProfiles(config);
      const newCfg = types.AppConfig.createFrom({
        ...config,
        activeDeviceProfileId: wifiProfile?.id || (config as any).activeDeviceProfileId,
        activeDeviceProfileIdsByTransport: {
          ...activeDeviceProfileIdsByTransport,
          wifi: wifiProfile.id,
        },
        deviceProfiles: nextProfiles,
        deviceTransport: 'wifi',
        fanControlDeviceIp: nextIp,
        wifiCompatibilityEnabled: true,
      });
      setDeviceIpInput(nextIp);
      setWiFiCompatibilityEnabled(true);
      await apiService.updateConfig(newCfg);
      await refreshDeviceConfig();
      await loadDeviceProfiles();
      toast.success(t('controlPanel.system.deviceConnection.toasts.wifiSaved'));
    } catch (error) {
      toast.error(t('controlPanel.system.deviceConnection.toasts.wifiFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoading('deviceIp', false);
    }
  }, [
    activeDeviceProfileIdsByTransport,
    availableDeviceProfiles,
    config,
    deviceIpInput,
    loadDeviceProfiles,
    refreshDeviceConfig,
    t,
    wifiProfile,
  ]);

  const handleWiFiDynamicIPCompatibilityChange = useCallback(async (enabled: boolean) => {
    setWiFiDynamicIPCompatibilityEnabled(enabled);
    setLoading('wifiDynamicIPCompatibility', true);
    try {
      const newCfg = types.AppConfig.createFrom({
        ...config,
        wifiDynamicIpCompatibilityEnabled: enabled,
      });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
      toast.success(t(enabled
        ? 'controlPanel.system.deviceConnection.toasts.dynamicIpEnabled'
        : 'controlPanel.system.deviceConnection.toasts.dynamicIpDisabled'));
    } catch (error) {
      setWiFiDynamicIPCompatibilityEnabled(!enabled);
      toast.error(t('controlPanel.system.deviceConnection.toasts.transportFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoading('wifiDynamicIPCompatibility', false);
    }
  }, [config, onConfigChange, t]);

  const handleNativeAutoConnect = useCallback(async () => {
    setLoading('nativeAutoConnect', true);
    try {
      const connected = await apiService.connectNativeDevice();
      await refreshConnectedDeviceContext();
      if (connected) {
        const status = await apiService.getDeviceStatus().catch(() => null) as {
          deviceName?: string;
          deviceProfile?: types.DeviceProfile | null;
          model?: string;
          deviceSettings?: { model?: string } | null;
        } | null;
        const profile = status?.deviceProfile || null;
        const deviceName = firstNonEmptyDeviceName(
          status?.deviceName,
          profile ? profileLabel(profile) : '',
          status?.model,
          status?.deviceSettings?.model,
          connectedDeviceProfile ? profileLabel(connectedDeviceProfile) : '',
          t('controlPanel.system.deviceConnection.autoNativeDevice'),
        );
        toast.success(t('controlPanel.system.deviceConnection.toasts.autoScanConnectedDevice', {
          device: deviceName,
        }));
      } else {
        toast.error(t('controlPanel.system.deviceConnection.toasts.autoScanConnectFailed', { error: t('controlPanel.system.deviceConnection.toasts.autoScanEmpty') }));
      }
    } catch (error) {
      toast.error(t('controlPanel.system.deviceConnection.toasts.autoScanConnectFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoading('nativeAutoConnect', false);
    }
  }, [connectedDeviceProfile, refreshConnectedDeviceContext, t]);

  return (
    <>
      <SettingRow
        icon={<RadioTower className="h-4 w-4 text-primary" />}
        title={t('controlPanel.system.deviceConnection.title')}
        description={t('controlPanel.system.deviceConnection.description')}
      >
        <div className="w-full space-y-1.5 sm:w-52">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void handleNativeAutoConnect()}
            loading={loadingStates.nativeAutoConnect}
            icon={<RotateCw className="h-4 w-4" />}
            className="w-full justify-center"
          >
            {t('controlPanel.system.deviceConnection.scanAvailableDevices')}
          </Button>
          <p className="text-[11px] leading-relaxed text-muted-foreground">
            {t('controlPanel.system.deviceConnection.bluetoothScanOnlyHint')}
          </p>
        </div>
      </SettingRow>

      <SettingRow
        icon={<Wifi className={clsx('h-4 w-4', wifiCompatibilityEnabled ? 'text-emerald-500' : 'text-muted-foreground')} />}
        title={t('controlPanel.system.deviceConnection.wifiCompatibilityTitle')}
        description={t('controlPanel.system.deviceConnection.wifiCompatibilityDescription')}
      >
        <ToggleSwitch
          enabled={wifiCompatibilityEnabled}
          onChange={(enabled) => void handleCompatibilityModeChange('wifi', enabled)}
          loading={loadingStates.wifiCompatibility}
          size="sm"
          color="green"
        />
      </SettingRow>

      <AnimatePresence>
        {wifiCompatibilityEnabled && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            className="overflow-hidden"
          >
            <CompatibilitySubmenu>
              <CompatibilitySubmenuRow
                icon={<CheckCircle2 className="h-4 w-4 text-emerald-500" />}
                title={t('controlPanel.system.deviceConnection.connectedDevicesTitle')}
                description={t('controlPanel.system.deviceConnection.connectedDevicesDescription')}
              >
                <DeviceProfileInline
                  profile={wifiConnectedProfile}
                  empty={t('controlPanel.system.deviceConnection.connectedDevicesEmpty')}
                />
              </CompatibilitySubmenuRow>

              <CompatibilitySubmenuRow
                icon={<HardDrive className="h-4 w-4 text-primary" />}
                title={t('controlPanel.system.deviceConnection.pairedDevicesTitle')}
                description={t('controlPanel.system.deviceConnection.pairedDevicesDescription')}
              >
                <div className="w-full space-y-2 md:w-[360px]">
                  <Select
                    value={wifiProfile?.id || EMPTY_PROFILE_SELECT_VALUE}
                    onChange={(value) => handleCompatibilityDeviceSelect('wifi', value)}
                    options={wifiProfileOptions}
                    disabled={wifiProfiles.length === 0 || loadingStates.wifiProfile}
                    size="sm"
                    className="w-full min-w-0"
                  />
                  {wifiProfile ? (
                    <div className="text-xs leading-relaxed text-muted-foreground md:text-right">
                      {`${summarizeConnection(wifiProfile)} 路 ${formatSpeedRange(wifiProfile)}`}
                    </div>
                  ) : (
                    <div className="text-xs leading-relaxed text-muted-foreground md:text-right">
                      {t('controlPanel.system.deviceConnection.pairedDevicesEmpty')}
                    </div>
                  )}
                </div>
              </CompatibilitySubmenuRow>

              <CompatibilitySubmenuRow
                icon={<Search className="h-4 w-4 text-primary" />}
                title={t('controlPanel.system.deviceConnection.wifiScanTitle')}
                description={t('controlPanel.system.deviceConnection.wifiScanDescription')}
                tip={wifiDiscovery.isScanning
                  ? t('controlPanel.system.deviceConnection.wifiScanElapsed', { elapsed: wifiDiscovery.elapsedText })
                  : wifiDiscovery.result
                  ? t('controlPanel.system.deviceConnection.wifiScanSummary', {
                    scanned: wifiDiscovery.result.scannedCount || wifiDiscovery.result.candidateCount || 0,
                    count: wifiDiscovery.result.candidateCount || 0,
                    elapsed: wifiDiscoveryElapsedLabel(wifiDiscovery.result.elapsedMs) || '-',
                  })
                  : undefined}
                below={
                  <div className="space-y-2">
                    {wifiDiscovery.isScanning && (
                      <div className="overflow-hidden rounded-md border border-primary/25 bg-primary/5 px-3 py-2">
                        <div className="flex min-w-0 items-center justify-between gap-3 text-xs text-primary">
                          <span className="flex min-w-0 items-center gap-2">
                            <RotateCw className={clsx('h-3.5 w-3.5 shrink-0', !wifiDiscovery.paused && 'animate-spin')} />
                            <span className="truncate">
                              {t(wifiDiscovery.runningKey)}
                            </span>
                          </span>
                          <span className="shrink-0 font-medium">
                            {t('controlPanel.system.deviceConnection.wifiScanElapsed', { elapsed: wifiDiscovery.elapsedText })}
                          </span>
                        </div>
                        <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-primary/15">
                          <motion.div
                            className="h-full rounded-full bg-primary/70"
                            initial={false}
                            animate={{ width: `${wifiDiscovery.progressPercent}%` }}
                            transition={{ duration: 0.25, ease: 'easeOut' }}
                          />
                        </div>
                        <div className="mt-1.5 text-[11px] leading-relaxed text-muted-foreground">
                          {t('controlPanel.system.deviceConnection.wifiScanProgressLabel', { percent: wifiDiscovery.progressPercent })}
                        </div>
                        {wifiDiscovery.mode === 'deep' && (
                          <div className="mt-2 flex flex-wrap justify-end gap-2">
                            <Button
                              variant="secondary"
                              size="sm"
                              icon={wifiDiscovery.paused ? <Play className="h-3.5 w-3.5" /> : <Pause className="h-3.5 w-3.5" />}
                              onClick={() => void wifiDiscovery.pauseToggle()}
                              disabled={wifiDiscovery.canceling}
                            >
                              {t(wifiDiscovery.paused
                                ? 'controlPanel.system.deviceConnection.wifiScanResumeAction'
                                : 'controlPanel.system.deviceConnection.wifiScanPauseAction')}
                            </Button>
                            <Button
                              variant="danger"
                              size="sm"
                              icon={<X className="h-3.5 w-3.5" />}
                              onClick={() => void wifiDiscovery.cancel()}
                              loading={wifiDiscovery.canceling}
                            >
                              {t('controlPanel.system.deviceConnection.wifiScanCancelAction')}
                            </Button>
                          </div>
                        )}
                      </div>
                    )}
                    {wifiDiscovery.error && (
                      <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                        {t('controlPanel.system.deviceConnection.wifiScanFailed', { error: wifiDiscovery.error })}
                      </div>
                    )}
                    {!wifiDiscovery.error && wifiDiscovery.devices.length > 0 && (
                      <div className="space-y-2">
                        <div className="text-xs font-medium text-muted-foreground">
                          {t('controlPanel.system.deviceConnection.wifiScanResultsTitle')}
                        </div>
                        {wifiDiscovery.devices.map((device) => (
                          <div
                            key={`${device.endpoint}-${device.source || ''}`}
                            className="flex min-w-0 flex-col gap-2 rounded-md border border-border bg-card/70 px-3 py-2 sm:flex-row sm:items-center sm:justify-between"
                          >
                            <div className="min-w-0">
                              <div className="truncate text-sm font-medium text-foreground">
                                {device.name || (wifiProfile ? profileLabel(wifiProfile) : t('controlPanel.system.deviceConnection.autoNativeDevice'))}
                              </div>
                              <div className="mt-0.5 truncate text-xs text-muted-foreground">
                                {device.endpoint}
                              </div>
                              <div className="mt-1 flex flex-wrap gap-1.5">
                                <span className="rounded-md bg-primary/10 px-2 py-0.5 text-[11px] font-medium text-primary">
                                  {t(wifiDiscoverySourceKey(device.source))}
                                </span>
                                {typeof device.speed === 'number' && (
                                  <span className="rounded-md bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
                                    {t('controlPanel.system.deviceConnection.wifiScanSpeed', { speed: device.speed })}
                                  </span>
                                )}
                                {typeof device.latencyMs === 'number' && (
                                  <span className="rounded-md bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
                                    {t('controlPanel.system.deviceConnection.wifiScanLatency', { latency: device.latencyMs })}
                                  </span>
                                )}
                              </div>
                            </div>
                            <Button
                              variant="secondary"
                              size="sm"
                              onClick={() => void handleWiFiConnectionSave(device.endpoint)}
                              loading={loadingStates.deviceIp}
                              className="shrink-0"
                            >
                              {t('controlPanel.system.deviceConnection.wifiScanUseAction')}
                            </Button>
                          </div>
                        ))}
                      </div>
                    )}
                    {!wifiDiscovery.isScanning && !wifiDiscovery.error && wifiDiscovery.normalScanAttempted && wifiDiscovery.devices.length === 0 && (
                      <EmptyConnectionState
                        action={wifiDiscovery.showDeepScanAction ? (
                          <Button
                            variant="secondary"
                            size="sm"
                            icon={<RotateCw className="h-4 w-4" />}
                            onClick={() => void wifiDiscovery.scan('deep')}
                            disabled={!wifiProfile}
                          >
                            {t('controlPanel.system.deviceConnection.wifiDeepScanAction')}
                          </Button>
                        ) : undefined}
                      >
                        {t('controlPanel.system.deviceConnection.wifiScanResultsEmpty')}
                      </EmptyConnectionState>
                    )}
                  </div>
                }
              >
                <div className="flex w-full justify-start md:w-auto md:justify-end">
                  <Button
                    variant="secondary"
                    size="sm"
                    icon={<Search className="h-4 w-4" />}
                    onClick={() => void wifiDiscovery.scan('normal')}
                    loading={wifiDiscovery.isScanning && wifiDiscovery.mode === 'normal'}
                    disabled={!wifiProfile || wifiDiscovery.isScanning}
                  >
                    {t('controlPanel.system.deviceConnection.wifiScanAction')}
                  </Button>
                </div>
              </CompatibilitySubmenuRow>

              <CompatibilitySubmenuRow
                icon={<MapPin className="h-4 w-4 text-primary" />}
                title={t('controlPanel.system.deviceConnection.wifiManualAddTitle')}
                description={t('controlPanel.system.deviceConnection.wifiManualAddHint')}
                below={
                  <div className="flex w-full flex-col gap-2 md:flex-row md:items-center">
                    <input
                      value={deviceIpInput}
                      onChange={(event) => setDeviceIpInput(event.target.value)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter') void handleWiFiConnectionSave();
                      }}
                      placeholder={t('controlPanel.system.deviceConnection.addressPlaceholder')}
                      aria-label={t('controlPanel.system.deviceConnection.addressPlaceholder')}
                      className="h-10 min-w-0 flex-1 rounded-md border border-input bg-background px-3 text-sm text-foreground outline-none ring-offset-background transition-colors focus-visible:ring-2 focus-visible:ring-ring"
                    />
                    <Button
                      variant="primary"
                      size="sm"
                      onClick={() => void handleWiFiConnectionSave()}
                      loading={loadingStates.deviceIp}
                      disabled={!wifiProfile}
                      className="shrink-0"
                    >
                      {t('controlPanel.system.deviceConnection.wifiManualAddAction')}
                    </Button>
                  </div>
                }
              />

              <CompatibilitySubmenuRow
                icon={<RotateCw className="h-4 w-4 text-primary" />}
                title={t('controlPanel.system.deviceConnection.wifiDynamicIPTitle')}
                description={t('controlPanel.system.deviceConnection.wifiDynamicIPDescription')}
                tip={t('controlPanel.system.deviceConnection.wifiDynamicIPTip')}
              >
                <ToggleSwitch
                  enabled={wifiDynamicIPCompatibilityEnabled}
                  onChange={(enabled) => void handleWiFiDynamicIPCompatibilityChange(enabled)}
                  loading={loadingStates.wifiDynamicIPCompatibility}
                  size="sm"
                  color="green"
                />
              </CompatibilitySubmenuRow>
            </CompatibilitySubmenu>
          </motion.div>
        )}
      </AnimatePresence>

      <SettingRow
        icon={<Usb className={clsx('h-4 w-4', serialCompatibilityEnabled ? 'text-emerald-500' : 'text-muted-foreground')} />}
        title={t('controlPanel.system.deviceConnection.serialCompatibilityTitle')}
        description={t('controlPanel.system.deviceConnection.serialCompatibilityDescription')}
      >
        <ToggleSwitch
          enabled={serialCompatibilityEnabled}
          onChange={(enabled) => void handleCompatibilityModeChange('serial', enabled)}
          loading={loadingStates.serialCompatibility}
          size="sm"
          color="green"
        />
      </SettingRow>

      <AnimatePresence>
        {serialCompatibilityEnabled && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            className="overflow-hidden"
          >
            <CompatibilitySubmenu>
              <CompatibilitySubmenuRow
                icon={<CheckCircle2 className="h-4 w-4 text-emerald-500" />}
                title={t('controlPanel.system.deviceConnection.connectedDevicesTitle')}
                description={t('controlPanel.system.deviceConnection.connectedDevicesDescription')}
              >
                <DeviceProfileInline
                  profile={serialConnectedProfile}
                  empty={t('controlPanel.system.deviceConnection.connectedDevicesEmpty')}
                />
              </CompatibilitySubmenuRow>

              <CompatibilitySubmenuRow
                icon={<HardDrive className="h-4 w-4 text-primary" />}
                title={t('controlPanel.system.deviceConnection.pairedDevicesTitle')}
                description={t('controlPanel.system.deviceConnection.pairedDevicesDescription')}
              >
                <div className="w-full space-y-2 md:w-[360px]">
                  <Select
                    value={serialProfile?.id || EMPTY_PROFILE_SELECT_VALUE}
                    onChange={(value) => handleCompatibilityDeviceSelect('serial', value)}
                    options={serialProfileOptions}
                    disabled={serialProfiles.length === 0 || loadingStates.serialProfile}
                    size="sm"
                    className="w-full min-w-0"
                  />
                  {serialProfile ? (
                    <div className="text-xs leading-relaxed text-muted-foreground md:text-right">
                      {`${summarizeConnection(serialProfile)} 路 ${formatSpeedRange(serialProfile)}`}
                    </div>
                  ) : (
                    <div className="text-xs leading-relaxed text-muted-foreground md:text-right">
                      {t('controlPanel.system.deviceConnection.pairedDevicesEmpty')}
                    </div>
                  )}
                </div>
              </CompatibilitySubmenuRow>

              <CompatibilitySubmenuRow
                icon={<Plug className="h-4 w-4 text-primary" />}
                title={t('controlPanel.system.deviceConnection.serialInterfaceTitle')}
                description={t('controlPanel.system.deviceConnection.serialInterfaceDescription')}
                tip={t('controlPanel.system.deviceConnection.serialInterfaceManagedInDevices')}
              >
                {serialProfile ? (
                  <DeviceProfileInline
                    profile={serialProfile}
                    empty={t('controlPanel.system.deviceConnection.pairedDevicesEmpty')}
                  />
                ) : (
                  <InlineHint>
                    {t('controlPanel.system.deviceConnection.pairedDevicesEmpty')}
                  </InlineHint>
                )}
              </CompatibilitySubmenuRow>
            </CompatibilitySubmenu>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}
