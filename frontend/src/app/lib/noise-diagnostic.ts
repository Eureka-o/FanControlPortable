export interface NoiseDiagnosticRange {
  unit: 'rpm' | 'percent' | string;
  min: number;
  max: number;
  step: number;
  minSource?: string;
  maxSource?: string;
}

export interface NoiseDiagnosticPoint {
  requested: number;
  actual: number;
  levelDb: number;
  spreadDb: number;
  valid: boolean;
  reason?: string;
}

export interface NoiseSample {
  levelDb: number;
  spreadDb: number;
  rangeDb?: number;
  validFrameRatio: number;
  frames: number;
  retryable: boolean;
  reason?: string;
}

export interface MicrophoneOption {
  deviceId: string;
  label: string;
}

export interface NoiseDiagnosticAnalysis {
  lowRiseDb: number;
  highRiseDb: number;
  riseDb: number;
  knee: number;
  confidence: 'low' | 'medium' | 'high';
  confidenceReason: string;
  suspectedPeak?: number;
}

export function deriveNoiseDiagnosticRange(
  profile: { id?: string; model?: string; transport?: string; speedUnit?: string; speedRange?: { min?: number; max?: number; step?: number } } | null | undefined,
  capabilities: { speedUnit?: string; speedRange?: { min?: number; max?: number; step?: number } } | null | undefined,
  flyDigiCapability?: { available?: boolean; maxRpm?: number } | null,
): NoiseDiagnosticRange | null {
  const unit = String(profile?.speedUnit || capabilities?.speedUnit || 'percent').toLowerCase() === 'rpm' ? 'rpm' : 'percent';
  const profileRange = profile?.speedRange || capabilities?.speedRange || {};
  const max = Number(profileRange.max || 0);
  if (!Number.isFinite(max) || max <= 0) return null;
  const isFlyDigi = String(profile?.id || '').toLowerCase().includes('flydigi')
    || String(profile?.model || '').toLowerCase().includes('bs1')
    || String(profile?.model || '').toLowerCase().includes('bs2');
  const min = unit === 'percent' ? 5 : isFlyDigi ? 1000 : Number(profileRange.min || 0);
  const runtimeMax = unit === 'rpm' && flyDigiCapability?.available && Number(flyDigiCapability.maxRpm) > 0
    ? Math.min(max, Number(flyDigiCapability.maxRpm))
    : max;
  if (runtimeMax <= min) return null;
  return { unit, min, max: runtimeMax, step: Math.max(1, Number(profileRange.step || 1)), minSource: unit === 'percent' ? 'percent-diagnostic-floor' : isFlyDigi ? 'flydigi-diagnostic-floor' : 'profile', maxSource: runtimeMax < max ? 'runtime-capability' : 'profile' };
}

const MIN_DB = -100;
const MAX_DB = 0;

