type Translate = (key: string) => string;

const MANUAL_MODE_ALIASES = new Set([
  'manual',
  'manual mode',
  'manual/fixed gear mode',
  'fixed gear mode',
  'fixed',
  'gear mode',
  '手动',
  '手动模式',
  '挡位工作模式',
]);

const AUTO_MODE_ALIASES = new Set([
  'auto',
  'auto mode',
  'automatic',
  'automatic mode',
  'auto/realtime rpm mode',
  'realtime rpm mode',
  'software',
  'software control',
  '智能温控',
  '自动模式',
  '自动模式(实时转速)',
  '自动模式(实时速度)',
  '软件控制',
]);

const TRANSPORT_ONLY_ALIASES = new Set([
  'wifi',
  'wi-fi',
  'ble',
  'bluetooth',
  'serial',
  'com',
  'hid',
  'usb',
  'hid 已连接',
]);

function normalizeWorkMode(value: string | null | undefined) {
  return (value || '').trim().toLowerCase().replace(/\s+/g, ' ');
}

export function translateWorkModeLabel(workMode: string | null | undefined, t: Translate) {
  const raw = (workMode || '').trim();
  if (!raw) {
    return '--';
  }

  const normalized = normalizeWorkMode(raw);
  if (MANUAL_MODE_ALIASES.has(normalized)) {
    return t('controlPanel.overview.workModes.manual');
  }
  if (AUTO_MODE_ALIASES.has(normalized)) {
    return t('controlPanel.overview.workModes.auto');
  }
  if (TRANSPORT_ONLY_ALIASES.has(normalized)) {
    return '--';
  }

  return raw;
}
