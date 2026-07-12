'use client';

import type { CSSProperties, KeyboardEvent, ReactNode } from 'react';
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Copy,
  Download,
  LineChart,
  LayoutGrid,
  Minus,
  Settings2,
  Square,
  TriangleAlert,
  X,
  Fan,
  Thermometer,
  Sparkles,
  Info,
  Wifi,
  WifiOff,
  Bluetooth,
  Boxes,
  Usb,
} from 'lucide-react';
import { Environment, Quit, WindowIsMaximised, WindowMinimise, WindowToggleMaximise } from '../../../wailsjs/runtime/runtime';
import { types } from '../../../wailsjs/go/models';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { BRAND } from '../lib/brand';
import { clampFanSpeedToRange, fanSpeedUnitLabel, getFanSpeedRange, getFanSpeedUnit, readCurrentFanSpeed } from '../lib/fan-speed';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';

const MAIN_TAB_ITEMS = [
  { id: 'status', titleKey: 'appShell.tabs.status', icon: LayoutGrid },
  { id: 'curve', titleKey: 'appShell.tabs.curve', icon: LineChart },
  { id: 'control', titleKey: 'appShell.tabs.control', icon: Settings2 },
  { id: 'devices', titleKey: 'appShell.tabs.devices', icon: Boxes },
] as const;

const ABOUT_TAB = { id: 'about', titleKey: 'appShell.tabs.about', icon: Info } as const;

type ActiveTab = (typeof MAIN_TAB_ITEMS)[number]['id'] | typeof ABOUT_TAB.id;

const TAB_TRANSITION_ORDER: ActiveTab[] = [...MAIN_TAB_ITEMS.map((tab) => tab.id), ABOUT_TAB.id];

function getTabTransitionDirection(fromTab: ActiveTab, toTab: ActiveTab) {
  const fromIndex = TAB_TRANSITION_ORDER.indexOf(fromTab);
  const toIndex = TAB_TRANSITION_ORDER.indexOf(toTab);
  if (fromIndex === -1 || toIndex === -1 || fromIndex === toIndex) {
    return 0;
  }
  return toIndex > fromIndex ? 1 : -1;
}

const TAB_CONTENT_VARIANTS = {
  enter: (direction: number) => ({
    opacity: 0,
    y: direction === 0 ? 8 : direction * 18,
  }),
  center: {
    opacity: 1,
    y: 0,
  },
  exit: (direction: number) => ({
    opacity: 0,
    y: direction === 0 ? -6 : direction * -14,
  }),
};

interface AppShellProps {
  activeTab: ActiveTab;
  onTabChange: (tab: ActiveTab) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  runtimeDeviceProfile?: types.DeviceProfile | null;
  config: types.AppConfig;
  autoControl: boolean;
  error: string | null;
  bridgeWarning: string | null;
  diagnosticsExporting?: boolean;
  onExportDiagnostics?: () => void;
  onDismissBridgeWarning: () => void;
  statusContent: ReactNode;
  curveContent: ReactNode;
  controlContent: ReactNode;
  devicesContent: ReactNode;
  aboutContent: ReactNode;
}

function getTempColor(temp?: number) {
  if (!temp) return 'text-muted-foreground';
  if (temp > 80) return 'text-red-500';
  if (temp > 70) return 'text-amber-500';
  return 'text-primary';
}

function getFanSpinDuration(speed?: number, minSpeed = 0, maxSpeed = 100) {
  if (!speed || speed <= 0) return 0;
  const speedSpan = Math.max(1, maxSpeed - minSpeed);
  const percent = Math.max(0, Math.min(100, ((speed - minSpeed) / speedSpan) * 100));
  if (percent >= 90) return 0.48;
  if (percent >= 70) return 0.72;
  if (percent >= 45) return 1;
  return 1.35;
}

type WailsDragStyle = CSSProperties & { ['--wails-draggable']?: 'drag' | 'no-drag' };