function finite(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

function clampDb(value: number) {
  return Math.max(MIN_DB, Math.min(MAX_DB, value));
}

export function percentile(values: number[], ratio: number) {
  const sorted = values.filter(Number.isFinite).sort((a, b) => a - b);
  if (sorted.length === 0) return 0;
  const index = Math.min(sorted.length - 1, Math.max(0, ratio * (sorted.length - 1)));
  const left = Math.floor(index);
  const right = Math.ceil(index);
  if (left === right) return sorted[left];
  return sorted[left] + (sorted[right] - sorted[left]) * (index - left);
}

export function robustSpread(values: number[]) {
  return Math.max(0, percentile(values, 0.9) - percentile(values, 0.1));
}

export function levelFromRms(rms: number) {
  if (!finite(rms) || rms <= 0) return MIN_DB;
  return clampDb(20 * Math.log10(rms));
}

export function evaluateNoiseBaseline(levels: number[], minFrames = 8): NoiseSample {
  const valid = levels.filter((level) => finite(level) && level >= MIN_DB && level <= MAX_DB);
  const ratio = levels.length > 0 ? valid.length / levels.length : 0;
  if (valid.length < minFrames || ratio < 0.6) {
    return {
      levelDb: 0,
      spreadDb: robustSpread(valid),
      rangeDb: robustSpread(valid),
      validFrameRatio: ratio,
      frames: valid.length,
      retryable: true,
      reason: 'baseline-unstable',
    };
  }
  const spreadDb = robustSpread(valid);
  return {
    levelDb: percentile(valid, 0.5),
    spreadDb,
    rangeDb: spreadDb,
    validFrameRatio: ratio,
    frames: valid.length,
    retryable: spreadDb > 12,
    reason: spreadDb > 12 ? 'baseline-drift' : undefined,
  };
}

export function buildDiagnosticSteps(range: NoiseDiagnosticRange): number[] {
  const min = Math.ceil(range.min);
  const max = Math.floor(range.max);
  if (!Number.isFinite(min) || !Number.isFinite(max) || max <= min) return [];
  const span = max - min;
  const count = Math.min(10, Math.max(5, Math.ceil(span / Math.max(range.step || 1, span / 8)) + 1));
  const step = span / (count - 1);
  const values = Array.from({ length: count }, (_, index) => Math.round(min + step * index));
  return [...new Set(values)].filter((value) => value >= min && value <= max);
}

export function analyzeNoiseDiagnostic(
  points: NoiseDiagnosticPoint[],
  initialBaseline: number,
  finalBaseline: number,
): NoiseDiagnosticAnalysis {
  const valid = points
    .filter((point) => point.valid && finite(point.actual) && finite(point.levelDb))
    .sort((left, right) => left.actual - right.actual);
  if (valid.length < 3) {
    return { lowRiseDb: 0, highRiseDb: 0, riseDb: 0, knee: 0, confidence: 'low', confidenceReason: 'too-few-points' };
  }

  const baseline = percentile([initialBaseline, finalBaseline].filter(finite), 0.5);
  const rises = valid.map((point) => ({ point, rise: point.levelDb - baseline }));
  const split = Math.max(1, Math.floor(rises.length / 3));
  const lowRiseDb = percentile(rises.slice(0, split).map((item) => item.rise), 0.5);
  const highRiseDb = percentile(rises.slice(-split).map((item) => item.rise), 0.5);
  const riseDb = Math.max(0, highRiseDb - lowRiseDb);

  let knee = rises[0].point.actual;
  let largestSlope = Number.NEGATIVE_INFINITY;
  let suspectedPeak: number | undefined;
  for (let index = 1; index < rises.length; index += 1) {
    const deltaSpeed = Math.max(1, rises[index].point.actual - rises[index - 1].point.actual);
    const slope = (rises[index].rise - rises[index - 1].rise) / deltaSpeed;
    if (slope > largestSlope) {
      largestSlope = slope;
      knee = rises[index].point.actual;
    }
    if (rises[index].rise > rises[index - 1].rise + 6) {
      suspectedPeak = rises[index].point.actual;
    }
  }

  const baselineDrift = Math.abs(finalBaseline - initialBaseline);
  const confidence = valid.length >= 6 && baselineDrift <= 6 && valid.every((point) => point.spreadDb <= 12)
    ? 'high'
    : valid.length >= 4 && baselineDrift <= 12
      ? 'medium'
      : 'low';
  const confidenceReason = baselineDrift > 12
    ? 'baseline-drift'
    : valid.length < 6
      ? 'limited-points'
      : 'stable-samples';
  return { lowRiseDb: Math.max(0, lowRiseDb), highRiseDb: Math.max(0, highRiseDb), riseDb, knee, confidence, confidenceReason, suspectedPeak };
}

export class NoiseMeter {
  private stream: MediaStream | null = null;
  private context: AudioContext | null = null;
  private analyser: AnalyserNode | null = null;
  private frequencyData: Float32Array | null = null;
  private weights: Float64Array | null = null;
  private closed = false;

  static async listMicrophones(): Promise<MicrophoneOption[]> {
    if (!navigator.mediaDevices?.getUserMedia) throw new Error('media-devices-unavailable');
    const probe = await navigator.mediaDevices.getUserMedia({ audio: true, video: false });
    try {
      const devices = await navigator.mediaDevices.enumerateDevices();
      return devices.filter((device) => device.kind === 'audioinput' && device.deviceId).map((device, index) => ({
        deviceId: device.deviceId,
        label: device.label || `Microphone ${index + 1}`,
      }));
    } finally {
      probe.getTracks().forEach((track) => track.stop());
    }
  }

  static async open(deviceId?: string): Promise<NoiseMeter> {
    const meter = new NoiseMeter();
    const media = await navigator.mediaDevices.getUserMedia({
      audio: {
        ...(deviceId ? { deviceId: { exact: deviceId } } : {}),
        echoCancellation: false,
        noiseSuppression: false,
        autoGainControl: false,
      },
      video: false,
    });
    meter.stream = media;
    const AudioContextCtor = window.AudioContext || (window as typeof window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
    if (!AudioContextCtor) {
      meter.close();
      throw new Error('audio-context-unavailable');
    }
    meter.context = new AudioContextCtor();
    meter.analyser = meter.context.createAnalyser();
    meter.analyser.fftSize = 4096;
    meter.analyser.smoothingTimeConstant = 0.4;
    meter.context.createMediaStreamSource(media).connect(meter.analyser);
    const binHz = meter.context.sampleRate / meter.analyser.fftSize;
    meter.frequencyData = new Float32Array(meter.analyser.frequencyBinCount);
    meter.weights = new Float64Array(meter.analyser.frequencyBinCount);
    for (let index = 0; index < meter.weights.length; index += 1) {
      const frequency = index * binHz;
      meter.weights[index] = frequency >= 40 && frequency <= 16000 ? aWeightDb(frequency) : Number.NEGATIVE_INFINITY;
    }
    if (meter.context.state === 'suspended') await meter.context.resume();
    return meter;
  }

  private readLevel() {
    if (!this.analyser || !this.frequencyData || !this.weights) return MIN_DB;
    this.analyser.getFloatFrequencyData(this.frequencyData as Float32Array<ArrayBuffer>);
    let energy = 0;
    for (let index = 1; index < this.frequencyData.length; index += 1) {
      const weight = this.weights[index];
      const value = this.frequencyData[index];
      if (!Number.isFinite(weight) || !Number.isFinite(value)) continue;
      energy += 10 ** ((value + weight) / 10);
    }
    return energy > 0 ? clampDb(10 * Math.log10(energy)) : MIN_DB;
  }

  async sampleLevel(durationMs = 900, discardMs = 180, signal?: AbortSignal): Promise<NoiseSample> {
    if (!this.analyser || this.closed) throw new Error('noise-meter-closed');
    const samples: number[] = [];
    const started = performance.now();
    while (performance.now() - started < durationMs) {
      if (signal?.aborted) throw new DOMException('noise test aborted', 'AbortError');
      if (performance.now() - started >= discardMs) samples.push(this.readLevel());
      await new Promise<void>((resolve, reject) => {
        const timer = window.setTimeout(() => { signal?.removeEventListener('abort', onAbort); resolve(); }, 50);
        const onAbort = () => { window.clearTimeout(timer); signal?.removeEventListener('abort', onAbort); reject(new DOMException('noise test aborted', 'AbortError')); };
        signal?.addEventListener('abort', onAbort, { once: true });
      });
    }
    return evaluateNoiseBaseline(samples);
  }

  close() {
    if (this.closed) return;
    this.closed = true;
    this.stream?.getTracks().forEach((track) => track.stop());
    this.stream = null;
    if (this.context) void this.context.close();
    this.context = null;
    this.analyser = null;
    this.frequencyData = null;
    this.weights = null;
  }
}

function aWeightDb(frequency: number) {
  const f2 = frequency * frequency;
  const ra = (12194 ** 2 * f2 * f2) / ((f2 + 20.6 ** 2) * Math.sqrt((f2 + 107.7 ** 2) * (f2 + 737.9 ** 2)) * (f2 + 12194 ** 2));
  return 20 * Math.log10(ra) + 2;
}
