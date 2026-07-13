'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import {
  BarChart3,
  CheckCircle2,
  Flame,
  Pause,
  Play,
  Settings,
  TriangleAlert,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { toast } from 'sonner';
import { types } from '../../../../wailsjs/go/models';
import {
  clampFanSpeedToRange,
  fanSpeedUnitLabel,
  formatFanSpeedValue,
  getFanSpeedRange,
  getFanSpeedUnit,
} from '../../lib/fan-speed';
import { useLocale } from '../../lib/i18n';
import { apiService } from '../../services/api';
import { Button, Select, ToggleSwitch } from '../ui';
import { Section, SettingRow } from './SettingLayout';
import TemperatureBaselineSection, { normalizeTemperatureSource } from './TemperatureBaselineSection';

interface FanControlSectionProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  runtimeDeviceProfile?: types.DeviceProfile | null;
  supportsCustomSpeed: boolean;
}

const SAMPLE_COUNT_OPTIONS = [
  { value: 1, labelKey: 'controlPanel.options.sampleCount.1' },
  { value: 2, labelKey: 'controlPanel.options.sampleCount.2' },
  { value: 3, labelKey: 'controlPanel.options.sampleCount.3' },
  { value: 5, labelKey: 'controlPanel.options.sampleCount.5' },
  { value: 10, labelKey: 'controlPanel.options.sampleCount.10' },
];

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function parseCustomSpeedDraft(value: unknown) {
  const text = String(value ?? '').trim();
  if (!text) return undefined;
  const numeric = Number(text);
  return Number.isFinite(numeric) ? Math.round(numeric) : undefined;
}

function normalizeSampleCount(count: string | number) {
  const parsed = typeof count === 'number' ? count : Number(count);
  return SAMPLE_COUNT_OPTIONS.some((item) => item.value === parsed) ? parsed : 1;
}

