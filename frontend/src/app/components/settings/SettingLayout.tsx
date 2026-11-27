import React from 'react';
import { X } from 'lucide-react';
import clsx from 'clsx';
import { types } from '../../../../wailsjs/go/models';
import { formatSpeedRange, summarizeConnection } from '../devices/profile-utils';
import { profileLabel } from './device-connection-utils';

export type SettingIconTone = 'default' | 'blue' | 'emerald' | 'amber' | 'purple' | 'rose';

// Shared settings layout primitives keep ControlPanel sections visually consistent.
export function Section({
  title,
  description,
  icon: Icon,
  iconTone = 'blue',
  children,
  className,
  action,
}: {
  title: string;
  description?: string;
  icon: React.ComponentType<{ className?: string }>;
  iconTone?: SettingIconTone;
  children?: React.ReactNode;
  className?: string;
  action?: React.ReactNode;
}) {
  return (
    <section data-theme-ui="setting-section" className={clsx('rounded-[20px] border border-border/75 bg-card shadow-sm', className)}>
      <div data-theme-ui="setting-section-header" className="flex flex-wrap items-center gap-3 border-b border-border/55 px-5 py-4">
        <div className="flex min-w-0 items-center gap-3">
          <div
            data-theme-ui="setting-section-icon"
            data-icon-tone={iconTone}
            className="flex h-7 w-7 shrink-0 items-center justify-center rounded-[9px] bg-muted/70 text-primary"
          >
            <Icon className="h-4 w-4 shrink-0" />
          </div>
          <div className="min-w-0">
            <h3 className="text-base font-semibold tracking-[-0.015em] text-foreground">{title}</h3>
            {description && <p className="mt-0.5 text-[13px] leading-[1.45] text-muted-foreground">{description}</p>}
          </div>
        </div>
        {action && <div className="ml-auto flex max-w-full flex-wrap items-center justify-end gap-2">{action}</div>}
      </div>
      <div className="divide-y divide-border/45">{children}</div>
    </section>
  );
}

function containsSwitchControl(node: React.ReactNode, depth = 0): boolean {
  if (depth > 4 || node == null || typeof node === 'boolean' || typeof node === 'string' || typeof node === 'number') {
    return false;
  }
  if (Array.isArray(node)) {
    return node.some((child) => containsSwitchControl(child, depth + 1));
  }
  if (!React.isValidElement(node)) {
    return false;
  }
  const props = node.props as Record<string, unknown> | undefined;
  if (props?.['data-theme-ui'] === 'switch') {
    return true;
  }
  const typeObj = node.type as { displayName?: string; name?: string } | undefined;
  const typeName = typeof node.type === 'function' || typeof node.type === 'object'
    ? (typeObj?.displayName || typeObj?.name || '')
    : '';
  if (typeName === 'ToggleSwitch' || typeName === 'Switch') {
    return true;
  }
  return props?.children ? containsSwitchControl(props.children as React.ReactNode, depth + 1) : false;
}

export function SettingRow({
  icon,
  iconTone,
  title,
  description,
  tip,
  children,
  disabled,
}: {
  icon?: React.ReactNode;
  iconTone?: SettingIconTone;
  title: React.ReactNode;
  description?: string;
  tip?: string;
  children?: React.ReactNode;
  disabled?: boolean;
}) {
  const hasControl = Boolean(children);
  const hasSwitch = hasControl && containsSwitchControl(children);
  return (
    <div
      data-theme-ui="setting-row"
      data-has-control={hasControl ? 'true' : undefined}
      data-has-switch={hasSwitch ? 'true' : undefined}
      className={clsx('flex min-h-[4.25rem] flex-col gap-3.5 px-5 py-4 transition-colors duration-200 hover:bg-muted/22 sm:flex-row sm:items-center sm:justify-between', disabled && 'opacity-50')}
    >
      <div className="flex min-w-0 flex-1 items-center gap-3">
        {icon && (
          <div
            data-theme-ui="setting-row-icon"
            data-icon-tone={iconTone}
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[11px] bg-muted/70 text-muted-foreground shadow-inner shadow-white/20"
          >
            {icon}
          </div>
        )}
        <div className="min-w-0">
          <div className="text-[15px] font-semibold tracking-[-0.01em] text-foreground">{title}</div>
          {description && <div className="mt-0.5 text-[13px] leading-[1.45] text-muted-foreground line-clamp-2">{description}</div>}
          {tip && <div className="mt-0.5 text-xs text-primary/80">{tip}</div>}
        </div>
      </div>
      {children && (
        <div
          data-theme-ui="setting-row-control"
          data-has-switch={hasSwitch ? 'true' : undefined}
          className="flex w-full min-w-0 justify-start sm:ml-auto sm:w-auto sm:max-w-[36rem] sm:shrink-0 sm:justify-end"
        >
          {children}
        </div>
      )}
    </div>
  );
}

