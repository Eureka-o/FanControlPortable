'use client';

import { useAppStore } from '../store/app-store';

export function useTemperatureHistory() {
  const sessionHistoryPoints = useAppStore((state) => state.sessionHistoryPoints);
  const coreHistoryPoints = useAppStore((state) => state.temperatureHistoryPoints);
  const enabled = useAppStore((state) => state.temperatureHistoryEnabled);
  const loading = useAppStore((state) => state.temperatureHistoryLoading);
  const saving = useAppStore((state) => state.temperatureHistorySaving);
  const loadTemperatureHistory = useAppStore((state) => state.loadTemperatureHistory);
  const setEnabled = useAppStore((state) => state.setTemperatureHistoryEnabled);

  return {
    points: enabled ? coreHistoryPoints : sessionHistoryPoints,
    enabled,
    loading,
    saving,
    setEnabled,
    source: enabled ? 'core' as const : 'session' as const,
    reload: () => loadTemperatureHistory(true),
  };
}
