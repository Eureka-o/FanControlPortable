'use client';

import { useMemo } from 'react';
import { types } from '../../../wailsjs/go/models';
import { selectTemperatureHistoryPoints, useAppStore } from '../store/app-store';
import { applyPowerSpoofToHistoryPoints } from '../lib/power-spoof';

export function useTemperatureHistory() {
  const points = useAppStore(selectTemperatureHistoryPoints);
  const enabled = useAppStore((state) => state.temperatureHistoryEnabled);
  const retentionHours = useAppStore((state) => state.temperatureHistoryRetentionHours);
  const loading = useAppStore((state) => state.temperatureHistoryLoading);
  const saving = useAppStore((state) => state.temperatureHistorySaving);
  const loadTemperatureHistory = useAppStore((state) => state.loadTemperatureHistory);
  const setEnabled = useAppStore((state) => state.setTemperatureHistoryEnabled);
  const setRetentionHours = useAppStore((state) => state.setTemperatureHistoryRetentionHours);
  const config = useAppStore((state) => state.config);
  const displayPoints = useMemo(
    () => applyPowerSpoofToHistoryPoints(points, config || ({} as types.AppConfig)),
    [config, points],
  );

  return {
    points: displayPoints,
    enabled,
    retentionHours,
    loading,
    saving,
    setEnabled,
    setRetentionHours,
    source: enabled ? 'core' as const : 'session' as const,
    reload: () => loadTemperatureHistory(true),
  };
}
