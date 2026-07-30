"use client";

import type { CSSProperties, ReactNode } from "react";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { motion, AnimatePresence, type Variants } from "framer-motion";
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
  PanelLeftClose,
  PanelLeftOpen,
  Usb,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import {
  Environment,
  Quit,
  WindowIsMaximised,
  WindowMinimise,
  WindowToggleMaximise,
} from "../../../wailsjs/runtime/runtime";
import { types } from "../../../wailsjs/go/models";
import clsx from "clsx";
import { useTranslation } from "react-i18next";
import { BRAND } from "../lib/brand";
import {
  clampFanSpeedToRange,
  fanSpeedUnitLabel,
  getFanSpeedRange,
  getFanSpeedUnit,
  readCurrentFanSpeed,
} from "../lib/fan-speed";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import UpdateProgressWidget from "./UpdateProgressWidget";

const MAIN_TAB_ITEMS = [
  { id: "status", titleKey: "appShell.tabs.status", icon: LayoutGrid },
  { id: "curve", titleKey: "appShell.tabs.curve", icon: LineChart },
  { id: "control", titleKey: "appShell.tabs.control", icon: Settings2 },
  { id: "devices", titleKey: "appShell.tabs.devices", icon: Boxes },
] as const;

const ABOUT_TAB = {
  id: "about",
  titleKey: "appShell.tabs.about",
  icon: Info,
} as const;

type ActiveTab = (typeof MAIN_TAB_ITEMS)[number]["id"] | typeof ABOUT_TAB.id;

const DOCK_COLLAPSED_WIDTH = 64;
const DOCK_EXPANDED_WIDTH = 192;
const DOCK_EXPANDED_STORAGE_KEY = "fancontrol.dock.expanded";

function readStoredDockExpanded(): boolean {
  try {
    return window.localStorage.getItem(DOCK_EXPANDED_STORAGE_KEY) === "1";
  } catch {
    return false;
  }
}

const TAB_TRANSITION_ORDER: ActiveTab[] = [
  ...MAIN_TAB_ITEMS.map((tab) => tab.id),
  ABOUT_TAB.id,
];

function getTabTransitionDirection(fromTab: ActiveTab, toTab: ActiveTab) {
  const fromIndex = TAB_TRANSITION_ORDER.indexOf(fromTab);
  const toIndex = TAB_TRANSITION_ORDER.indexOf(toTab);
  if (fromIndex === -1 || toIndex === -1 || fromIndex === toIndex) {
    return 0;
  }
  return toIndex > fromIndex ? 1 : -1;
}

