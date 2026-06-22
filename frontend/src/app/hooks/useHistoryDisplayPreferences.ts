'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import type { HistorySeriesKey } from '../lib/temperature-history';

export const HISTORY_SERIES_ORDER: HistorySeriesKey[] = ['cpu', 'gpu', 'fan', 'cpuPower', 'gpuPower'];

export type HistorySeriesVisibility = Record<HistorySeriesKey, boolean>;

export interface HistoryDisplayPreferences {
  visible: HistorySeriesVisibility;
  order: HistorySeriesKey[];
}

const STORAGE_KEY = 'fancontrol.historyDisplayPreferences.v1';
const CHANGE_EVENT = 'fancontrol-history-display-preferences-change';

const DEFAULT_VISIBILITY: HistorySeriesVisibility = {
  cpu: true,
  gpu: true,
  fan: true,
  cpuPower: true,
  gpuPower: true,
};

const isHistorySeriesKey = (value: unknown): value is HistorySeriesKey => (
  typeof value === 'string' && HISTORY_SERIES_ORDER.includes(value as HistorySeriesKey)
);

export function normalizeHistoryDisplayPreferences(input?: Partial<HistoryDisplayPreferences> | null): HistoryDisplayPreferences {
  const visible = { ...DEFAULT_VISIBILITY };
  if (input?.visible && typeof input.visible === 'object') {
    for (const key of HISTORY_SERIES_ORDER) {
      if (typeof input.visible[key] === 'boolean') {
        visible[key] = input.visible[key];
      }
    }
  }

  const order = Array.isArray(input?.order)
    ? input.order.filter(isHistorySeriesKey)
    : [];
  const uniqueOrder = Array.from(new Set(order));
  for (const key of HISTORY_SERIES_ORDER) {
    if (!uniqueOrder.includes(key)) {
      uniqueOrder.push(key);
    }
  }

  return { visible, order: uniqueOrder };
}

function readHistoryDisplayPreferences() {
  if (typeof window === 'undefined') {
    return normalizeHistoryDisplayPreferences();
  }

  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    return normalizeHistoryDisplayPreferences(raw ? JSON.parse(raw) : null);
  } catch {
    return normalizeHistoryDisplayPreferences();
  }
}

function writeHistoryDisplayPreferences(preferences: HistoryDisplayPreferences) {
  if (typeof window === 'undefined') {
    return;
  }

  const normalized = normalizeHistoryDisplayPreferences(preferences);
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(normalized));
  window.dispatchEvent(new CustomEvent(CHANGE_EVENT, { detail: normalized }));
}

export function useHistoryDisplayPreferences() {
  const [preferences, setPreferences] = useState<HistoryDisplayPreferences>(() => readHistoryDisplayPreferences());

  useEffect(() => {
    const syncFromStorage = () => setPreferences(readHistoryDisplayPreferences());
    const syncFromEvent = (event: Event) => {
      const detail = (event as CustomEvent<HistoryDisplayPreferences>).detail;
      setPreferences(normalizeHistoryDisplayPreferences(detail));
    };

    window.addEventListener('storage', syncFromStorage);
    window.addEventListener(CHANGE_EVENT, syncFromEvent);
    return () => {
      window.removeEventListener('storage', syncFromStorage);
      window.removeEventListener(CHANGE_EVENT, syncFromEvent);
    };
  }, []);

  const updatePreferences = useCallback((updater: (current: HistoryDisplayPreferences) => HistoryDisplayPreferences) => {
    setPreferences((current) => {
      const next = normalizeHistoryDisplayPreferences(updater(current));
      writeHistoryDisplayPreferences(next);
      return next;
    });
  }, []);

  const setSeriesVisible = useCallback((series: HistorySeriesKey, visible: boolean) => {
    updatePreferences((current) => ({
      ...current,
      visible: {
        ...current.visible,
        [series]: visible,
      },
    }));
  }, [updatePreferences]);

  const toggleSeriesVisible = useCallback((series: HistorySeriesKey) => {
    updatePreferences((current) => ({
      ...current,
      visible: {
        ...current.visible,
        [series]: !current.visible[series],
      },
    }));
  }, [updatePreferences]);

  const moveSeries = useCallback((series: HistorySeriesKey, direction: -1 | 1) => {
    updatePreferences((current) => {
      const order = [...current.order];
      const index = order.indexOf(series);
      const nextIndex = index + direction;
      if (index < 0 || nextIndex < 0 || nextIndex >= order.length) {
        return current;
      }
      [order[index], order[nextIndex]] = [order[nextIndex], order[index]];
      return { ...current, order };
    });
  }, [updatePreferences]);

  const reorderSeries = useCallback((dragged: HistorySeriesKey, target: HistorySeriesKey, placement: 'before' | 'after' = 'before') => {
    updatePreferences((current) => {
      if (dragged === target) {
        return current;
      }

      const order = current.order.filter((key) => key !== dragged);
      const targetIndex = order.indexOf(target);
      if (targetIndex < 0) {
        return current;
      }
      order.splice(placement === 'after' ? targetIndex + 1 : targetIndex, 0, dragged);
      return { ...current, order };
    });
  }, [updatePreferences]);

  const resetPreferences = useCallback(() => {
    updatePreferences(() => normalizeHistoryDisplayPreferences());
  }, [updatePreferences]);

  return useMemo(() => ({
    preferences,
    orderedSeries: preferences.order,
    seriesVisibility: preferences.visible,
    setSeriesVisible,
    toggleSeriesVisible,
    moveSeries,
    reorderSeries,
    resetPreferences,
  }), [moveSeries, preferences, reorderSeries, resetPreferences, setSeriesVisible, toggleSeriesVisible]);
}
