'use client';

import { useEffect, useState } from 'react';
import { Fan } from 'lucide-react';
import { Skeleton } from '@/components/ui/skeleton';
import { DOCK_COLLAPSED_WIDTH, DOCK_EXPANDED_WIDTH, DockBrandMark, readStoredDockExpanded } from './AppShell';
import { DeviceHeroArtwork, SemiGauge } from './DeviceStatus';

export default function AppLoadingSkeleton() {
  const [dockExpanded, setDockExpanded] = useState(false);
  useEffect(() => {
    setDockExpanded(readStoredDockExpanded());
  }, []);
  const dockWidth = dockExpanded ? DOCK_EXPANDED_WIDTH : DOCK_COLLAPSED_WIDTH;
  const dockItemClass = dockExpanded ? 'h-[42px] w-full rounded-xl' : 'size-[42px] rounded-xl';

  return (
    <div
      data-theme-page="status"
      data-theme-section="app-shell"
      aria-busy="true"
      className="glacier-shell glacier-native-backdrop relative flex h-dvh w-full overflow-hidden bg-background text-foreground"
    >
      <div
        className="glacier-titlebar pointer-events-none absolute right-0 top-0 flex h-10 items-center justify-between bg-background"
        style={{ left: dockWidth, zIndex: 'var(--layer-titlebar)' }}
        aria-hidden="true"
      >
        <div
          data-app-ui="titlebar-home-heading"
          className="glacier-titlebar-home-heading absolute inset-y-0 left-5 flex items-center pt-0.5 sm:left-6 lg:left-7"
        >
          <Skeleton className="h-5 w-28" />
        </div>
        <div data-app-ui="titlebar-controls" className="glacier-titlebar-controls ml-auto mr-1 flex h-8 items-center gap-0.5">
          <Skeleton className="h-8 w-10 rounded-lg" />
          <Skeleton className="h-8 w-10 rounded-lg" />
          <Skeleton className="h-8 w-10 rounded-lg" />
        </div>
      </div>

      <aside
        data-theme-section="sidebar"
        data-dock-expanded={dockExpanded ? 'true' : 'false'}
        className="glacier-sidebar flex shrink-0 flex-col overflow-hidden border-r border-sidebar-border bg-sidebar text-sidebar-foreground shadow-[1px_0_0_rgba(15,23,42,0.04)] dark:shadow-[1px_0_0_rgba(255,255,255,0.04)]"
        style={{ width: dockWidth }}
      >
        <div className="flex h-[76px] shrink-0 items-center overflow-hidden">
          <DockBrandMark />
        </div>
        <div className="flex flex-1 flex-col items-center gap-1 px-2" aria-hidden="true">
          <Skeleton className={dockItemClass} />
          <Skeleton className={dockItemClass} />
          <Skeleton className={dockItemClass} />
          <Skeleton className={dockItemClass} />
        </div>
        <div className="flex flex-col items-center gap-1 px-2 pb-5" aria-hidden="true">
          <Skeleton className={dockItemClass} />
          <Skeleton className={dockItemClass} />
        </div>
      </aside>

      <section data-theme-section="content" className="glacier-content relative flex min-w-0 flex-1 flex-col overflow-hidden">
        <div data-theme-section="content-panel" className="glacier-content-panel relative min-h-0 flex-1 overflow-hidden">
          <div className="app-scroll-root app-scroll-root--hide-native h-full" aria-hidden="true">
            <div className="flex min-h-full flex-col px-4 pb-4 pt-2 sm:px-5 lg:px-6">
              <main className="mx-auto flex w-full min-w-0 max-w-[1360px] flex-1 flex-col overflow-hidden px-1 pb-2 min-[1680px]:max-w-[1520px] min-[2200px]:max-w-[1720px]">
                <div className="home-dashboard-root flex flex-1 flex-col gap-3">
                  <div data-theme-section="hero" data-theme-card="device-hero" className="glacier-hero-card relative flex items-center overflow-hidden rounded-xl border border-border bg-card px-4 py-3.5 shadow-sm shadow-black/5">
                    <DeviceHeroArtwork />
                    <div className="glacier-hero-content relative z-10 flex w-full flex-wrap items-center justify-between gap-3">
                      <div className="flex min-w-0 items-center gap-3">
                        <div data-theme-ui="hero-brand-mark" className="flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-primary/10 text-primary">
                          <Fan className="h-8 w-8" />
                        </div>
                        <div className="min-w-0 space-y-2">
                          <Skeleton className="h-4 w-32 max-w-full" />
                          <Skeleton className="h-3 w-40 max-w-full" />
                        </div>
                      </div>
                      <div data-theme-ui="hero-actions" className="glacier-hero-actions flex items-center gap-3">
                        <Skeleton className="h-7 w-16 rounded-lg" />
                      </div>
                    </div>
                  </div>
                  <div data-theme-section="home-bento" className="home-bento-layout">
                    <div className="home-metrics-grid">
                      {['cpu-temperature', 'gpu-temperature', 'fan-speed'].map((card) => (
                        <div key={card} data-theme-card={card} className="glacier-metric-card flex h-full min-h-[218px] flex-col rounded-xl border border-border bg-card px-4 pb-3.5 pt-3 shadow-sm shadow-black/5">
                          <Skeleton className="mx-auto h-4 w-24" />
                          <div className="flex flex-1 items-center justify-center">
                            <SemiGauge value={0} color="transparent">
                              <Skeleton className="h-6 w-14" />
                            </SemiGauge>
                          </div>
                          {card === 'fan-speed' ? (
                            <div data-theme-section="control-protection" className="mt-3 grid grid-cols-2 gap-2">
                              <div data-theme-card="control-mode" className="glacier-stat-tile min-h-[56px] rounded-lg border border-border/75 bg-background/55" />
                              <div data-theme-card="temperature-state" className="glacier-stat-tile min-h-[56px] rounded-lg border border-border/75 bg-background/55" />
                            </div>
                          ) : (
                            <div data-theme-card={card === 'cpu-temperature' ? 'cpu-power' : 'gpu-power'} className="glacier-stat-tile mt-3 flex min-h-[56px] items-center justify-between gap-2.5 rounded-lg border border-border/75 bg-background/55 px-3.5 py-2">
                              <Skeleton className="h-3 w-16" />
                              <Skeleton className="h-5 w-12" />
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                    <div className="home-charts-grid">
                      {['fan-curve-preview', 'temperature-history'].map((card) => (
                        <div key={card} data-theme-card={card} className="glacier-chart-card flex h-full min-h-[200px] flex-col rounded-xl border border-border bg-card px-4 py-3.5 shadow-sm shadow-black/5">
                          <div className="mb-2.5 flex h-5 items-center justify-between">
                            <Skeleton className="h-3 w-24" />
                            <Skeleton className="h-3 w-12" />
                          </div>
                          <Skeleton className="glacier-chart-canvas min-h-[124px] flex-1 rounded-lg bg-muted/25" />
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              </main>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}
