'use client';

import { useMemo } from 'react';
import { types } from '../../../wailsjs/go/models';
import { useAppStore } from '../store/app-store';
import { applyPowerSpoofToHistoryPoints } from '../lib/power-spoof';

export function useTemperatureHistory() {
  const sessionHistoryPoints = useAppStore((state) => state.sessionHistoryPoints);
  const coreHistoryPoints = useAppStore((state) => state.temperatureHistoryPoints);
  const enabled = useAppStore((state) => state.temperatureHistoryEnabled);
  const loading = useAppStore((state) => state.temperatureHistoryLoading);
  const saving = useAppStore((state) => state.temperatureHistorySaving);
  const loadTemperatureHistory = useAppStore((state) => state.loadTemperatureHistory);
  const setEnabled = useAppStore((state) => state.setTemperatureHistoryEnabled);
  const config = useAppStore((state) => state.config);
  const points = enabled ? coreHistoryPoints : sessionHistoryPoints;
  const displayPoints = useMemo(
    () => applyPowerSpoofToHistoryPoints(points, config || ({} as types.AppConfig)),
    [config, points],
  );

  return {
    points: displayPoints,
    enabled,
    loading,
    saving,
    setEnabled,
    source: enabled ? 'core' as const : 'session' as const,
    reload: () => loadTemperatureHistory(true),
  };
}
