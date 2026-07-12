'use client';

import { AnimatePresence, motion } from 'framer-motion';
import { Pause, Play, RadioTower, RotateCw, Search, X } from 'lucide-react';
import clsx from 'clsx';
import { type DeviceCandidate, type DeviceScanResult } from '../../../services/api';
import { Button } from '../../ui';
import { candidateBadges } from './helpers';
import { type useWiFiDiscovery } from '../useWiFiDiscovery';

type TranslationFn = (key: string, options?: Record<string, unknown>) => string;
type WiFiDiscoveryState = ReturnType<typeof useWiFiDiscovery>;

interface DeviceConnectionScanPanelProps {
  t: TranslationFn;
  loadingKey: string;
  scanResult: DeviceScanResult | null;
  scanDevices: DeviceCandidate[];
  wifiDiscovery: WiFiDiscoveryState;
  wifiScanStatus: string;
  showDeepScan: boolean;
  showScanSection: boolean;
  isNormalScanning: boolean;
  isDeepScanning: boolean;
  hasConnectedDevice: boolean;
  currentDeviceName: string;
  currentDeviceDetail: string;
  onScan: () => void;
  onConnectCandidate: (candidate: DeviceCandidate) => void;
}

export function DeviceConnectionScanPanel({
  t,
  loadingKey,
  scanResult,
  scanDevices,
  wifiDiscovery,
  wifiScanStatus,
  showDeepScan,
  showScanSection,
  isNormalScanning,
  isDeepScanning,
  hasConnectedDevice,
  currentDeviceName,
  currentDeviceDetail,
  onScan,
  onConnectCandidate,
}: DeviceConnectionScanPanelProps) {
  return (
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
              onClick={() => void onScan()}
              loading={loadingKey === 'scan'}
              disabled={wifiDiscovery.isScanning}
            >
              {t('controlPanel.system.deviceConnection.scanAvailableDevices')}
            </Button>
          </div>
          <AnimatePresence initial={false}>
            {isDeepScanning && (
              <motion.div
                role="status"
                aria-live="polite"
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
                    onClick={() => void onConnectCandidate(candidate)}
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
  );
}