const TAB_CONTENT_VARIANTS: Variants = {
  enter: (direction: number) => ({
    opacity: 0,
    y: direction === 0 ? 8 : direction * 18,
  }),
  center: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.18,
      ease: [0.22, 1, 0.36, 1],
    },
  },
  exit: (direction: number) => ({
    opacity: 0,
    y: direction === 0 ? -6 : direction * -14,
    transition: {
      duration: 0.15,
      ease: [0.22, 1, 0.36, 1],
    },
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
  if (!temp) return "text-muted-foreground";
  if (temp > 80) return "text-red-500";
  if (temp > 70) return "text-amber-500";
  return "text-primary";
}

function getFanSpinDuration(speed?: number, minSpeed = 0, maxSpeed = 100) {
  if (!speed || speed <= 0) return 0;
  const speedSpan = Math.max(1, maxSpeed - minSpeed);
  const percent = Math.max(
    0,
    Math.min(100, ((speed - minSpeed) / speedSpan) * 100),
  );
  if (percent >= 90) return 0.48;
  if (percent >= 70) return 0.72;
  if (percent >= 45) return 1;
  return 1.35;
}

type WailsDragStyle = CSSProperties & {
  ["--wails-draggable"]?: "drag" | "no-drag";
};

const DRAG_STYLE: WailsDragStyle = { "--wails-draggable": "drag" };
const NO_DRAG_STYLE: WailsDragStyle = { "--wails-draggable": "no-drag" };

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
      data-app-ui="titlebar-window-button"
      aria-label={label}
      title={label}
      style={NO_DRAG_STYLE}
      onClick={(event) => {
        event.stopPropagation();
        onClick();
      }}
      className={clsx(
        "flex h-8 w-10 cursor-pointer items-center justify-center rounded-md text-muted-foreground transition-colors",
        danger
          ? "hover:bg-red-500 hover:text-white"
          : "hover:bg-foreground/10 hover:text-foreground",
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
  leftOffset,
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
  leftOffset: number;
  onMinimise: () => void;
  onToggleMaximise: () => void;
  onClose: () => void;
}) {
  return (
    <div
      className="glacier-titlebar pointer-events-auto absolute right-0 top-0 flex h-10 items-center justify-between bg-background transition-[left] duration-[420ms] ease-[cubic-bezier(0.16,1,0.3,1)]"
      style={{
        ...DRAG_STYLE,
        left: leftOffset,
        zIndex: "var(--layer-titlebar)",
      }}
      onDoubleClick={onToggleMaximise}
    >
      <div className="flex h-full min-w-0 flex-1 items-center px-3 pt-1">
        <div
          data-app-ui="titlebar-info"
          className="glacier-titlebar-info flex min-w-0 items-center"
        >
          {leftSlot}
        </div>
      </div>
      <div
        data-app-ui="titlebar-controls"
        className="glacier-titlebar-controls mr-1 flex h-8 items-center gap-0.5"
        style={NO_DRAG_STYLE}
      >
        <TitleBarButton
          icon={<Minus className="h-3.5 w-3.5" />}
          label={minimizeLabel}
          onClick={onMinimise}
        />
        <TitleBarButton
          icon={
            isMaximised ? (
              <Copy className="h-3 w-3" />
            ) : (
              <Square className="h-3 w-3" />
            )
          }
          label={isMaximised ? restoreLabel : maximizeLabel}
          onClick={onToggleMaximise}
        />
        <TitleBarButton
          icon={<X className="h-3.5 w-3.5" />}
          label={closeLabel}
          onClick={onClose}
          danger
        />
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
  const fanSpeedUnit = getFanSpeedUnit(
    fanData as any,
    config as any,
    runtimeDeviceProfile as any,
  );
  const fanSpeedRange = getFanSpeedRange(
    config as any,
    fanSpeedUnit,
    runtimeDeviceProfile as any,
  );
  const fanSpeed = clampFanSpeedToRange(
    readCurrentFanSpeed(
      fanData,
      fanSpeedUnit,
      config as any,
      runtimeDeviceProfile as any,
    ),
    fanSpeedRange,
  );
  const fanSpeedLabel = fanSpeedUnitLabel(fanSpeedUnit);
  const fanSpinDuration = getFanSpinDuration(
    fanSpeed,
    fanSpeedRange.min,
    fanSpeedRange.max,
  );
  const baseClass = compact
    ? "inline-flex h-6 items-center gap-1.5 rounded-full border px-2.5 text-[11px] font-medium"
    : "inline-flex h-8 items-center gap-1.5 rounded-xl border px-3 text-[13px] font-medium";
  const fanSpinStyle = fanSpinDuration
    ? { animationDuration: `${fanSpinDuration}s` }
    : undefined;
  const transport = String(
    runtimeDeviceProfile?.transport ||
      (fanData as any)?.transport ||
      (config as any).deviceTransport ||
      "",
  ).toLowerCase();
  const ConnectedIcon =
    transport === "ble"
      ? Bluetooth
      : transport === "hid" || transport === "serial"
        ? Usb
        : Wifi;

  return (
    <div
      className={clsx(
        "flex min-w-0 items-center gap-2 text-[13px] tabular-nums",
        compact && "translate-y-px overflow-hidden whitespace-nowrap",
      )}
    >
      <span
        data-app-ui="titlebar-status-card"
        className={clsx(
          baseClass,
          isConnected
            ? "border-primary/20 bg-primary/10 text-primary"
            : "border-border bg-card text-muted-foreground",
        )}
      >
        {isConnected ? (
          <ConnectedIcon className="h-3.5 w-3.5" />
        ) : (
          <WifiOff className="h-3.5 w-3.5" />
        )}
        {isConnected
          ? t("appShell.status.connected")
          : t("appShell.status.offline")}
      </span>

      <span
        data-app-ui="titlebar-status-card"
        className={clsx(
          baseClass,
          autoControl
            ? "border-primary/20 bg-primary/10 text-primary"
            : "border-border bg-card text-muted-foreground",
        )}
      >
        <Sparkles className="h-3.5 w-3.5" />
        {autoControl
          ? t("appShell.status.smartControl")
          : t("appShell.status.manualMode")}
      </span>

      {isConnected && (
        <>
          <span
            data-app-ui="titlebar-status-card"
            className={clsx(
              baseClass,
              "border-border bg-card font-semibold shadow-sm shadow-black/5",
            )}
          >
            <Thermometer
              className={clsx(
                "h-3.5 w-3.5",
                getTempColor(temperature?.maxTemp),
              )}
            />
            <span className={clsx(getTempColor(temperature?.maxTemp))}>
              {temperature?.maxTemp ?? "--"}°C
            </span>
          </span>
          <span
            data-app-ui="titlebar-status-card"
            className={clsx(
              baseClass,
              "border-border bg-card font-semibold text-primary shadow-sm shadow-black/5",
            )}
          >
            <span
              className={clsx("inline-flex", fanSpinDuration && "animate-spin")}
              style={fanSpinStyle}
            >
              <Fan className="h-3.5 w-3.5" />
            </span>
            {fanSpeed ?? "--"}
            {fanSpeedLabel}
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
  const draggingRef = useRef<{ startY: number; startScroll: number } | null>(
    null,
  );
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

    el.addEventListener("scroll", onActivity, { passive: true });
    el.addEventListener("mouseenter", onActivity);
    el.addEventListener("wheel", onActivity, { passive: true });
    el.addEventListener("touchstart", onActivity, { passive: true });

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
      el.removeEventListener("scroll", onActivity);
      el.removeEventListener("mouseenter", onActivity);
      el.removeEventListener("wheel", onActivity);
      el.removeEventListener("touchstart", onActivity);
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
      draggingRef.current = {
        startY: event.clientY,
        startScroll: el.scrollTop,
      };
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
      className={clsx("app-overlay-scrollbar", visible && "is-visible")}
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

function DockButton({
  icon: Icon,
  label,
  expanded,
  onClick,
  isActive = false,
  tabId,
  role,
  ariaExpanded,
}: {
  icon: LucideIcon;
  label: string;
  expanded: boolean;
  onClick: () => void;
  isActive?: boolean;
  tabId?: string;
  role?: "tab";
  ariaExpanded?: boolean;
}) {
  const button = (
    <button
      type="button"
      role={role}
      data-theme-ui="sidebar-item"
      data-theme-tab={tabId}
      aria-label={label}
      aria-selected={role === "tab" ? isActive : undefined}
      aria-expanded={ariaExpanded}
      onPointerDown={(event) => {
        event.preventDefault();
        event.currentTarget.blur();
      }}
      onClick={(event) => {
        const pointerTarget = event.detail > 0 ? event.currentTarget : null;
        pointerTarget?.blur();
        onClick();
        if (pointerTarget) {
          window.requestAnimationFrame(() => pointerTarget.blur());
        }
      }}
      className={clsx(
        "group/nav relative flex h-11 w-full cursor-pointer items-center overflow-hidden rounded-xl text-left transition-colors duration-200",
        isActive
          ? "text-primary"
          : "text-sidebar-foreground/62 hover:text-sidebar-foreground",
      )}
    >
      <i
        aria-hidden="true"
        className={clsx(
          "pointer-events-none absolute inset-y-px left-[11px] right-[11px] rounded-[13px] transition-colors duration-200",
          !isActive && "group-hover/nav:bg-sidebar-accent",
        )}
      />
      <i
        data-app-ui="sidebar-item-icon"
        className="relative z-10 flex w-16 shrink-0 items-center justify-center not-italic"
      >
        <Icon className="h-4.5 w-4.5" />
      </i>
      <i
        data-app-ui="sidebar-item-label"
        className={clsx(
          "relative z-10 min-w-0 flex-1 truncate pr-4 text-sm font-medium not-italic transition-[opacity,transform] duration-300 ease-out",
          expanded
            ? "translate-x-0 opacity-100 delay-100"
            : "-translate-x-2 opacity-0 delay-0",
        )}
      >
        {label}
      </i>
    </button>
  );

  if (expanded) return button;

  return (
    <Tooltip>
      <TooltipTrigger asChild>{button}</TooltipTrigger>
      <TooltipContent side="right">{label}</TooltipContent>
    </Tooltip>
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
  const [isWindowsChrome, setIsWindowsChrome] = useState(
    () =>
      typeof document !== "undefined" &&
      document.documentElement.dataset.os === "win",
  );
  const [isMaximised, setIsMaximised] = useState(false);
  const [dockExpanded, setDockExpanded] = useState(readStoredDockExpanded);
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
        const isWindows = env.platform === "windows";
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
        window.addEventListener("resize", handleResize);
        cleanup = () => {
          window.removeEventListener("resize", handleResize);
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

  const handleToggleDock = useCallback(() => {
    setDockExpanded((previous) => {
      const next = !previous;
      try {
        window.localStorage.setItem(
          DOCK_EXPANDED_STORAGE_KEY,
          next ? "1" : "0",
        );
      } catch {
        // Persistence failure should not block the dock interaction.
      }
      return next;
    });
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
  const transitionDirection = getTabTransitionDirection(
    previousActiveTabRef.current,
    activeTab,
  );
  const windowBlurMode = String((config as any)?.windowBlur || "acrylic");
  const dockWidth = dockExpanded ? DOCK_EXPANDED_WIDTH : DOCK_COLLAPSED_WIDTH;
  const dockToggleLabel = t(
    dockExpanded ? "appShell.sidebar.collapse" : "appShell.sidebar.expand",
  );
  const DockToggleIcon = dockExpanded ? PanelLeftClose : PanelLeftOpen;

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
        "glacier-shell relative flex h-dvh w-full overflow-hidden bg-background text-foreground",
        isWindowsChrome && "glacier-native-backdrop",
      )}
    >
      {isWindowsChrome && (
        <TitleBar
          minimizeLabel={t("appShell.titleBar.minimize")}
          maximizeLabel={t("appShell.titleBar.maximize")}
          restoreLabel={t("appShell.titleBar.restore")}
          closeLabel={t("appShell.titleBar.close")}
          isMaximised={isMaximised}
          leftOffset={dockWidth}
          leftSlot={
            <StatusBadges
              isConnected={isConnected}
              fanData={fanData}
              temperature={temperature}
              runtimeDeviceProfile={runtimeDeviceProfile}
              config={config}
              autoControl={autoControl}
              compact
            />
          }
          onMinimise={() => WindowMinimise()}
          onToggleMaximise={handleToggleMaximise}
          onClose={() => Quit()}
        />
      )}

      <aside
        data-theme-section="sidebar"
        data-dock-expanded={dockExpanded ? "true" : "false"}
        className="glacier-sidebar flex shrink-0 flex-col overflow-hidden border-r border-sidebar-border bg-sidebar text-sidebar-foreground shadow-[1px_0_0_rgba(15,23,42,0.04)] transition-[width] duration-[420ms] ease-[cubic-bezier(0.16,1,0.3,1)] dark:shadow-[1px_0_0_rgba(255,255,255,0.04)]"
        style={{ width: dockWidth }}
      >
        <div
          className="flex h-[76px] shrink-0 items-center overflow-hidden"
          style={DRAG_STYLE}
        >
          <div
            data-app-ui="brand-mark"
            role="img"
            className="relative h-10 w-full overflow-hidden"
            style={NO_DRAG_STYLE}
          >
            <span className="sr-only">{BRAND.name}</span>
            <img
              data-app-ui="brand-compact"
              src="/brand/fancontrol-fc-mark-v1-dark.png"
              alt=""
              draggable={false}
              className="dark:hidden"
            />
            <img
              data-app-ui="brand-compact"
              src="/brand/fancontrol-fc-mark-v1.png"
              alt=""
              draggable={false}
              className="hidden dark:block"
            />
            <span data-app-ui="brand-wordmark" aria-hidden="true">
              <span data-brand-part="f" />
              <span data-brand-part="an" />
              <span data-brand-part="c" />
              <span data-brand-part="ontrol" />
            </span>
          </div>
        </div>

        <div data-app-ui="sidebar-active-indicator" aria-hidden="true" />

        <nav
          className="flex flex-1 flex-col gap-1"
          role="tablist"
          style={NO_DRAG_STYLE}
        >
          {MAIN_TAB_ITEMS.map((tab) => (
            <DockButton
              key={tab.id}
              icon={tab.icon}
              label={t(tab.titleKey)}
              expanded={dockExpanded}
              isActive={activeTab === tab.id}
              tabId={tab.id}
              role="tab"
              onClick={() => handleTabChange(tab.id)}
            />
          ))}
        </nav>

        <div className="flex flex-col gap-1 pb-5" style={NO_DRAG_STYLE}>
          <DockButton
            icon={ABOUT_TAB.icon}
            label={t(ABOUT_TAB.titleKey)}
            expanded={dockExpanded}
            isActive={activeTab === ABOUT_TAB.id}
            tabId={ABOUT_TAB.id}
            role="tab"
            onClick={() => handleTabChange(ABOUT_TAB.id)}
          />
          <DockButton
            icon={DockToggleIcon}
            label={dockToggleLabel}
            expanded={dockExpanded}
            ariaExpanded={dockExpanded}
            onClick={handleToggleDock}
          />
        </div>
      </aside>

      <section
        data-theme-section="content"
        className="glacier-content relative flex min-w-0 flex-1 flex-col overflow-hidden"
      >
        {!isWindowsChrome && (
          <header
            className="shrink-0 border-b border-border/65 bg-background/92 px-4 pb-3 pt-3 backdrop-blur-xl sm:px-5 lg:px-6"
            style={DRAG_STYLE}
          >
            <div
              className="mx-auto flex min-h-9 max-w-[1120px] min-[1680px]:max-w-[1280px] min-[2200px]:max-w-[1480px] items-center justify-start gap-3"
              style={NO_DRAG_STYLE}
            >
              <StatusBadges
                isConnected={isConnected}
                fanData={fanData}
                temperature={temperature}
                runtimeDeviceProfile={runtimeDeviceProfile}
                config={config}
                autoControl={autoControl}
              />
            </div>
          </header>
        )}

        <div
          data-theme-section="content-panel"
          className="glacier-content-panel relative min-h-0 flex-1 overflow-hidden"
        >
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
                      animate={{ opacity: 1, height: "auto" }}
                      exit={{ opacity: 0, height: 0 }}
                      className="overflow-hidden"
                    >
                      <div className="mb-3 flex items-start gap-3 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-2.5 text-sm text-destructive">
                        <p className="min-w-0 flex-1 leading-relaxed">
                          {error}
                        </p>
                        {onExportDiagnostics && (
                          <button
                            type="button"
                            disabled={diagnosticsExporting}
                            onClick={onExportDiagnostics}
                            className="inline-flex shrink-0 cursor-pointer items-center gap-1.5 rounded-md border border-destructive/25 bg-background/55 px-2.5 py-1 text-xs font-medium text-destructive transition hover:bg-destructive/10 disabled:cursor-not-allowed disabled:opacity-60"
                          >
                            <Download className="h-3.5 w-3.5" />
                            {diagnosticsExporting
                              ? t("appShell.diagnostics.exporting")
                              : t("appShell.diagnostics.export")}
                          </button>
                        )}
                      </div>
                    </motion.div>
                  )}

                  {bridgeWarning && (
                    <motion.div
                      initial={{ opacity: 0, height: 0 }}
                      animate={{ opacity: 1, height: "auto" }}
                      exit={{ opacity: 0, height: 0 }}
                      className="overflow-hidden"
                    >
                      <div className="mb-3 flex items-start gap-3 rounded-lg border border-amber-300/50 bg-amber-50/80 px-4 py-2.5 text-amber-800 dark:border-amber-700/40 dark:bg-amber-900/15 dark:text-amber-200">
                        <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" />
                        <p className="flex-1 text-sm leading-relaxed">
                          {bridgeWarning}
                        </p>
                        {onExportDiagnostics && (
                          <button
                            type="button"
                            disabled={diagnosticsExporting}
                            onClick={onExportDiagnostics}
                            className="inline-flex shrink-0 cursor-pointer items-center gap-1.5 rounded-md border border-amber-300/70 bg-amber-100/70 px-2.5 py-1 text-xs font-medium text-amber-900 transition hover:bg-amber-200/80 disabled:cursor-not-allowed disabled:opacity-60 dark:border-amber-700/70 dark:bg-amber-900/35 dark:text-amber-100 dark:hover:bg-amber-800/50"
                          >
                            <Download className="h-3.5 w-3.5" />
                            {diagnosticsExporting
                              ? t("appShell.diagnostics.exporting")
                              : t("appShell.diagnostics.export")}
                          </button>
                        )}
                        <button
                          type="button"
                          aria-label={t("appShell.bridgeWarning.closeAria")}
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
                <AnimatePresence
                  mode="wait"
                  initial={false}
                  custom={transitionDirection}
                >
                  <motion.div
                    key={activeTab}
                    custom={transitionDirection}
                    variants={TAB_CONTENT_VARIANTS}
                    initial="enter"
                    animate="center"
                    exit="exit"
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
      <UpdateProgressWidget />
    </div>
  );
}
