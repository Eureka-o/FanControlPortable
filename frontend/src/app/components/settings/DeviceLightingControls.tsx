'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { types } from '../../../../wailsjs/go/models';
import { apiService } from '../../services/api';
import { useLocale } from '../../lib/i18n';
import { Button, Select, Slider } from '../ui';

interface DeviceLightingControlsProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  supportsBrightness: boolean;
  nativeBlackSharkModes?: boolean;
}

const LIGHT_MODE_OPTIONS = [
  { value: 'off', labelKey: 'controlPanel.options.lightMode.off.label', descriptionKey: 'controlPanel.options.lightMode.off.description' },
  { value: 'smart_temp', labelKey: 'controlPanel.options.lightMode.smart_temp.label', descriptionKey: 'controlPanel.options.lightMode.smart_temp.description' },
  { value: 'static_single', labelKey: 'controlPanel.options.lightMode.static_single.label', descriptionKey: 'controlPanel.options.lightMode.static_single.description' },
  { value: 'static_multi', labelKey: 'controlPanel.options.lightMode.static_multi.label', descriptionKey: 'controlPanel.options.lightMode.static_multi.description' },
  { value: 'rotation', labelKey: 'controlPanel.options.lightMode.rotation.label', descriptionKey: 'controlPanel.options.lightMode.rotation.description' },
  { value: 'flowing', labelKey: 'controlPanel.options.lightMode.flowing.label', descriptionKey: 'controlPanel.options.lightMode.flowing.description' },
  { value: 'breathing', labelKey: 'controlPanel.options.lightMode.breathing.label', descriptionKey: 'controlPanel.options.lightMode.breathing.description' },
];

// 黑鲨 BRB02 原生模式 (LIGHTING.md §4): 02=彩色循环 03=呼吸 04=常亮 05=闪烁
// 06=刷新 08=响应 11=彩色流动; 0x07 音频同步需要配套 0x15 电平流, 与官方 UI 一样不提供。
// 彩色循环/彩色流动忽略 RGB; 其余取单色。
const BLACKSHARK_LIGHT_MODE_OPTIONS = [
  { value: 'off', labelKey: 'controlPanel.options.lightMode.off.label', descriptionKey: 'controlPanel.options.lightMode.off.description' },
  { value: 'blackshark_color_flow', labelKey: 'controlPanel.options.lightMode.blackshark_color_flow.label', descriptionKey: 'controlPanel.options.lightMode.blackshark_color_flow.description' },
  { value: 'blackshark_color_cycle', labelKey: 'controlPanel.options.lightMode.blackshark_color_cycle.label', descriptionKey: 'controlPanel.options.lightMode.blackshark_color_cycle.description' },
  { value: 'blackshark_breathing', labelKey: 'controlPanel.options.lightMode.blackshark_breathing.label', descriptionKey: 'controlPanel.options.lightMode.blackshark_breathing.description' },
  { value: 'blackshark_static', labelKey: 'controlPanel.options.lightMode.blackshark_static.label', descriptionKey: 'controlPanel.options.lightMode.blackshark_static.description' },
  { value: 'blackshark_flashing', labelKey: 'controlPanel.options.lightMode.blackshark_flashing.label', descriptionKey: 'controlPanel.options.lightMode.blackshark_flashing.description' },
  { value: 'blackshark_refresh', labelKey: 'controlPanel.options.lightMode.blackshark_refresh.label', descriptionKey: 'controlPanel.options.lightMode.blackshark_refresh.description' },
  { value: 'blackshark_response', labelKey: 'controlPanel.options.lightMode.blackshark_response.label', descriptionKey: 'controlPanel.options.lightMode.blackshark_response.description' },
];

const LIGHT_SPEED_OPTIONS = [
  { value: 'fast', labelKey: 'controlPanel.options.lightSpeed.fast' },
  { value: 'medium', labelKey: 'controlPanel.options.lightSpeed.medium' },
  { value: 'slow', labelKey: 'controlPanel.options.lightSpeed.slow' },
];

const LIGHT_COLOR_PRESETS = [
  { nameKey: 'controlPanel.options.lightPresets.neon', colors: [{ r: 255, g: 0, b: 128 }, { r: 0, g: 255, b: 255 }, { r: 128, g: 0, b: 255 }] },
  { nameKey: 'controlPanel.options.lightPresets.forest', colors: [{ r: 86, g: 169, b: 84 }, { r: 161, g: 210, b: 106 }, { r: 44, g: 120, b: 115 }] },
  { nameKey: 'controlPanel.options.lightPresets.glacier', colors: [{ r: 80, g: 170, b: 255 }, { r: 116, g: 214, b: 255 }, { r: 200, g: 240, b: 255 }] },
];

function getDefaultLightStripConfig(): types.LightStripConfig {
  return types.LightStripConfig.createFrom({
    mode: 'smart_temp',
    speed: 'medium',
    brightness: 100,
    colors: [
      { r: 255, g: 0, b: 0 },
      { r: 0, g: 255, b: 0 },
      { r: 0, g: 128, b: 255 },
    ],
  });
}

