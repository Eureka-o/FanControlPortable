'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { types } from '../../../../wailsjs/go/models';

type ConfigChangeHandler = (config: types.AppConfig) => void | Promise<void>;

export function normalizeTemperatureSource(source: string) {
  return source === 'cpu' || source === 'gpu' || source === 'max' ? source : 'max';
}

export function normalizeSampleCount(count: string | number) {
  const parsed = typeof count === 'number' ? count : Number(count);
  return [1, 2, 3, 5, 10].includes(parsed) ? parsed : 1;
}

export function useSharedConfigPatch(config: types.AppConfig | null | undefined, onConfigChange: ConfigChangeHandler) {
  const latestConfigRef = useRef<types.AppConfig | null>(config ?? null);
  const saveConfigChainRef = useRef<Promise<void>>(Promise.resolve());

  useEffect(() => {
    if (config) latestConfigRef.current = config;
  }, [config]);

  return useCallback(async (patch: Record<string, unknown>) => {
    let nextCfg = latestConfigRef.current;
    const saveTask = saveConfigChainRef.current
      .catch(() => undefined)
      .then(async () => {
        const baseCfg = latestConfigRef.current || config;
        if (!baseCfg) return;
        nextCfg = types.AppConfig.createFrom({ ...baseCfg, ...patch });
        latestConfigRef.current = nextCfg;
        await onConfigChange(nextCfg);
      });
    saveConfigChainRef.current = saveTask;
    await saveTask;
    return nextCfg;
  }, [config, onConfigChange]);
}

export function useTemperatureBaselineSettings(config: types.AppConfig | null | undefined, onConfigChange: ConfigChangeHandler) {
  const [loadingStates, setLoadingStates] = useState<Record<string, boolean>>({});
  const saveConfigPatch = useSharedConfigPatch(config, onConfigChange);

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

  return {
    loadingStates,
    runWithLoading,
    saveConfigPatch,
    handleTempSourceChange,
    handleGpuReadModeChange,
    handleGpuDeviceChange,
    handleTempSensorChange,
    handlePowerSensorChange,
  };
}
