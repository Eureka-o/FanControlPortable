'use client';

import { useEffect, useMemo, useState } from 'react';
import { BarChart3, Cpu, Gpu } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { types } from '../../../../wailsjs/go/models';
import { useLocale } from '../../lib/i18n';
import { apiService } from '../../services/api';
import { useAppStore } from '../../store/app-store';
import { Button, Select } from '../ui';
import { SelectionField } from './SettingLayout';

interface TemperatureBaselineSectionProps {
  config: types.AppConfig;
  temperature: types.TemperatureData | null;
  loadingStates: Record<string, boolean>;
  onTempSourceChange: (source: string) => void | Promise<void>;
  onGpuReadModeChange: (mode: string) => void | Promise<void>;
  onGpuDeviceChange: (deviceKey: string) => void | Promise<void>;
  onTempSensorChange: (kind: 'cpu' | 'gpu', sensorKey: string) => void | Promise<void>;
  onPowerSensorChange: (kind: 'cpu' | 'gpu', sensorKey: string) => void | Promise<void>;
}

const TEMP_SOURCE_OPTIONS = [
  { value: 'max', labelKey: 'controlPanel.options.tempSource.max' },
  { value: 'cpu', labelKey: 'controlPanel.options.tempSource.cpu' },
  { value: 'gpu', labelKey: 'controlPanel.options.tempSource.gpu' },
];

const GPU_READ_MODE_OPTIONS = [
  { value: 'auto', labelKey: 'controlPanel.options.gpuReadMode.auto' },
  { value: 'always', labelKey: 'controlPanel.options.gpuReadMode.always' },
  { value: 'never', labelKey: 'controlPanel.options.gpuReadMode.never' },
];

const formatPowerSensorValue = (value: number) => (
  Number.isFinite(value) && value >= 0 ? `${value.toFixed(1)} W` : '-- W'
);

export function normalizeTemperatureSource(source: string) {
  return source === 'cpu' || source === 'gpu' || source === 'max' ? source : 'max';
}

export function formatFallbackSensorLabel(sensorKey: string): string {
  const trimmed = sensorKey.trim();
  if (!trimmed) {
    return trimmed;
  }
  const parts = trimmed.split('/').map((part) => part.trim()).filter(Boolean);
  return parts.length >= 2 ? parts[parts.length - 1] : trimmed;
}

/**
 * 把「温度来源」取值翻成界面文案。
 */
function translateControlSource(
  source: string | null | undefined,
  t: (key: string) => string,
) {
  switch (source) {
    case 'cpu':
      return t('controlPanel.options.tempSource.cpu');
    case 'gpu':
      return t('controlPanel.options.tempSource.gpu');
    default:
      return t('controlPanel.options.tempSource.max');
  }
}

