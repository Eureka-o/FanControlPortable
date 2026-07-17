export interface FanCurveEditorPoint {
  temperature: number;
  rpm: number;
}

export interface ResampleFanCurveOptions {
  minTemperature: number;
  maxTemperature: number;
  temperatureStep: number;
  minSpeed: number;
  maxSpeed: number;
  speedStep?: number;
}

function snapSpeed(value: number, minSpeed: number, maxSpeed: number, speedStep: number) {
  const min = Math.min(minSpeed, maxSpeed);
  const max = Math.max(minSpeed, maxSpeed);
  const step = Number.isFinite(speedStep) && speedStep > 0 ? speedStep : 1;
  const bounded = Math.max(min, Math.min(max, Number.isFinite(value) ? value : min));
  const snapped = min + Math.round((bounded - min) / step) * step;
  return Number(Math.max(min, Math.min(max, snapped)).toFixed(6));
}

export function resampleFanCurve(points: FanCurveEditorPoint[], options: ResampleFanCurveOptions) {
  const temperatureStep = Number.isFinite(options.temperatureStep) && options.temperatureStep > 0
    ? options.temperatureStep
    : 5;
  const minTemperature = Math.min(options.minTemperature, options.maxTemperature);
  const maxTemperature = Math.max(options.minTemperature, options.maxTemperature);
  const source = points
    .filter((point) => Number.isFinite(point.temperature) && Number.isFinite(point.rpm))
    .map((point) => ({ ...point }))
    .sort((left, right) => left.temperature - right.temperature)
    .filter((point, index, sorted) => index === sorted.length - 1 || point.temperature !== sorted[index + 1].temperature);

  const speedAt = (temperature: number) => {
    if (source.length === 0) return options.minSpeed;
    if (temperature <= source[0].temperature) return source[0].rpm;
    const last = source[source.length - 1];
    if (temperature >= last.temperature) return last.rpm;
    const rightIndex = source.findIndex((point) => point.temperature >= temperature);
    const right = source[rightIndex];
    const left = source[rightIndex - 1];
    if (right.temperature === temperature) return right.rpm;
    const ratio = (temperature - left.temperature) / (right.temperature - left.temperature);
    return left.rpm + (right.rpm - left.rpm) * ratio;
  };

  const result: FanCurveEditorPoint[] = [];
  for (let temperature = minTemperature; temperature <= maxTemperature; temperature += temperatureStep) {
    result.push({
      temperature: Number(temperature.toFixed(6)),
      rpm: snapSpeed(speedAt(temperature), options.minSpeed, options.maxSpeed, options.speedStep ?? 1),
    });
  }
  if (result[result.length - 1]?.temperature !== maxTemperature) {
    result.push({
      temperature: maxTemperature,
      rpm: snapSpeed(speedAt(maxTemperature), options.minSpeed, options.maxSpeed, options.speedStep ?? 1),
    });
  }
  for (let index = 1; index < result.length; index += 1) {
    result[index].rpm = Math.max(result[index - 1].rpm, result[index].rpm);
  }
  return result;
}

export function updateFanCurvePointSpeed(
  points: FanCurveEditorPoint[],
  index: number,
  targetSpeed: number,
  minSpeed: number,
  maxSpeed: number,
  speedStep = 1,
  enforceMonotonic = true,
) {
  if (!points[index]) return points;

  const next = points.map((point) => ({ ...point }));
  next[index].rpm = snapSpeed(targetSpeed, minSpeed, maxSpeed, speedStep);

  if (enforceMonotonic) {
    for (let left = index - 1; left >= 0 && next[left].rpm > next[left + 1].rpm; left -= 1) {
      next[left].rpm = next[left + 1].rpm;
    }
    for (let right = index + 1; right < next.length && next[right].rpm < next[right - 1].rpm; right += 1) {
      next[right].rpm = next[right - 1].rpm;
    }
  }

  if (next.every((point, pointIndex) => point.temperature === points[pointIndex].temperature && point.rpm === points[pointIndex].rpm)) {
    return points;
  }
  return next;
}