export function HotkeyField({
  title,
  description,
  value,
  placeholder,
  clearAriaLabel,
  recording,
  onFocus,
  onBlur,
  onKeyDown,
  onClear,
}: {
  title: string;
  description: string;
  value: string;
  placeholder: string;
  clearAriaLabel: string;
  recording: boolean;
  onFocus: () => void;
  onBlur: () => void;
  onKeyDown: (e: React.KeyboardEvent<HTMLInputElement>) => void;
  onClear: () => void;
}) {
  return (
    <div className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 md:flex-row md:items-center md:gap-4">
      <div className="min-w-0 flex-1 pr-2">
        <div className="text-sm font-medium text-foreground">{title}</div>
        <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{description}</div>
      </div>

      <div className="w-full md:ml-auto md:w-[240px] md:flex-none">
        <div className="relative">
          <input
            value={value}
            onFocus={onFocus}
            onBlur={onBlur}
            onKeyDown={onKeyDown}
            readOnly
            placeholder={placeholder}
            className={clsx(
              'h-9 w-full rounded-lg border bg-background/80 px-3 pr-9 text-center font-sans text-sm text-foreground outline-none transition',
              recording
                ? 'border-primary shadow-[0_0_0_1px_var(--color-primary)] ring-2 ring-primary/20'
                : 'border-border/70 hover:border-border',
            )}
          />
          {value && (
            <button
              type="button"
              aria-label={clearAriaLabel}
              onMouseDown={(e) => e.preventDefault()}
              onClick={onClear}
              className="absolute right-2 top-1/2 -translate-y-1/2 rounded-full p-0.5 text-muted-foreground transition hover:bg-muted hover:text-foreground"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

export function SelectionField({
  label,
  children,
  hint,
  hintClassName,
  hintTitle,
}: {
  label: string;
  children: React.ReactNode;
  hint?: string;
  hintClassName?: string;
  hintTitle?: string;
}) {
  return (
    <div className="space-y-1.5">
      <div className="text-[12px] font-medium text-muted-foreground">{label}</div>
      {children}
      {hint && (
        <div
          title={hintTitle ?? hint}
          className={clsx('text-[12px] leading-relaxed text-muted-foreground', hintClassName)}
        >
          {hint}
        </div>
      )}
    </div>
  );
}

export function ConnectionPanel({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: React.ReactNode;
}) {
  return (
    <div data-theme-ui="connection-panel" className="min-w-0 rounded-2xl border border-border/60 bg-background/45 p-3 shadow-inner shadow-white/10">
      <div className="mb-2 min-w-0 text-sm font-medium text-foreground">{title}</div>
      {description && <p className="mb-3 text-xs leading-relaxed text-muted-foreground">{description}</p>}
      {children}
    </div>
  );
}

export function CompatibilitySubmenu({ children }: { children: React.ReactNode }) {
  return (
    <div data-theme-ui="compatibility-submenu" className="border-t border-border/50 bg-muted/10 px-5 py-3">
      <div className="sm:pl-12">
        <div className="overflow-hidden rounded-2xl border border-border/60 bg-card/86 shadow-sm shadow-black/5">
          <div className="divide-y divide-border/45">{children}</div>
        </div>
      </div>
    </div>
  );
}

export function CompatibilitySubmenuRow({
  icon,
  iconTone,
  title,
  description,
  tip,
  children,
  below,
}: {
  icon?: React.ReactNode;
  iconTone?: SettingIconTone;
  title: string;
  description?: string;
  tip?: string;
  children?: React.ReactNode;
  below?: React.ReactNode;
}) {
  return (
    <div data-theme-ui="compatibility-submenu-row" className="px-5 py-4 transition-colors duration-200 hover:bg-muted/16">
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          {icon && (
            <div
              data-theme-ui="setting-row-icon"
              data-icon-tone={iconTone}
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[11px] bg-muted/70 text-muted-foreground shadow-inner shadow-white/20"
            >
              {icon}
            </div>
          )}
          <div className="min-w-0">
            <div className="text-[15px] font-semibold tracking-[-0.01em] text-foreground">{title}</div>
            {description && <div className="mt-0.5 text-[13px] leading-[1.45] text-muted-foreground">{description}</div>}
            {tip && <div className="mt-0.5 text-xs leading-relaxed text-primary/80">{tip}</div>}
          </div>
        </div>
        {children && <div className="w-full md:w-auto md:shrink-0">{children}</div>}
      </div>
      {below && <div className={clsx('mt-3', icon && 'md:pl-12')}>{below}</div>}
    </div>
  );
}

export function EmptyConnectionState({ children, action }: { children: React.ReactNode; action?: React.ReactNode }) {
  return (
    <div
      className={clsx(
        'rounded-md border border-dashed border-border/80 bg-muted/20 px-3 py-2 text-xs leading-relaxed text-muted-foreground',
        action && 'flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'
      )}
    >
      <div className="min-w-0">{children}</div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  );
}

export function DeviceProfileSummary({ profile }: { profile: types.DeviceProfile }) {
  return (
    <div className="rounded-md border border-border bg-card/70 px-3 py-2">
      <div className="truncate text-sm font-medium text-foreground">{profileLabel(profile)}</div>
      <div className="mt-1 text-[11px] leading-relaxed text-muted-foreground">
        {`${summarizeConnection(profile)} · ${formatSpeedRange(profile)}`}
      </div>
    </div>
  );
}

export function DeviceProfileInline({
  profile,
  empty,
}: {
  profile?: types.DeviceProfile | null;
  empty: string;
}) {
  if (!profile) {
    return <InlineHint>{empty}</InlineHint>;
  }

  return (
    <div className="min-w-0 text-left md:max-w-[520px] md:text-right">
      <div className="truncate text-sm font-medium text-foreground">{profileLabel(profile)}</div>
      <div className="mt-0.5 truncate text-xs text-muted-foreground">
        {`${summarizeConnection(profile)} · ${formatSpeedRange(profile)}`}
      </div>
    </div>
  );
}

export function InlineHint({ children }: { children: React.ReactNode }) {
  return (
    <div className="max-w-[420px] text-sm leading-relaxed text-muted-foreground">
      {children}
    </div>
  );
}