export default function TemperatureBaselineSection({
  config,
  temperature,
  loadingStates,
  onTempSourceChange,
  onGpuReadModeChange,
  onGpuDeviceChange,
  onTempSensorChange,
  onPowerSensorChange,
}: TemperatureBaselineSectionProps) {
  const { t } = useTranslation();
  const { locale } = useLocale();

  const currentTempSource = normalizeTemperatureSource((((config as any).tempSource as string) || 'max'));
  const configuredGpuReadMode = ((config as any).gpuReadMode as string) || (((config as any).gpuLowPowerProtection === false) ? 'always' : 'auto');
  const currentGpuReadMode = (configuredGpuReadMode === 'always' || configuredGpuReadMode === 'never' ? configuredGpuReadMode : 'auto') as 'auto' | 'always' | 'never';
  const gpuReadDisabled = currentGpuReadMode === 'never';
  const gpuReadState = (((temperature as any)?.gpuReadState as string) || 'unknown');
  const gpuNotPolled = gpuReadDisabled || gpuReadState === 'notPolled';
  const gpuLowPowerProtectionEnabled = currentGpuReadMode !== 'always';
  const [probingGpuSensors, setProbingGpuSensors] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void apiService.setSensorCatalogActive(true)
      .then((fresh) => {
        if (!cancelled && fresh && fresh.updateTime > 0) {
          useAppStore.getState().handleTemperaturePayload(fresh);
        }
      })
      .catch(() => {});
    return () => {
      cancelled = true;
      void apiService.setSensorCatalogActive(false).catch(() => {});
    };
  }, []);

  const handleProbeGpuSensors = async () => {
    if (probingGpuSensors) {
      return;
    }
    setProbingGpuSensors(true);
    try {
      const fresh = await apiService.probeGpuSensors();
      if (fresh && fresh.updateTime > 0) {
        useAppStore.getState().handleTemperaturePayload(fresh);
      }
    } catch {
      // 忽略临时唤醒探测失败，保持现有提示状态
    } finally {
      setProbingGpuSensors(false);
    }
  };

  const cpuSensors = useMemo(() => (Array.isArray(temperature?.cpuSensors) ? temperature.cpuSensors : []), [temperature?.cpuSensors]);
  const gpuSensors = useMemo(() => (Array.isArray(temperature?.gpuSensors) ? temperature.gpuSensors : []), [temperature?.gpuSensors]);
  const cpuPowerSensors = useMemo(
    () => (Array.isArray((temperature as any)?.cpuPowerSensors) ? ((temperature as any).cpuPowerSensors as types.PowerSensor[]) : []),
    [temperature?.cpuPowerWatts, temperature?.cpuPowerSensors],
  );
  const gpuPowerSensors = useMemo(
    () => (Array.isArray((temperature as any)?.gpuPowerSensors) ? ((temperature as any).gpuPowerSensors as types.PowerSensor[]) : []),
    [temperature?.gpuPowerWatts, temperature?.gpuPowerSensors],
  );
  const rawGpuDevices = (temperature as any)?.gpuDevices;
  const gpuDevices = useMemo(
    () => (Array.isArray(rawGpuDevices) ? (rawGpuDevices as types.TemperatureGPUDevice[]) : []),
    [rawGpuDevices],
  );
  const selectedGpuDevice = useMemo(() => {
    const configured = (((config as any).gpuDevice as string) || 'auto');
    return configured || 'auto';
  }, [config]);
  const detectedGpuDevice = (temperature as any)?.selectedGpuDevice;
  const activeGpuDeviceKey = useMemo(() => {
    if (selectedGpuDevice !== 'auto') {
      return selectedGpuDevice;
    }
    const detected = (detectedGpuDevice as string) || 'auto';
    return gpuDevices.some((device) => device.key === detected) ? detected : 'auto';
  }, [selectedGpuDevice, detectedGpuDevice, gpuDevices]);
  const activeGpuDevice = useMemo(() => {
    return gpuDevices.find((device) => device.key === activeGpuDeviceKey) || null;
  }, [activeGpuDeviceKey, gpuDevices]);
  const effectiveGpuSensors = useMemo(() => {
    if (activeGpuDevice && Array.isArray(activeGpuDevice.sensors) && activeGpuDevice.sensors.length > 0) {
      return activeGpuDevice.sensors;
    }
    return gpuSensors;
  }, [activeGpuDevice, gpuSensors]);
  const effectiveGpuPowerSensors = useMemo(() => {
    const devicePowerSensors = (activeGpuDevice as any)?.powerSensors;
    if (Array.isArray(devicePowerSensors) && devicePowerSensors.length > 0) {
      return devicePowerSensors as types.PowerSensor[];
    }
    return gpuPowerSensors;
  }, [activeGpuDevice, gpuPowerSensors]);

  const selectedCpuSensor = (((config as any).cpuSensor as string) || 'auto');
  const selectedGpuSensor = (((config as any).gpuSensor as string) || 'auto');
  const selectedCpuPowerSensor = (((config as any).cpuPowerSensor as string) || 'auto');
  const selectedGpuPowerSensor = (((config as any).gpuPowerSensor as string) || 'auto');

  const tempSourceOptions = useMemo(
    () => TEMP_SOURCE_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey), disabled: gpuReadDisabled && item.value === 'gpu' })),
    [gpuReadDisabled, locale, t],
  );
  const gpuReadModeOptions = useMemo(
    () => GPU_READ_MODE_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey) })),
    [locale, t],
  );
  const gpuDeviceOptions = useMemo(() => [
    { value: 'auto', label: gpuDevices.length > 0 ? t('controlPanel.options.gpuDevice.autoPreferred') : t('controlPanel.options.gpuDevice.auto') },
    ...gpuDevices.map((device) => ({
      value: device.key,
      label: `${device.vendor ? `${device.vendor.toUpperCase()} · ` : ''}${device.name}`,
    })),
    ...(selectedGpuDevice !== 'auto' && !gpuDevices.some((device) => device.key === selectedGpuDevice)
      ? [{ value: selectedGpuDevice, label: formatFallbackSensorLabel(selectedGpuDevice), disabled: true }]
      : []),
  ], [gpuDevices, locale, selectedGpuDevice, t]);
  const cpuSensorOptions = useMemo(() => [
    { value: 'auto', label: cpuSensors.length > 0 ? t('controlPanel.options.sensor.autoRecommended') : t('controlPanel.options.sensor.auto') },
    ...cpuSensors.map((sensor) => ({ value: sensor.key, label: `${sensor.name} (${sensor.value}°C)` })),
    ...(selectedCpuSensor !== 'auto' && !cpuSensors.some((sensor) => sensor.key === selectedCpuSensor)
      ? [{ value: selectedCpuSensor, label: formatFallbackSensorLabel(selectedCpuSensor), disabled: true }]
      : []),
  ], [cpuSensors, locale, selectedCpuSensor, t]);
  const gpuSensorOptions = useMemo(() => [
    { value: 'auto', label: effectiveGpuSensors.length > 0 ? t('controlPanel.options.sensor.autoRecommended') : t('controlPanel.options.sensor.auto') },
    ...effectiveGpuSensors.map((sensor) => ({ value: sensor.key, label: `${sensor.name} (${sensor.value}°C)` })),
    ...(selectedGpuSensor !== 'auto' && !effectiveGpuSensors.some((sensor) => sensor.key === selectedGpuSensor)
      ? [{ value: selectedGpuSensor, label: formatFallbackSensorLabel(selectedGpuSensor), disabled: true }]
      : []),
  ], [effectiveGpuSensors, locale, selectedGpuSensor, t]);
  const cpuPowerSensorOptions = useMemo(() => [
    { value: 'auto', label: cpuPowerSensors.length > 0 ? t('controlPanel.options.sensor.autoRecommended') : t('controlPanel.options.sensor.auto') },
    ...cpuPowerSensors.map((sensor) => ({ value: sensor.key, label: `${sensor.name} (${formatPowerSensorValue(sensor.value)})` })),
    ...(selectedCpuPowerSensor !== 'auto' && !cpuPowerSensors.some((sensor) => sensor.key === selectedCpuPowerSensor)
      ? [{ value: selectedCpuPowerSensor, label: formatFallbackSensorLabel(selectedCpuPowerSensor), disabled: true }]
      : []),
  ], [cpuPowerSensors, locale, selectedCpuPowerSensor, t]);
  const gpuPowerSensorOptions = useMemo(() => [
    { value: 'auto', label: effectiveGpuPowerSensors.length > 0 ? t('controlPanel.options.sensor.autoRecommended') : t('controlPanel.options.sensor.auto') },
    ...effectiveGpuPowerSensors.map((sensor) => ({ value: sensor.key, label: `${sensor.name} (${formatPowerSensorValue(sensor.value)})` })),
    ...(selectedGpuPowerSensor !== 'auto' && !effectiveGpuPowerSensors.some((sensor) => sensor.key === selectedGpuPowerSensor)
      ? [{ value: selectedGpuPowerSensor, label: formatFallbackSensorLabel(selectedGpuPowerSensor), disabled: true }]
      : []),
  ], [effectiveGpuPowerSensors, locale, selectedGpuPowerSensor, t]);

  const cpuDeviceHint = temperature?.cpuModel?.trim()
    ? t('controlPanel.fan.processorDeviceHintDetected')
    : t('controlPanel.fan.processorDeviceHintWaiting');
  const cpuFooterText = temperature?.cpuTemp && temperature.cpuTemp > 0
    ? t('controlPanel.fan.currentBaselineTemperature', { temperature: temperature.cpuTemp })
    : t('controlPanel.fan.noCpuTemperatureData');
  const gpuDeviceHint = gpuReadDisabled
    ? t('controlPanel.fan.gpuDeviceHintDisabled')
    : gpuNotPolled
      ? t('controlPanel.fan.gpuDeviceHintNotPolled')
      : selectedGpuDevice === 'auto'
        ? (temperature?.gpuModel?.trim() ? t('controlPanel.fan.gpuDeviceHintDetected', { model: temperature.gpuModel }) : t('controlPanel.fan.gpuDeviceHintAuto'))
        : t('controlPanel.fan.gpuDeviceHintLocked');
  const gpuFooterText = gpuReadDisabled
    ? t('controlPanel.fan.gpuTemperatureDisabled')
    : gpuNotPolled
      ? t('controlPanel.fan.gpuTemperatureNotPolled')
      : temperature?.gpuTemp && temperature.gpuTemp > 0
        ? t('controlPanel.fan.currentBaselineTemperature', { temperature: temperature.gpuTemp })
        : t('controlPanel.fan.noGpuTemperatureData');

  return (
    <div className="space-y-3.5 px-5 py-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <div data-theme-ui="setting-row-icon" data-icon-tone="amber" className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[11px] bg-muted/70 text-muted-foreground shadow-inner shadow-white/20">
            <BarChart3 className="h-4 w-4" />
          </div>
          <div className="min-w-0">
            <div className="text-[15px] font-semibold tracking-[-0.01em] text-foreground">{t('controlPanel.fan.temperatureBaselineTitle')}</div>
            <div className="mt-0.5 text-[13px] leading-[1.45] text-muted-foreground">{t('controlPanel.fan.temperatureBaselineDescription')}</div>
          </div>
        </div>
        <div className="w-full md:w-40">
          <Select
            value={currentTempSource}
            onChange={(value: string | number) => onTempSourceChange(String(value))}
            options={tempSourceOptions}
            size="sm"
            className="w-full min-w-0"
          />
        </div>
      </div>

      <div className="grid gap-3.5 md:grid-cols-2">
        <div data-theme-ui="setting-subcard" className="rounded-2xl border border-border/60 bg-muted/25 p-4 md:col-span-2">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex min-w-0 items-start gap-2.5">
              <Gpu className={clsx('mt-0.5 h-4 w-4 shrink-0', gpuLowPowerProtectionEnabled ? 'text-primary' : 'text-muted-foreground')} />
              <div className="min-w-0">
                <div className="text-[14px] font-semibold text-foreground">{t('controlPanel.fan.gpuLowPowerProtectionTitle')}</div>
                <div className="mt-0.5 text-[13px] leading-[1.45] text-muted-foreground">{t('controlPanel.fan.gpuLowPowerProtectionDescription')}</div>
              </div>
            </div>
            <div className="w-full sm:w-40">
              <Select
                value={currentGpuReadMode}
                onChange={(value: string | number) => onGpuReadModeChange(String(value))}
                options={gpuReadModeOptions}
                size="sm"
                className="w-full min-w-0"
                disabled={loadingStates.gpuReadMode}
              />
            </div>
          </div>
        </div>

        <div data-theme-ui="setting-subcard" className="flex flex-col rounded-2xl border border-border/60 bg-muted/25 p-4">
          <div className="flex h-8 items-center justify-between gap-2">
            <div className="flex min-w-0 items-center gap-2 text-[14px] font-semibold text-foreground">
              <Cpu className="h-4 w-4 shrink-0 text-primary" />
              <span className="truncate">{t('controlPanel.fan.cpuBaseline')}</span>
            </div>
          </div>
          <div className="mt-3 space-y-3">
            <SelectionField
              label={t('controlPanel.fan.processorDevice')}
              hint={cpuDeviceHint}
              hintClassName="h-5 truncate leading-5"
            >
              <div className="flex h-10 items-center rounded-lg border border-border/70 bg-background px-3 text-sm text-foreground">
                <span className="truncate">{temperature?.cpuModel?.trim() || t('controlPanel.fan.waitingRecognition')}</span>
              </div>
            </SelectionField>

            <SelectionField label={t('controlPanel.fan.temperatureSensor')}>
              <Select
                value={selectedCpuSensor}
                onChange={(value: string | number) => onTempSensorChange('cpu', String(value))}
                options={cpuSensorOptions}
                size="sm"
                className="w-full min-w-0"
                disabled={loadingStates.cpuSensor}
              />
            </SelectionField>

            <SelectionField label={t('controlPanel.fan.powerSensor')}>
              <Select
                value={selectedCpuPowerSensor}
                onChange={(value: string | number) => onPowerSensorChange('cpu', String(value))}
                options={cpuPowerSensorOptions}
                size="sm"
                className="w-full min-w-0"
                disabled={loadingStates.cpuPowerSensor}
              />
            </SelectionField>
          </div>
          <div className="mt-2.5 flex h-5 items-center text-xs text-muted-foreground">
            <span className="truncate" title={cpuFooterText}>{cpuFooterText}</span>
          </div>
        </div>

        <div data-theme-ui="setting-subcard" className="flex flex-col rounded-2xl border border-border/60 bg-muted/25 p-4">
          <div className="flex h-8 items-center justify-between gap-2">
            <div className="flex min-w-0 items-center gap-2 text-[14px] font-semibold text-foreground">
              <Gpu className="h-4 w-4 shrink-0 text-primary" />
              <span className="truncate">{t('controlPanel.fan.gpuBaseline')}</span>
            </div>
            {currentGpuReadMode === 'auto' && gpuReadState === 'notPolled' && (
              <Button
                type="button"
                variant="secondary"
                size="sm"
                className="h-7 shrink-0 px-2.5 text-xs"
                title={t('controlPanel.fan.probeGpuSensorsTooltip')}
                onClick={() => { void handleProbeGpuSensors(); }}
                disabled={probingGpuSensors}
              >
                {probingGpuSensors ? t('controlPanel.fan.probingGpuSensors') : t('controlPanel.fan.probeGpuSensors')}
              </Button>
            )}
          </div>
          <div className="mt-3 space-y-3">
            <SelectionField
              label={t('controlPanel.fan.gpuDevice')}
              hint={gpuDeviceHint}
              hintClassName="h-5 truncate leading-5"
            >
              <Select
                value={selectedGpuDevice}
                onChange={(value: string | number) => onGpuDeviceChange(String(value))}
                options={gpuDeviceOptions}
                size="sm"
                className="w-full min-w-0"
                disabled={gpuNotPolled || gpuDevices.length === 0 || loadingStates.gpuDevice}
              />
            </SelectionField>

            <SelectionField label={t('controlPanel.fan.temperatureSensor')}>
              <Select
                value={selectedGpuSensor}
                onChange={(value: string | number) => onTempSensorChange('gpu', String(value))}
                options={gpuSensorOptions}
                size="sm"
                className="w-full min-w-0"
                disabled={gpuNotPolled || loadingStates.gpuSensor}
              />
            </SelectionField>

            <SelectionField label={t('controlPanel.fan.powerSensor')}>
              <Select
                value={selectedGpuPowerSensor}
                onChange={(value: string | number) => onPowerSensorChange('gpu', String(value))}
                options={gpuPowerSensorOptions}
                size="sm"
                className="w-full min-w-0"
                disabled={gpuNotPolled || loadingStates.gpuPowerSensor}
              />
            </SelectionField>
          </div>
          <div className="mt-2.5 flex h-5 items-center text-xs text-muted-foreground">
            <span className="truncate" title={gpuFooterText}>{gpuFooterText}</span>
          </div>
        </div>
        </div>

      <div className="rounded-xl border border-border/55 bg-muted/20 px-3.5 py-2 text-xs text-muted-foreground">
        {temperature?.controlTemp && temperature.controlTemp > 0
          ? t('controlPanel.fan.currentControlSource', { source: translateControlSource(temperature.controlSource, t), temperature: temperature.controlTemp })
          : t('controlPanel.fan.noControlTemperature')}
      </div>
    </div>
  );
}
