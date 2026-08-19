export interface TemperatureHistoryPoint {
  timestamp: number;
  cpuTemp: number;
  gpuTemp: number;
  fanRpm: number;
  cpuPowerWatts?: number;
  gpuPowerWatts?: number;
}

export type HistorySeriesKey = 'cpu' | 'gpu' | 'fan' | 'cpuPower' | 'gpuPower' | 'totalPower';

export const CORE_HISTORY_LIMIT = 720;
export const SESSION_HISTORY_LIMIT = 60;
export const CORE_HISTORY_RETENTION_MS = 60 * 60 * 1000;
export const SESSION_HISTORY_RETENTION_MS = 5 * 60 * 1000;
export const HISTORY_SAMPLE_INTERVAL_MS = 5 * 1000;
export const HISTORY_LIMIT = CORE_HISTORY_LIMIT;
export const DEFAULT_HISTORY_RETENTION_HOURS = 1;
export const MAX_HISTORY_RETENTION_HOURS = 24;
export const HISTORY_RETENTION_HOUR_OPTIONS = [1, 2, 3, 6, 12, 24] as const;
export const HOME_CHART_WINDOW_MS = 60 * 60 * 1000;

export const clampHistoryRetentionHours = (hours: number | null | undefined) => {
  const numeric = Math.round(Number(hours || 0));
  if (!Number.isFinite(numeric) || numeric < DEFAULT_HISTORY_RETENTION_HOURS) return DEFAULT_HISTORY_RETENTION_HOURS;
  return Math.min(MAX_HISTORY_RETENTION_HOURS, numeric);
};

export const historyRetentionLimit = (hours: number) => (
  clampHistoryRetentionHours(hours) * Math.round((60 * 60 * 1000) / HISTORY_SAMPLE_INTERVAL_MS)
);

export const clipHistoryToRecentWindow = (
  points: TemperatureHistoryPoint[],
  windowMs = HOME_CHART_WINDOW_MS,
) => {
  if (points.length === 0 || windowMs <= 0) return points;
  const cutoff = points[points.length - 1].timestamp - windowMs;
  if (points[0].timestamp >= cutoff) return points;

  let low = 0;
  let high = points.length - 1;
  while (low < high) {
    const mid = (low + high) >> 1;
    if (points[mid].timestamp < cutoff) low = mid + 1;
    else high = mid;
  }
  return points.slice(low);
};

export const downsampleHistoryPoints = <T extends { timestamp: number }>(points: T[], maxPoints: number) => {
  if (maxPoints <= 0 || points.length <= maxPoints) return points;
  const stride = Math.ceil(points.length / maxPoints);
  const sampled: T[] = [];
  for (let index = 0; index < points.length; index += stride) sampled.push(points[index]);
  const last = points[points.length - 1];
  if (sampled[sampled.length - 1] !== last) sampled.push(last);
  return sampled;
};

export const normalizeHistoryTimestamp = (timestamp: number | null | undefined) => {
  const numeric = Number(timestamp || 0);
  if (numeric <= 0) return 0;
  if (numeric < 1_000_000_000_000) {
    return numeric * 1000;
  }
  return numeric;
};

const normalizePositiveNumber = (value: number | null | undefined) => {
  const numeric = Number(value || 0);
  return Number.isFinite(numeric) && numeric > 0 ? numeric : 0;
};

export const historyPointEquals = (left: TemperatureHistoryPoint | null | undefined, right: TemperatureHistoryPoint | null | undefined) => {
  if (!left || !right) return false;
  return left.timestamp === right.timestamp &&
    left.cpuTemp === right.cpuTemp &&
    left.gpuTemp === right.gpuTemp &&
    left.fanRpm === right.fanRpm &&
    (left.cpuPowerWatts || 0) === (right.cpuPowerWatts || 0) &&
    (left.gpuPowerWatts || 0) === (right.gpuPowerWatts || 0);
};

