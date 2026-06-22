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
  Sparkles,
  Spline,
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
import {
  getFlyDigiRuntimeCapability,
  getManualGearLabel,
  getManualLevelLabel,
  isManualGearAllowedForFlyDigi,
} from '../../lib/manualGearPresets';
import { apiService } from '../../services/api';
import FanCurveProfileSelect from '../FanCurveProfileSelect';
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
  supportsManualGears: boolean;
  configuredDeviceCurveKey: string;
}

type CurveProfile = { id: string; name: string; curve: types.FanCurvePoint[] };

const FAN_GEAR_VALUES = ['静音', '标准', '强劲', '超频'] as const;
const FAN_LEVEL_VALUES = ['低', '中', '高'] as const;

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
  supportsManualGears,
  configuredDeviceCurveKey,
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
  const [manualGearDraft, setManualGearDraft] = useState<string>(() => ((config as any).manualGear || '标准') as string);
  const [manualLevelDraft, setManualLevelDraft] = useState<string>(() => ((config as any).manualLevel || '中') as string);
  const [curveProfiles, setCurveProfiles] = useState<CurveProfile[]>([]);
  const [curveActiveProfileId, setCurveActiveProfileId] = useState('');
  const [curveProfileLoading, setCurveProfileLoading] = useState(false);
  const [temperatureHistoryEnabled, setTemperatureHistoryEnabled] = useState(false);

  const customSpeedInputValue = useMemo(
    () => clampFanSpeedToRange(customSpeedInput, overviewSpeedRange, defaultCustomSpeed) ?? defaultCustomSpeed,
    [customSpeedInput, defaultCustomSpeed, overviewSpeedRange],
  );
  const customSpeedMinLabel = `${formatFanSpeedValue(overviewSpeedRange.min)}${overviewSpeedLabel}`;
  const customSpeedMaxLabel = `${formatFanSpeedValue(overviewSpeedRange.max)}${overviewSpeedLabel}`;
  const activeCurveProfileId = curveActiveProfileId || (((config as any).activeFanCurveProfileId || '') as string);

  const fanGearOptions = useMemo(
    () => {
      const capability = getFlyDigiRuntimeCapability(fanData as any);
      return FAN_GEAR_VALUES.map((value) => ({
        value,
        label: getManualGearLabel(value),
        disabled: !isManualGearAllowedForFlyDigi(value, capability),
      }));
    },
    [fanData, locale],
  );
  const fanLevelOptions = useMemo(
    () => FAN_LEVEL_VALUES.map((value) => ({ value, label: getManualLevelLabel(value) })),
    [locale],
  );
  const sampleCountOptions = useMemo(
    () => SAMPLE_COUNT_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey) })),
    [locale, t],
  );
  const setLoading = useCallback((key: string, value: boolean) => {
    setLoadingStates((prev) => ({ ...prev, [key]: value }));
  }, []);

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

  const handleManualGearApply = useCallback(async () => {
    if (!supportsManualGears) {
      toast.error(t('controlPanel.fan.manualGearUnavailable'));
      return;
    }
    const gear = FAN_GEAR_VALUES.includes(manualGearDraft as any) ? manualGearDraft : '标准';
    const level = FAN_LEVEL_VALUES.includes(manualLevelDraft as any) ? manualLevelDraft : '中';
    const capability = getFlyDigiRuntimeCapability(fanData as any);
    if (!isManualGearAllowedForFlyDigi(gear, capability)) {
      const limit = capability?.maxGearLabel && capability?.maxRpm
        ? t('fanCurve.manualGear.runtimeLimit', { gear: getManualGearLabel(capability.maxGearLabel), rpm: capability.maxRpm })
        : '';
      toast.error(t('fanCurve.manualGear.runtimeUnavailable', { gear: getManualGearLabel(gear), limit }));
      return;
    }
    await runWithLoading('manualGear', async () => {
      try {
        const ok = await apiService.setManualGear(gear, level);
        if (!ok) {
          throw new Error(t('controlPanel.fan.manualGearUnavailable'));
        }
        const latest = types.AppConfig.createFrom(await apiService.getConfig());
        setManualGearDraft(((latest as any).manualGear || gear) as string);
        setManualLevelDraft(((latest as any).manualLevel || level) as string);
        onConfigChange(latest);
        toast.success(t('controlPanel.fan.manualGearApplied'));
      } catch (error) {
        toast.error(t('controlPanel.fan.manualGearApplyFailed', { error: getErrorMessage(error) }));
      }
    });
  }, [fanData, manualGearDraft, manualLevelDraft, onConfigChange, runWithLoading, supportsManualGears, t]);

  const handleCustomSpeedApply = useCallback(async (enabled: boolean, speed: unknown) => {
    await runWithLoading('customSpeed', async () => {
      const safeSpeed = clampFanSpeedToRange(speed, overviewSpeedRange, defaultCustomSpeed) ?? defaultCustomSpeed;
      try {
        await apiService.setCustomSpeed(enabled, safeSpeed);
        onConfigChange(types.AppConfig.createFrom({
          ...config,
          customSpeedEnabled: enabled,
          customSpeedRPM: safeSpeed,
          autoControl: enabled ? false : config.autoControl,
        }));
      } catch (error) {
        toast.error(getErrorMessage(error));
      }
    });
  }, [config, defaultCustomSpeed, onConfigChange, overviewSpeedRange, runWithLoading]);

  const handleCustomSpeedToggle = useCallback((enabled: boolean) => {
    if (enabled) setShowCustomSpeedWarning(true);
    else void handleCustomSpeedApply(false, customSpeedInput);
  }, [customSpeedInput, handleCustomSpeedApply]);

  const handleSampleCountChange = useCallback(async (count: string | number) => {
    try {
      await saveConfigPatch({ tempSampleCount: normalizeSampleCount(count) });
    } catch {
      /* noop */
    }
  }, [saveConfigPatch]);

  const handleTempSourceChange = useCallback(async (source: string) => {
    await runWithLoading('tempSource', async () => {
      try {
        await saveConfigPatch({ tempSource: normalizeTemperatureSource(source) });
      } catch {
        /* noop */
      }
    });
  }, [runWithLoading, saveConfigPatch]);

  const handleGpuDeviceChange = useCallback(async (deviceKey: string) => {
    await runWithLoading('gpuDevice', async () => {
      try {
        await saveConfigPatch({
          gpuDevice: deviceKey,
          gpuSensor: 'auto',
          gpuPowerSensor: 'auto',
        });
      } catch {
        /* noop */
      }
    });
  }, [runWithLoading, saveConfigPatch]);

  const handleTempSensorChange = useCallback(async (kind: 'cpu' | 'gpu', sensorKey: string) => {
    const loadingKey = kind === 'cpu' ? 'cpuSensor' : 'gpuSensor';
    await runWithLoading(loadingKey, async () => {
      try {
        await saveConfigPatch(kind === 'cpu' ? { cpuSensor: sensorKey } : { gpuSensor: sensorKey });
      } catch {
        /* noop */
      }
    });
  }, [runWithLoading, saveConfigPatch]);

  const handlePowerSensorChange = useCallback(async (kind: 'cpu' | 'gpu', sensorKey: string) => {
    const loadingKey = kind === 'cpu' ? 'cpuPowerSensor' : 'gpuPowerSensor';
    await runWithLoading(loadingKey, async () => {
      try {
        await saveConfigPatch(kind === 'cpu' ? { cpuPowerSensor: sensorKey } : { gpuPowerSensor: sensorKey });
      } catch {
        /* noop */
      }
    });
  }, [runWithLoading, saveConfigPatch]);

  const handleGpuReadModeChange = useCallback(async (mode: string) => {
    const normalizedMode = mode === 'always' ? 'always' : 'auto';
    await runWithLoading('gpuReadMode', async () => {
      try {
        await saveConfigPatch({
          gpuReadMode: normalizedMode,
          gpuLowPowerProtection: normalizedMode !== 'always',
        });
      } catch {
        /* noop */
      }
    });
  }, [runWithLoading, saveConfigPatch]);

  const handleTransientSpikeFilterChange = useCallback(async (enabled: boolean) => {
    await runWithLoading('transientSpikeFilter', async () => {
      try {
        await saveConfigPatch({
          smartControl: types.SmartControlConfig.createFrom({
            ...(config.smartControl || {}),
            filterTransientSpike: enabled,
          }),
        });
      } catch {
        /* noop */
      }
    });
  }, [config.smartControl, runWithLoading, saveConfigPatch]);

  const handleLearningToggle = useCallback(async (enabled: boolean) => {
    await runWithLoading('learning', async () => {
      try {
        await saveConfigPatch({
          smartControl: types.SmartControlConfig.createFrom({
            ...(config.smartControl || {}),
            learning: enabled,
          }),
        });
      } catch {
        /* noop */
      }
    });
  }, [config.smartControl, runWithLoading, saveConfigPatch]);

  const handleTemperatureHistoryChange = useCallback(async (enabled: boolean) => {
    await runWithLoading('temperatureHistory', async () => {
      try {
        await apiService.setTemperatureHistoryEnabled(enabled);
        setTemperatureHistoryEnabled(enabled);
      } catch {
        /* noop */
      }
    });
  }, [runWithLoading]);

  const loadCurveProfiles = useCallback(async () => {
    try {
      const payload = await apiService.getFanCurveProfiles();
      const profiles = Array.isArray(payload?.profiles) ? payload.profiles : [];
      setCurveProfiles(profiles);
      setCurveActiveProfileId(payload?.activeId || profiles[0]?.id || '');
    } catch {
      setCurveProfiles([]);
      setCurveActiveProfileId('');
    }
  }, []);

  const handleCurveProfileChange = useCallback(async (profileId: string) => {
    if (!profileId || profileId === activeCurveProfileId) return;
    try {
      setCurveProfileLoading(true);
      await apiService.setActiveFanCurveProfile(profileId);
      setCurveActiveProfileId(profileId);
      onConfigChange(types.AppConfig.createFrom(await apiService.getConfig()));
      await loadCurveProfiles();
      toast.success(t('controlPanel.fan.toasts.profileSwitched'));
    } catch (error) {
      toast.error(t('controlPanel.fan.toasts.profileSwitchFailed', { error: getErrorMessage(error) }));
    } finally {
      setCurveProfileLoading(false);
    }
  }, [activeCurveProfileId, loadCurveProfiles, onConfigChange, t]);

  useEffect(() => {
    let cancelled = false;
    const loadTelemetryState = async () => {
      try {
        const payload = await apiService.getTemperatureHistory();
        if (!cancelled) {
          setTemperatureHistoryEnabled(payload?.enabled !== false);
        }
      } catch {
        if (!cancelled) {
          setTemperatureHistoryEnabled(false);
        }
      }
    };

    loadTelemetryState();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => { loadCurveProfiles(); }, [loadCurveProfiles]);
  useEffect(() => { loadCurveProfiles(); }, [configuredDeviceCurveKey, loadCurveProfiles]);
  useEffect(() => {
    setCustomSpeedInput(String(clampFanSpeedToRange((config as any).customSpeedRPM, overviewSpeedRange, defaultCustomSpeed) ?? defaultCustomSpeed));
  }, [(config as any).customSpeedRPM, defaultCustomSpeed, overviewSpeedRange]);
  useEffect(() => {
    setManualGearDraft(((config as any).manualGear || '标准') as string);
    setManualLevelDraft(((config as any).manualLevel || '中') as string);
  }, [(config as any).manualGear, (config as any).manualLevel]);

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

        {supportsManualGears && (
          <SettingRow
            icon={<Settings className="h-4 w-4" />}
            title={t('controlPanel.fan.manualGearTitle')}
            description={isConnected ? t('controlPanel.fan.manualGearDescription') : t('controlPanel.fan.manualGearDisconnected')}
            disabled={!isConnected}
          >
            <div className="grid w-full grid-cols-1 gap-2 sm:w-[27rem] sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
              <Select
                value={manualGearDraft}
                onChange={(value: string | number) => setManualGearDraft(String(value))}
                options={fanGearOptions}
                size="sm"
                disabled={!isConnected || loadingStates.manualGear}
              />
              <Select
                value={manualLevelDraft}
                onChange={(value: string | number) => setManualLevelDraft(String(value))}
                options={fanLevelOptions}
                size="sm"
                disabled={!isConnected || loadingStates.manualGear}
              />
              <Button
                variant="primary"
                size="sm"
                onClick={handleManualGearApply}
                loading={loadingStates.manualGear}
                disabled={!isConnected}
                icon={<CheckCircle2 className="h-3.5 w-3.5" />}
                className="w-full sm:w-auto"
              >
                {t('common.actions.apply')}
              </Button>
            </div>
          </SettingRow>
        )}

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

        <SettingRow
          icon={<Sparkles className={clsx('h-4 w-4', (config.smartControl as any)?.learning ? 'text-amber-500' : 'text-muted-foreground')} />}
          title={t('controlPanel.fan.learningTitle')}
          description={t('controlPanel.fan.learningDescription')}
        >
          <ToggleSwitch
            enabled={!!(config.smartControl as any)?.learning}
            onChange={handleLearningToggle}
            loading={loadingStates.learning}
            size="sm"
            color="purple"
            srLabel={t('controlPanel.fan.learningAria')}
          />
        </SettingRow>

        <SettingRow
          icon={<BarChart3 className="h-4 w-4" />}
          title={t('controlPanel.fan.temperatureHistoryTitle')}
          description={t('controlPanel.fan.temperatureHistoryDescription')}
        >
          <ToggleSwitch
            enabled={temperatureHistoryEnabled}
            onChange={handleTemperatureHistoryChange}
            loading={loadingStates.temperatureHistory}
            size="sm"
            color="blue"
            srLabel={t('controlPanel.fan.temperatureHistoryAria')}
          />
        </SettingRow>

        <SettingRow
          icon={<Spline className="h-4 w-4" />}
          title={t('controlPanel.fan.curveProfileTitle')}
          description={t('controlPanel.fan.curveProfileDescription')}
        >
          <FanCurveProfileSelect
            profiles={curveProfiles}
            activeProfileId={activeCurveProfileId}
            onChange={handleCurveProfileChange}
            loading={curveProfileLoading}
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
                      min={overviewSpeedRange.min}
                      max={overviewSpeedRange.max}
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
                <div className="text-xl font-bold text-amber-600">{formatFanSpeedValue(customSpeedInputValue)}{overviewSpeedLabel}</div>
              </div>

              <div className="flex gap-3">
                <Button variant="secondary" onClick={() => setShowCustomSpeedWarning(false)} className="flex-1">
                  {t('common.actions.cancel')}
                </Button>
                <Button
                  variant="primary"
                  onClick={() => { setShowCustomSpeedWarning(false); void handleCustomSpeedApply(true, customSpeedInput); }}
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
