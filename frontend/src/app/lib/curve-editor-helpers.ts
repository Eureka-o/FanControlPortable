export interface CurvePointLike {
  temperature: number;
  rpm: number;
}

export interface CurveChartPixelBounds {
  top: number;
  bottom: number;
  left: number;
  right: number;
  yMin: number;
  yMax: number;
  xMin: number;
  xMax: number;
}

export const CURVE_UNDO_LIMIT = 30;

export function cloneCurvePoints<T extends CurvePointLike>(curve: readonly T[]): T[] {
  return curve.map((point) => ({ ...point }));
}

export function curvesEqual(a: readonly CurvePointLike[], b: readonly CurvePointLike[]): boolean {
  if (a.length !== b.length) {
    return false;
  }
  for (let i = 0; i < a.length; i += 1) {
    if (a[i].temperature !== b[i].temperature || a[i].rpm !== b[i].rpm) {
      return false;
    }
  }
  return true;
}

export function normalizeCurveSpeed(value: number, minSpeed: number, maxSpeed: number): number {
  return Math.max(minSpeed, Math.min(maxSpeed, Math.round(value)));
}

export function snapCurveSpeedValue(
  rawValue: number,
  minSpeed: number,
  maxSpeed: number,
  speedUnit: string,
  snapEnabled: boolean,
  shiftKey: boolean,
): number {
  let step = 1;
  if (speedUnit === 'rpm') {
    if (snapEnabled && shiftKey) {
      step = 100;
    } else if (snapEnabled || shiftKey) {
      step = 50;
    }
  } else {
    if (snapEnabled && shiftKey) {
      step = 10;
    } else if (snapEnabled || shiftKey) {
      step = 5;
    }
  }

  if (step <= 1) {
    return normalizeCurveSpeed(rawValue, minSpeed, maxSpeed);
  }

  const snapped = Math.round(rawValue / step) * step;
  return normalizeCurveSpeed(snapped, minSpeed, maxSpeed);
}

export function syncCurveSpeedAtIndexWithAffected<T extends CurvePointLike>(
  curve: readonly T[],
  index: number,
  targetSpeed: number,
  minSpeed: number,
  maxSpeed: number,
): { curve: T[]; changed: boolean; affectedIndices: number[] } {
  const currentPoint = curve[index];
  if (!currentPoint) {
    return { curve: [...curve], changed: false, affectedIndices: [] };
  }

  const normalizedSpeed = normalizeCurveSpeed(targetSpeed, minSpeed, maxSpeed);
  const nextCurve = [...curve];
  const affected: number[] = [];
  let changed = false;

  if (currentPoint.rpm !== normalizedSpeed) {
    nextCurve[index] = { ...currentPoint, rpm: normalizedSpeed };
    changed = true;
  }

  for (let left = index - 1; left >= 0; left -= 1) {
    if (nextCurve[left].rpm <= nextCurve[left + 1].rpm) {
      break;
    }

    nextCurve[left] = {
      ...nextCurve[left],
      rpm: nextCurve[left + 1].rpm,
    };
    affected.push(left);
    changed = true;
  }

  for (let right = index + 1; right < nextCurve.length; right += 1) {
    if (nextCurve[right].rpm >= nextCurve[right - 1].rpm) {
      break;
    }

    nextCurve[right] = {
      ...nextCurve[right],
      rpm: nextCurve[right - 1].rpm,
    };
    affected.push(right);
    changed = true;
  }

  affected.sort((a, b) => a - b);

  return {
    curve: nextCurve,
    changed,
    affectedIndices: affected,
  };
}

export function shiftCurveSpeeds<T extends CurvePointLike>(
  curve: readonly T[],
  delta: number,
  minSpeed: number,
  maxSpeed: number,
): { curve: T[]; changed: boolean } {
  let changed = false;
  const nextCurve = curve.map((point) => {
    const nextRpm = normalizeCurveSpeed(point.rpm + delta, minSpeed, maxSpeed);
    if (nextRpm !== point.rpm) {
      changed = true;
    }
    return { ...point, rpm: nextRpm };
  });

  return { curve: nextCurve, changed };
}

export function pushCurveUndoSnapshot<T extends CurvePointLike>(
  stack: readonly T[][],
  snapshot: readonly T[],
  maxSize = CURVE_UNDO_LIMIT,
): T[][] {
  if (snapshot.length === 0) {
    return [...stack];
  }
  const top = stack[stack.length - 1];
  if (top && curvesEqual(top, snapshot)) {
    return [...stack];
  }
  const next = [...stack, cloneCurvePoints(snapshot)];
  if (next.length > maxSize) {
    return next.slice(next.length - maxSize);
  }
  return next;
}

export function findClosestCurvePointIndex(
  curve: readonly CurvePointLike[],
  clientX: number,
  clientY: number,
  bounds: CurveChartPixelBounds,
  options?: { allowHorizontal?: boolean },
): number | null {
  if (
    curve.length === 0 ||
    bounds.right <= bounds.left ||
    bounds.bottom <= bounds.top ||
    bounds.xMax <= bounds.xMin ||
    bounds.yMax <= bounds.yMin
  ) {
    return null;
  }

  const outerPad = 18;
  if (
    clientX < bounds.left - outerPad ||
    clientX > bounds.right + outerPad ||
    clientY < bounds.top - outerPad ||
    clientY > bounds.bottom + outerPad
  ) {
    return null;
  }

  const allowHorizontal = Boolean(options?.allowHorizontal);
  const halfCol = curve.length > 1
    ? Math.max(22, ((bounds.right - bounds.left) / (curve.length - 1)) * 0.52)
    : 36;

  let bestIndex: number | null = null;
  let bestDist = Infinity;
  let bestDx = Infinity;

  for (let i = 0; i < curve.length; i += 1) {
    const point = curve[i];
    const px = bounds.left + ((point.temperature - bounds.xMin) / (bounds.xMax - bounds.xMin)) * (bounds.right - bounds.left);
    const py = bounds.bottom - ((point.rpm - bounds.yMin) / (bounds.yMax - bounds.yMin)) * (bounds.bottom - bounds.top);
    const dx = Math.abs(clientX - px);
    const dy = Math.abs(clientY - py);
    const dist = Math.hypot(clientX - px, clientY - py);

    const eligible = allowHorizontal
      ? dist <= 44
      : (dist <= 36 || (dx <= halfCol && dy <= 58));

    if (!eligible) {
      continue;
    }

    if (dist < bestDist - 1e-6 || (Math.abs(dist - bestDist) <= 1e-6 && dx < bestDx)) {
      bestIndex = i;
      bestDist = dist;
      bestDx = dx;
    }
  }

  return bestIndex;
}
