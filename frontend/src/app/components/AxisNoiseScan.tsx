'use client';

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Check, Clock3, Ear, Info, RotateCcw, ShieldCheck, TriangleAlert, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { types } from '../../../wailsjs/go/models';
import { apiService } from '../services/api';
import {
  buildAxisNoiseRefinementSteps,
  buildDiagnosticSteps,
  confirmAxisNoiseSeverity,
  deriveNoiseDiagnosticRange,
  fanSpeedDisplaySuffix,
  noiseDiagnosticDeviceKey,
  type AxisNoisePoint,
  type AxisNoiseProfile,
  type AxisNoiseSeverity,
} from '../lib/noise-diagnostic';
import { Badge, Button, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, NumberInput, ToggleSwitch } from './ui/index';

type Phase = 'setup' | 'confirm' | 'running' | 'result';

interface AxisNoiseScanProps {
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

function message(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export default function AxisNoiseScan({
  open,
  onOpenChange,
  config,
  onConfigChange,
  isConnected,
  fanData,
  runtimeDeviceProfile,
  runtimeDeviceCapabilities,
  deviceModel,
}: AxisNoiseScanProps) {
  const { t } = useTranslation();
  const [phase, setPhase] = useState<Phase>('setup');
  const [range, setRange] = useState<types.NoiseDiagnosticRange | null>(null);
  const [savedRange, setSavedRange] = useState<types.NoiseDiagnosticRange | null>(null);
  const [acknowledged, setAcknowledged] = useState(false);
  const [current, setCurrent] = useState<types.NoiseDiagnosticTargetResult | null>(null);
  const [progress, setProgress] = useState(0);
  const [busy, setBusy] = useState(false);
  const [confirmingNoise, setConfirmingNoise] = useState(false);
  const [result, setResult] = useState<AxisNoiseProfile | null>(null);
  const sessionRef = useRef<types.NoiseDiagnosticSession | null>(null);
  const stepsRef = useRef<number[]>([]);
  const coarseStepsRef = useRef<number[]>([]);
  const indexRef = useRef(0);
  const pointsRef = useRef<AxisNoisePoint[]>([]);
  const pendingConfirmationRef = useRef<{ requested: number; severity: AxisNoiseSeverity } | null>(null);
  const cancelRequestedRef = useRef(false);

  const derivedRange = useMemo(() => deriveNoiseDiagnosticRange(
    runtimeDeviceProfile,
    runtimeDeviceCapabilities,
    fanData?.flyDigiCapability,
  ), [fanData, runtimeDeviceCapabilities, runtimeDeviceProfile]);
  const deviceKey = useMemo(() => noiseDiagnosticDeviceKey(runtimeDeviceProfile), [runtimeDeviceProfile]);
  const plannedSteps = useMemo(() => range ? buildDiagnosticSteps(range) : [], [range]);
  const estimatedMinutes = Math.max(1, Math.ceil((plannedSteps.length * 25) / 60));
  const existingProfile = (config.axisNoiseProfilesByDevice?.[deviceKey] || null) as AxisNoiseProfile | null;

  const refreshConfig = useCallback(async () => {
    onConfigChange(types.AppConfig.createFrom(await apiService.getConfig()));
  }, [onConfigChange]);

  useEffect(() => {
    if (!open || phase !== 'setup' || !derivedRange) return;
    const next = types.NoiseDiagnosticRange.createFrom(derivedRange);
    setRange(next);
    setSavedRange(next);
    setAcknowledged(false);
  }, [derivedRange, open, phase]);

  const discard = useCallback(async () => {
    cancelRequestedRef.current = true;
    const session = sessionRef.current;
    sessionRef.current = null;
    if (session) {
      try { await apiService.cancelNoiseDiagnostic(session.sessionId); } catch { /* lease cleanup is idempotent */ }
    }
    stepsRef.current = [];
    coarseStepsRef.current = [];
    pointsRef.current = [];
    pendingConfirmationRef.current = null;
    indexRef.current = 0;
    setCurrent(null);
    setProgress(0);
    setBusy(false);
    setConfirmingNoise(false);
    setResult(null);
    setPhase('setup');
  }, []);

  useEffect(() => {
    if (open && !isConnected && sessionRef.current) void discard();
  }, [discard, isConnected, open]);

  useEffect(() => () => {
    cancelRequestedRef.current = true;
    const session = sessionRef.current;
    sessionRef.current = null;
    if (session) void apiService.cancelNoiseDiagnostic(session.sessionId);
  }, []);

  const start = useCallback(async () => {
    if (!range || !isConnected || busy) return;
    if (savedRange && (range.min !== savedRange.min || range.max !== savedRange.max) && !acknowledged) {
      setPhase('confirm');
      return;
    }
    cancelRequestedRef.current = false;
    setConfirmingNoise(false);
    setPhase('running');
    setBusy(true);
    try {
      const session = types.NoiseDiagnosticSession.createFrom(await apiService.beginNoiseDiagnostic(
        types.NoiseDiagnosticBeginRequest.createFrom({ deviceKey, range }),
      ));
      if (cancelRequestedRef.current) {
        await apiService.cancelNoiseDiagnostic(session.sessionId);
        return;
      }
      const steps = buildDiagnosticSteps(session.range);
      if (steps.length < 2) throw new Error(t('axisNoise.errors.invalidRange'));
      sessionRef.current = session;
      coarseStepsRef.current = steps;
      stepsRef.current = [...steps];
      pointsRef.current = [];
      pendingConfirmationRef.current = null;
      indexRef.current = 0;
      setRange(session.range);
      setProgress(0);
      setCurrent(types.NoiseDiagnosticTargetResult.createFrom(await apiService.setNoiseDiagnosticTarget(session.sessionId, steps[0])));
    } catch (error) {
      if (!cancelRequestedRef.current) toast.error(t('axisNoise.errors.failed', { error: message(error) }));
      await discard();
    } finally {
      setBusy(false);
    }
  }, [acknowledged, busy, deviceKey, discard, isConnected, range, savedRange, t]);

  const finish = useCallback(async (session: types.NoiseDiagnosticSession) => {
    await apiService.endNoiseDiagnostic(session.sessionId);
    sessionRef.current = null;
    const saved = await apiService.saveAxisNoiseProfile({
      deviceKey: session.deviceKey,
      unit: session.range.unit,
      enabled: true,
      range: session.range,
      points: pointsRef.current,
      zones: [],
      testedAt: Date.now(),
    });
    await refreshConfig();
    setResult(saved);
    setCurrent(null);
    setConfirmingNoise(false);
    setProgress(100);
    setPhase('result');
  }, [refreshConfig]);

  const rateCurrent = useCallback(async (severity: AxisNoiseSeverity) => {
    const session = sessionRef.current;
    if (!session || !current || busy) return;
    setBusy(true);
    try {
      const requested = Math.round(current.requested);
      const isCoarsePoint = coarseStepsRef.current.some((step) => Math.round(step) === requested);
      const pending = pendingConfirmationRef.current;
      if (severity !== 'none' && isCoarsePoint && (!pending || pending.requested !== requested)) {
        pendingConfirmationRef.current = { requested, severity };
        setConfirmingNoise(true);
        setCurrent(null);
        const repeated = await apiService.setNoiseDiagnosticTarget(session.sessionId, requested);
        if (!cancelRequestedRef.current) setCurrent(types.NoiseDiagnosticTargetResult.createFrom(repeated));
        return;
      }

      const confirmedSeverity = pending?.requested === requested
        ? confirmAxisNoiseSeverity(pending.severity, severity)
        : severity;
      pendingConfirmationRef.current = null;
      setConfirmingNoise(false);
      pointsRef.current = [...pointsRef.current, { requested, actual: current.actual, severity: confirmedSeverity }];
      if (confirmedSeverity !== 'none' && isCoarsePoint) {
        const refined = buildAxisNoiseRefinementSteps(session.range, current.actual || requested, stepsRef.current);
        stepsRef.current.splice(indexRef.current + 1, 0, ...refined);
      }
      indexRef.current += 1;
      setProgress((previous) => Math.max(previous, Math.round((pointsRef.current.length / stepsRef.current.length) * 100)));
      if (indexRef.current >= stepsRef.current.length) {
        await finish(session);
        return;
      }
      setCurrent(null);
      const next = await apiService.setNoiseDiagnosticTarget(session.sessionId, stepsRef.current[indexRef.current]);
      if (!cancelRequestedRef.current) setCurrent(types.NoiseDiagnosticTargetResult.createFrom(next));
    } catch (error) {
      if (!cancelRequestedRef.current) toast.error(t('axisNoise.errors.failed', { error: message(error) }));
      await discard();
    } finally {
      setBusy(false);
    }
  }, [busy, current, discard, finish, t]);

  const saveExisting = useCallback(async (next: AxisNoiseProfile) => {
    setBusy(true);
    try {
      await apiService.saveAxisNoiseProfile(next);
      await refreshConfig();
    } catch (error) {
      toast.error(t('axisNoise.errors.failed', { error: message(error) }));
    } finally {
      setBusy(false);
    }
  }, [refreshConfig, t]);

  const resetExisting = useCallback(async () => {
    if (!existingProfile) return;
    await saveExisting({ ...existingProfile, points: [], zones: [], testedAt: 0, enabled: false });
  }, [existingProfile, saveExisting]);

  const handleOpenChange = useCallback((nextOpen: boolean) => {
    if (!nextOpen && phase === 'running') void discard();
    if (!nextOpen && phase === 'confirm') {
      setAcknowledged(false);
      setPhase('setup');
    }
    if (!nextOpen && phase === 'result') {
      setResult(null);
      setPhase('setup');
    }
    onOpenChange(nextOpen);
  }, [discard, onOpenChange, phase]);

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-h-[calc(100vh-2rem)] w-[calc(100vw-2rem)] max-w-2xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2"><Ear className="h-4 w-4 text-primary" />{t('axisNoise.title')}</DialogTitle>
          <DialogDescription>{t('axisNoise.description')}</DialogDescription>
        </DialogHeader>

        {phase === 'setup' && (
          <div className="space-y-4">
            <div className="flex items-center justify-between gap-3 border-b border-border/70 pb-3 text-sm">
              <span className="text-muted-foreground">{t('noiseDiagnostic.device')}</span>
              <Badge variant={isConnected ? 'success' : 'warning'}>{isConnected ? (deviceModel || t('noiseDiagnostic.connected')) : t('noiseDiagnostic.disconnected')}</Badge>
            </div>
            {existingProfile && (
              <div className="flex flex-wrap items-center gap-3 rounded-xl border border-border/70 bg-muted/20 px-3 py-3">
                <div className="min-w-0 flex-1"><div className="text-sm font-medium">{t('axisNoise.saved.title')}</div><div className="text-xs text-muted-foreground">{t('axisNoise.saved.zones', { count: existingProfile.zones?.length || 0 })}</div></div>
                <ToggleSwitch enabled={existingProfile.enabled} onChange={() => void saveExisting({ ...existingProfile, enabled: !existingProfile.enabled })} size="sm" color="blue" srLabel={t('axisNoise.saved.enabled')} />
                <Button variant="ghost" size="sm" disabled={busy} onClick={() => void resetExisting()} icon={<RotateCcw className="h-3.5 w-3.5" />}>{t('axisNoise.saved.reset')}</Button>
              </div>
            )}
            {range ? (
              <div className="grid gap-3 md:grid-cols-2">
                <NumberInput label={t('noiseDiagnostic.min')} value={range.min} onChange={(value) => setRange({ ...range, min: Math.max(derivedRange?.min || range.min, Math.min(value, range.max - range.step)) })} min={derivedRange?.min} max={range.max - range.step} step={range.step} suffix={fanSpeedDisplaySuffix(range.unit)} />
                <NumberInput label={t('noiseDiagnostic.max')} value={range.max} onChange={(value) => setRange({ ...range, max: Math.min(derivedRange?.max || range.max, Math.max(value, range.min + range.step)) })} min={range.min + range.step} max={derivedRange?.max} step={range.step} suffix={fanSpeedDisplaySuffix(range.unit)} />
              </div>
            ) : <div className="text-sm text-muted-foreground">{t('noiseDiagnostic.errors.unavailable')}</div>}
            {range && (
              <div className="grid gap-2 rounded-lg border border-border/70 bg-muted/20 p-3 text-xs leading-relaxed text-muted-foreground sm:grid-cols-2">
                <div className="flex items-start gap-2"><Clock3 className="mt-0.5 h-4 w-4 shrink-0 text-primary" /><span>{t('axisNoise.estimatedTime', { count: plannedSteps.length, minutes: estimatedMinutes })}</span></div>
                <div className="flex items-start gap-2"><Info className="mt-0.5 h-4 w-4 shrink-0 text-primary" /><span>{t('axisNoise.operationReminder')}</span></div>
                <div className="flex items-start gap-2 sm:col-span-2"><ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-primary" /><span>{t('axisNoise.refinementReminder')}</span></div>
              </div>
            )}
            <div className="flex gap-2 rounded-xl border border-amber-500/30 bg-amber-500/10 px-3 py-3 text-xs leading-relaxed text-amber-800 dark:text-amber-200"><TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" />{t('axisNoise.disclaimer')}</div>
            <DialogFooter>
              <Button variant="secondary" size="sm" onClick={() => handleOpenChange(false)} icon={<X className="h-3.5 w-3.5" />}>{t('common.actions.cancel')}</Button>
              <Button variant="primary" size="sm" disabled={!isConnected || !range || busy} onClick={() => void start()} icon={<Ear className="h-3.5 w-3.5" />}>{t('axisNoise.start')}</Button>
            </DialogFooter>
          </div>
        )}

        {phase === 'running' && (
          <div className="space-y-4">
            <div className="flex items-center justify-between text-sm"><span>{t('axisNoise.progress')}</span><span className="font-semibold tabular-nums">{progress}%</span></div>
            <div className="h-2 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-primary transition-[width] duration-300" style={{ width: `${progress}%` }} /></div>
            <div aria-live="polite" aria-atomic="true" className="flex items-start gap-2 rounded-lg border border-border/70 bg-muted/20 px-3 py-2 text-xs leading-relaxed text-muted-foreground"><Info className="mt-0.5 h-4 w-4 shrink-0 text-primary" /><span>{t('axisNoise.runningReminder')}</span></div>
            <div className="py-5 text-center"><div className="text-xs text-muted-foreground">{t('axisNoise.current')}</div><div className="mt-1 text-3xl font-semibold tabular-nums">{current?.actual || current?.requested || '--'} {fanSpeedDisplaySuffix(range?.unit)}</div><div className={confirmingNoise ? 'mt-2 text-sm font-medium text-foreground' : 'mt-2 text-sm text-muted-foreground'}>{t(confirmingNoise ? 'axisNoise.confirmRatePrompt' : 'axisNoise.ratePrompt')}</div></div>
            <div className="grid grid-cols-1 gap-2 min-[560px]:grid-cols-3">
              <Button variant="secondary" disabled={busy} onClick={() => void rateCurrent('none')} icon={<Check className="h-3.5 w-3.5" />}>{t('axisNoise.severity.none')}</Button>
              <Button variant="outline" disabled={busy} onClick={() => void rateCurrent('mild')} icon={<Ear className="h-3.5 w-3.5" />}>{t('axisNoise.severity.mild')}</Button>
              <Button variant="primary" disabled={busy} onClick={() => void rateCurrent('obvious')} icon={<TriangleAlert className="h-3.5 w-3.5" />}>{t('axisNoise.severity.obvious')}</Button>
            </div>
            <DialogFooter><Button variant="danger" size="sm" onClick={() => void discard()} icon={<X className="h-3.5 w-3.5" />}>{t('axisNoise.stopDiscard')}</Button></DialogFooter>
          </div>
        )}

        {phase === 'confirm' && (
          <div className="space-y-4">
            <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm leading-relaxed">{t('axisNoise.confirmation')}</div>
            <div className="flex items-start gap-2 rounded-lg border border-border/70 bg-muted/20 px-3 py-2 text-xs text-muted-foreground"><Clock3 className="mt-0.5 h-4 w-4 shrink-0 text-primary" /><span>{t('axisNoise.estimatedTime', { count: plannedSteps.length, minutes: estimatedMinutes })}</span></div>
            <label className="flex items-start gap-2 text-sm text-muted-foreground">
              <input type="checkbox" checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)} className="mt-1" />
              <span>{t('axisNoise.confirmAcknowledgement')}</span>
            </label>
            <DialogFooter>
              <Button variant="secondary" size="sm" onClick={() => setPhase('setup')} icon={<X className="h-3.5 w-3.5" />}>{t('common.actions.cancel')}</Button>
              <Button variant="primary" size="sm" disabled={!acknowledged || busy} onClick={() => void start()} icon={<Check className="h-3.5 w-3.5" />}>{t('axisNoise.confirmStart')}</Button>
            </DialogFooter>
          </div>
        )}

