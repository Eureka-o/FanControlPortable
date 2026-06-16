'use client';

import { useCallback, useMemo, useState } from 'react';
import { Bug, ChevronDown, Play, RotateCw, TriangleAlert } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { toast } from 'sonner';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { types } from '../../../wailsjs/go/models';
import { apiService } from '../services/api';
import { DebugInfo, type DeviceDebugCommandResult } from '../types/app';
import { Button, ToggleSwitch } from './ui/index';

type ParsedGearTable = { type?: string; table?: Array<{ gear?: number; label?: string; rpm?: number }> };

interface DeviceDebugPanelProps {
  config: types.AppConfig;
  isConnected: boolean;
  onConfigChange: (config: types.AppConfig) => void;
}

const DANGEROUS_DEBUG_COMMANDS = new Set<number>([0xed, 0xee, 0xf0, 0xf1, 0xf2]);

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function renderDebugFrameSummary(frame: { decoded?: string; parsed?: unknown; command?: string; payloadHex?: string }) {
  const parsed = frame.parsed as ParsedGearTable | null | undefined;
  if (parsed?.type === 'gearRpmTable' && Array.isArray(parsed.table)) {
    return parsed.table
      .map((item) => `${item.label || item.gear}: ${item.rpm}%`)
      .join(' | ');
  }
  return frame.decoded || `${frame.command || '--'} ${frame.payloadHex || ''}`.trim();
}

function parseDebugCommandByte(input: string): number | null {
  const bytes = input
    .trim()
    .split(/[^0-9a-fA-F]+/)
    .filter(Boolean)
    .map((h) => Number.parseInt(h, 16));
  if (bytes.length === 0 || bytes.some((b) => Number.isNaN(b))) return null;
  if (bytes.length >= 3 && bytes[0] === 0x5a && bytes[1] === 0xa5) return bytes[2];
  return bytes[0];
}

