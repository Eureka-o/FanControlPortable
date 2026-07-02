'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { types } from '../../../../../wailsjs/go/models';
import { apiService, type DeviceCandidate, type DeviceScanResult } from '../../../services/api';
import {
  isSerialCompatibilityEnabled,
  isWiFiCompatibilityEnabled,
  isWiFiDynamicIPCompatibilityEnabled,
  profileLabel,
  activeProfileForTransport,
} from '../device-connection-utils';
import { useWiFiDiscovery } from '../useWiFiDiscovery';
import { DeviceCompatibilityPanel } from './DeviceCompatibilityPanel';
import { DeviceConnectionScanPanel } from './DeviceConnectionScanPanel';
import {
  currentDeviceSummary,
  getErrorMessage,
  mergeDeviceCandidates,
  profileEndpoint,
  wifiCandidateFromDiscovery,
} from './helpers';

interface DeviceConnectionPanelProps {
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

export default function DeviceConnectionPanel({
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
}: DeviceConnectionPanelProps) {
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
  const wifiEndpoint = profileEndpoint(wifiProfile);
  const [deviceIpInput, setDeviceIpInput] = useState(() => wifiEndpoint || (((config as any).fanControlDeviceIp || '') as string));
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
  const { hasConnectedDevice, name: currentDeviceName, detail: currentDeviceDetail } = currentDeviceSummary({
    connectedDeviceProfile,
    connectedDeviceTransport,
    t,
  });
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
    setDeviceIpInput(wifiEndpoint || (((config as any).fanControlDeviceIp || '') as string));
  }, [activeDeviceProfileId, config, wifiEndpoint]);

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
      <DeviceConnectionScanPanel
        t={t}
        loadingKey={loadingKey}
        scanResult={scanResult}
        scanDevices={scanDevices}
        wifiDiscovery={wifiDiscovery}
        wifiScanStatus={wifiScanStatus}
        showDeepScan={showDeepScan}
        showScanSection={showScanSection}
        isNormalScanning={isNormalScanning}
        isDeepScanning={isDeepScanning}
        hasConnectedDevice={hasConnectedDevice}
        currentDeviceName={currentDeviceName}
        currentDeviceDetail={currentDeviceDetail}
        onScan={scanDevicesNow}
        onConnectCandidate={connectCandidate}
      />

      <DeviceCompatibilityPanel
        t={t}
        loadingKey={loadingKey}
        compatibilityOpen={compatibilityOpen}
        manualAddOpen={manualAddOpen}
        wifiCompatibilityEnabled={wifiCompatibilityEnabled}
        wifiDynamicIPCompatibilityEnabled={wifiDynamicIPCompatibilityEnabled}
        serialCompatibilityEnabled={serialCompatibilityEnabled}
        deviceIpInput={deviceIpInput}
        onCompatibilityOpenChange={setCompatibilityOpen}
        onManualAddOpenChange={setManualAddOpen}
        onDeviceIpInputChange={setDeviceIpInput}
        onWiFiCompatibilityChange={handleWiFiCompatibilityChange}
        onWiFiDynamicIPCompatibilityChange={handleWiFiDynamicIPCompatibilityChange}
        onSerialCompatibilityChange={handleSerialCompatibilityChange}
        onManualAdd={handleManualAdd}
      />
    </>
  );
}
