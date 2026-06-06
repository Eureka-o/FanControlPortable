export const FAN_SPEED_UNIT_PERCENT = 'percent';
export const FAN_SPEED_UNIT_RPM = 'rpm';
export const DEVICE_TRANSPORT_WIFI = 'wifi';
export const DEVICE_TRANSPORT_BLE = 'ble';
export const DEVICE_TRANSPORT_HID = 'hid';
export const DEFAULT_RPM_SPEED_MAX = 4000;

export type FanSpeedUnit = typeof FAN_SPEED_UNIT_PERCENT | typeof FAN_SPEED_UNIT_RPM;

export type DeviceSpeedRange = {
  min?: number;
  max?: number;
  step?: number;
  tickScale?: number;
};

type SpeedCarrier = {
  transport?: string;
  speedUnit?: string;
} | null | undefined;

type ConfigCarrier = {
  activeDeviceProfileId?: string;
  activeDeviceProfileIdsByTransport?: Record<string, string>;
  deviceProfiles?: DeviceProfileCarrier[];
  deviceTransport?: string;
} | null | undefined;

type DeviceProfileCarrier = {
  id?: string;
  transport?: string;
  speedUnit?: string;
  speedRange?: DeviceSpeedRange;
  capabilities?: {
    speedUnit?: string;
    speedRange?: DeviceSpeedRange;
  };
};

export function normalizeFanSpeedUnit(unit?: string): FanSpeedUnit {
  return unit === FAN_SPEED_UNIT_RPM ? FAN_SPEED_UNIT_RPM : FAN_SPEED_UNIT_PERCENT;
}

function normalizeDeviceTransport(transport?: string) {
  return (transport || '').trim().toLowerCase();
}

function configuredDeviceProfiles(config?: ConfigCarrier): DeviceProfileCarrier[] {
  return Array.isArray(config?.deviceProfiles) ? config.deviceProfiles : [];
}

export function getActiveDeviceProfile(config?: ConfigCarrier): DeviceProfileCarrier | undefined {
  const profiles = configuredDeviceProfiles(config);
  if (profiles.length === 0) {
    return undefined;
  }

  const transport = normalizeDeviceTransport(config?.deviceTransport);
  const activeTransportId = transport ? (config?.activeDeviceProfileIdsByTransport || {})[transport] : '';
  if (activeTransportId) {
    const activeForTransport = profiles.find((profile) => profile.id === activeTransportId && normalizeDeviceTransport(profile.transport) === transport);
    if (activeForTransport) {
      return activeForTransport;
    }
  }

  const activeId = (config?.activeDeviceProfileId || '').trim();
  if (activeId) {
    const active = profiles.find((profile) => profile.id === activeId);
    if (active && (!transport || normalizeDeviceTransport(active.transport) === transport)) {
      return active;
    }
  }

  if (transport) {
    const firstForTransport = profiles.find((profile) => normalizeDeviceTransport(profile.transport) === transport);
    if (firstForTransport) {
      return firstForTransport;
    }
    return undefined;
  }

  if (activeId) {
    const active = profiles.find((profile) => profile.id === activeId);
    if (active) {
      return active;
    }
  }

  return profiles[0];
}

export function getConfiguredFanSpeedUnit(config?: ConfigCarrier): FanSpeedUnit {
  const activeProfile = getActiveDeviceProfile(config);
  if (activeProfile?.speedUnit) {
    return normalizeFanSpeedUnit(activeProfile.speedUnit);
  }
  if (activeProfile?.capabilities?.speedUnit) {
    return normalizeFanSpeedUnit(activeProfile.capabilities.speedUnit);
  }
  if (normalizeDeviceTransport(config?.deviceTransport) === DEVICE_TRANSPORT_HID) {
    return FAN_SPEED_UNIT_RPM;
  }
  return FAN_SPEED_UNIT_PERCENT;
}

export function getConfiguredDeviceTransport(config?: ConfigCarrier) {
  const activeProfile = getActiveDeviceProfile(config);
  return normalizeDeviceTransport(activeProfile?.transport || config?.deviceTransport);
}

export function isFanDataForConfiguredDevice(fanData?: SpeedCarrier, config?: ConfigCarrier) {
  const fanDataTransport = normalizeDeviceTransport(fanData?.transport);
  const configuredTransport = getConfiguredDeviceTransport(config);
  if (fanDataTransport && configuredTransport && fanDataTransport !== configuredTransport) {
    return false;
  }

  const fanDataUnit = fanData?.speedUnit ? normalizeFanSpeedUnit(fanData.speedUnit) : '';
  const configuredProfile = getActiveDeviceProfile(config);
  if (fanDataUnit && configuredProfile) {
    return fanDataUnit === getConfiguredFanSpeedUnit(config);
  }

  return true;
}