// 通用名 -> 黑鲨原生模式 (与 Backend blackSharkLightingMode 的同语义映射一致,
// 使 UI 展示与设备实际播放的效果一致; 旧版把这些模式错迁到不相关的通用名)。
const BLACKSHARK_MODE_ALIASES: Record<string, string> = {
  smart_temp: 'blackshark_breathing',
  static_single: 'blackshark_static',
  static_multi: 'blackshark_color_cycle',
  rotation: 'blackshark_color_cycle',
  flowing: 'blackshark_color_flow',
  breathing: 'blackshark_breathing',
};

// 黑鲨模式 -> 通用近似名 (仅当配置从黑鲨设备切到飞智等通用设备时兜底)。
const GENERIC_MODE_ALIASES: Record<string, string> = {
  blackshark_breathing: 'breathing',
  blackshark_static: 'static_single',
  blackshark_flashing: 'breathing',
  blackshark_refresh: 'flowing',
  blackshark_response: 'breathing',
  blackshark_color_cycle: 'rotation',
  blackshark_color_flow: 'flowing',
};

function normalizeLightMode(mode: unknown, blacksharkModes: boolean): string {
  const rawMode = typeof mode === 'string' ? mode : '';
  const options = blacksharkModes ? BLACKSHARK_LIGHT_MODE_OPTIONS : LIGHT_MODE_OPTIONS;
  const fallback = blacksharkModes ? 'blackshark_breathing' : 'smart_temp';
  let normalized = rawMode;
  if (blacksharkModes) {
    normalized = BLACKSHARK_MODE_ALIASES[rawMode] || rawMode;
  } else {
    normalized = GENERIC_MODE_ALIASES[rawMode] || rawMode;
  }
  return options.some((option) => option.value === normalized) ? normalized : fallback;
}

function normalizeLightStripConfig(config: types.AppConfig, blacksharkModes: boolean): types.LightStripConfig {
  const defaults = getDefaultLightStripConfig();
  const raw = (config as any).lightStrip;
  if (!raw) return defaults;

  const normalized = types.LightStripConfig.createFrom({
    mode: normalizeLightMode(raw.mode, blacksharkModes),
    speed: raw.speed || defaults.speed,
    brightness: typeof raw.brightness === 'number' ? Math.max(0, Math.min(100, raw.brightness)) : defaults.brightness,
    colors: Array.isArray(raw.colors) && raw.colors.length > 0 ? raw.colors : defaults.colors,
  });

  normalized.colors = normalizeColors(normalized.colors);
  return normalized;
}

function normalizeColors(colors?: types.RGBColor[]) {
  const defaults = getDefaultLightStripConfig().colors || [];
  const merged = [...(colors || [])];
  while (merged.length < 3) merged.push(defaults[merged.length] || types.RGBColor.createFrom({ r: 255, g: 255, b: 255 }));
  return merged;
}

function rgbToHex(color: types.RGBColor): string {
  const h = (value: number) => value.toString(16).padStart(2, '0');
  return `#${h(color.r || 0)}${h(color.g || 0)}${h(color.b || 0)}`;
}

function hexToRgb(hex: string): types.RGBColor {
  const n = Number.parseInt(hex.replace('#', ''), 16);
  return types.RGBColor.createFrom({ r: (n >> 16) & 255, g: (n >> 8) & 255, b: n & 255 });
}

