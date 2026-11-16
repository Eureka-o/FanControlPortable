// FanCurve 主文件与 recharts 图表分包（fan-curve-charts.tsx）共用的小份纯值：
// 放在独立模块里是为了让 fan-curve-charts 可以被 dynamic import 单独分包，
// 同时这些"两边都必须一致"的常量/格式化函数仍然只有一份所有者。

import { HISTORY_TIMELINE_EVENT_ORDER } from '../hooks/useHistoryDisplayPreferences';

// 纵轴下端定为 0（与横轴口径一致）。`YAxis` 与拖拽的 Y 映射都必须用它 ——
// 两边各写一个数就会漂（拖拽把光标位置按错误的域换算成转速）。
export const FAN_CURVE_AXIS_MIN_SPEED = 0;

export function formatSpeedValue(value: number) {
  if (!Number.isFinite(value)) {
    return '0';
  }
  const rounded = Math.round(value * 10) / 10;
  return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1);
}

export function formatPowerValue(value: number) {
  if (!Number.isFinite(value) || value <= 0) {
    return '--';
  }
  const rounded = value < 10 ? Math.round(value * 10) / 10 : Math.round(value);
  return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1);
}

export const TIMELINE_EVENT_COLORS: Record<(typeof HISTORY_TIMELINE_EVENT_ORDER)[number], string> = {
  disconnect: '#ef4444',
  reconnect: '#10b981',
  resume: '#8b5cf6',
  profile: '#0ea5e9',
};