const DRAG_STYLE: WailsDragStyle = { '--wails-draggable': 'drag' };
const NO_DRAG_STYLE: WailsDragStyle = { '--wails-draggable': 'no-drag' };

/* ──────────────────────────────────────────────────────────────
 * TitleBar — slim, fixed at the very top of the window.
 * Outside the scroll viewport, so window controls never scroll.
 * ────────────────────────────────────────────────────────────── */

function TitleBarButton({
  icon,
  label,
  onClick,
  danger = false,
}: {
  icon: ReactNode;
  label: string;
  onClick: () => void;
  danger?: boolean;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      style={NO_DRAG_STYLE}
      onClick={(event) => {
        event.stopPropagation();
        onClick();
      }}
      className={clsx(
        'flex h-8 w-10 cursor-pointer items-center justify-center rounded-md text-muted-foreground transition-colors',
        danger
          ? 'hover:bg-red-500 hover:text-white'
          : 'hover:bg-foreground/10 hover:text-foreground',
      )}
    >
      {icon}
    </button>
  );
}

function TitleBar({
  minimizeLabel,
  maximizeLabel,
  restoreLabel,
  closeLabel,
  isMaximised,
  leftSlot,
  onMinimise,
  onToggleMaximise,
  onClose,
}: {
  minimizeLabel: string;
  maximizeLabel: string;
  restoreLabel: string;
  closeLabel: string;
  isMaximised: boolean;
  leftSlot?: ReactNode;
  onMinimise: () => void;
  onToggleMaximise: () => void;
  onClose: () => void;
}) {
  return (
    <div
      className="glacier-titlebar pointer-events-auto absolute left-16 right-0 top-0 flex h-10 items-center justify-between bg-background"
      style={{ ...DRAG_STYLE, zIndex: 'var(--layer-titlebar)' }}
      onDoubleClick={onToggleMaximise}
    >
      <div className="flex h-full min-w-0 flex-1 items-center px-3 pt-1">
        {leftSlot}
      </div>
      <div className="flex h-full items-center gap-0.5 pr-1" style={NO_DRAG_STYLE}>
        <TitleBarButton icon={<Minus className="h-3.5 w-3.5" />} label={minimizeLabel} onClick={onMinimise} />
        <TitleBarButton
          icon={isMaximised ? <Copy className="h-3 w-3" /> : <Square className="h-3 w-3" />}
          label={isMaximised ? restoreLabel : maximizeLabel}
          onClick={onToggleMaximise}
        />
        <TitleBarButton icon={<X className="h-3.5 w-3.5" />} label={closeLabel} onClick={onClose} danger />
      </div>
    </div>
  );
}