function getRequiredColorCount(mode: string): number {
  switch (mode) {
    case 'static_single':
    case 'breathing':
      return 1;
    case 'off':
    case 'smart_temp':
    case 'flowing':
    case 'rotation':
      return 0;
    case 'static_multi':
      return 3;
    case 'blackshark_color_flow':
    case 'blackshark_color_cycle':
      return 0;
    case 'blackshark_static':
    case 'blackshark_breathing':
    case 'blackshark_flashing':
    case 'blackshark_refresh':
    case 'blackshark_response':
      return 1;
    default:
      return 3;
  }
}

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export default function DeviceLightingControls({
  config,
  onConfigChange,
  isConnected,
  supportsBrightness,
  nativeBlackSharkModes,
}: DeviceLightingControlsProps) {
  const { t } = useTranslation();
  const { locale } = useLocale();
  const [loading, setLoading] = useState(false);
  const [lightStripConfig, setLightStripConfig] = useState<types.LightStripConfig>(
    () => normalizeLightStripConfig(config, Boolean(nativeBlackSharkModes)),
  );

  const lightModeOptions = useMemo(
    () => (nativeBlackSharkModes ? BLACKSHARK_LIGHT_MODE_OPTIONS : LIGHT_MODE_OPTIONS).map(
      (item) => ({ value: item.value, label: t(item.labelKey), description: t(item.descriptionKey) }),
    ),
    [locale, t, nativeBlackSharkModes],
  );
  const lightSpeedOptions = useMemo(
    () => LIGHT_SPEED_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey) })),
    [locale, t],
  );
  const lightColorPresets = useMemo(
    () => LIGHT_COLOR_PRESETS.map((item) => ({ name: t(item.nameKey), colors: normalizeColors(item.colors as types.RGBColor[]) })),
    [locale, t],
  );
  const requiredColorCount = getRequiredColorCount(lightStripConfig.mode);

  useEffect(() => {
    setLightStripConfig(normalizeLightStripConfig(config, Boolean(nativeBlackSharkModes)));
  }, [config, nativeBlackSharkModes]);

  const updateDraft = useCallback((patch: Partial<types.LightStripConfig>) => {
    setLightStripConfig((prev) => types.LightStripConfig.createFrom({
      ...prev,
      ...patch,
      colors: patch.colors ? normalizeColors(patch.colors) : normalizeColors(prev.colors),
    }));
  }, []);

  const handleLightColorChange = useCallback((index: number, hex: string) => {
    setLightStripConfig((prev) => {
      const colors = normalizeColors(prev.colors);
      colors[index] = hexToRgb(hex);
      return types.LightStripConfig.createFrom({ ...prev, colors });
    });
  }, []);

  const handleApplyLightStrip = useCallback(async () => {
    setLoading(true);
    try {
      const normalizedColors = normalizeColors(lightStripConfig.colors);
      const submitConfig = types.LightStripConfig.createFrom({
        ...lightStripConfig,
        colors: requiredColorCount > 0 ? normalizedColors.slice(0, Math.max(requiredColorCount, 3)) : normalizedColors,
      });
      await apiService.setLightStrip(submitConfig);
      onConfigChange(types.AppConfig.createFrom({ ...config, lightStrip: submitConfig }));
    } catch (error) {
      alert(t('controlPanel.alerts.lightStripFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoading(false);
    }
  }, [config, lightStripConfig, onConfigChange, requiredColorCount, t]);

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        <Select
          value={lightStripConfig.mode}
          onChange={(value: string | number) => updateDraft({ mode: value as string })}
          options={lightModeOptions}
          size="sm"
          label={t('controlPanel.light.effectMode')}
        />
        <Select
          value={lightStripConfig.speed}
          onChange={(value: string | number) => updateDraft({ speed: value as string })}
          options={lightSpeedOptions}
          size="sm"
          label={t('controlPanel.light.animationSpeed')}
          disabled={nativeBlackSharkModes
            ? ['off', 'blackshark_static'].includes(lightStripConfig.mode)
            : ['off', 'smart_temp', 'static_single', 'static_multi'].includes(lightStripConfig.mode)}
        />
      </div>

      {supportsBrightness && (
        <Slider
          min={0}
          max={100}
          step={1}
          value={lightStripConfig.brightness}
          onChange={(value) => updateDraft({ brightness: value })}
          label={t('controlPanel.light.brightness')}
          valueFormatter={(value) => `${value}%`}
          disabled={lightStripConfig.mode === 'off' || (!nativeBlackSharkModes && lightStripConfig.mode === 'smart_temp')}
        />
      )}

      {!nativeBlackSharkModes && lightStripConfig.mode === 'smart_temp' && (
        <div className="rounded-lg border border-amber-300/40 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-300">
          {t('controlPanel.light.smartTempWarning')}
        </div>
      )}

      <AnimatePresence>
        {requiredColorCount > 0 && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            className="space-y-3 overflow-hidden"
          >
            <div className="flex flex-wrap gap-2">
              {lightColorPresets.map((preset) => (
                <button
                  key={preset.name}
                  type="button"
                  onClick={() => updateDraft({ colors: preset.colors })}
                  className="cursor-pointer rounded-lg border border-border px-3 py-1.5 text-xs text-foreground transition-colors hover:bg-muted"
                >
                  {preset.name}
                </button>
              ))}
            </div>

            <div className={clsx('grid gap-3', requiredColorCount === 1 ? 'grid-cols-1' : 'grid-cols-3')}>
              {Array.from({ length: requiredColorCount }).map((_, index) => (
                <div key={index}>
                  <label className="mb-1 block text-xs text-muted-foreground">{t('controlPanel.light.colorLabel', { index: index + 1 })}</label>
                  <input
                    type="color"
                    value={rgbToHex(normalizeColors(lightStripConfig.colors)[index])}
                    onChange={(event) => handleLightColorChange(index, event.target.value)}
                    className="h-9 w-full cursor-pointer rounded-lg border border-border bg-card"
                  />
                </div>
              ))}
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      <div className="flex items-center justify-between pt-1">
        <span className="text-xs text-muted-foreground">
          {isConnected ? t('controlPanel.light.applyHintConnected') : t('controlPanel.light.applyHintDisconnected')}
        </span>
        <Button variant="primary" size="sm" onClick={handleApplyLightStrip} loading={loading}>
          {t('common.actions.apply')}
        </Button>
      </div>
    </div>
  );
}
