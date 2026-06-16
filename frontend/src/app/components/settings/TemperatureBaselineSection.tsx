'use client';

import { useMemo } from 'react';
import { BarChart3, Cpu, Gpu } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { types } from '../../../../wailsjs/go/models';
import { useLocale } from '../../lib/i18n';
import { Select } from '../ui';
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
];

const formatPowerSensorValue = (value: number) => (
  Number.isFinite(value) && value >= 0 ? `${value.toFixed(1)} W` : '-- W'
);

export function normalizeTemperatureSource(source: string) {
  return source === 'cpu' || source === 'gpu' || source === 'max' ? source : 'max';
}

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
  const currentGpuReadMode = (((config as any).gpuReadMode as string) || (((config as any).gpuLowPowerProtection === false) ? 'always' : 'auto')) as 'auto' | 'always';
  const gpuReadState = (((temperature as any)?.gpuReadState as string) || 'unknown');
  const gpuNotPolled = gpuReadState === 'notPolled';
  const gpuLowPowerProtectionEnabled = currentGpuReadMode !== 'always';

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
    return configured === 'auto' || gpuDevices.some((device) => device.key === configured) ? configured : 'auto';
  }, [config, gpuDevices]);
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

  const selectedCpuSensor = useMemo(() => {
    const configured = (((config as any).cpuSensor as string) || 'auto');
    return cpuSensors.some((sensor) => sensor.key === configured) ? configured : 'auto';
  }, [config, cpuSensors]);
  const selectedGpuSensor = useMemo(() => {
    const configured = (((config as any).gpuSensor as string) || 'auto');
    return effectiveGpuSensors.some((sensor) => sensor.key === configured) ? configured : 'auto';
  }, [config, effectiveGpuSensors]);
  const selectedCpuPowerSensor = useMemo(() => {
    const configured = (((config as any).cpuPowerSensor as string) || 'auto');
    return cpuPowerSensors.some((sensor) => sensor.key === configured) ? configured : 'auto';
  }, [config, cpuPowerSensors]);
  const selectedGpuPowerSensor = useMemo(() => {
    const configured = (((config as any).gpuPowerSensor as string) || 'auto');
    return effectiveGpuPowerSensors.some((sensor) => sensor.key === configured) ? configured : 'auto';
  }, [config, effectiveGpuPowerSensors]);

  const tempSourceOptions = useMemo(
    () => TEMP_SOURCE_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey) })),
    [locale, t],
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
  ], [gpuDevices, locale, t]);
  const cpuSensorOptions = useMemo(() => [
    { value: 'auto', label: cpuSensors.length > 0 ? t('controlPanel.options.sensor.autoRecommended') : t('controlPanel.options.sensor.auto') },
    ...cpuSensors.map((sensor) => ({ value: sensor.key, label: `${sensor.name} (${sensor.value}°C)` })),
  ], [cpuSensors, locale, t]);
  const gpuSensorOptions = useMemo(() => [
    { value: 'auto', label: effectiveGpuSensors.length > 0 ? t('controlPanel.options.sensor.autoRecommended') : t('controlPanel.options.sensor.auto') },
    ...effectiveGpuSensors.map((sensor) => ({ value: sensor.key, label: `${sensor.name} (${sensor.value}°C)` })),
  ], [effectiveGpuSensors, locale, t]);
  const cpuPowerSensorOptions = useMemo(() => [
    { value: 'auto', label: cpuPowerSensors.length > 0 ? t('controlPanel.options.sensor.autoRecommended') : t('controlPanel.options.sensor.auto') },
    ...cpuPowerSensors.map((sensor) => ({ value: sensor.key, label: `${sensor.name} (${formatPowerSensorValue(sensor.value)})` })),
  ], [cpuPowerSensors, locale, t]);
  const gpuPowerSensorOptions = useMemo(() => [
    { value: 'auto', label: effectiveGpuPowerSensors.length > 0 ? t('controlPanel.options.sensor.autoRecommended') : t('controlPanel.options.sensor.auto') },
    ...effectiveGpuPowerSensors.map((sensor) => ({ value: sensor.key, label: `${sensor.name} (${formatPowerSensorValue(sensor.value)})` })),
  ], [effectiveGpuPowerSensors, locale, t]);

  return (
    <div className="px-5 py-4">
      <div className="rounded-2xl border border-border/70 bg-muted/25 p-4">
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <BarChart3 className="h-4 w-4" />
            </div>
            <div className="min-w-0">
              <div className="text-base font-medium text-foreground">{t('controlPanel.fan.temperatureBaselineTitle')}</div>
              <div className="text-sm text-muted-foreground">{t('controlPanel.fan.temperatureBaselineDescription')}</div>
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

        <div className="mt-4 grid gap-3 md:grid-cols-2">
          <div className="rounded-xl border border-border/70 bg-background/55 px-4 py-3 md:col-span-2">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex min-w-0 items-start gap-2">
                <Gpu className={clsx('mt-0.5 h-4 w-4', gpuLowPowerProtectionEnabled ? 'text-primary' : 'text-muted-foreground')} />
                <div className="min-w-0">
                  <div className="text-sm font-medium text-foreground">{t('controlPanel.fan.gpuLowPowerProtectionTitle')}</div>
                  <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{t('controlPanel.fan.gpuLowPowerProtectionDescription')}</div>
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

          <div className="rounded-xl border border-border/70 bg-card px-4 py-3">
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <Cpu className="h-4 w-4 text-primary" />
              <span>{t('controlPanel.fan.cpuBaseline')}</span>
            </div>
            <div className="mt-3 space-y-3">
              <SelectionField
                label={t('controlPanel.fan.processorDevice')}
                hint={temperature?.cpuModel?.trim() ? t('controlPanel.fan.processorDeviceHintDetected') : t('controlPanel.fan.processorDeviceHintWaiting')}
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
                  disabled={!cpuSensors.length || loadingStates.cpuSensor}
                />
              </SelectionField>

              <SelectionField label={t('controlPanel.fan.powerSensor')}>
                <Select
                  value={selectedCpuPowerSensor}
                  onChange={(value: string | number) => onPowerSensorChange('cpu', String(value))}
                  options={cpuPowerSensorOptions}
                  size="sm"
                  className="w-full min-w-0"
                  disabled={!cpuPowerSensors.length || loadingStates.cpuPowerSensor}
                />
              </SelectionField>
            </div>
            <div className="mt-2 text-xs text-muted-foreground">
              {temperature?.cpuTemp && temperature.cpuTemp > 0 ? t('controlPanel.fan.currentBaselineTemperature', { temperature: temperature.cpuTemp }) : t('controlPanel.fan.noCpuTemperatureData')}
            </div>
          </div>

          <div className="rounded-xl border border-border/70 bg-card px-4 py-3">
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <Gpu className="h-4 w-4 text-primary" />
              <span>{t('controlPanel.fan.gpuBaseline')}</span>
            </div>
            <div className="mt-3 space-y-3">
              <SelectionField
                label={t('controlPanel.fan.gpuDevice')}
                hint={gpuNotPolled
                  ? t('controlPanel.fan.gpuDeviceHintNotPolled')
                  : selectedGpuDevice === 'auto'
                    ? (temperature?.gpuModel?.trim() ? t('controlPanel.fan.gpuDeviceHintDetected', { model: temperature.gpuModel }) : t('controlPanel.fan.gpuDeviceHintAuto'))
                    : t('controlPanel.fan.gpuDeviceHintLocked')}
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
                  disabled={gpuNotPolled || !effectiveGpuSensors.length || loadingStates.gpuSensor}
                />
              </SelectionField>

              <SelectionField label={t('controlPanel.fan.powerSensor')}>
                <Select
                  value={selectedGpuPowerSensor}
                  onChange={(value: string | number) => onPowerSensorChange('gpu', String(value))}
                  options={gpuPowerSensorOptions}
                  size="sm"
                  className="w-full min-w-0"
                  disabled={gpuNotPolled || !effectiveGpuPowerSensors.length || loadingStates.gpuPowerSensor}
                />
              </SelectionField>
            </div>
            <div className="mt-2 text-xs text-muted-foreground">
              {gpuNotPolled
                ? t('controlPanel.fan.gpuTemperatureNotPolled')
                : temperature?.gpuTemp && temperature.gpuTemp > 0 ? t('controlPanel.fan.currentBaselineTemperature', { temperature: temperature.gpuTemp }) : t('controlPanel.fan.noGpuTemperatureData')}
            </div>
          </div>
        </div>

        <div className="mt-3 text-xs text-muted-foreground">
          {temperature?.controlTemp && temperature.controlTemp > 0
            ? t('controlPanel.fan.currentControlSource', { source: translateControlSource(temperature.controlSource, t), temperature: temperature.controlTemp })
            : t('controlPanel.fan.noControlTemperature')}
        </div>
      </div>
    </div>
  );
}