export default function DeviceDebugPanel({ config, isConnected, onConfigChange }: DeviceDebugPanelProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [debugInfo, setDebugInfo] = useState<DebugInfo | null>(null);
  const [debugInfoLoading, setDebugInfoLoading] = useState(false);
  const [debugCommandInput, setDebugCommandInput] = useState('27');
  const [debugCommandResult, setDebugCommandResult] = useState<DeviceDebugCommandResult | null>(null);
  const [debugCommandLoading, setDebugCommandLoading] = useState(false);
  const [pawnIOReinstallLoading, setPawnIOReinstallLoading] = useState(false);

  const debugCommandByte = useMemo(() => parseDebugCommandByte(debugCommandInput), [debugCommandInput]);
  const isDangerousDebugCommand = debugCommandByte !== null && DANGEROUS_DEBUG_COMMANDS.has(debugCommandByte);

  const toggleDebugMode = useCallback(async () => {
    try {
      await apiService.setDebugMode(!config.debugMode);
      onConfigChange(types.AppConfig.createFrom({ ...config, debugMode: !config.debugMode }));
    } catch {
      /* noop */
    }
  }, [config, onConfigChange]);

  const fetchDebugInfo = useCallback(async () => {
    setDebugInfoLoading(true);
    try {
      setDebugInfo(await apiService.getDebugInfo());
    } catch {
      /* noop */
    } finally {
      setDebugInfoLoading(false);
    }
  }, []);

  const sendDeviceDebugCommand = useCallback(async (command?: string) => {
    const hexCommand = (command ?? debugCommandInput).trim();
    if (!hexCommand || !isConnected || !config.debugMode) return;
    setDebugCommandLoading(true);
    try {
      const result = await apiService.sendDeviceDebugCommand(hexCommand, 900);
      setDebugCommandResult(result);
      toast.success(t('controlPanel.debug.toasts.commandSent', { frame: result.frameHex }));
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      setDebugCommandLoading(false);
    }
  }, [config.debugMode, debugCommandInput, isConnected, t]);

  const handleReinstallPawnIO = useCallback(async () => {
    setPawnIOReinstallLoading(true);
    try {
      const result = await apiService.reinstallPawnIO();
      toast.success(t('controlPanel.debug.toasts.reinstallExecuted'));
      if (result?.warning) {
        toast.warning(result.warning);
      }
      if (result?.uninstallWarning) {
        toast.warning(t('controlPanel.debug.toasts.uninstallWarning', { warning: result.uninstallWarning }));
      }
      if (result?.bridgeWarning) {
        toast.warning(t('controlPanel.debug.toasts.bridgeWarning', { warning: result.bridgeWarning }));
      }
      await fetchDebugInfo();
    } catch (error) {
      toast.error(t('controlPanel.debug.toasts.reinstallFailed', { error: getErrorMessage(error) }));
    } finally {
      setPawnIOReinstallLoading(false);
    }
  }, [fetchDebugInfo, t]);

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <div className="overflow-hidden rounded-2xl border border-border bg-card">
        <CollapsibleTrigger asChild>
          <button type="button" className="flex w-full cursor-pointer items-center justify-between px-4 py-3 transition-colors hover:bg-muted/40">
            <div className="flex items-center gap-2">
              <Bug className="h-4 w-4 text-muted-foreground" />
              <span className="text-sm font-semibold text-foreground">{t('controlPanel.debug.panelTitle')}</span>
            </div>
            <ChevronDown className={clsx('h-4 w-4 text-muted-foreground transition-transform duration-200', open && 'rotate-180')} />
          </button>
        </CollapsibleTrigger>

        <CollapsibleContent>
          <div className="space-y-3 border-t border-border/60 p-4">
            <div className="flex items-center justify-between rounded-xl bg-muted/50 px-3 py-2.5">
              <div className="flex items-center gap-2">
                <Bug className="h-4 w-4 text-muted-foreground" />
                <div>
                  <div className="text-sm font-medium">{t('controlPanel.debug.modeTitle')}</div>
                  <div className="text-[11px] text-muted-foreground">{t('controlPanel.debug.modeDescription')}</div>
                </div>
              </div>
              <ToggleSwitch enabled={config.debugMode} onChange={toggleDebugMode} size="sm" color="purple" />
            </div>

            <Button variant="secondary" size="sm" onClick={fetchDebugInfo} loading={debugInfoLoading} className="w-full">
              {t('controlPanel.debug.refresh')}
            </Button>

            <div className="rounded-xl border border-border/70 bg-background px-3 py-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="min-w-0">
                  <div className="text-sm font-medium text-foreground">{t('controlPanel.debug.pawnTitle')}</div>
                  <div className="text-[11px] leading-relaxed text-muted-foreground">{t('controlPanel.debug.pawnDescription')}</div>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleReinstallPawnIO}
                  loading={pawnIOReinstallLoading}
                  icon={<RotateCw className="h-3.5 w-3.5" />}
                >
                  {t('controlPanel.debug.reinstall')}
                </Button>
              </div>
            </div>

            {debugInfo && (
              <div className="min-h-56 max-h-[min(55vh,30rem)] w-full cursor-text overflow-auto rounded-xl border border-border bg-background overscroll-contain select-text">
                <pre className="min-w-max whitespace-pre p-3 font-mono text-xs leading-5 text-foreground/90">{JSON.stringify(debugInfo, null, 2)}</pre>
              </div>
            )}

            <div className="rounded-xl border border-border/70 bg-background px-3 py-3">
              <div className="flex gap-2">
                <input
                  value={debugCommandInput}
                  onChange={(event) => setDebugCommandInput(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') void sendDeviceDebugCommand();
                  }}
                  placeholder={t('controlPanel.debug.commandPlaceholder')}
                  className={clsx(
                    'h-9 min-w-0 flex-1 rounded-md border bg-background px-3 font-mono text-xs outline-none ring-offset-background transition-colors focus-visible:ring-2',
                    isDangerousDebugCommand
                      ? 'border-red-500 text-red-600 focus-visible:ring-red-500 dark:text-red-400'
                      : 'border-input focus-visible:ring-ring',
                  )}
                />
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => sendDeviceDebugCommand()}
                  loading={debugCommandLoading}
                  disabled={!isConnected || !config.debugMode}
                  icon={<Play className="h-3.5 w-3.5" />}
                >
                  {t('controlPanel.debug.sendCommand')}
                </Button>
              </div>
              <div className="mt-2 flex items-start gap-1.5 text-[11px] leading-relaxed text-red-600 dark:text-red-400">
                <TriangleAlert className="mt-px h-3.5 w-3.5 shrink-0" />
                {isDangerousDebugCommand ? (
                  <span className="font-semibold">
                    {t('controlPanel.debug.dangerousCommandWarning', { command: debugCommandByte?.toString(16).toUpperCase().padStart(2, '0') })}
                  </span>
                ) : (
                  <span>{t('controlPanel.debug.rawCommandWarning')}</span>
                )}
              </div>
              {debugCommandResult && (
                <div className="mt-3 max-h-48 cursor-text overflow-auto rounded-md bg-muted/45 p-2 font-mono text-[11px] leading-5 select-text">
                  <div>TX {debugCommandResult.rawHex}</div>
                  {(debugCommandResult.frames || []).map((frame) => (
                    <div key={frame.id} className={frame.direction === 'rx' ? 'text-emerald-600 dark:text-emerald-400' : 'text-sky-600 dark:text-sky-400'}>
                      <div>{frame.direction.toUpperCase()} {frame.command || '--'} {frame.frameHex || frame.rawHex} {frame.checksumOk ? 'OK' : 'BAD'}</div>
                      {frame.decoded && <div className="pl-4 text-foreground/80">{renderDebugFrameSummary(frame)}</div>}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </CollapsibleContent>
      </div>
    </Collapsible>
  );
}
