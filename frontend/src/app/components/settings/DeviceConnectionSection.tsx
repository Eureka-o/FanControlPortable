'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { ChevronDown, ChevronRight, CheckCircle2, MapPin, Pause, Play, RadioTower, RotateCw, Search, Usb, Wifi, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { toast } from 'sonner';
import { types } from '../../../../wailsjs/go/models';
import { apiService, type DeviceCandidate, type DeviceScanResult, type WiFiDiscoveredDevice } from '../../services/api';
import { Button, ToggleSwitch } from '../ui';
import { CompatibilitySubmenuRow } from './SettingLayout';
import {
  isSerialCompatibilityEnabled,
  isWiFiCompatibilityEnabled,
  isWiFiDynamicIPCompatibilityEnabled,
  profileConnection,
  profileLabel,
  activeProfileForTransport,
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

function transportLabel(transport: string | undefined, t: (key: string) => string) {
  switch (normalizeTransport(transport)) {
    case 'ble':
      return t('controlPanel.system.deviceConnection.transportBle');
    case 'hid':
      return t('controlPanel.system.deviceConnection.transportHid');
    case 'serial':
      return t('controlPanel.system.deviceConnection.transportSerial');
    default:
      return t('controlPanel.system.deviceConnection.transportWifi');
  }
}

function candidateKey(candidate: DeviceCandidate) {
  return [
    normalizeTransport(candidate.transport),
    candidate.profileId || '',
    candidate.endpoint || '',
    candidate.id || '',
  ].join(':');
}

function mergeDeviceCandidates(primary: DeviceCandidate[], secondary: DeviceCandidate[]) {
  const seen = new Set<string>();
  const merged: DeviceCandidate[] = [];
  [...primary, ...secondary].forEach((candidate) => {
    const key = candidateKey(candidate);
    if (seen.has(key)) return;
    seen.add(key);
    merged.push(candidate);
  });
  return merged;
}

function wifiCandidateFromDiscovery(device: WiFiDiscoveredDevice, profile: types.DeviceProfile | null | undefined, fallbackName: string): DeviceCandidate | null {
  const endpoint = (device.endpoint || device.ip || '').trim();
  if (!endpoint) return null;
  const profileId = (device.profileId || profile?.id || '').trim();
  return {
    id: `wifi:${profileId || 'wifi'}:${endpoint}`,
    transport: 'wifi',
    name: (device.name || profile?.displayName || profile?.model || fallbackName).trim(),
    profileId,
    endpoint,
    source: device.source || 'wifi',
    network: device.network,
    speed: device.speed,
    targetSpeed: device.targetSpeed,
    temperature: device.temperature,
    latencyMs: device.latencyMs,
    connectable: true,
  };
}

function candidateSourceLabel(source: string | undefined, t: (key: string) => string) {
  switch ((source || '').trim().toLowerCase()) {
    case 'native':
      return t('controlPanel.system.deviceConnection.candidateSourceNative');
    case 'manual':
      return t('controlPanel.system.deviceConnection.candidateSourceManual');
    case 'saved':
      return t('controlPanel.system.deviceConnection.candidateSourceSaved');
    case 'wifi':
      return t('controlPanel.system.deviceConnection.candidateSourceWiFi');
    case '':
      return '';
    default:
      return t(wifiDiscoverySourceKey(source));
  }
}

function candidateEndpointLabel(candidate: DeviceCandidate) {
  const endpoint = (candidate.endpoint || '').trim();
  if (!endpoint) return '';
  const transport = normalizeTransport(candidate.transport);
  if (transport === 'hid' || transport === 'ble') return '';
  const compact = endpoint.replace(/^https?:\/\//i, '');
  return compact.length > 42 ? `${compact.slice(0, 26)}...${compact.slice(-10)}` : compact;
}

function candidateBadges(candidate: DeviceCandidate, t: (key: string, options?: Record<string, unknown>) => string) {
  const badges = [candidateSourceLabel(candidate.source, t), transportLabel(candidate.transport, t)].filter(Boolean);
  const endpoint = candidateEndpointLabel(candidate);
  if (endpoint) badges.push(t('controlPanel.system.deviceConnection.candidateEndpoint', { endpoint }));
  if (typeof candidate.speed === 'number') {
    badges.push(t('controlPanel.system.deviceConnection.wifiScanSpeed', { speed: candidate.speed }));
  }
  if (typeof candidate.latencyMs === 'number') {
    badges.push(t('controlPanel.system.deviceConnection.wifiScanLatency', { latency: candidate.latencyMs }));
  }
  return badges;
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
  const [loadingKey, setLoadingKey] = useState('');
  const [scanResult, setScanResult] = useState<DeviceScanResult | null>(null);
  const [compatibilityOpen, setCompatibilityOpen] = useState(false);
  const [manualAddOpen, setManualAddOpen] = useState(false);
  const [wifiCompatibilityEnabled, setWiFiCompatibilityEnabled] = useState(() => isWiFiCompatibilityEnabled(config));
  const [wifiDynamicIPCompatibilityEnabled, setWiFiDynamicIPCompatibilityEnabled] = useState(() => isWiFiDynamicIPCompatibilityEnabled(config));
  const [serialCompatibilityEnabled, setSerialCompatibilityEnabled] = useState(() => isSerialCompatibilityEnabled(config));

  const wifiProfile = useMemo(
    () => activeProfileForTransport(availableDeviceProfiles, activeDeviceProfileIdsByTransport, 'wifi'),
    [activeDeviceProfileIdsByTransport, availableDeviceProfiles],
  );
  const wifiConnection = profileConnection(wifiProfile);
  const [deviceIpInput, setDeviceIpInput] = useState(() => wifiConnection.endpoint || (((config as any).fanControlDeviceIp || '') as string));
  const wifiDiscovery = useWiFiDiscovery({
    profileAvailable: wifiCompatibilityEnabled,
    resetKey: `${activeDeviceProfileId}:${wifiCompatibilityEnabled}`,
  });
  const wifiDeepScanDevices = useMemo(
    () => wifiDiscovery.devices
      .map((device) => wifiCandidateFromDiscovery(device, wifiProfile, t('controlPanel.system.deviceConnection.wifiCompatibilityTitle')))
      .filter((device): device is DeviceCandidate => device !== null),
    [t, wifiDiscovery.devices, wifiProfile],
  );
  const scanDevices = useMemo(
    () => mergeDeviceCandidates(Array.isArray(scanResult?.devices) ? scanResult.devices : [], wifiDeepScanDevices),
    [scanResult?.devices, wifiDeepScanDevices],
  );
  const isNormalScanning = loadingKey === 'scan';
  const isDeepScanning = wifiDiscovery.isScanning && wifiDiscovery.mode === 'deep';
  const isScanning = isNormalScanning || wifiDiscovery.isScanning;
  const showDeepScan = Boolean(scanResult?.showDeepScan && wifiCompatibilityEnabled && !isScanning);
  const showScanSection = Boolean(scanResult || isNormalScanning || scanDevices.length > 0);
  const hasConnectedDevice = Boolean(connectedDeviceProfile || connectedDeviceTransport);
  const currentDeviceName = connectedDeviceProfile
    ? profileLabel(connectedDeviceProfile)
    : connectedDeviceTransport
      ? transportLabel(connectedDeviceTransport, t)
      : t('controlPanel.system.deviceConnection.connectedDevicesEmpty');
  const currentDeviceDetail = connectedDeviceProfile
    ? `${summarizeConnection(connectedDeviceProfile)} · ${formatSpeedRange(connectedDeviceProfile)}`
    : connectedDeviceTransport
      ? t('controlPanel.system.deviceConnection.statusConnected')
      : t('controlPanel.system.deviceConnection.statusDisconnected');
  const wifiScanStatus = wifiDiscovery.isScanning
    ? t(wifiDiscovery.runningKey)
    : wifiDiscovery.error
      ? wifiDiscovery.error
      : wifiDiscovery.result?.canceled
        ? t('controlPanel.system.deviceConnection.toasts.wifiScanCanceled')
        : wifiDiscovery.result
          ? wifiDiscovery.devices.length > 0
            ? t('controlPanel.system.deviceConnection.toasts.wifiScanFound', { count: wifiDiscovery.devices.length })
            : t('controlPanel.system.deviceConnection.toasts.wifiScanEmpty')
          : '';

  useEffect(() => {
    setWiFiCompatibilityEnabled(isWiFiCompatibilityEnabled(config));
    setWiFiDynamicIPCompatibilityEnabled(isWiFiDynamicIPCompatibilityEnabled(config));
    setSerialCompatibilityEnabled(isSerialCompatibilityEnabled(config));
    setDeviceIpInput(wifiConnection.endpoint || (((config as any).fanControlDeviceIp || '') as string));
  }, [activeDeviceProfileId, config, wifiConnection.endpoint]);

  const refreshAfterConnection = useCallback(async () => {
    await refreshConnectedDeviceContext();
    const nextConfig = await refreshDeviceConfig();
    onConfigChange(nextConfig);
    await loadDeviceProfiles();
  }, [loadDeviceProfiles, onConfigChange, refreshConnectedDeviceContext, refreshDeviceConfig]);

  const scanDevicesNow = useCallback(async () => {
    wifiDiscovery.reset();
    setLoadingKey('scan');
    try {
      const result = await apiService.scanDeviceCandidates('normal');
      setScanResult(result);
      const devices = Array.isArray(result.devices) ? result.devices : [];
      if (result.error && devices.length === 0) {
        toast.warning(t('controlPanel.system.deviceConnection.toasts.scanPartialFailed', { error: result.error }));
      } else if (devices.length === 0) {
        toast.info(t('controlPanel.system.deviceConnection.toasts.scanEmpty'));
      } else {
        toast.success(t('controlPanel.system.deviceConnection.toasts.scanFound', { count: devices.length }));
      }
      await refreshConnectedDeviceContext();
    } catch (error) {
      toast.error(t('controlPanel.system.deviceConnection.toasts.scanFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoadingKey('');
    }
  }, [refreshConnectedDeviceContext, t, wifiDiscovery]);

  const connectCandidate = useCallback(async (candidate: DeviceCandidate) => {
    setLoadingKey(`connect:${candidate.id}`);
    try {
      const success = await apiService.connectDeviceCandidate(candidate);
      if (!success) {
        toast.error(t('controlPanel.system.deviceConnection.toasts.autoScanConnectFailed', { error: t('controlPanel.system.deviceConnection.toasts.connectRejected') }));
        return;
      }
      if (candidate.profileId) {
        onActiveDeviceProfileIdChange(candidate.profileId);
      }
      await refreshAfterConnection();
      toast.success(t('controlPanel.system.deviceConnection.toasts.autoScanConnectedDevice', { device: candidate.name || t('controlPanel.system.deviceConnection.autoNativeDevice') }));
    } catch (error) {
      toast.error(t('controlPanel.system.deviceConnection.toasts.autoScanConnectFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoadingKey('');
    }
  }, [onActiveDeviceProfileIdChange, refreshAfterConnection, t]);

  const updateCompatibilityConfig = useCallback(async (patch: Partial<types.AppConfig>, toastKey: string) => {
    const nextConfig = types.AppConfig.createFrom({ ...config, ...patch });
    await apiService.updateConfig(nextConfig);
    onConfigChange(nextConfig);
    toast.success(t(toastKey));
  }, [config, onConfigChange, t]);

  const handleWiFiCompatibilityChange = useCallback(async (enabled: boolean) => {
    setWiFiCompatibilityEnabled(enabled);
    setLoadingKey('wifiCompatibility');
    try {
      await updateCompatibilityConfig(
        { wifiCompatibilityEnabled: enabled },
        enabled
          ? 'controlPanel.system.deviceConnection.toasts.compatibilityEnabled'
          : 'controlPanel.system.deviceConnection.toasts.compatibilityDisabled',
      );
    } catch (error) {
      setWiFiCompatibilityEnabled(!enabled);
      toast.error(t('controlPanel.system.deviceConnection.toasts.transportFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoadingKey('');
    }
  }, [t, updateCompatibilityConfig]);

  const handleSerialCompatibilityChange = useCallback(async (enabled: boolean) => {
    setSerialCompatibilityEnabled(enabled);
    setLoadingKey('serialCompatibility');
    try {
      await updateCompatibilityConfig(
        { serialCompatibilityEnabled: enabled },
        enabled
          ? 'controlPanel.system.deviceConnection.toasts.compatibilityEnabled'
          : 'controlPanel.system.deviceConnection.toasts.compatibilityDisabled',
      );
    } catch (error) {
      setSerialCompatibilityEnabled(!enabled);
      toast.error(t('controlPanel.system.deviceConnection.toasts.transportFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoadingKey('');
    }
  }, [t, updateCompatibilityConfig]);

  const handleWiFiDynamicIPCompatibilityChange = useCallback(async (enabled: boolean) => {
    setWiFiDynamicIPCompatibilityEnabled(enabled);
    setLoadingKey('wifiDynamicIPCompatibility');
    try {
      await updateCompatibilityConfig(
        { wifiDynamicIpCompatibilityEnabled: enabled },
        enabled
          ? 'controlPanel.system.deviceConnection.toasts.dynamicIpEnabled'
          : 'controlPanel.system.deviceConnection.toasts.dynamicIpDisabled',
      );
    } catch (error) {
      setWiFiDynamicIPCompatibilityEnabled(!enabled);
      toast.error(t('controlPanel.system.deviceConnection.toasts.transportFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoadingKey('');
    }
  }, [t, updateCompatibilityConfig]);

  const handleManualAdd = useCallback(async () => {
    const endpoint = deviceIpInput.trim();
    if (!endpoint) {
      toast.info(t('controlPanel.system.deviceConnection.toasts.addressRequired'));
      return;
    }
    const candidate: DeviceCandidate = {
      id: `manual-wifi:${wifiProfile?.id || 'wifi'}:${endpoint}`,
      transport: 'wifi',
      name: wifiProfile ? profileLabel(wifiProfile) : t('controlPanel.system.deviceConnection.wifiCompatibilityTitle'),
      profileId: wifiProfile?.id || '',
      endpoint,
      source: 'manual',
      connectable: true,
    };
    setLoadingKey('manualAdd');
    try {
      const success = await apiService.connectDeviceCandidate(candidate);
      if (!success) {
        toast.error(t('controlPanel.system.deviceConnection.toasts.wifiFailed', { error: t('controlPanel.system.deviceConnection.toasts.connectRejected') }));
        return;
      }
      await refreshAfterConnection();
      setManualAddOpen(false);
      setCompatibilityOpen(false);
      toast.success(t('controlPanel.system.deviceConnection.toasts.wifiSaved'));
    } catch (error) {
      toast.error(t('controlPanel.system.deviceConnection.toasts.wifiFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoadingKey('');
    }
  }, [deviceIpInput, refreshAfterConnection, t, wifiProfile]);

  return (
    <>
      <div data-theme-ui="setting-row" className="px-5 py-4 transition-colors duration-200 hover:bg-muted/18">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <div data-theme-ui="setting-row-icon" className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-muted/70 text-muted-foreground shadow-inner shadow-white/20">
              <RadioTower className="h-4 w-4 text-primary" />
            </div>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <div className="text-base font-medium text-foreground">{t('controlPanel.system.deviceConnection.title')}</div>
                <span className={clsx(
                  'rounded-full px-2 py-0.5 text-[11px] font-medium',
                  hasConnectedDevice ? 'bg-emerald-500/12 text-emerald-600' : 'bg-muted text-muted-foreground',
                )}>
                  {hasConnectedDevice
                    ? t('controlPanel.system.deviceConnection.statusConnected')
                    : t('controlPanel.system.deviceConnection.statusDisconnected')}
                </span>
              </div>
              <div className="text-sm text-muted-foreground line-clamp-2">{t('controlPanel.system.deviceConnection.description')}</div>
              <div className="mt-2 inline-flex max-w-full items-center gap-2 rounded-full border border-border/70 bg-background/70 px-3 py-1.5 text-xs shadow-sm shadow-black/5">
                <span className={clsx(
                  'h-2.5 w-2.5 shrink-0 rounded-full',
                  hasConnectedDevice ? 'bg-emerald-500 shadow-[0_0_0_3px_rgba(16,185,129,0.14)]' : 'bg-muted-foreground/45',
                )} />
                <span className="min-w-0 truncate font-medium text-foreground">{currentDeviceName}</span>
                <span className="hidden min-w-0 truncate text-muted-foreground sm:inline">{currentDeviceDetail}</span>
              </div>
            </div>
          </div>

	          <div data-theme-ui="setting-row-control" className="flex w-full min-w-0 flex-col gap-2 sm:ml-auto sm:w-auto sm:shrink-0 sm:items-end">
	            <div className="flex shrink-0 flex-nowrap items-center justify-end gap-2">
	              <AnimatePresence initial={false}>
	                {showDeepScan && (
	                  <motion.div
                    initial={{ opacity: 0, x: 12, width: 0 }}
                    animate={{ opacity: 1, x: 0, width: 'auto' }}
                    exit={{ opacity: 0, x: 12, width: 0 }}
                    className="overflow-hidden"
                  >
	                    <Button
	                      variant="outline"
	                      size="sm"
	                      icon={<RotateCw className="h-4 w-4" />}
	                      onClick={() => void wifiDiscovery.scan('deep')}
	                    >
	                      {t('controlPanel.system.deviceConnection.wifiDeepScanAction')}
	                    </Button>
                  </motion.div>
	                )}
	              </AnimatePresence>
	              <Button
	                variant="secondary"
	                size="sm"
	                icon={<Search className="h-4 w-4" />}
	                onClick={() => void scanDevicesNow()}
	                loading={loadingKey === 'scan'}
	                disabled={wifiDiscovery.isScanning}
	              >
	                {t('controlPanel.system.deviceConnection.scanAvailableDevices')}
	              </Button>
	            </div>
            <AnimatePresence initial={false}>
              {isDeepScanning && (
                <motion.div
                  initial={{ opacity: 0, y: -4 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -4 }}
	                  className="w-full max-w-[18rem] rounded-xl border border-border/60 bg-background/50 px-3 py-2 shadow-sm shadow-black/5 sm:w-[18rem]"
                >
                  <div className="flex items-center justify-between gap-3">
                    <div className="min-w-0">
                      <div className="truncate text-xs font-medium text-foreground">{wifiScanStatus}</div>
                      <div className="mt-0.5 text-[11px] text-muted-foreground">{wifiDiscovery.elapsedText}</div>
                    </div>
                    <div className="flex shrink-0 items-center gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        icon={wifiDiscovery.paused ? <Play className="h-4 w-4" /> : <Pause className="h-4 w-4" />}
                        onClick={() => void wifiDiscovery.pauseToggle()}
                        disabled={wifiDiscovery.canceling}
                        className="h-8 px-2"
                      >
                        {wifiDiscovery.paused
                          ? t('controlPanel.system.deviceConnection.wifiScanResumeAction')
                          : t('controlPanel.system.deviceConnection.wifiScanPauseAction')}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        icon={<X className="h-4 w-4" />}
                        onClick={() => void wifiDiscovery.cancel()}
                        disabled={wifiDiscovery.canceling}
                        className="h-8 px-2"
                      >
                        {t('controlPanel.system.deviceConnection.wifiScanCancelAction')}
                      </Button>
                    </div>
                  </div>
                  <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted/45">
                    <div
                      className="h-full rounded-full bg-primary transition-[width] duration-300"
                      style={{ width: `${wifiDiscovery.progressPercent}%` }}
                    />
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        </div>

        {showScanSection && (
          <div className="mt-4 border-t border-border/45 pt-3">
		            <div className="mb-2">
		              <div className="text-sm font-medium text-foreground">{t('controlPanel.system.deviceConnection.discoveredDevicesTitle')}</div>
		              {scanResult && <div className="mt-0.5 text-xs text-muted-foreground">{t('controlPanel.system.deviceConnection.discoveredDevicesDescription')}</div>}
		            </div>
		            {scanDevices.length > 0 ? (
		              <div className="space-y-2">
		                {scanDevices.map((candidate) => (
                  <div
                    key={candidate.id}
                    className="flex min-w-0 flex-col gap-2 rounded-xl border border-border/60 bg-background/35 px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between"
                  >
	                    <div className="min-w-0">
	                      <div className="truncate text-sm font-medium text-foreground">{candidate.name}</div>
		                      <div className="mt-1 flex flex-wrap gap-1.5">
		                        {candidateBadges(candidate, t).map((badge) => (
		                          <span
		                            key={badge}
		                            className="inline-flex max-w-[16rem] items-center truncate rounded-full border border-border/70 bg-background/70 px-2.5 py-0.5 text-[11px] font-medium leading-4 text-muted-foreground shadow-sm shadow-black/5"
		                            title={badge}
		                          >
		                            {badge}
	                          </span>
	                        ))}
	                      </div>
	                    </div>
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => void connectCandidate(candidate)}
                      loading={loadingKey === `connect:${candidate.id}`}
                      disabled={candidate.connectable === false}
                      className="shrink-0"
                    >
                      {candidate.connectable === false
                        ? t('controlPanel.system.deviceConnection.deviceStatusUnavailable')
                        : t('controlPanel.system.deviceConnection.autoScanConnectAction')}
                    </Button>
                  </div>
                ))}
              </div>
	            ) : (
	              <div className="rounded-xl border border-dashed border-border/70 bg-muted/15 px-3 py-2 text-xs text-muted-foreground">
	                {isNormalScanning
	                  ? t('controlPanel.system.deviceConnection.scanRunning')
	                  : t('controlPanel.system.deviceConnection.discoveredDevicesEmpty')}
	              </div>
            )}
          </div>
        )}
      </div>

      <div data-theme-ui="setting-row" className="px-5 py-4 transition-colors duration-200 hover:bg-muted/18">
        <button
          type="button"
          onClick={() => setCompatibilityOpen((value) => !value)}
          className="flex w-full cursor-pointer flex-col gap-4 text-left sm:flex-row sm:items-center sm:justify-between"
        >
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <div data-theme-ui="setting-row-icon" className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-muted/70 text-muted-foreground shadow-inner shadow-white/20">
              <Wifi className={clsx('h-4 w-4', (wifiCompatibilityEnabled || serialCompatibilityEnabled) ? 'text-emerald-500' : 'text-primary')} />
            </div>
            <div className="min-w-0">
              <div className="text-base font-medium text-foreground">{t('controlPanel.system.deviceConnection.compatibilityTitle')}</div>
              <div className="text-sm text-muted-foreground line-clamp-2">{t('controlPanel.system.deviceConnection.compatibilityDescription')}</div>
            </div>
          </div>
          <ChevronDown className={clsx('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', compatibilityOpen && 'rotate-180')} />
        </button>

        <AnimatePresence initial={false}>
          {compatibilityOpen && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              className="mt-3 overflow-hidden border-t border-border/50"
            >
              <div className="divide-y divide-border/45">
                <CompatibilitySubmenuRow
                  icon={<Wifi className={clsx('h-4 w-4', wifiCompatibilityEnabled ? 'text-emerald-500' : 'text-muted-foreground')} />}
                  title={t('controlPanel.system.deviceConnection.wifiCompatibilityTitle')}
                  description={t('controlPanel.system.deviceConnection.wifiCompatibilityDescription')}
                >
                  <ToggleSwitch
                    enabled={wifiCompatibilityEnabled}
                    onChange={(enabled) => void handleWiFiCompatibilityChange(enabled)}
                    loading={loadingKey === 'wifiCompatibility'}
                    size="sm"
                    color="green"
                  />
                </CompatibilitySubmenuRow>

                <AnimatePresence initial={false}>
                  {wifiCompatibilityEnabled && (
                    <motion.div
                      initial={{ opacity: 0, height: 0 }}
                      animate={{ opacity: 1, height: 'auto' }}
                      exit={{ opacity: 0, height: 0 }}
                      className="overflow-hidden"
                    >
                      <CompatibilitySubmenuRow
                        icon={<MapPin className="h-4 w-4 text-primary" />}
                        title={t('controlPanel.system.deviceConnection.wifiManualAddTitle')}
                        description={t('controlPanel.system.deviceConnection.wifiManualAddHint')}
                      >
                        <Button
                          variant="outline"
                          size="sm"
                          icon={<ChevronRight className={clsx('h-4 w-4 transition-transform', manualAddOpen && 'rotate-90')} />}
                          onClick={() => setManualAddOpen((value) => !value)}
                        >
                          {t('controlPanel.system.deviceConnection.wifiManualAddAction')}
                        </Button>
                      </CompatibilitySubmenuRow>

                      <AnimatePresence initial={false}>
                        {manualAddOpen && (
                          <motion.div
                            initial={{ opacity: 0, height: 0 }}
                            animate={{ opacity: 1, height: 'auto' }}
                            exit={{ opacity: 0, height: 0 }}
                            className="overflow-hidden"
                          >
                            <CompatibilitySubmenuRow
                              icon={<CheckCircle2 className="h-4 w-4 text-primary" />}
                              title={t('controlPanel.system.deviceConnection.addressLabel')}
                            >
                              <div className="flex w-full flex-col gap-2 sm:flex-row sm:items-center md:w-[560px]">
                                <input
                                  value={deviceIpInput}
                                  onChange={(event) => setDeviceIpInput(event.target.value)}
                                  onKeyDown={(event) => {
                                    if (event.key === 'Enter') void handleManualAdd();
                                  }}
                                  placeholder={t('controlPanel.system.deviceConnection.addressPlaceholder')}
                                  aria-label={t('controlPanel.system.deviceConnection.addressPlaceholder')}
                                  className="h-10 min-w-0 flex-1 rounded-md border border-input bg-background px-3 text-sm text-foreground outline-none ring-offset-background transition-colors focus-visible:ring-2 focus-visible:ring-ring"
                                />
                                <Button
                                  variant="primary"
                                  size="sm"
                                  onClick={() => void handleManualAdd()}
                                  loading={loadingKey === 'manualAdd'}
                                  className="shrink-0"
                                >
                                  {t('controlPanel.system.deviceConnection.wifiManualSaveAction')}
                                </Button>
                              </div>
                            </CompatibilitySubmenuRow>
                          </motion.div>
                        )}
                      </AnimatePresence>

                      <CompatibilitySubmenuRow
                        icon={<RotateCw className="h-4 w-4 text-primary" />}
                        title={t('controlPanel.system.deviceConnection.wifiDynamicIPTitle')}
                        description={t('controlPanel.system.deviceConnection.wifiDynamicIPDescription')}
                      >
                        <ToggleSwitch
                          enabled={wifiDynamicIPCompatibilityEnabled}
                          onChange={(enabled) => void handleWiFiDynamicIPCompatibilityChange(enabled)}
                          loading={loadingKey === 'wifiDynamicIPCompatibility'}
                          size="sm"
                          color="green"
                        />
                      </CompatibilitySubmenuRow>
                    </motion.div>
                  )}
                </AnimatePresence>

                <CompatibilitySubmenuRow
                  icon={<Usb className={clsx('h-4 w-4', serialCompatibilityEnabled ? 'text-emerald-500' : 'text-muted-foreground')} />}
                  title={t('controlPanel.system.deviceConnection.serialCompatibilityTitle')}
                  description={t('controlPanel.system.deviceConnection.serialCompatibilityDescription')}
                >
                  <ToggleSwitch
                    enabled={serialCompatibilityEnabled}
                    onChange={(enabled) => void handleSerialCompatibilityChange(enabled)}
                    loading={loadingKey === 'serialCompatibility'}
                    size="sm"
                    color="green"
                  />
                </CompatibilitySubmenuRow>
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </>
  );
}
