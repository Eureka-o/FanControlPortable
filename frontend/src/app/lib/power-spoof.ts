import { types } from '../../../wailsjs/go/models';

function spoofPower(value: number | undefined, percent: number, offset: number) {
  if (!Number.isFinite(value) || Number(value) <= 0) return value;
  const next = Number(value) * percent / 100 + offset;
  return Number.isFinite(next) ? Math.max(0, next) : value;
}

function spoofPowerSensors(sensors: types.PowerSensor[] | undefined, percent: number, offset: number) {
  return Array.isArray(sensors)
    ? sensors.map((sensor) => ({ ...sensor, value: spoofPower(sensor.value, percent, offset) ?? sensor.value }))
    : sensors;
}

function powerSpoofPair(config: types.AppConfig, kind: 'cpu' | 'gpu') {
  const source = config as any;
  const percentValue = kind === 'cpu' ? source.cpuPowerSpoofPercent : source.gpuPowerSpoofPercent;
  const offsetValue = kind === 'cpu' ? source.cpuPowerSpoofOffsetWatts : source.gpuPowerSpoofOffsetWatts;
  return {
    percent: Number.isFinite(Number(percentValue)) ? Number(percentValue) : 100,
    offset: Number.isFinite(Number(offsetValue)) ? Number(offsetValue) : 0,
  };
}

export function applyPowerSpoofToTemperature(
  temperature: types.TemperatureData | null,
  config: types.AppConfig,
) {
  if (!temperature || !(config as any).powerSpoofEnabled) return temperature;
  const cpu = powerSpoofPair(config, 'cpu');
  const gpu = powerSpoofPair(config, 'gpu');
  return {
    ...temperature,
    cpuPowerWatts: spoofPower(temperature.cpuPowerWatts, cpu.percent, cpu.offset),
    gpuPowerWatts: spoofPower(temperature.gpuPowerWatts, gpu.percent, gpu.offset),
    cpuPowerSensors: spoofPowerSensors(temperature.cpuPowerSensors, cpu.percent, cpu.offset),
    gpuPowerSensors: spoofPowerSensors(temperature.gpuPowerSensors, gpu.percent, gpu.offset),
    gpuDevices: Array.isArray(temperature.gpuDevices)
      ? temperature.gpuDevices.map((device) => ({
        ...device,
        powerSensors: spoofPowerSensors(device.powerSensors, gpu.percent, gpu.offset),
      }))
      : temperature.gpuDevices,
  } as types.TemperatureData;
}

export function applyPowerSpoofToHistoryPoints(
  points: types.TemperatureHistoryPoint[],
  config: types.AppConfig,
) {
  if (!(config as any).powerSpoofEnabled) return points;
  const cpu = powerSpoofPair(config, 'cpu');
  const gpu = powerSpoofPair(config, 'gpu');
  return points.map((point) => ({
    ...point,
    cpuPowerWatts: spoofPower(point.cpuPowerWatts, cpu.percent, cpu.offset),
    gpuPowerWatts: spoofPower(point.gpuPowerWatts, gpu.percent, gpu.offset),
  }));
}
