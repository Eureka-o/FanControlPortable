'use client';

import type { CSSProperties, PointerEvent as ReactPointerEvent } from 'react';
import { useEffect, useRef, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { Minus, Pause, Play, RotateCcw, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { create } from 'zustand';
import { apiService, type UpdateProgressPayload } from '../services/api';

type UpdateStage = 'idle' | UpdateProgressPayload['stage'];

export interface UpdateRequest {
  downloadURL: string;
  expectedSHA256: string;
  windowTitle: string;
  windowBody: string;
  windowRestarting: string;
}

interface UpdateState {
  stage: UpdateStage;
  percent: number;
  received: number;
  total: number;
  message: string;
  attempt: number;
  maxAttempts: number;
  request: UpdateRequest | null;
  startUpdate: (request: UpdateRequest) => Promise<void>;
  retryUpdate: () => Promise<void>;
  pauseUpdate: () => Promise<void>;
  resumeUpdate: () => Promise<void>;
  cancelUpdate: () => Promise<void>;
  dismissUpdate: () => void;
  handleProgress: (progress: UpdateProgressPayload) => void;
}

const isBusy = (stage: UpdateStage) => stage === 'downloading' || stage === 'paused' || stage === 'retrying' || stage === 'installing';

const errorMessage = (error: unknown) => error instanceof Error ? error.message : String(error);

export const useUpdateStore = create<UpdateState>((set, get) => ({
  stage: 'idle',
  percent: 0,
  received: 0,
  total: 0,
  message: '',
  attempt: 0,
  maxAttempts: 3,
  request: null,
  startUpdate: async (request) => {
    if (isBusy(get().stage)) return;
    const sameDownload = get().request?.downloadURL === request.downloadURL
      && get().request?.expectedSHA256 === request.expectedSHA256;
    set({
      stage: 'downloading',
      percent: sameDownload ? get().percent : 0,
      received: sameDownload ? get().received : 0,
      total: sameDownload ? get().total : 0,
      message: '',
      attempt: 1,
      request,
    });
    try {
      await apiService.downloadAndInstallUpdate(
        request.downloadURL,
        request.windowTitle,
        request.windowBody,
        request.windowRestarting,
        request.expectedSHA256,
      );
    } catch (error) {
      if (get().stage !== 'error' && get().stage !== 'canceled') {
        set({ stage: 'error', message: errorMessage(error) });
      }
    }
  },
  retryUpdate: async () => {
    const request = get().request;
    if (request) await get().startUpdate(request);
  },
  pauseUpdate: async () => {
    if (get().stage !== 'downloading' && get().stage !== 'retrying') return;
    if (await apiService.pauseUpdateDownload()) {
      set({ stage: 'paused' });
    }
  },
  resumeUpdate: async () => {
    if (get().stage !== 'paused') return;
    if (await apiService.resumeUpdateDownload()) {
      set({ stage: 'downloading' });
    }
  },
  cancelUpdate: async () => {
    const request = get().request;
    if (!request || get().stage === 'installing') return;
    await apiService.cancelUpdateDownload(request.downloadURL);
    set({ stage: 'canceled', percent: 0, received: 0, total: 0, message: '' });
  },
  dismissUpdate: () => set({ stage: 'idle', percent: 0, received: 0, total: 0, message: '', request: null }),
  handleProgress: (progress) => {
    if (!progress || !progress.stage) return;
    if (progress.stage === 'canceled') {
      set({ stage: 'canceled', percent: 0, received: 0, total: 0, message: '' });
      return;
    }
    set((state) => ({
      stage: progress.stage,
      percent: progress.percent >= 0 ? Math.max(0, Math.min(100, progress.percent)) : state.percent,
      received: progress.received > 0 ? progress.received : state.received,
      total: progress.total > 0 ? progress.total : state.total,
      message: progress.message || '',
      attempt: progress.attempt || state.attempt,
      maxAttempts: progress.maxAttempts || state.maxAttempts,
    }));
  },
}));

function formatBytes(value: number) {
  if (!value || value < 0) return '';
  if (value < 1024 * 1024) return `${Math.round(value / 1024)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function ProgressRing({ percent, error }: { percent: number; error: boolean }) {
  const radius = 20;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference * (1 - percent / 100);

  return (
    <div className="relative size-12 shrink-0" aria-label={`${percent}%`}>
      <svg className="size-12 -rotate-90" viewBox="0 0 48 48" aria-hidden="true">
        <circle cx="24" cy="24" r={radius} fill="none" stroke="currentColor" strokeWidth="3" className="text-border/80" />
        <circle
          cx="24"
          cy="24"
          r={radius}
          fill="none"
          stroke="currentColor"
          strokeWidth="3"
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          className={`transition-[stroke-dashoffset] duration-200 ease-out ${error ? 'text-destructive' : 'text-primary'}`}
        />
      </svg>
      <span className="absolute inset-0 flex items-center justify-center text-[10px] font-semibold tabular-nums text-foreground">
        {percent}%
      </span>
    </div>
  );
}

export default function UpdateProgressWidget() {
  const { t } = useTranslation();
  const stage = useUpdateStore((state) => state.stage);
  const percent = useUpdateStore((state) => state.percent);
  const received = useUpdateStore((state) => state.received);
  const total = useUpdateStore((state) => state.total);
  const message = useUpdateStore((state) => state.message);
  const attempt = useUpdateStore((state) => state.attempt);
  const maxAttempts = useUpdateStore((state) => state.maxAttempts);
  const retryUpdate = useUpdateStore((state) => state.retryUpdate);
  const pauseUpdate = useUpdateStore((state) => state.pauseUpdate);
  const resumeUpdate = useUpdateStore((state) => state.resumeUpdate);
  const cancelUpdate = useUpdateStore((state) => state.cancelUpdate);
  const dismissUpdate = useUpdateStore((state) => state.dismissUpdate);
  const handleProgress = useUpdateStore((state) => state.handleProgress);
  const [collapsed, setCollapsed] = useState(false);
  const [commandPending, setCommandPending] = useState(false);
  const [position, setPosition] = useState<{ x: number; y: number } | null>(null);
  const widgetRef = useRef<HTMLElement | null>(null);
  const dragRef = useRef<{ offsetX: number; offsetY: number; startX: number; startY: number; moved: boolean } | null>(null);
  const suppressCollapsedClickRef = useRef(false);
  const completionCheckedRef = useRef(false);

  useEffect(() => apiService.onUpdateDownloadProgress(handleProgress), [handleProgress]);

  useEffect(() => {
    if (completionCheckedRef.current) return;
    completionCheckedRef.current = true;
    apiService.updateCompletedOnLaunch()
      .then((completed) => {
        if (completed) toast.success(t('aboutPanel.version.updateComplete'));
      })
      .catch(() => undefined);
  }, [t]);

  useEffect(() => {
    if (stage === 'error') setCollapsed(false);
  }, [stage]);

  const handlePointerDown = (event: ReactPointerEvent<HTMLElement>) => {
    const button = (event.target as HTMLElement).closest('button');
    if (button && !button.hasAttribute('data-update-drag-handle')) return;
    suppressCollapsedClickRef.current = false;
    const rect = event.currentTarget.getBoundingClientRect();
    dragRef.current = {
      offsetX: event.clientX - rect.left,
      offsetY: event.clientY - rect.top,
      startX: event.clientX,
      startY: event.clientY,
      moved: false,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const handlePointerMove = (event: ReactPointerEvent<HTMLElement>) => {
    const drag = dragRef.current;
    const widget = widgetRef.current;
    if (!drag || !widget) return;
    const rect = widget.getBoundingClientRect();
    if (Math.abs(event.clientX - drag.startX) > 3 || Math.abs(event.clientY - drag.startY) > 3) {
      drag.moved = true;
    }
    setPosition({
      x: Math.max(8, Math.min(event.clientX - drag.offsetX, window.innerWidth - rect.width - 8)),
      y: Math.max(48, Math.min(event.clientY - drag.offsetY, window.innerHeight - rect.height - 8)),
    });
  };

  const handlePointerUp = (event: ReactPointerEvent<HTMLElement>) => {
    if (dragRef.current?.moved) {
      suppressCollapsedClickRef.current = true;
    }
    dragRef.current = null;
    try {
      event.currentTarget.releasePointerCapture(event.pointerId);
    } catch {
      // Pointer capture may already be released when the window loses focus.
    }
  };

  const runCommand = async (command: () => Promise<void>) => {
    if (commandPending) return;
    setCommandPending(true);
    try {
      await command();
    } catch (error) {
      toast.error(errorMessage(error));
    } finally {
      setCommandPending(false);
    }
  };

  const titleKey = stage === 'paused'
    ? 'aboutPanel.version.progressPaused'
    : stage === 'canceled'
      ? 'aboutPanel.version.progressCanceled'
      : stage === 'retrying'
    ? 'aboutPanel.version.progressRetrying'
    : stage === 'installing'
      ? 'aboutPanel.version.progressInstalling'
      : stage === 'error'
        ? 'aboutPanel.version.progressFailed'
        : 'aboutPanel.version.progressDownloading';
  const detail = stage === 'paused'
    ? t('aboutPanel.version.pausedHint')
    : stage === 'canceled'
      ? t('aboutPanel.version.canceledHint')
      : stage === 'retrying'
    ? t('aboutPanel.version.retryingHint', { attempt, max: maxAttempts })
    : stage === 'installing'
      ? t('aboutPanel.version.installingHint')
      : stage === 'error'
        ? message
        : total > 0
          ? `${formatBytes(received)} / ${formatBytes(total)}`
          : t('aboutPanel.version.downloadingHint');
  const positionStyle = position ? { left: position.x, top: position.y, right: 'auto' } : undefined;

  return (
    <AnimatePresence>
      {stage !== 'idle' && (
        <motion.aside
          ref={widgetRef}
          role="status"
          aria-live="polite"
          initial={{ opacity: 0, scale: 0.94, y: -6 }}
          animate={{ opacity: 1, scale: 1, y: 0 }}
          exit={{ opacity: 0, scale: 0.94, y: -6 }}
          transition={{ duration: 0.2, ease: [0.22, 1, 0.36, 1] }}
          onPointerDown={handlePointerDown}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
          onPointerCancel={handlePointerUp}
          className={`fixed right-4 top-14 cursor-grab select-none active:cursor-grabbing ${collapsed ? 'rounded-full' : 'w-[304px] rounded-xl'} border border-border/75 bg-card/95 p-2.5 shadow-xl shadow-black/10 backdrop-blur-xl`}
          style={{ ...positionStyle, zIndex: 'var(--layer-floating-popover)', '--wails-draggable': 'no-drag' } as CSSProperties}
        >
          {collapsed ? (
            <button
              type="button"
              data-update-drag-handle
              aria-label={t(titleKey)}
              title={t(titleKey)}
              onClick={() => {
                if (suppressCollapsedClickRef.current) {
                  suppressCollapsedClickRef.current = false;
                  return;
                }
                setCollapsed(false);
              }}
              className="block cursor-pointer rounded-full"
            >
              <ProgressRing percent={percent} error={stage === 'error'} />
            </button>
          ) : (
            <div className="flex items-center gap-3">
              <div className={isBusy(stage) ? 'motion-safe:animate-pulse' : ''}>
                <ProgressRing percent={percent} error={stage === 'error'} />
              </div>
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm font-semibold text-foreground">{t(titleKey)}</div>
                <div className={`mt-1 truncate text-[11px] ${stage === 'error' ? 'text-destructive' : 'text-muted-foreground'}`} title={detail}>
                  {detail}
                </div>
              </div>
              <div className="flex shrink-0 items-center gap-1">
                {stage === 'error' && (
                  <button
                    type="button"
                    aria-label={t('common.actions.retry')}
                    title={t('common.actions.retry')}
                    onClick={() => void retryUpdate()}
                    className="inline-flex size-8 cursor-pointer items-center justify-center rounded-lg text-destructive transition-colors hover:bg-destructive/10"
                  >
                    <RotateCcw className="size-3.5" />
                  </button>
                )}
                {(stage === 'downloading' || stage === 'retrying' || stage === 'paused') && (
                  <button
                    type="button"
                    aria-label={t(stage === 'paused' ? 'aboutPanel.version.resumeDownload' : 'aboutPanel.version.pauseDownload')}
                    title={t(stage === 'paused' ? 'aboutPanel.version.resumeDownload' : 'aboutPanel.version.pauseDownload')}
                    disabled={commandPending}
                    onClick={() => void runCommand(stage === 'paused' ? resumeUpdate : pauseUpdate)}
                    className="inline-flex size-8 cursor-pointer items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {stage === 'paused' ? <Play className="size-3.5" /> : <Pause className="size-3.5" />}
                  </button>
                )}
                {(stage === 'downloading' || stage === 'retrying' || stage === 'paused' || stage === 'error') && (
                  <button
                    type="button"
                    aria-label={t('aboutPanel.version.cancelDownload')}
                    title={t('aboutPanel.version.cancelDownload')}
                    disabled={commandPending}
                    onClick={() => void runCommand(cancelUpdate)}
                    className="inline-flex size-8 cursor-pointer items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <X className="size-3.5" />
                  </button>
                )}
                {stage === 'canceled' && (
                  <button
                    type="button"
                    aria-label={t('common.actions.close')}
                    title={t('common.actions.close')}
                    onClick={dismissUpdate}
                    className="inline-flex size-8 cursor-pointer items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  >
                    <X className="size-3.5" />
                  </button>
                )}
                <button
                  type="button"
                  aria-label={t('aboutPanel.version.minimizeProgress')}
                  title={t('aboutPanel.version.minimizeProgress')}
                  onClick={() => setCollapsed(true)}
                  className="inline-flex size-8 cursor-pointer items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                >
                  <Minus className="size-3.5" />
                </button>
              </div>
            </div>
          )}
        </motion.aside>
      )}
    </AnimatePresence>
  );
}
