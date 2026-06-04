export const FAN_SPEED_UNIT_PERCENT = 'percent';
export const FAN_SPEED_UNIT_RPM = 'rpm';
export const DEVICE_TRANSPORT_WIFI = 'wifi';
export const DEVICE_TRANSPORT_BLE = 'ble';

type SpeedCarrier = {
  transport?: string;
  speedUnit?: string;
} | null | undefined;

type ConfigCarrier = {
  deviceTransport?: string;
} | null | undefined;

export type FanSpeedUnit = typeof FAN_SPEED_UNIT_PERCENT | typeof FAN_SPEED_UNIT_RPM;

function isPercentTransport(transport?: string) {
  return transport === DEVICE_TRANSPORT_WIFI || transport === DEVICE_TRANSPORT_BLE;
}

export function getFanSpeedUnit(fanData?: SpeedCarrier, config?: ConfigCarrier): FanSpeedUnit {
  if (fanData?.speedUnit === FAN_SPEED_UNIT_RPM && !isPercentTransport(fanData?.transport) && !isPercentTransport(config?.deviceTransport)) {
    return FAN_SPEED_UNIT_RPM;
  }
  return FAN_SPEED_UNIT_PERCENT;
}

export function sanitizeFanSpeed(value: unknown, unit: FanSpeedUnit = FAN_SPEED_UNIT_PERCENT): number | undefined {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return undefined;
  }
  const rounded = Math.round(value);
  if (unit === FAN_SPEED_UNIT_PERCENT) {
    return rounded >= 0 && rounded <= 100 ? rounded : undefined;
  }
  return rounded > 0 ? rounded : undefined;
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

export function readCurrentFanSpeed(fanData: unknown, unit: FanSpeedUnit = FAN_SPEED_UNIT_PERCENT): number | undefined {
  const data = (fanData ?? {}) as Record<string, unknown>;
  return sanitizeFanSpeed(
    firstFiniteNumber(data.currentRpm, data.currentRPM, data.CurrentRPM, data.speed, data.fanSpeed),
    unit,
  );
}

export function readTargetFanSpeed(fanData: unknown, unit: FanSpeedUnit = FAN_SPEED_UNIT_PERCENT): number | undefined {
  const data = (fanData ?? {}) as Record<string, unknown>;
  return sanitizeFanSpeed(
    firstFiniteNumber(data.targetRpm, data.targetRPM, data.TargetRPM, data.wifiTargetSpeed, data.targetSpeed),
    unit,
  );
}

export function fanSpeedUnitLabel(unit: FanSpeedUnit) {
  return unit === FAN_SPEED_UNIT_PERCENT ? '%' : 'RPM';
}
