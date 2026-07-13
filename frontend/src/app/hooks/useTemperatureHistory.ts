'use client';

import { startTransition, useCallback, useEffect, useRef, useState } from 'react';
import { apiService } from '../services/api';
import {
  appendSampledHistoryPoint,
  normalizeHistoryPoints,
  type TemperatureHistoryPoint,
} from '../lib/temperature-history';
import { useAppStore } from '../store/app-store';

export function useTemperatureHistory() {
  const sessionHistoryPoints = useAppStore((state) => state.sessionHistoryPoints);
  const [points, setPoints] = useState<TemperatureHistoryPoint[]>([]);
  const [enabled, setEnabledState] = useState(false);
  const [saving, setSaving] = useState(false);
  const enabledRef = useRef(enabled);
  const sessionPointsRef = useRef(sessionHistoryPoints);

  useEffect(() => {
    enabledRef.current = enabled;
  }, [enabled]);

  useEffect(() => {
    sessionPointsRef.current = sessionHistoryPoints;
    if (!enabledRef.current) {
      setPoints(sessionHistoryPoints);
    }
  }, [sessionHistoryPoints]);

  const loadSnapshot = useCallback(async (activeGuard?: { active: boolean }) => {
    try {
      const payload = await apiService.getTemperatureHistory();
      if (activeGuard && !activeGuard.active) {
        return;
      }

      const nextEnabled = payload?.enabled !== false;
      setEnabledState(nextEnabled);
      startTransition(() => {
        setPoints(nextEnabled
          ? normalizeHistoryPoints((payload?.points || []) as TemperatureHistoryPoint[])
          : sessionPointsRef.current);
      });
    } catch {
      if (activeGuard && !activeGuard.active) {
        return;
      }

      setEnabledState(false);
      startTransition(() => setPoints(sessionPointsRef.current));
    }
  }, []);

  useEffect(() => {
    const activeGuard = { active: true };
    void loadSnapshot(activeGuard);
    return () => {
      activeGuard.active = false;
    };
  }, [loadSnapshot]);

  useEffect(() => {
    return apiService.onTemperatureHistoryUpdate((point) => {
      if (!enabledRef.current) {
        return;
      }
      setPoints((prev) => appendSampledHistoryPoint(prev, point as TemperatureHistoryPoint));
    });
  }, []);

  const setEnabled = useCallback(async (nextEnabled: boolean) => {
    setSaving(true);
    try {
      await apiService.setTemperatureHistoryEnabled(nextEnabled);
      await loadSnapshot();
    } catch (error) {
      console.error('设置温度历史失败:', error);
    } finally {
      setSaving(false);
    }
  }, [loadSnapshot]);

  return {
    points,
    enabled,
    saving,
    setEnabled,
    source: enabled ? 'core' as const : 'session' as const,
    reload: loadSnapshot,
  };
}