export default function FanControlSection({
  config,
  onConfigChange,
  isConnected,
  fanData,
  temperature,
  runtimeDeviceProfile,
  supportsCustomSpeed,
}: FanControlSectionProps) {
  const { t } = useTranslation();
  const { locale } = useLocale();
  const [loadingStates, setLoadingStates] = useState<Record<string, boolean>>({});
  const [showCustomSpeedWarning, setShowCustomSpeedWarning] = useState(false);
  const overviewRuntimeProfile = isConnected ? runtimeDeviceProfile : null;
  const overviewSpeedUnit = getFanSpeedUnit(fanData as any, config as any, overviewRuntimeProfile as any);
  const overviewSpeedLabel = fanSpeedUnitLabel(overviewSpeedUnit);
  const overviewSpeedRange = useMemo(() => getFanSpeedRange(config as any, overviewSpeedUnit, overviewRuntimeProfile as any), [config, overviewRuntimeProfile, overviewSpeedUnit]);
  const defaultCustomSpeed = useMemo(() => {
    const fallback = overviewSpeedUnit === 'rpm' ? 2000 : 45;
    return clampFanSpeedToRange(fallback, overviewSpeedRange, overviewSpeedRange.min) ?? overviewSpeedRange.min;
  }, [overviewSpeedRange, overviewSpeedUnit]);
  const [customSpeedInput, setCustomSpeedInput] = useState<string>(
    () => String(clampFanSpeedToRange((config as any).customSpeedRPM, overviewSpeedRange, defaultCustomSpeed) ?? defaultCustomSpeed),
  );

  const customSpeedDraftValue = useMemo(
    () => parseCustomSpeedDraft(customSpeedInput),
    [customSpeedInput],
  );
  const customSpeedMinLabel = `${formatFanSpeedValue(overviewSpeedRange.min)}${overviewSpeedLabel}`;
  const customSpeedMaxLabel = `${formatFanSpeedValue(overviewSpeedRange.max)}${overviewSpeedLabel}`;

  const sampleCountOptions = useMemo(
    () => SAMPLE_COUNT_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey) })),
    [locale, t],
  );
  const setLoading = useCallback((key: string, value: boolean) => {
    setLoadingStates((prev) => ({ ...prev, [key]: value }));
  }, []);

  const reportSettingsError = useCallback((error: unknown) => {
    toast.error(t('controlPanel.alerts.settingsOperationFailed', { error: getErrorMessage(error) }));
  }, [t]);

  const runWithLoading = useCallback(async (key: string, task: () => Promise<void>) => {
    setLoading(key, true);
    try {
      await task();
    } finally {
      setLoading(key, false);
    }
  }, [setLoading]);

  const saveConfigPatch = useCallback(async (patch: Record<string, unknown>) => {
    const newCfg = types.AppConfig.createFrom({ ...config, ...patch });
    await apiService.updateConfig(newCfg);
    onConfigChange(newCfg);
    return newCfg;
  }, [config, onConfigChange]);

  const handleAutoControlChange = useCallback(async (enabled: boolean) => {
    await runWithLoading('autoControl', async () => {
      try {
        await apiService.setAutoControl(enabled);
        const latest = await apiService.getConfig();
        onConfigChange(types.AppConfig.createFrom({ ...latest, autoControl: enabled }));
      } catch (error) {
        toast.error(t('controlPanel.fan.autoControlApplyFailed', { error: getErrorMessage(error) }));
        try {
          onConfigChange(types.AppConfig.createFrom(await apiService.getConfig()));
        } catch {
          /* noop */
        }
      }
    });
  }, [onConfigChange, runWithLoading, t]);

  const handleCustomSpeedApply = useCallback(async (enabled: boolean, speed: unknown) => {
    const safeSpeed = enabled
      ? parseCustomSpeedDraft(speed)
      : clampFanSpeedToRange((config as any).customSpeedRPM, overviewSpeedRange, defaultCustomSpeed) ?? defaultCustomSpeed;
    if (safeSpeed === undefined || safeSpeed < overviewSpeedRange.min || safeSpeed > overviewSpeedRange.max) {
      toast.error(t('controlPanel.fan.customSpeedInvalid', {
        min: customSpeedMinLabel,
        max: customSpeedMaxLabel,
      }));
      return false;
    }
    let applied = false;
    await runWithLoading('customSpeed', async () => {
      try {
        await apiService.setCustomSpeed(enabled, safeSpeed);
        setCustomSpeedInput(String(safeSpeed));
        onConfigChange(types.AppConfig.createFrom({
          ...config,
          customSpeedEnabled: enabled,
          customSpeedRPM: safeSpeed,
          autoControl: enabled ? false : config.autoControl,
        }));
        applied = true;
      } catch (error) {
        toast.error(getErrorMessage(error));
      }
    });
    return applied;
  }, [config, customSpeedMaxLabel, customSpeedMinLabel, defaultCustomSpeed, onConfigChange, overviewSpeedRange, runWithLoading, t]);

  const handleCustomSpeedToggle = useCallback((enabled: boolean) => {
    if (enabled) setShowCustomSpeedWarning(true);
    else void handleCustomSpeedApply(false, customSpeedInput);
  }, [customSpeedInput, handleCustomSpeedApply]);

  const handleSampleCountChange = useCallback(async (count: string | number) => {
    try {
      await saveConfigPatch({ tempSampleCount: normalizeSampleCount(count) });
    } catch (error) {
      reportSettingsError(error);
    }
  }, [reportSettingsError, saveConfigPatch]);

  const handleTempSourceChange = useCallback(async (source: string) => {
    await runWithLoading('tempSource', async () => {
      try {
        await saveConfigPatch({ tempSource: normalizeTemperatureSource(source) });
      } catch (error) {
        reportSettingsError(error);
      }
    });
  }, [reportSettingsError, runWithLoading, saveConfigPatch]);

  const handleGpuDeviceChange = useCallback(async (deviceKey: string) => {
    await runWithLoading('gpuDevice', async () => {
      try {
        await saveConfigPatch({
          gpuDevice: deviceKey,
          gpuSensor: 'auto',
          gpuPowerSensor: 'auto',
        });
      } catch (error) {
        reportSettingsError(error);
      }
    });
  }, [reportSettingsError, runWithLoading, saveConfigPatch]);

  const handleTempSensorChange = useCallback(async (kind: 'cpu' | 'gpu', sensorKey: string) => {
    const loadingKey = kind === 'cpu' ? 'cpuSensor' : 'gpuSensor';
    await runWithLoading(loadingKey, async () => {
      try {
        await saveConfigPatch(kind === 'cpu' ? { cpuSensor: sensorKey } : { gpuSensor: sensorKey });
      } catch (error) {
        reportSettingsError(error);
      }
    });
  }, [reportSettingsError, runWithLoading, saveConfigPatch]);

  const handlePowerSensorChange = useCallback(async (kind: 'cpu' | 'gpu', sensorKey: string) => {
    const loadingKey = kind === 'cpu' ? 'cpuPowerSensor' : 'gpuPowerSensor';
    await runWithLoading(loadingKey, async () => {
      try {
        await saveConfigPatch(kind === 'cpu' ? { cpuPowerSensor: sensorKey } : { gpuPowerSensor: sensorKey });
      } catch (error) {
        reportSettingsError(error);
      }
    });
  }, [reportSettingsError, runWithLoading, saveConfigPatch]);

  const handleGpuReadModeChange = useCallback(async (mode: string) => {
    const normalizedMode = mode === 'always' ? 'always' : 'auto';
    await runWithLoading('gpuReadMode', async () => {
      try {
        await saveConfigPatch({
          gpuReadMode: normalizedMode,
          gpuLowPowerProtection: normalizedMode !== 'always',
        });
      } catch (error) {
        reportSettingsError(error);
      }
    });
  }, [reportSettingsError, runWithLoading, saveConfigPatch]);

  const handleTransientSpikeFilterChange = useCallback(async (enabled: boolean) => {
    await runWithLoading('transientSpikeFilter', async () => {
      try {
        await saveConfigPatch({
          smartControl: types.SmartControlConfig.createFrom({
            ...(config.smartControl || {}),
            filterTransientSpike: enabled,
          }),
        });
      } catch (error) {
        reportSettingsError(error);
      }
    });
  }, [config.smartControl, reportSettingsError, runWithLoading, saveConfigPatch]);

  useEffect(() => {
    setCustomSpeedInput(String(clampFanSpeedToRange((config as any).customSpeedRPM, overviewSpeedRange, defaultCustomSpeed) ?? defaultCustomSpeed));
  }, [(config as any).customSpeedRPM, defaultCustomSpeed, overviewSpeedRange]);

  return (
    <>
      <Section title={t('controlPanel.fan.sectionTitle')} icon={Settings}>
        <SettingRow
          icon={config.autoControl ? <Play className="h-4 w-4 text-emerald-500" /> : <Pause className="h-4 w-4" />}
          title={t('controlPanel.fan.autoControlTitle')}
          description={t('controlPanel.fan.autoControlDescription')}
          disabled={(config as any).customSpeedEnabled}
        >
          <ToggleSwitch
            enabled={config.autoControl}
            onChange={handleAutoControlChange}
            disabled={(config as any).customSpeedEnabled}
            loading={loadingStates.autoControl}
            size="sm"
            color="green"
          />
        </SettingRow>

        <AnimatePresence>
          {config.autoControl && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              className="overflow-hidden"
            >
              <SettingRow
                icon={<BarChart3 className="h-4 w-4" />}
                title={t('controlPanel.fan.sampleSmoothingTitle')}
                description={t('controlPanel.fan.sampleSmoothingDescription')}
              >
                <div className="w-32">
                  <Select
                    value={(config as any).tempSampleCount || 1}
                    onChange={(value: string | number) => handleSampleCountChange(value)}
                    options={sampleCountOptions}
                    size="sm"
                  />
                </div>
              </SettingRow>
            </motion.div>
          )}
        </AnimatePresence>

        <TemperatureBaselineSection
          config={config}
          temperature={temperature}
          loadingStates={loadingStates}
          onTempSourceChange={handleTempSourceChange}
          onGpuReadModeChange={handleGpuReadModeChange}
          onGpuDeviceChange={handleGpuDeviceChange}
          onTempSensorChange={handleTempSensorChange}
          onPowerSensorChange={handlePowerSensorChange}
        />

        <SettingRow
          icon={<TriangleAlert className={clsx('h-4 w-4', (config.smartControl as any)?.filterTransientSpike !== false ? 'text-blue-500' : 'text-muted-foreground')} />}
          title={t('controlPanel.fan.transientSpikeFilterTitle')}
          description={t('controlPanel.fan.transientSpikeFilterDescription')}
        >
          <ToggleSwitch
            enabled={(config.smartControl as any)?.filterTransientSpike !== false}
            onChange={handleTransientSpikeFilterChange}
            loading={loadingStates.transientSpikeFilter}
            size="sm"
            color="blue"
            srLabel={t('controlPanel.fan.transientSpikeFilterAria')}
          />
        </SettingRow>

        {supportsCustomSpeed && (
          <div className="px-5 py-4">
            <div className="flex items-center justify-between">
              <div className="flex min-w-0 items-center gap-3">
                <div className={clsx(
                  'flex h-9 w-9 shrink-0 items-center justify-center rounded-lg transition-colors',
                  (config as any).customSpeedEnabled ? 'bg-amber-500/15 text-amber-600' : 'bg-muted text-muted-foreground',
                )}>
                  <Flame className="h-4 w-4" />
                </div>
                <div>
                  <div className="text-base font-medium text-foreground">{t('controlPanel.fan.customSpeedTitle')}</div>
                  <div className="text-sm text-muted-foreground">{t('controlPanel.fan.customSpeedDescription')}</div>
                </div>
              </div>
              <ToggleSwitch
                enabled={(config as any).customSpeedEnabled || false}
                onChange={handleCustomSpeedToggle}
                disabled={!isConnected}
                loading={loadingStates.customSpeed}
                size="sm"
                color="orange"
              />
            </div>

            <AnimatePresence>
              {(config as any).customSpeedEnabled && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  className="overflow-hidden"
                >
                  <div className="mt-3 flex items-center gap-3 rounded-xl border border-amber-300/40 bg-amber-50/50 p-3.5 dark:bg-amber-900/10">
                    <input
                      type="number"
                      value={customSpeedInput}
                      onChange={(event) => setCustomSpeedInput(event.target.value)}
                      className="flex-1 rounded-lg border border-border bg-card px-3 py-2 text-sm text-foreground focus:border-transparent focus:ring-2 focus:ring-amber-500/50"
                      step={overviewSpeedRange.step}
                    />
                    <span className="text-sm font-semibold text-amber-700 dark:text-amber-300">{overviewSpeedLabel}</span>
                    <Button variant="primary" size="sm" onClick={() => handleCustomSpeedApply(true, customSpeedInput)} className="bg-amber-600 text-white hover:bg-amber-700">
                      {t('common.actions.apply')}
                    </Button>
                  </div>
                  <p className="mt-2 text-[11px] text-amber-700 dark:text-amber-300">
                    {t('controlPanel.fan.customSpeedRangeHint', { min: customSpeedMinLabel, max: customSpeedMaxLabel })}
                  </p>
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        )}
      </Section>

      <AnimatePresence>
        {showCustomSpeedWarning && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
            onClick={() => setShowCustomSpeedWarning(false)}
          >
            <motion.div
              initial={{ scale: 0.95, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0.95, opacity: 0 }}
              transition={{ duration: 0.2 }}
              className="w-full max-w-sm rounded-2xl border border-border bg-card p-6 shadow-xl"
              onClick={(event) => event.stopPropagation()}
            >
              <div className="mb-4 flex justify-center">
                <div className="flex h-14 w-14 items-center justify-center rounded-full bg-amber-500/15">
                  <TriangleAlert className="h-8 w-8 text-amber-600" />
                </div>
              </div>

              <h3 className="mb-3 text-center text-lg font-bold text-foreground">{t('controlPanel.customSpeedDialog.title')}</h3>

              <div className="mb-4 rounded-xl border border-amber-300/40 bg-amber-500/10 p-3 text-sm">
                <p className="mb-2 font-medium text-foreground">{t('controlPanel.customSpeedDialog.enabledTitle')}</p>
                <ul className="space-y-1 text-xs text-muted-foreground">
                  <li>{t('controlPanel.customSpeedDialog.bullets.disableAutoControl')}</li>
                  <li>{t('controlPanel.customSpeedDialog.bullets.fixedSpeed')}</li>
                  <li>{t('controlPanel.customSpeedDialog.bullets.insufficientCooling')}</li>
                </ul>
              </div>

              <div className="mb-5 rounded-xl bg-muted/60 p-3 text-center">
                <span className="text-xs text-muted-foreground">{t('controlPanel.customSpeedDialog.speedLabel')}</span>
                <div className="text-xl font-bold text-amber-600">{formatFanSpeedValue(customSpeedDraftValue)}{overviewSpeedLabel}</div>
              </div>

              <div className="flex gap-3">
                <Button variant="secondary" onClick={() => setShowCustomSpeedWarning(false)} className="flex-1">
                  {t('common.actions.cancel')}
                </Button>
                <Button
                  variant="primary"
                  onClick={async () => {
                    if (await handleCustomSpeedApply(true, customSpeedInput)) {
                      setShowCustomSpeedWarning(false);
                    }
                  }}
                  className="flex-1 bg-amber-600 text-white hover:bg-amber-700"
                  icon={<CheckCircle2 className="h-4 w-4" />}
                >
                  {t('controlPanel.customSpeedDialog.confirm')}
                </Button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}
