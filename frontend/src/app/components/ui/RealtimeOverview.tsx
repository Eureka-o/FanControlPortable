import type { ReactNode } from 'react';
import clsx from 'clsx';

export interface RealtimeOverviewMetric {
  id: string;
  icon?: ReactNode;
  label: string;
  value: ReactNode;
}

export interface RealtimeOverviewHardware {
  id: string;
  icon: ReactNode;
  label: string;
  model?: string;
  metrics: RealtimeOverviewMetric[];
}

export interface RealtimeOverviewDevice {
  icon: ReactNode;
  name: string;
  connected: boolean;
  connectionLabel: string;
  details: Array<{
    id: string;
    label: string;
    value: ReactNode;
  }>;
}

interface RealtimeOverviewProps {
  title: string;
  titleIcon: ReactNode;
  hardware: RealtimeOverviewHardware[];
  device: RealtimeOverviewDevice;
}

export function RealtimeOverview({ title, titleIcon, hardware, device }: RealtimeOverviewProps) {
  return (
    <section data-theme-card="settings-overview" className="rounded-2xl border border-border bg-card p-5 shadow-sm">
      <div className="mb-4 flex items-center gap-2 text-muted-foreground">
        {titleIcon}
        <h3 className="text-base font-semibold text-foreground">{title}</h3>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-[minmax(0,1.2fr)_minmax(220px,0.8fr)]">
        <div data-theme-card="settings-overview-temperature" className="min-w-0 divide-y divide-border/55 rounded-xl border border-border/70 bg-muted/30 px-4">
          {hardware.map((item) => (
            <div key={item.id} className="flex min-w-0 items-center gap-3 py-3.5">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-background/65 text-muted-foreground shadow-inner shadow-white/15">
                {item.icon}
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex min-h-5 min-w-0 items-center gap-2">
                  <div className="shrink-0 text-sm font-semibold text-foreground">{item.label}</div>
                  {item.model && (
                    <span
                      data-theme-ui="settings-overview-model"
                      title={item.model}
                      className="ml-auto min-w-0 max-w-[min(68%,22rem)] truncate rounded-full border border-primary/20 bg-background/80 px-2 py-0.5 text-[10px] font-medium leading-4 text-foreground/75 shadow-sm shadow-black/15 backdrop-blur-md"
                    >
                      {item.model}
                    </span>
                  )}
                </div>
                <div
                  data-theme-ui="settings-overview-metrics"
                  className="mt-2 grid min-w-0 grid-cols-[repeat(auto-fit,minmax(5.75rem,1fr))] gap-x-3 gap-y-2"
                >
                  {item.metrics.map((metric) => (
                    <div key={metric.id} className="grid min-w-0 grid-cols-[1rem_auto_minmax(0,1fr)] items-center gap-x-1.5 text-[11px] leading-none text-muted-foreground">
                      <span className="flex h-4 w-4 items-center justify-center">{metric.icon}</span>
                      <span className="whitespace-nowrap">{metric.label}</span>
                      <span className="truncate text-sm font-semibold tabular-nums text-foreground">{metric.value}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          ))}
        </div>

        <div data-theme-card="settings-overview-device" className="grid min-h-[10rem] min-w-0 grid-rows-2 divide-y divide-border/55 rounded-xl border border-border/70 bg-muted/30 px-4">
          <div className="flex min-w-0 items-center gap-3 py-3.5">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-background/65 text-muted-foreground shadow-inner shadow-white/15">
              {device.icon}
            </div>
            <div className="min-w-0 flex-1">
              <div title={device.name} className="line-clamp-2 break-words text-sm font-semibold leading-snug text-foreground">
                {device.name}
              </div>
              <div className="mt-1.5 flex min-w-0 items-center gap-2 text-[11px] text-muted-foreground">
                <span className={clsx('h-2 w-2 shrink-0 rounded-full', device.connected ? 'bg-emerald-500' : 'bg-muted-foreground/45')} />
                <span className="truncate">{device.connectionLabel}</span>
              </div>
            </div>
          </div>

          <div
            className="grid min-w-0 items-center gap-4 py-3.5"
            style={{ gridTemplateColumns: `repeat(${Math.max(device.details.length, 1)}, minmax(0, 1fr))` }}
          >
            {device.details.map((detail) => (
              <div key={detail.id} className="min-w-0">
                <div className="text-[11px] text-muted-foreground">{detail.label}</div>
                <div className="mt-0.5 truncate text-base font-semibold tabular-nums text-foreground">{detail.value}</div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
