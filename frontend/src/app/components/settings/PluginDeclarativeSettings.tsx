'use client';

import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { apiService } from '../../services/api';
import type { PluginSettingField } from '../../types/app';
import { Button, NumberInput, Select, Slider, ToggleSwitch } from '../ui';

interface PluginDeclarativeSettingsProps {
  pluginId: string;
  fields: PluginSettingField[];
}

export default function PluginDeclarativeSettings({ pluginId, fields }: PluginDeclarativeSettingsProps) {
  const { t } = useTranslation();
  const [values, setValues] = useState<Record<string, unknown>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let active = true;
    setLoading(true);
    void apiService
      .getPluginSettings(pluginId)
      .then((res) => {
        if (active) setValues(res);
      })
      .catch(() => {})
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [pluginId]);

  const persistValues = useCallback(
    async (next: Record<string, unknown>) => {
      setSaving(true);
      try {
        await apiService.savePluginSettings(pluginId, next);
        toast.success(t('controlPanel.plugins.settingsSaved'));
      } catch (error) {
        toast.error(
          t('controlPanel.alerts.settingsOperationFailed', {
            error: error instanceof Error ? error.message : String(error),
          }),
        );
      } finally {
        setSaving(false);
      }
    },
    [pluginId, t],
  );

  if (fields.length === 0) return null;

  return (
    <div className="mt-3 space-y-2.5 rounded-xl border border-border/70 bg-background/45 px-4 py-3">
      {fields.map((field) => {
        const currentValue = values[field.key] ?? field.default;
        if (field.type === 'boolean') {
          return (
            <div key={field.key} className="flex items-center justify-between gap-3">
              <div className="min-w-0">
                <div className="text-sm font-medium text-foreground">{field.title}</div>
                {field.description && (
                  <div className="text-xs text-muted-foreground">{field.description}</div>
                )}
              </div>
              <ToggleSwitch
                enabled={Boolean(currentValue)}
                disabled={loading || saving}
                onChange={(checked) => {
                  const next = { ...values, [field.key]: checked };
                  setValues(next);
                  void persistValues(next);
                }}
                size="sm"
                color="blue"
              />
            </div>
          );
        }

        if (field.type === 'select') {
          return (
            <div key={field.key} className="flex flex-wrap items-center justify-between gap-3">
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium text-foreground">{field.title}</div>
                {field.description && (
                  <div className="text-xs text-muted-foreground">{field.description}</div>
                )}
              </div>
              <div className="w-44 shrink-0">
                <Select
                  value={String(currentValue ?? '')}
                  disabled={loading || saving}
                  options={(field.options || []).map((opt) => ({
                    value: opt.value,
                    label: opt.label,
                  }))}
                  size="sm"
                  onChange={(val) => {
                    const next = { ...values, [field.key]: String(val) };
                    setValues(next);
                    void persistValues(next);
                  }}
                />
              </div>
            </div>
          );
        }

        if (field.type === 'slider') {
          const min = field.min ?? 0;
          const max = field.max ?? 100;
          const numeric = Number.isFinite(Number(currentValue)) ? Number(currentValue) : min;
          return (
            <div key={field.key} className="space-y-1.5">
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <div className="text-sm font-medium text-foreground">{field.title}</div>
                  {field.description && (
                    <div className="text-xs text-muted-foreground">{field.description}</div>
                  )}
                </div>
                <span className="text-xs font-semibold tabular-nums text-primary">{numeric}</span>
              </div>
              <Slider
                value={numeric}
                min={min}
                max={max}
                step={field.step ?? 1}
                showValue={false}
                disabled={loading || saving}
                onChange={(val) => {
                  setValues((prev) => ({ ...prev, [field.key]: val }));
                }}
                onChangeEnd={() => {
                  void persistValues(values);
                }}
              />
            </div>
          );
        }

        if (field.type === 'number') {
          const numeric = Number.isFinite(Number(currentValue)) ? Number(currentValue) : (field.min ?? 0);
          return (
            <div key={field.key} className="flex flex-wrap items-center justify-between gap-3">
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium text-foreground">{field.title}</div>
                {field.description && (
                  <div className="text-xs text-muted-foreground">{field.description}</div>
                )}
              </div>
              <div className="flex items-center gap-2">
                <NumberInput
                  value={numeric}
                  min={field.min}
                  max={field.max}
                  step={field.step ?? 1}
                  disabled={loading || saving}
                  className="w-28"
                  onChange={(val) => {
                    setValues((prev) => ({ ...prev, [field.key]: val }));
                  }}
                />
                <Button
                  variant="outline"
                  size="sm"
                  loading={saving}
                  onClick={() => { void persistValues(values); }}
                >
                  {t('controlPanel.plugins.saveSettingsButton')}
                </Button>
              </div>
            </div>
          );
        }

        return (
          <div key={field.key} className="flex flex-wrap items-center justify-between gap-3">
            <div className="min-w-0 flex-1">
              <div className="text-sm font-medium text-foreground">{field.title}</div>
              {field.description && (
                <div className="text-xs text-muted-foreground">{field.description}</div>
              )}
            </div>
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={currentValue === undefined || currentValue === null ? '' : String(currentValue)}
                disabled={loading || saving}
                onChange={(e) => {
                  setValues((prev) => ({ ...prev, [field.key]: e.target.value }));
                }}
                className="h-8 w-36 rounded-lg border border-border bg-card px-2.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
              />
              <Button
                variant="outline"
                size="sm"
                loading={saving}
                onClick={() => { void persistValues(values); }}
              >
                {t('controlPanel.plugins.saveSettingsButton')}
              </Button>
            </div>
          </div>
        );
      })}
    </div>
  );
}