export const normalizeHistoryPoint = (point: Partial<TemperatureHistoryPoint> | null | undefined): TemperatureHistoryPoint | null => {
  if (!point) return null;

  const timestamp = normalizeHistoryTimestamp(Number(point.timestamp || 0));
  const cpuTemp = normalizePositiveNumber(point.cpuTemp);
  const gpuTemp = normalizePositiveNumber(point.gpuTemp);
  const fanRpm = normalizePositiveNumber(point.fanRpm);
  const cpuPowerWatts = normalizePositiveNumber(point.cpuPowerWatts);
  const gpuPowerWatts = normalizePositiveNumber(point.gpuPowerWatts);

  if (timestamp <= 0 || (cpuTemp <= 0 && gpuTemp <= 0 && fanRpm <= 0 && cpuPowerWatts <= 0 && gpuPowerWatts <= 0)) {
    return null;
  }

  return {
    timestamp,
    cpuTemp,
    gpuTemp,
    fanRpm,
    ...(cpuPowerWatts > 0 ? { cpuPowerWatts } : {}),
    ...(gpuPowerWatts > 0 ? { gpuPowerWatts } : {}),
  };
};

export const trimHistoryPoints = (
  points: TemperatureHistoryPoint[] | undefined,
  retentionMs = CORE_HISTORY_RETENTION_MS,
  limit = HISTORY_LIMIT,
) => {
  if (!Array.isArray(points)) return [];

  const normalized = points
    .map((point) => normalizeHistoryPoint(point))
    .filter((point): point is TemperatureHistoryPoint => !!point)
    .sort((a, b) => a.timestamp - b.timestamp);

  if (normalized.length === 0) {
    return [];
  }

  const newestTimestamp = normalized[normalized.length - 1]?.timestamp || 0;
  const cutoffTimestamp = newestTimestamp > 0 ? Math.max(0, newestTimestamp - retentionMs) : 0;

  return normalized
    .filter((point) => point.timestamp >= cutoffTimestamp)
    .slice(-limit);
};

export const normalizeHistoryPoints = (points: TemperatureHistoryPoint[] | undefined) => {
  return trimHistoryPoints(points, CORE_HISTORY_RETENTION_MS, CORE_HISTORY_LIMIT);
};

export const appendHistoryPoint = (
  points: TemperatureHistoryPoint[],
  point: TemperatureHistoryPoint | null,
  options?: { retentionMs?: number; limit?: number },
) => {
  const normalized = normalizeHistoryPoint(point);
  if (!normalized) return points;

  const next = [...points];
  const last = next[next.length - 1];

  if (last && last.timestamp === normalized.timestamp) {
    if (historyPointEquals(last, normalized)) {
      return points;
    }
    next[next.length - 1] = normalized;
  } else if (last && normalized.timestamp < last.timestamp) {
    return normalizeHistoryPoints([...next, normalized]);
  } else {
    next.push(normalized);
  }

  return trimHistoryPoints(next, options?.retentionMs ?? CORE_HISTORY_RETENTION_MS, options?.limit ?? HISTORY_LIMIT);
};

export const appendSampledHistoryPoint = (
  points: TemperatureHistoryPoint[],
  point: TemperatureHistoryPoint | null,
  options?: { retentionMs?: number; limit?: number; minIntervalMs?: number },
) => {
  const normalized = normalizeHistoryPoint(point);
  if (!normalized) return points;

  const last = points[points.length - 1];
  const minIntervalMs = options?.minIntervalMs ?? HISTORY_SAMPLE_INTERVAL_MS;
  if (last && normalized.timestamp-last.timestamp < minIntervalMs) {
    return points;
  }

  return appendHistoryPoint(points, normalized, options);
};

export const createLiveHistoryPoint = (
  payload: { updateTime?: number; cpuTemp?: number; gpuTemp?: number; cpuPowerWatts?: number; gpuPowerWatts?: number } | null | undefined,
  fanRpm = 0,
) => {
  if (!payload) return null;

  return normalizeHistoryPoint({
    timestamp: normalizeHistoryTimestamp(payload.updateTime ?? 0) || Date.now(),
    cpuTemp: Number(payload.cpuTemp || 0),
    gpuTemp: Number(payload.gpuTemp || 0),
    fanRpm: Number(fanRpm || 0),
    cpuPowerWatts: Number(payload.cpuPowerWatts || 0),
    gpuPowerWatts: Number(payload.gpuPowerWatts || 0),
  });
};