        {phase === 'result' && result && (
          <div className="space-y-4">
            <div className="flex items-center gap-3 border-b border-border/70 pb-3"><ShieldCheck className="h-5 w-5 text-primary" /><div><div className="text-sm font-medium">{t('axisNoise.result.title')}</div><div className="text-xs text-muted-foreground">{t('axisNoise.result.description', { count: result.zones.length })}</div></div></div>
            {result.zones.length > 0 ? <div className="space-y-2">{result.zones.map((zone) => <div key={`${zone.min}-${zone.max}`} className="flex items-center justify-between rounded-xl border border-border/70 bg-muted/20 px-3 py-2 text-sm"><span className="tabular-nums">{zone.min}–{zone.max} {fanSpeedDisplaySuffix(result.unit)}</span><Badge variant={zone.severity === 'obvious' ? 'warning' : 'info'}>{t(`axisNoise.severity.${zone.severity}`)}</Badge></div>)}</div> : <div className="text-sm text-muted-foreground">{t('axisNoise.result.none')}</div>}
            <div className="flex items-start gap-2 rounded-lg border border-border/70 bg-muted/20 px-3 py-2 text-xs leading-relaxed text-muted-foreground"><Info className="mt-0.5 h-4 w-4 shrink-0 text-primary" /><span>{t('axisNoise.result.automaticOnly')}</span></div>
            <DialogFooter><Button variant="primary" size="sm" onClick={() => handleOpenChange(false)} icon={<Check className="h-3.5 w-3.5" />}>{t('common.actions.close')}</Button></DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
