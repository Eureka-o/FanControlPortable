'use client';

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Activity, Check, Mic, Pause, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { types } from '../../../wailsjs/go/models';
import { apiService } from '../services/api';
import { buildDiagnosticSteps, deriveNoiseDiagnosticRange, fanSpeedDisplaySuffix, NoiseMeter, analyzeNoiseDiagnostic, type NoiseDiagnosticPoint } from '../lib/noise-diagnostic';
import { Badge, Button, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, NumberInput, Select } from './ui/index';

type Phase = 'setup' | 'confirm' | 'running' | 'result';

interface NoiseDiagnosticProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  runtimeDeviceProfile?: types.DeviceProfile | null;
  runtimeDeviceCapabilities?: types.DeviceCapabilities | null;
  deviceModel?: string | null;
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function formatDb(value: number) {
  return Number.isFinite(value) ? `${value.toFixed(1)} dB` : '--';
}

function formatRelativeDb(value: number) {
  return Number.isFinite(value) ? `+${Math.max(0, value).toFixed(1)} dB` : '--';
}

export default function NoiseDiagnostic({
  open,
  onOpenChange,
  config: _config,
  onConfigChange,
  isConnected,
  fanData,
  runtimeDeviceProfile,
  runtimeDeviceCapabilities,
  deviceModel,
}: NoiseDiagnosticProps) {
  const { t } = useTranslation();
  const [phase, setPhase] = useState<Phase>('setup');
  const [range, setRange] = useState<types.NoiseDiagnosticRange | null>(null);
  const [savedRange, setSavedRange] = useState<types.NoiseDiagnosticRange | null>(null);
  const [acknowledged, setAcknowledged] = useState(false);
  const [progress, setProgress] = useState(0);
  const [currentTarget, setCurrentTarget] = useState<number | null>(null);
  const [points, setPoints] = useState<NoiseDiagnosticPoint[]>([]);
  const [result, setResult] = useState<types.NoiseDiagnosticResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [microphones, setMicrophones] = useState<Array<{ deviceId: string; label: string }>>([]);
  const [selectedMicrophone, setSelectedMicrophone] = useState('');
  const [microphoneLoading, setMicrophoneLoading] = useState(false);
  const meterRef = useRef<NoiseMeter | null>(null);
  const sessionRef = useRef<types.NoiseDiagnosticSession | null>(null);
  const cancelRequestedRef = useRef(false);
  const abortRef = useRef<AbortController | null>(null);

  const derivedRange = useMemo(() => deriveNoiseDiagnosticRange(
    runtimeDeviceProfile,
    runtimeDeviceCapabilities,
    (fanData as any)?.flyDigiCapability,
  ), [fanData, runtimeDeviceCapabilities, runtimeDeviceProfile]);
  const relativeResultPoints = useMemo(() => {
    if (!result?.points?.length) return [];
    const floor = Math.min(...result.points.map((point) => point.levelDb));
    return result.points.map((point) => ({ ...point, relativeDb: Math.max(0, point.levelDb - floor) }));
  }, [result]);
  const relativeResultMax = Math.max(1, ...relativeResultPoints.map((point) => point.relativeDb));

  useEffect(() => {
    if (!open || phase !== 'setup' || !derivedRange) return;
    const next = types.NoiseDiagnosticRange.createFrom(derivedRange);
    setRange(next);
    setSavedRange(next);
    setAcknowledged(false);
  }, [derivedRange, open, phase]);

  useEffect(() => {
    if (!open || phase !== 'setup') return;
    let alive = true;
    setMicrophoneLoading(true);
    void NoiseMeter.listMicrophones().then((options) => {
      if (!alive) return;
      setMicrophones(options);
      setSelectedMicrophone((current) => options.some((option) => option.deviceId === current) ? current : options[0]?.deviceId || '');
    }).catch(() => {
      if (alive) {
        setMicrophones([]);
        setSelectedMicrophone('');
      }
    }).finally(() => {
      if (alive) setMicrophoneLoading(false);
    });
    return () => { alive = false; };
  }, [open, phase]);

  const closeMeter = useCallback(() => {
    meterRef.current?.close();
    meterRef.current = null;
  }, []);

  const cancelDiagnostic = useCallback(async () => {
    cancelRequestedRef.current = true;
    abortRef.current?.abort();
    abortRef.current = null;
    closeMeter();
    const session = sessionRef.current;
    sessionRef.current = null;
    if (session) {
      try {
        await apiService.cancelNoiseDiagnostic(session.sessionId);
      } catch {
        // Core expiry/disconnect cleanup is idempotent; the UI still discards data.
      }
    }
    setBusy(false);
    setProgress(0);
    setCurrentTarget(null);
    setPoints([]);
    setResult(null);
    setPhase('setup');
  }, [closeMeter]);

  useEffect(() => {
    if (open && !isConnected && sessionRef.current) void cancelDiagnostic();
  }, [cancelDiagnostic, isConnected, open]);

  useEffect(() => () => {
    cancelRequestedRef.current = true;
    closeMeter();
    const session = sessionRef.current;
    sessionRef.current = null;
    if (session) void apiService.cancelNoiseDiagnostic(session.sessionId);
  }, [closeMeter]);

  const startDiagnostic = useCallback(async () => {
    if (!range || !isConnected || busy) return;
    if (range.min >= range.max) {
      toast.error(t('noiseDiagnostic.errors.invalidRange'));
      return;
    }
    if (savedRange && (range.min !== savedRange.min || range.max !== savedRange.max) && !acknowledged) {
      setPhase('confirm');
      return;
    }
    cancelRequestedRef.current = false;
    const abortController = new AbortController();
    abortRef.current = abortController;
    setBusy(true);
    setPhase('running');
    setPoints([]);
    try {
      const meter = await NoiseMeter.open(selectedMicrophone || undefined);
      meterRef.current = meter;
      const session = await apiService.beginNoiseDiagnostic(types.NoiseDiagnosticBeginRequest.createFrom({ deviceKey: '', range }));
      sessionRef.current = types.NoiseDiagnosticSession.createFrom(session);
      const steps = buildDiagnosticSteps(session.range);
      if (steps.length === 0) throw new Error(t('noiseDiagnostic.errors.invalidRange'));
      const startTarget = await apiService.setNoiseDiagnosticTarget(session.sessionId, steps[0]);
      setCurrentTarget(startTarget.actual);
      let baseline = await meter.sampleLevel(3000, 500, abortController.signal);
      if (baseline.retryable) {
        const retry = await meter.sampleLevel(3000, 500, abortController.signal);
        if (retry.spreadDb < baseline.spreadDb) baseline = retry;
      }
      if (baseline.retryable) throw new Error(t('noiseDiagnostic.errors.unstableBaseline'));
      const collected: NoiseDiagnosticPoint[] = [{
        requested: startTarget.requested,
        actual: startTarget.actual,
        levelDb: baseline.levelDb,
        spreadDb: baseline.spreadDb,
        valid: true,
      }];
      setPoints([...collected]);
      setProgress(Math.round((1 / steps.length) * 100));
      for (let index = 1; index < steps.length; index += 1) {
        if (cancelRequestedRef.current) throw new Error('noise-cancelled');
        const target = steps[index];
        const targetResult = await apiService.setNoiseDiagnosticTarget(session.sessionId, target);
        setCurrentTarget(targetResult.actual || targetResult.requested);
        const sample = await meter.sampleLevel(1200, 250, abortController.signal);
        let settledSample = sample;
        if ((sample.rangeDb || sample.spreadDb) > 6) {
          const retry = await meter.sampleLevel(1200, 250, abortController.signal);
          if (retry.spreadDb < sample.spreadDb) settledSample = retry;
        }
        collected.push({
          requested: targetResult.requested,
          actual: targetResult.actual,
          levelDb: settledSample.levelDb,
          spreadDb: settledSample.spreadDb,
          valid: !settledSample.retryable,
          reason: settledSample.retryable ? settledSample.reason : undefined,
        });
        setPoints([...collected]);
        setProgress(Math.round(((index + 1) / steps.length) * 100));
      }
      const restoredStart = await apiService.setNoiseDiagnosticTarget(session.sessionId, steps[0]);
      setCurrentTarget(restoredStart.actual);
      const finalBaseline = await meter.sampleLevel(2000, 350, abortController.signal);
      await apiService.endNoiseDiagnostic(session.sessionId);
      sessionRef.current = null;
      closeMeter();
      const analysis = analyzeNoiseDiagnostic(collected, baseline.levelDb, finalBaseline.levelDb);
      const nextResult = types.NoiseDiagnosticResult.createFrom({
        deviceKey: session.deviceKey,
        unit: session.range.unit,
        points: collected,
        baselineDb: baseline.levelDb,
        baselineDriftDb: finalBaseline.levelDb - baseline.levelDb,
        riseDb: analysis.riseDb,
        knee: analysis.knee,
        suspectedPeak: analysis.suspectedPeak,
        confidence: analysis.confidence,
        confidenceReason: analysis.confidenceReason,
        microphone: microphones.find((microphone) => microphone.deviceId === selectedMicrophone)?.label || 'system-default',
        testedAt: Date.now(),
      });
      await apiService.saveNoiseDiagnosticResult(nextResult);
      try {
        onConfigChange(types.AppConfig.createFrom(await apiService.getConfig()));
      } catch {
        // The result is already persisted; the config event will refresh the store.
      }
      setResult(nextResult);
      setPhase('result');
    } catch (error) {
      if (!cancelRequestedRef.current && errorMessage(error) !== 'noise-cancelled') {
        toast.error(t('noiseDiagnostic.errors.failed', { error: errorMessage(error) }));
      }
      const session = sessionRef.current;
      sessionRef.current = null;
      closeMeter();
      if (session) {
        try { await apiService.cancelNoiseDiagnostic(session.sessionId); } catch { /* Core cleanup remains idempotent. */ }
      }
      setPhase('setup');
    } finally {
      abortRef.current = null;
      setBusy(false);
      setCurrentTarget(null);
    }
  }, [acknowledged, busy, closeMeter, isConnected, microphones, onConfigChange, range, savedRange, selectedMicrophone, t]);

  const handleOpenChange = useCallback((nextOpen: boolean) => {
    if (!nextOpen && phase === 'running') {
      void cancelDiagnostic();
      onOpenChange(false);
      return;
    }
    if (!nextOpen && phase === 'result') {
      setPhase('setup');
      setResult(null);
      setPoints([]);
      setProgress(0);
    }
    onOpenChange(nextOpen);
  }, [cancelDiagnostic, onOpenChange, phase]);

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-h-[calc(100vh-2rem)] w-[calc(100vw-2rem)] max-w-2xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Activity className="h-4 w-4 text-primary" />
            {t('noiseDiagnostic.title')}
          </DialogTitle>
          <DialogDescription>{t('noiseDiagnostic.description')}</DialogDescription>
        </DialogHeader>

        {phase === 'setup' && (
          <div className="space-y-4">
            <div className="flex items-center justify-between rounded-xl border border-border/70 bg-muted/30 px-3 py-2 text-sm">
              <span className="text-muted-foreground">{t('noiseDiagnostic.device')}</span>
              <Badge variant={isConnected ? 'success' : 'warning'}>{isConnected ? (deviceModel || t('noiseDiagnostic.connected')) : t('noiseDiagnostic.disconnected')}</Badge>
            </div>
            {range ? (
              <div className="grid gap-3 md:grid-cols-2">
                <NumberInput label={t('noiseDiagnostic.min')} value={range.min} onChange={(value) => setRange({ ...range, min: Math.max(derivedRange?.min || range.min, Math.min(value, range.max - 1)) })} min={derivedRange?.min} max={range.max - 1} step={range.step} suffix={fanSpeedDisplaySuffix(range.unit)} />
                <NumberInput label={t('noiseDiagnostic.max')} value={range.max} onChange={(value) => setRange({ ...range, max: Math.min(derivedRange?.max || range.max, Math.max(value, range.min + 1)) })} min={range.min + 1} max={derivedRange?.max} step={range.step} suffix={fanSpeedDisplaySuffix(range.unit)} />
              </div>
            ) : (
              <div className="rounded-xl border border-dashed border-border/70 px-3 py-4 text-sm text-muted-foreground">{t('noiseDiagnostic.errors.unavailable')}</div>
            )}
            <Select
              value={selectedMicrophone}
              onChange={setSelectedMicrophone}
              options={microphones.map((microphone) => ({ value: microphone.deviceId, label: microphone.label }))}
              disabled={microphoneLoading || microphones.length === 0}
              label={t('noiseDiagnostic.microphone')}
              placeholder={microphoneLoading ? t('noiseDiagnostic.loadingMicrophones') : t('noiseDiagnostic.noMicrophone')}
              className="w-full"
            />
            {range && <div className="text-xs text-muted-foreground">{t('noiseDiagnostic.bounds', { min: range.min, max: range.max, unit: fanSpeedDisplaySuffix(range.unit) })}</div>}
            <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 px-3 py-3 text-xs leading-relaxed text-amber-800 dark:text-amber-200">{t('noiseDiagnostic.disclaimer')}</div>
            <DialogFooter>
              <Button variant="secondary" size="sm" onClick={() => handleOpenChange(false)} icon={<X className="h-3.5 w-3.5" />}>{t('common.actions.cancel')}</Button>
              <Button variant="primary" size="sm" disabled={!isConnected || !range} onClick={() => void startDiagnostic()} icon={<Mic className="h-3.5 w-3.5" />}>{t('noiseDiagnostic.start')}</Button>
            </DialogFooter>
          </div>
        )}

        {phase === 'confirm' && range && (
          <div className="space-y-4">
            <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm leading-relaxed">{t('noiseDiagnostic.confirmation')}</div>
            <label className="flex items-start gap-2 text-sm text-muted-foreground">
              <input type="checkbox" checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)} className="mt-1" />
              <span>{t('noiseDiagnostic.confirmAcknowledgement')}</span>
            </label>
            <DialogFooter>
              <Button variant="secondary" size="sm" onClick={() => setPhase('setup')} icon={<X className="h-3.5 w-3.5" />}>{t('common.actions.cancel')}</Button>
              <Button variant="primary" size="sm" disabled={!acknowledged} onClick={() => void startDiagnostic()} icon={<Check className="h-3.5 w-3.5" />}>{t('noiseDiagnostic.confirmStart')}</Button>
            </DialogFooter>
          </div>
        )}

        {phase === 'running' && (
          <div className="space-y-4">
            <div className="flex items-center justify-between text-sm"><span>{t('noiseDiagnostic.progress')}</span><span className="font-semibold tabular-nums">{progress}%</span></div>
            <div className="h-2 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-primary transition-[width] duration-300" style={{ width: `${progress}%` }} /></div>
            <div className="grid grid-cols-2 gap-3 text-center">
              <div className="rounded-xl border border-border/70 bg-muted/20 p-3"><div className="text-xs text-muted-foreground">{t('noiseDiagnostic.currentTarget')}</div><div className="mt-1 text-xl font-semibold tabular-nums">{currentTarget ?? '--'} {fanSpeedDisplaySuffix(range?.unit)}</div></div>
              <div className="rounded-xl border border-border/70 bg-muted/20 p-3"><div className="text-xs text-muted-foreground">{t('noiseDiagnostic.samples')}</div><div className="mt-1 text-xl font-semibold tabular-nums">{points.length}</div></div>
            </div>
            <div className="max-h-48 space-y-2 overflow-y-auto rounded-xl border border-border/70 p-3">
              {points.map((point) => <div key={`${point.actual}-${point.requested}`} className="flex items-center justify-between text-xs"><span className="tabular-nums">{point.actual} {fanSpeedDisplaySuffix(range?.unit)}</span><span className={point.valid ? 'text-foreground' : 'text-amber-600'}>{point.valid ? formatDb(point.levelDb) : t('noiseDiagnostic.invalidSample')}</span></div>)}
            </div>
            <DialogFooter><Button variant="danger" size="sm" onClick={() => void cancelDiagnostic()} icon={<Pause className="h-3.5 w-3.5" />}>{t('noiseDiagnostic.cancel')}</Button></DialogFooter>
          </div>
        )}

        {phase === 'result' && result && (
          <div className="space-y-4">
            <div className="grid gap-3 md:grid-cols-4">
              <div className="rounded-xl border border-border/70 bg-muted/20 p-3"><div className="text-xs text-muted-foreground">{t('noiseDiagnostic.rise')}</div><div className="mt-1 text-xl font-semibold">{formatDb(result.riseDb)}</div></div>
              <div className="rounded-xl border border-border/70 bg-muted/20 p-3"><div className="text-xs text-muted-foreground">{t('noiseDiagnostic.knee')}</div><div className="mt-1 text-xl font-semibold">{result.knee} {fanSpeedDisplaySuffix(result.unit)}</div></div>
              <div className="rounded-xl border border-border/70 bg-muted/20 p-3"><div className="text-xs text-muted-foreground">{t('noiseDiagnostic.peak')}</div><div className="mt-1 text-xl font-semibold">{result.suspectedPeak ? `${result.suspectedPeak} ${fanSpeedDisplaySuffix(result.unit)}` : '--'}</div></div>
              <div className="rounded-xl border border-border/70 bg-muted/20 p-3"><div className="text-xs text-muted-foreground">{t('noiseDiagnostic.confidence')}</div><div className="mt-1 text-xl font-semibold capitalize">{result.confidence}</div></div>
            </div>
            <div className="space-y-2 rounded-xl border border-border/70 p-3">
              {relativeResultPoints.map((point) => <div key={`${point.actual}-${point.requested}`} className="flex items-center gap-3 text-xs"><span className="w-16 tabular-nums">{point.actual}</span><div className="h-2 flex-1 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-primary/70" style={{ width: `${Math.max(2, (point.relativeDb / relativeResultMax) * 100)}%` }} /></div><span className="w-16 text-right tabular-nums">{formatRelativeDb(point.relativeDb)}</span></div>)}
            </div>
            <div className="rounded-xl border border-border/70 bg-muted/20 px-3 py-2 text-xs text-muted-foreground">{t('noiseDiagnostic.resultNote', { reason: result.confidenceReason })}</div>
            <DialogFooter><Button variant="primary" size="sm" onClick={() => handleOpenChange(false)} icon={<Check className="h-3.5 w-3.5" />}>{t('common.actions.close')}</Button></DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
