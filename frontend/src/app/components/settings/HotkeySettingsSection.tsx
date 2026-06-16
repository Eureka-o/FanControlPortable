'use client';

import { useCallback, useEffect, useState, type KeyboardEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { types } from '../../../../wailsjs/go/models';
import { apiService } from '../../services/api';
import { HotkeyField } from './SettingLayout';

interface HotkeySettingsSectionProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
}

type HotkeyTarget = 'manual' | 'auto' | 'curve';

interface HotkeyValues {
  manual: string;
  auto: string;
  curve: string;
}

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function normalizeHotkeyForDisplay(value: string): string {
  return (value || '').trim();
}

function hotkeyValuesFromConfig(config: types.AppConfig): HotkeyValues {
  return {
    manual: normalizeHotkeyForDisplay((config as any).manualGearToggleHotkey),
    auto: normalizeHotkeyForDisplay((config as any).autoControlToggleHotkey),
    curve: normalizeHotkeyForDisplay((config as any).curveProfileToggleHotkey),
  };
}

function buildShortcutFromKeyboardEvent(e: {
  key: string;
  ctrlKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
  metaKey: boolean;
}): string {
  if (e.key === 'Backspace' || e.key === 'Delete') {
    return '';
  }

  const parts: string[] = [];
  if (e.ctrlKey) parts.push('Ctrl');
  if (e.altKey) parts.push('Alt');
  if (e.shiftKey) parts.push('Shift');
  if (e.metaKey) parts.push('Win');

  const key = e.key;
  if (['Control', 'Alt', 'Shift', 'Meta'].includes(key)) {
    return '';
  }

  let mainKey = '';
  if (/^[a-z]$/i.test(key)) {
    mainKey = key.toUpperCase();
  } else if (/^[0-9]$/.test(key)) {
    mainKey = key;
  } else if (/^F\d{1,2}$/i.test(key)) {
    mainKey = key.toUpperCase();
  }

  if (!mainKey || parts.length === 0) {
    return '';
  }

  return [...parts, mainKey].join('+');
}

export default function HotkeySettingsSection({ config, onConfigChange }: HotkeySettingsSectionProps) {
  const { t } = useTranslation();
  const [hotkeys, setHotkeys] = useState<HotkeyValues>(() => hotkeyValuesFromConfig(config));
  const [recordingTarget, setRecordingTarget] = useState<HotkeyTarget | null>(null);

  useEffect(() => {
    setHotkeys(hotkeyValuesFromConfig(config));
  }, [
    (config as any).manualGearToggleHotkey,
    (config as any).autoControlToggleHotkey,
    (config as any).curveProfileToggleHotkey,
  ]);

  const saveHotkeys = useCallback(async (silent = false, values: HotkeyValues = hotkeys) => {
    try {
      const manualValue = normalizeHotkeyForDisplay(values.manual);
      const autoValue = normalizeHotkeyForDisplay(values.auto);
      const curveValue = normalizeHotkeyForDisplay(values.curve);

      const nonEmptyValues = [manualValue, autoValue, curveValue].filter((value) => value !== '');
      const uniq = new Set(nonEmptyValues);
      if (uniq.size !== nonEmptyValues.length) {
        if (!silent) toast.error(t('controlPanel.system.hotkeys.toasts.duplicate'));
        return false;
      }

      const newCfg = types.AppConfig.createFrom({
        ...config,
        manualGearToggleHotkey: manualValue,
        autoControlToggleHotkey: autoValue,
        curveProfileToggleHotkey: curveValue,
      });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
      if (!silent) toast.success(t('controlPanel.system.hotkeys.toasts.saved'));
      return true;
    } catch (error) {
      if (!silent) toast.error(t('controlPanel.system.hotkeys.toasts.saveFailed', { error: getErrorMessage(error) }));
      return false;
    }
  }, [config, hotkeys, onConfigChange, t]);

  const setHotkey = useCallback((target: HotkeyTarget, value: string) => {
    setHotkeys((current) => ({ ...current, [target]: value }));
  }, []);

  const handleHotkeyInputKeyDown = (target: HotkeyTarget) => (e: KeyboardEvent<HTMLInputElement>) => {
    e.preventDefault();
    e.stopPropagation();

    if (e.key === 'Escape') {
      setRecordingTarget(null);
      e.currentTarget.blur();
      return;
    }

    if (e.key === 'Backspace' || e.key === 'Delete') {
      setHotkey(target, '');
      return;
    }

    const shortcut = buildShortcutFromKeyboardEvent(e);
    if (!shortcut) return;

    setHotkey(target, shortcut);
  };

  const handleHotkeyInputBlur = useCallback(async () => {
    setRecordingTarget(null);
    await saveHotkeys(true);
  }, [saveHotkeys]);

  const clearHotkeyInput = useCallback(async (target: HotkeyTarget) => {
    const nextHotkeys = { ...hotkeys, [target]: '' };
    setHotkeys(nextHotkeys);
    await saveHotkeys(true, nextHotkeys);
  }, [hotkeys, saveHotkeys]);

  return (
    <div className="px-5 py-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <div>
          <div className="text-base font-medium text-foreground">{t('controlPanel.system.hotkeys.title')}</div>
          <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
            {t('controlPanel.system.hotkeys.description')}
          </p>
        </div>
      </div>

      <div className="rounded-xl border border-border/70 bg-background/45 px-4 py-2">
        <HotkeyField
          title={t('controlPanel.system.hotkeys.manual.title')}
          description={t('controlPanel.system.hotkeys.manual.description')}
          value={hotkeys.manual}
          placeholder={t('controlPanel.system.hotkeys.emptyPlaceholder')}
          clearAriaLabel={t('controlPanel.system.hotkeys.clearAria')}
          recording={recordingTarget === 'manual'}
          onFocus={() => setRecordingTarget('manual')}
          onBlur={handleHotkeyInputBlur}
          onKeyDown={handleHotkeyInputKeyDown('manual')}
          onClear={() => clearHotkeyInput('manual')}
        />

        <div className="border-t border-border/60" />

        <HotkeyField
          title={t('controlPanel.system.hotkeys.auto.title')}
          description={t('controlPanel.system.hotkeys.auto.description')}
          value={hotkeys.auto}
          placeholder={t('controlPanel.system.hotkeys.emptyPlaceholder')}
          clearAriaLabel={t('controlPanel.system.hotkeys.clearAria')}
          recording={recordingTarget === 'auto'}
          onFocus={() => setRecordingTarget('auto')}
          onBlur={handleHotkeyInputBlur}
          onKeyDown={handleHotkeyInputKeyDown('auto')}
          onClear={() => clearHotkeyInput('auto')}
        />

        <div className="border-t border-border/60" />

        <HotkeyField
          title={t('controlPanel.system.hotkeys.curve.title')}
          description={t('controlPanel.system.hotkeys.curve.description')}
          value={hotkeys.curve}
          placeholder={t('controlPanel.system.hotkeys.emptyPlaceholder')}
          clearAriaLabel={t('controlPanel.system.hotkeys.clearAria')}
          recording={recordingTarget === 'curve'}
          onFocus={() => setRecordingTarget('curve')}
          onBlur={handleHotkeyInputBlur}
          onKeyDown={handleHotkeyInputKeyDown('curve')}
          onClear={() => clearHotkeyInput('curve')}
        />
      </div>
    </div>
  );
}