function StatusBadges({
  isConnected,
  fanData,
  temperature,
  runtimeDeviceProfile,
  config,
  autoControl,
  compact = false,
}: {
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  runtimeDeviceProfile?: types.DeviceProfile | null;
  config: types.AppConfig;
  autoControl: boolean;
  compact?: boolean;
}) {
  const { t } = useTranslation();
  const fanSpeedUnit = getFanSpeedUnit(fanData as any, config as any, runtimeDeviceProfile as any);
  const fanSpeedRange = getFanSpeedRange(config as any, fanSpeedUnit, runtimeDeviceProfile as any);
  const fanSpeed = clampFanSpeedToRange(readCurrentFanSpeed(fanData, fanSpeedUnit, config as any, runtimeDeviceProfile as any), fanSpeedRange);
  const fanSpeedLabel = fanSpeedUnitLabel(fanSpeedUnit);
  const fanSpinDuration = getFanSpinDuration(fanSpeed, fanSpeedRange.min, fanSpeedRange.max);
  const baseClass = compact
    ? 'inline-flex h-6 items-center gap-1.5 rounded-full border px-2.5 text-[11px] font-medium'
    : 'inline-flex h-8 items-center gap-1.5 rounded-xl border px-3 text-[13px] font-medium';
  const fanSpinStyle = fanSpinDuration ? { animationDuration: `${fanSpinDuration}s` } : undefined;
  const transport = String(
    runtimeDeviceProfile?.transport
      || (fanData as any)?.transport
      || (config as any).deviceTransport
      || '',
  ).toLowerCase();
  const ConnectedIcon = transport === 'ble'
    ? Bluetooth
    : transport === 'hid' || transport === 'serial'
      ? Usb
      : Wifi;

  return (
    <div
      className={clsx(
        'flex min-w-0 items-center gap-2 text-[13px] tabular-nums',
        compact && 'translate-y-px overflow-hidden whitespace-nowrap',
      )}
    >
      <span
        className={clsx(
          baseClass,
          isConnected
            ? 'border-primary/20 bg-primary/10 text-primary'
            : 'border-border bg-card text-muted-foreground',
        )}
      >
        {isConnected ? <ConnectedIcon className="h-3.5 w-3.5" /> : <WifiOff className="h-3.5 w-3.5" />}
        {isConnected ? t('appShell.status.connected') : t('appShell.status.offline')}
      </span>

      <span
        className={clsx(
          baseClass,
          autoControl ? 'border-primary/20 bg-primary/10 text-primary' : 'border-border bg-card text-muted-foreground',
        )}
      >
        <Sparkles className="h-3.5 w-3.5" />
        {autoControl ? t('appShell.status.smartControl') : t('appShell.status.manualMode')}
      </span>

      {isConnected && (
        <>
          <span className={clsx(baseClass, 'border-border bg-card font-semibold shadow-sm shadow-black/5')}>
            <Thermometer className={clsx('h-3.5 w-3.5', getTempColor(temperature?.maxTemp))} />
            <span className={clsx(getTempColor(temperature?.maxTemp))}>
              {temperature?.maxTemp ?? '--'}°C
            </span>
          </span>
          <span className={clsx(baseClass, 'border-border bg-card font-semibold text-primary shadow-sm shadow-black/5')}>
            <span className={clsx('inline-flex', fanSpinDuration && 'animate-spin')} style={fanSpinStyle}>
              <Fan className="h-3.5 w-3.5" />
            </span>
            {fanSpeed ?? '--'}{fanSpeedLabel}
          </span>
        </>
      )}
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────
 * OverlayScrollbar — floating thumb, never reserves width.
 * Native scrollbar is hidden via .app-scroll-root--hide-native.
 * ────────────────────────────────────────────────────────────── */

function OverlayScrollbar({
  scrollRef,
}: {
  scrollRef: React.RefObject<HTMLDivElement | null>;
}) {
  const trackRef = useRef<HTMLDivElement | null>(null);
  const thumbRef = useRef<HTMLDivElement | null>(null);
  const hideTimerRef = useRef<number | null>(null);
  const draggingRef = useRef<{ startY: number; startScroll: number } | null>(null);
  const [visible, setVisible] = useState(false);
  const [hasOverflow, setHasOverflow] = useState(false);

  const updateThumb = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;

    const { scrollTop, scrollHeight, clientHeight } = el;
    const overflow = scrollHeight - clientHeight;
    if (overflow <= 1) {
      setHasOverflow(false);
      setVisible(false);
      return;
    }
    setHasOverflow(true);

    const thumb = thumbRef.current;
    const track = trackRef.current;
    if (!thumb || !track) return;

    const trackHeight = track.clientHeight;
    const ratio = clientHeight / scrollHeight;
    const thumbHeight = Math.max(28, trackHeight * ratio);
    const maxThumbTop = trackHeight - thumbHeight;
    const top = (scrollTop / overflow) * maxThumbTop;
    thumb.style.height = `${thumbHeight}px`;
    thumb.style.transform = `translateY(${top}px)`;
  }, [scrollRef]);

  const flashVisible = useCallback(() => {
    setVisible(true);
    if (hideTimerRef.current) {
      window.clearTimeout(hideTimerRef.current);
    }
    hideTimerRef.current = window.setTimeout(() => {
      if (!draggingRef.current) {
        setVisible(false);
      }
    }, 1400);
  }, []);

  useLayoutEffect(() => {
    updateThumb();
  }, [hasOverflow, updateThumb]);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;

    const onActivity = () => {
      updateThumb();
      flashVisible();
    };

    el.addEventListener('scroll', onActivity, { passive: true });
    el.addEventListener('mouseenter', onActivity);
    el.addEventListener('wheel', onActivity, { passive: true });
    el.addEventListener('touchstart', onActivity, { passive: true });

    const ro = new ResizeObserver(() => updateThumb());
    ro.observe(el);
    const content = el.firstElementChild;
    if (content instanceof HTMLElement) {
      ro.observe(content);
    }

    updateThumb();
    if (el.scrollHeight - el.clientHeight > 1) {
      flashVisible();
    }

    return () => {
      el.removeEventListener('scroll', onActivity);
      el.removeEventListener('mouseenter', onActivity);
      el.removeEventListener('wheel', onActivity);
      el.removeEventListener('touchstart', onActivity);
      ro.disconnect();
      if (hideTimerRef.current) window.clearTimeout(hideTimerRef.current);
    };
  }, [scrollRef, updateThumb, flashVisible]);

  const handleThumbPointerDown = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      const el = scrollRef.current;
      if (!el) return;
      event.preventDefault();
      (event.target as HTMLElement).setPointerCapture(event.pointerId);
      draggingRef.current = { startY: event.clientY, startScroll: el.scrollTop };
      setVisible(true);
    },
    [scrollRef],
  );

  const handleThumbPointerMove = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      const drag = draggingRef.current;
      const el = scrollRef.current;
      const track = trackRef.current;
      const thumb = thumbRef.current;
      if (!drag || !el || !track || !thumb) return;
      const dy = event.clientY - drag.startY;
      const trackHeight = track.clientHeight;
      const thumbHeight = thumb.clientHeight;
      const maxThumbTop = trackHeight - thumbHeight;
      if (maxThumbTop <= 0) return;
      const overflow = el.scrollHeight - el.clientHeight;
      const scrollDelta = (dy / maxThumbTop) * overflow;
      el.scrollTop = drag.startScroll + scrollDelta;
    },
    [scrollRef],
  );

  const handleThumbPointerUp = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      draggingRef.current = null;
      try {
        (event.target as HTMLElement).releasePointerCapture(event.pointerId);
      } catch {
        /* noop */
      }
      flashVisible();
    },
    [flashVisible],
  );

  if (!hasOverflow) return null;

  return (
    <div
      ref={trackRef}
      className={clsx('app-overlay-scrollbar', visible && 'is-visible')}
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={flashVisible}
    >
      <div
        ref={thumbRef}
        className="app-overlay-scrollbar-thumb"
        onPointerDown={handleThumbPointerDown}
        onPointerMove={handleThumbPointerMove}
        onPointerUp={handleThumbPointerUp}
        onPointerCancel={handleThumbPointerUp}
      />
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────
 * AppShell — layout
 * ────────────────────────────────────────────────────────────── */

export default function AppShell({
  activeTab,
  onTabChange,
  isConnected,
  fanData,
  temperature,
  runtimeDeviceProfile,
  config,
  autoControl,
  error,
  bridgeWarning,
  diagnosticsExporting = false,
  onExportDiagnostics,
  onDismissBridgeWarning,
  statusContent,
  curveContent,
  controlContent,
  devicesContent,
  aboutContent,
}: AppShellProps) {
  const { t } = useTranslation();
  const [isWindowsChrome, setIsWindowsChrome] = useState(() => (
    typeof document !== 'undefined' && document.documentElement.dataset.os === 'win'
  ));
  const [isMaximised, setIsMaximised] = useState(false);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const previousActiveTabRef = useRef<ActiveTab>(activeTab);

  const syncWindowState = useCallback(async () => {
    try {
      setIsMaximised(await WindowIsMaximised());
    } catch {
      setIsMaximised(false);
    }
  }, []);

  useEffect(() => {
    let disposed = false;
    let cleanup = () => {};
    let resizeFrame: number | null = null;
    let resizeSyncing = false;
    let resizeQueued = false;

    const initializeWindowChrome = async () => {
      try {
        const env = await Environment();
        if (disposed) return;
        const isWindows = env.platform === 'windows';
        setIsWindowsChrome(isWindows);
        if (!isWindows) {
          setIsMaximised(false);
          return;
        }
        const handleResize = () => {
          if (resizeFrame !== null) return;
          resizeFrame = window.requestAnimationFrame(async () => {
            resizeFrame = null;
            if (resizeSyncing) {
              resizeQueued = true;
              return;
            }
            resizeSyncing = true;
            await syncWindowState();
            resizeSyncing = false;
            if (resizeQueued && !disposed) {
              resizeQueued = false;
              handleResize();
            }
          });
        };
        window.addEventListener('resize', handleResize);
        cleanup = () => {
          window.removeEventListener('resize', handleResize);
          resizeQueued = false;
          if (resizeFrame !== null) window.cancelAnimationFrame(resizeFrame);
        };
        await syncWindowState();
      } catch {
        if (!disposed) {
          setIsWindowsChrome(false);
          setIsMaximised(false);
        }
      }
    };

    void initializeWindowChrome();

    return () => {
      disposed = true;
      cleanup();
    };
  }, [syncWindowState]);

  const scheduleWindowStateSync = useCallback(() => {
    window.setTimeout(() => void syncWindowState(), 80);
  }, [syncWindowState]);

  const handleToggleMaximise = useCallback(() => {
    WindowToggleMaximise();
    scheduleWindowStateSync();
  }, [scheduleWindowStateSync]);

  const handleLogoKeyDown = useCallback((event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
    }
  }, []);

  const handleTabChange = (tab: ActiveTab) => {
    if (tab === activeTab) return;
    onTabChange(tab);
  };

  const contentMap: Record<ActiveTab, ReactNode> = {
    status: statusContent,
    curve: curveContent,
    control: controlContent,
    devices: devicesContent,
    about: aboutContent,
  };
  const transitionDirection = getTabTransitionDirection(previousActiveTabRef.current, activeTab);
  const windowBlurMode = String((config as any)?.windowBlur || 'acrylic');

  useEffect(() => {
    if (previousActiveTabRef.current === activeTab) {
      return;
    }
    const scrollElement = scrollRef.current;
    if (scrollElement) {
      scrollElement.scrollTop = 0;
      scrollElement.scrollLeft = 0;
    }
    previousActiveTabRef.current = activeTab;
  }, [activeTab]);

  return (
    <div
      data-theme-page={activeTab}
      data-theme-section="app-shell"
      data-window-blur-mode={windowBlurMode}
      className={clsx(
        'glacier-shell relative flex h-dvh w-full overflow-hidden bg-background text-foreground',
        isWindowsChrome && 'glacier-native-backdrop',
      )}
    >
      {isWindowsChrome && (
        <TitleBar
          minimizeLabel={t('appShell.titleBar.minimize')}
          maximizeLabel={t('appShell.titleBar.maximize')}
          restoreLabel={t('appShell.titleBar.restore')}
          closeLabel={t('appShell.titleBar.close')}
          isMaximised={isMaximised}
          leftSlot={<StatusBadges isConnected={isConnected} fanData={fanData} temperature={temperature} runtimeDeviceProfile={runtimeDeviceProfile} config={config} autoControl={autoControl} compact />}
          onMinimise={() => WindowMinimise()}
          onToggleMaximise={handleToggleMaximise}
          onClose={() => Quit()}
        />
      )}

      <aside data-theme-section="sidebar" className="glacier-sidebar flex w-16 shrink-0 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground shadow-[1px_0_0_rgba(15,23,42,0.04)] dark:shadow-[1px_0_0_rgba(255,255,255,0.04)]">
        <div className="flex h-[76px] items-center justify-center px-2" style={DRAG_STYLE}>
          <div
            aria-label={BRAND.name}
            role="img"
            data-theme-ui="brand-mark"
            tabIndex={0}
            onKeyDown={handleLogoKeyDown}
            className="flex h-9 w-9 items-center justify-center rounded-lg border border-border bg-card text-primary shadow-sm shadow-black/5 outline-none"
            style={NO_DRAG_STYLE}
          >
            <Fan className="h-5 w-5" />
          </div>
        </div>

        <nav className="flex flex-1 flex-col items-center gap-1 px-2" role="tablist" style={NO_DRAG_STYLE}>
          {MAIN_TAB_ITEMS.map((tab) => {
            const Icon = tab.icon;
            const isActive = activeTab === tab.id;
            const tabTitle = t(tab.titleKey);
            return (
              <Tooltip key={tab.id}>
                <TooltipTrigger asChild>
                  <button
                    role="tab"
                    data-theme-ui="sidebar-item"
                    data-theme-tab={tab.id}
                    aria-label={tabTitle}
                    aria-selected={isActive}
                    onClick={() => handleTabChange(tab.id)}
                    className={clsx(
                      'relative flex h-11 w-11 cursor-pointer items-center justify-center overflow-hidden rounded-xl transition-colors duration-200',
                      isActive
                        ? 'text-primary'
                        : 'text-sidebar-foreground/62 hover:bg-sidebar-accent hover:text-sidebar-foreground',
                    )}
                  >
                    {isActive && (
                      <span
                        className="pointer-events-none absolute inset-0 rounded-xl"
                      />
                    )}
                    <Icon className="relative z-10 h-4.5 w-4.5" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="right">{tabTitle}</TooltipContent>
              </Tooltip>
            );
          })}
        </nav>

        <div className="px-2 pb-5" style={NO_DRAG_STYLE}>
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                data-theme-ui="sidebar-item"
                data-theme-tab={ABOUT_TAB.id}
                aria-label={t(ABOUT_TAB.titleKey)}
                aria-selected={activeTab === ABOUT_TAB.id}
                onClick={() => handleTabChange(ABOUT_TAB.id)}
                className={clsx(
                  'relative mx-auto flex h-11 w-11 cursor-pointer items-center justify-center overflow-hidden rounded-xl transition-colors duration-200',
                  activeTab === ABOUT_TAB.id
                    ? 'text-primary'
                    : 'text-sidebar-foreground/62 hover:bg-sidebar-accent hover:text-sidebar-foreground',
                )}
              >
                {activeTab === ABOUT_TAB.id && (
                  <span className="pointer-events-none absolute inset-0 rounded-xl" />
                )}
                <ABOUT_TAB.icon className="relative z-10 h-4.5 w-4.5" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="right">{t(ABOUT_TAB.titleKey)}</TooltipContent>
          </Tooltip>
        </div>
      </aside>

      <section data-theme-section="content" className="glacier-content relative flex min-w-0 flex-1 flex-col overflow-hidden">
        {!isWindowsChrome && (
          <header
            className="shrink-0 border-b border-border/65 bg-background/92 px-4 pb-3 pt-3 backdrop-blur-xl sm:px-5 lg:px-6"
            style={DRAG_STYLE}
          >
            <div className="mx-auto flex min-h-9 max-w-[1120px] min-[1680px]:max-w-[1280px] min-[2200px]:max-w-[1480px] items-center justify-start gap-3" style={NO_DRAG_STYLE}>
              <StatusBadges isConnected={isConnected} fanData={fanData} temperature={temperature} runtimeDeviceProfile={runtimeDeviceProfile} config={config} autoControl={autoControl} />
            </div>
          </header>
        )}

        <div data-theme-section="content-panel" className="glacier-content-panel relative min-h-0 flex-1 overflow-hidden">
          <div
            ref={scrollRef}
            className="app-scroll-root app-scroll-root--hide-native h-full"
            style={NO_DRAG_STYLE}
          >
            <div className="min-h-full px-4 pb-6 pt-4 sm:px-5 lg:px-6">

          {/* Alerts */}
          <div className="mx-auto max-w-[1120px] min-[1680px]:max-w-[1280px] min-[2200px]:max-w-[1480px]">
            <AnimatePresence>
              {error && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  className="overflow-hidden"
                >
                  <div className="mb-3 flex items-start gap-3 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-2.5 text-sm text-destructive">
                    <p className="min-w-0 flex-1 leading-relaxed">{error}</p>
                    {onExportDiagnostics && (
                      <button
                        type="button"
                        disabled={diagnosticsExporting}
                        onClick={onExportDiagnostics}
                        className="inline-flex shrink-0 cursor-pointer items-center gap-1.5 rounded-md border border-destructive/25 bg-background/55 px-2.5 py-1 text-xs font-medium text-destructive transition hover:bg-destructive/10 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        <Download className="h-3.5 w-3.5" />
                        {diagnosticsExporting ? t('appShell.diagnostics.exporting') : t('appShell.diagnostics.export')}
                      </button>
                    )}
                  </div>
                </motion.div>
              )}

              {bridgeWarning && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  className="overflow-hidden"
                >
                  <div className="mb-3 flex items-start gap-3 rounded-lg border border-amber-300/50 bg-amber-50/80 px-4 py-2.5 text-amber-800 dark:border-amber-700/40 dark:bg-amber-900/15 dark:text-amber-200">
                    <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" />
                    <p className="flex-1 text-sm leading-relaxed">{bridgeWarning}</p>
                    {onExportDiagnostics && (
                      <button
                        type="button"
                        disabled={diagnosticsExporting}
                        onClick={onExportDiagnostics}
                        className="inline-flex shrink-0 cursor-pointer items-center gap-1.5 rounded-md border border-amber-300/70 bg-amber-100/70 px-2.5 py-1 text-xs font-medium text-amber-900 transition hover:bg-amber-200/80 disabled:cursor-not-allowed disabled:opacity-60 dark:border-amber-700/70 dark:bg-amber-900/35 dark:text-amber-100 dark:hover:bg-amber-800/50"
                      >
                        <Download className="h-3.5 w-3.5" />
                        {diagnosticsExporting ? t('appShell.diagnostics.exporting') : t('appShell.diagnostics.export')}
                      </button>
                    )}
                    <button
                      type="button"
                      aria-label={t('appShell.bridgeWarning.closeAria')}
                      onClick={onDismissBridgeWarning}
                      className="cursor-pointer rounded p-0.5 transition hover:bg-amber-200/60 dark:hover:bg-amber-800/40"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </div>

          {/* Tab content */}
          <main className="mx-auto w-full max-w-[1120px] min-[1680px]:max-w-[1280px] min-[2200px]:max-w-[1480px] min-w-0 overflow-hidden">
            <AnimatePresence mode="wait" initial={false} custom={transitionDirection}>
              <motion.div
                key={activeTab}
                custom={transitionDirection}
                variants={TAB_CONTENT_VARIANTS}
                initial="enter"
                animate="center"
                exit="exit"
                transition={{
                  duration: 0.2,
                  ease: [0.22, 1, 0.36, 1],
                }}
                className="w-full min-w-0 px-1 pb-2 will-change-transform"
              >
                {contentMap[activeTab]}
              </motion.div>
            </AnimatePresence>
          </main>
          </div>
        </div>

        {/* Floating overlay scrollbar — never reserves width */}
        <OverlayScrollbar scrollRef={scrollRef} />
        </div>
      </section>
    </div>
  );
}
