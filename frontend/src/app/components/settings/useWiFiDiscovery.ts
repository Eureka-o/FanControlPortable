'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { useTranslation } from 'react-i18next';
import { apiService, type WiFiDiscoveryResult } from '../../services/api';
import { wifiDiscoveryDevices, wifiDiscoveryElapsedLabel } from './device-connection-utils';

export type WiFiDiscoveryMode = 'normal' | 'deep';

export function useWiFiDiscovery({
  profileAvailable,
  resetKey,
}: {
  profileAvailable: boolean;
  resetKey?: string;
}) {
  const { t } = useTranslation();
  const [result, setResult] = useState<WiFiDiscoveryResult | null>(null);
  const [error, setError] = useState('');
  const [mode, setMode] = useState<WiFiDiscoveryMode | null>(null);
  const [startedAt, setStartedAt] = useState<number | null>(null);
  const [now, setNow] = useState<number>(() => Date.now());
  const [paused, setPaused] = useState(false);
  const [pausedAt, setPausedAt] = useState<number | null>(null);
  const [pausedTotalMs, setPausedTotalMs] = useState(0);
  const [canceling, setCanceling] = useState(false);
  const [normalScanAttempted, setNormalScanAttempted] = useState(false);

  const isScanning = mode !== null && startedAt !== null;
  const devices = useMemo(() => wifiDiscoveryDevices(result), [result]);
  const liveElapsedMs = isScanning && startedAt !== null ? Math.max(0, now - startedAt) : undefined;
  const currentPausedMs = isScanning && paused && pausedAt !== null ? Math.max(0, now - pausedAt) : 0;
  const activeElapsedMs = typeof liveElapsedMs === 'number'
    ? Math.max(0, liveElapsedMs - pausedTotalMs - currentPausedMs)
    : undefined;
  const elapsedText = isScanning
    ? wifiDiscoveryElapsedLabel(activeElapsedMs) || '0ms'
    : wifiDiscoveryElapsedLabel(result?.elapsedMs);
  const progressLimitMs = mode === 'deep' ? 75000 : 8000;
  const progressPercent = isScanning
    ? Math.min(96, Math.max(4, Math.round(((activeElapsedMs || 0) / progressLimitMs) * 100)))
    : 100;
  const showDeepScanAction = normalScanAttempted
    && devices.length === 0
    && (!isScanning || mode === 'deep');
  const runningKey = canceling
    ? 'controlPanel.system.deviceConnection.wifiScanCanceling'
    : paused
    ? 'controlPanel.system.deviceConnection.wifiScanPaused'
    : mode === 'deep'
    ? (activeElapsedMs || 0) >= 12000
      ? 'controlPanel.system.deviceConnection.wifiScanRunningExpanded'
      : 'controlPanel.system.deviceConnection.wifiScanRunningDeep'
    : 'controlPanel.system.deviceConnection.wifiScanRunningNormal';

  const reset = useCallback(() => {
    setResult(null);
    setError('');
    setMode(null);
    setStartedAt(null);
    setPaused(false);
    setPausedAt(null);
    setPausedTotalMs(0);
    setCanceling(false);
    setNormalScanAttempted(false);
  }, []);

  const scan = useCallback(async (nextMode: WiFiDiscoveryMode) => {
    if (!profileAvailable) {
      toast.info(t('controlPanel.system.deviceConnection.toasts.profileRequired'));
      return;
    }

    const scanStartedAt = Date.now();
    setMode(nextMode);
    setStartedAt(scanStartedAt);
    setNow(scanStartedAt);
    setPaused(false);
    setPausedAt(null);
    setPausedTotalMs(0);
    setCanceling(false);
    setError('');
    if (nextMode === 'normal') {
      setResult(null);
      setNormalScanAttempted(true);
    }

    try {
      const scanResult = await apiService.scanWiFiDevices(nextMode);
      const elapsedMs = typeof scanResult.elapsedMs === 'number' && scanResult.elapsedMs > 0
        ? scanResult.elapsedMs
        : Date.now() - scanStartedAt;
      const normalizedResult = { ...scanResult, elapsedMs };
      setResult(normalizedResult);

      if (normalizedResult.canceled) {
        toast.info(t('controlPanel.system.deviceConnection.toasts.wifiScanCanceled'));
        return;
      }
      if (normalizedResult.error) {
        setError(normalizedResult.error);
        toast.error(t('controlPanel.system.deviceConnection.toasts.wifiScanFailed', { error: normalizedResult.error }));
        return;
      }

      const count = normalizedResult.devices?.length || 0;
      toast.info(count > 0
        ? t('controlPanel.system.deviceConnection.toasts.wifiScanFound', { count })
        : t('controlPanel.system.deviceConnection.toasts.wifiScanEmpty'));
    } catch (scanError) {
      const message = scanError instanceof Error ? scanError.message : String(scanError);
      setError(message);
      toast.error(t('controlPanel.system.deviceConnection.toasts.wifiScanFailed', { error: message }));
    } finally {
      setMode(null);
      setStartedAt(null);
      setPaused(false);
      setPausedAt(null);
      setPausedTotalMs(0);
      setCanceling(false);
    }
  }, [profileAvailable, t]);

  const pauseToggle = useCallback(async () => {
    if (!isScanning || mode !== 'deep' || canceling) return;

    if (paused) {
      const ok = await apiService.controlWiFiScan('resume');
      if (!ok) return;
      const resumedAt = Date.now();
      if (pausedAt !== null) {
        setPausedTotalMs((value) => value + Math.max(0, resumedAt - pausedAt));
      }
      setNow(resumedAt);
      setPaused(false);
      setPausedAt(null);
      return;
    }

    const ok = await apiService.controlWiFiScan('pause');
    if (!ok) return;
    const pausedNow = Date.now();
    setNow(pausedNow);
    setPaused(true);
    setPausedAt(pausedNow);
  }, [canceling, isScanning, mode, paused, pausedAt]);

  const cancel = useCallback(async () => {
    if (!isScanning || mode !== 'deep' || canceling) return;

    const canceledAt = Date.now();
    setCanceling(true);
    if (pausedAt !== null) {
      setPausedTotalMs((value) => value + Math.max(0, canceledAt - pausedAt));
    }
    setNow(canceledAt);
    setPaused(false);
    setPausedAt(null);

    const ok = await apiService.controlWiFiScan('cancel');
    if (!ok) {
      setCanceling(false);
    }
  }, [canceling, isScanning, mode, pausedAt]);

  useEffect(() => reset(), [reset, resetKey]);

  useEffect(() => {
    if (!isScanning || startedAt === null) return undefined;
    if (paused && !canceling) {
      if (pausedAt !== null) {
        setNow(pausedAt);
      }
      return undefined;
    }

    setNow(Date.now());
    const timer = window.setInterval(() => {
      setNow(Date.now());
    }, 250);

    return () => {
      window.clearInterval(timer);
    };
  }, [canceling, isScanning, paused, pausedAt, startedAt]);

  return {
    result,
    error,
    mode,
    isScanning,
    paused,
    canceling,
    devices,
    normalScanAttempted,
    showDeepScanAction,
    elapsedText,
    progressPercent,
    runningKey,
    scan,
    pauseToggle,
    cancel,
    reset,
  };
}