export function getFanSpeedUnit(fanData?: SpeedCarrier, config?: ConfigCarrier): FanSpeedUnit {
  const configuredProfile = getActiveDeviceProfile(config);
  if (configuredProfile?.speedUnit || configuredProfile?.capabilities?.speedUnit) {
    return getConfiguredFanSpeedUnit(config);
  }
  if (fanData?.speedUnit && isFanDataForConfiguredDevice(fanData, config)) {
    return normalizeFanSpeedUnit(fanData.speedUnit);
  }
  return getConfiguredFanSpeedUnit(config);
}

export function getFanSpeedRange(config?: ConfigCarrier, unit: FanSpeedUnit = getConfiguredFanSpeedUnit(config)): Required<DeviceSpeedRange> {
  const activeProfile = getActiveDeviceProfile(config);
  const range = activeProfile?.speedRange || activeProfile?.capabilities?.speedRange;
  const fallback = unit === FAN_SPEED_UNIT_RPM
    ? { min: 0, max: DEFAULT_RPM_SPEED_MAX, step: 1, tickScale: 1 }
    : { min: 0, max: 100, step: 1, tickScale: 10 };

  const min = typeof range?.min === 'number' && Number.isFinite(range.min) ? range.min : fallback.min;
  const max = typeof range?.max === 'number' && Number.isFinite(range.max) && range.max > min ? range.max : fallback.max;
  const step = typeof range?.step === 'number' && Number.isFinite(range.step) && range.step > 0 ? range.step : fallback.step;
  const tickScale = typeof range?.tickScale === 'number' && Number.isFinite(range.tickScale) && range.tickScale > 0 ? range.tickScale : fallback.tickScale;
  return {
    min,
    max,
    step,
    tickScale,
  };
}

export function getFanSpeedTicks(min: number, max: number) {
  if (max <= 100) {
    return [0, 20, 40, 60, 80, 100].filter((tick) => tick >= min && tick <= max);
  }
  const step = max <= 3000 ? 500 : 1000;
  const start = Math.ceil(min / step) * step;
  const ticks: number[] = [];
  if (min === 0) {
    ticks.push(0);
  }
  for (let tick = start; tick <= max; tick += step) {
    if (!ticks.includes(tick)) {
      ticks.push(tick);
    }
  }
  if (!ticks.includes(max)) {
    ticks.push(max);
  }
  return ticks;
}

export function sanitizeFanSpeed(value: unknown, unit: FanSpeedUnit = FAN_SPEED_UNIT_PERCENT): number | undefined {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return undefined;
  }
  const rounded = Math.round(value);
  if (unit === FAN_SPEED_UNIT_PERCENT) {
    return rounded >= 0 && rounded <= 100 ? rounded : undefined;
  }
  return rounded >= 0 ? rounded : undefined;
}

export function clampFanSpeedToRange(
  value: unknown,
  range: Required<DeviceSpeedRange>,
  fallback?: number,
): number | undefined {
  const numeric = typeof value === 'string' && value.trim() !== '' ? Number(value) : value;
  const resolved = typeof numeric === 'number' && Number.isFinite(numeric) ? numeric : fallback;
  if (typeof resolved !== 'number' || !Number.isFinite(resolved)) {
    return undefined;
  }
  return Math.max(range.min, Math.min(range.max, Math.round(resolved)));
}

export function formatFanSpeedValue(value: unknown) {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return '--';
  }
  const rounded = Math.round(value * 10) / 10;
  return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1);
}

export function formatFanSpeedWithUnit(value: unknown, unit: FanSpeedUnit) {
  return `${formatFanSpeedValue(value)}${fanSpeedUnitLabel(unit)}`;
}

function firstFiniteNumber(...values: unknown[]) {
  for (const value of values) {
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value;
    }
    if (typeof value === 'string' && value.trim() !== '') {
      const numeric = Number(value);
      if (Number.isFinite(numeric)) {
        return numeric;
      }
    }
  }
  return undefined;
}

export function readCurrentFanSpeed(
  fanData: unknown,
  unit: FanSpeedUnit = FAN_SPEED_UNIT_PERCENT,
  config?: ConfigCarrier,
): number | undefined {
  const data = (fanData ?? {}) as Record<string, unknown>;
  if (config && !isFanDataForConfiguredDevice(data as SpeedCarrier, config)) {
    return undefined;
  }
  return sanitizeFanSpeed(
    firstFiniteNumber(data.currentRpm, data.currentRPM, data.CurrentRPM, data.speed, data.fanSpeed),
    unit,
  );
}

export function readTargetFanSpeed(
  fanData: unknown,
  unit: FanSpeedUnit = FAN_SPEED_UNIT_PERCENT,
  config?: ConfigCarrier,
): number | undefined {
  const data = (fanData ?? {}) as Record<string, unknown>;
  if (config && !isFanDataForConfiguredDevice(data as SpeedCarrier, config)) {
    return undefined;
  }
  return sanitizeFanSpeed(
    firstFiniteNumber(data.targetRpm, data.targetRPM, data.TargetRPM, data.wifiTargetSpeed, data.targetSpeed),
    unit,
  );
}

export function fanSpeedUnitLabel(unit: FanSpeedUnit) {
  return unit === FAN_SPEED_UNIT_PERCENT ? '%' : 'RPM';
}
