'use client';

import type { ComponentType, DragEvent, ReactNode } from 'react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Bluetooth,
  Boxes,
  CheckCircle2,
  Download,
  FileInput,
  Library,
  Loader2,
  Pencil,
  Plus,
  RadioTower,
  RefreshCw,
  Save,
  Send,
  ShieldAlert,
  Trash2,
  Usb,
  Wifi,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { types } from '../../../wailsjs/go/models';
import { apiService } from '../services/api';
import type { DeviceDebugCommandResult } from '../types/app';
import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  NumberInput,
} from './ui';
import DeviceProfileEditorDialog from './devices/DeviceProfileEditorDialog';
import {
  createDraftFromProfile,
  formatSpeedRange,
  getProfileDisplayName,
  getProfileIdentity,
  normalizeSpeedUnit,
  normalizeTransport,
  summarizeConnection,
  type DeviceTransport,
  type DeviceProfileDraft,
} from './devices/profile-utils';

interface AdvancedDevicesPanelProps {
  config: types.AppConfig;
  isConnected: boolean;
  onConfigChange: (config: types.AppConfig) => void;
}

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function transportIcon(transport?: string) {
  switch (normalizeTransport(transport)) {
    case 'ble':
      return Bluetooth;
    case 'serial':
      return Usb;
    case 'hid':
      return RadioTower;
    default:
      return Wifi;
  }
}

const DEVICE_TRANSPORT_VALUES: DeviceTransport[] = ['wifi', 'ble', 'serial', 'hid'];
const COMPATIBILITY_TRANSPORT_VALUES: DeviceTransport[] = ['wifi', 'serial'];

function isManualCompatibilityTransport(transport?: string) {
  const normalized = normalizeTransport(transport);
  return normalized === 'wifi' || normalized === 'serial';
}

function activeIdsByTransportFromConfig(config: types.AppConfig): Record<string, string> {
  const raw = (config as any).activeDeviceProfileIdsByTransport;
  return raw && typeof raw === 'object' ? raw as Record<string, string> : {};
}

function activeIdsByTransportFromPayload(payload?: types.DeviceProfilesPayload | null): Record<string, string> {
  const raw = (payload as any)?.activeIdsByTransport;
  return raw && typeof raw === 'object' ? raw as Record<string, string> : {};
}

function profileForTransport(profiles: types.DeviceProfile[], transport: DeviceTransport) {
  return profiles.find((profile) => normalizeTransport(profile.transport) === transport) || null;
}

function activeProfileForTransport(
  profiles: types.DeviceProfile[],
  activeIdsByTransport: Record<string, string>,
  transport: DeviceTransport,
) {
  const activeId = activeIdsByTransport[transport] || '';
  return profiles.find((profile) => profile.id === activeId && normalizeTransport(profile.transport) === transport)
    || profileForTransport(profiles, transport);
}

function isLikelyHexCommand(value: string) {
  const trimmed = value.trim();
  return trimmed.length > 0 && /^[0-9a-fA-FxX\s,;:\-]+$/.test(trimmed) && /[0-9a-fA-F]/.test(trimmed);
}

function profileDraftFromDebugResult({
  activeProfile,
  config,
  result,
  displayName,
  notes,
}: {
  activeProfile: types.DeviceProfile | null;
  config: types.AppConfig;
  result: DeviceDebugCommandResult;
  displayName: string;
  notes: string;
}): DeviceProfileDraft {
  const draft = createDraftFromProfile(activeProfile);
  const command = (result.rawHex || result.frameHex || result.inputHex || '').trim();
  const transport = normalizeTransport(result.transport || activeProfile?.transport || (config as any).deviceTransport);
  const speedUnit = normalizeSpeedUnit(activeProfile?.speedUnit);

  draft.displayName = displayName;
  draft.transport = transport;
  draft.speedUnit = speedUnit;
  draft.commandEncoding = isLikelyHexCommand(command) ? 'hex' : 'raw';
  draft.checksum = draft.checksum || 'none';
  draft.setSpeedCommand = command;
  draft.notes = [draft.notes, notes].filter(Boolean).join('\n');

  if (transport === 'wifi' && !draft.endpoint.trim()) {
    draft.endpoint = ((config as any).fanControlDeviceIp || '').trim();
  }
  return draft;
}

function Section({
  title,
  description,
  icon: Icon,
  children,
  action,
}: {
  title: string;
  description?: string;
  icon: ComponentType<{ className?: string }>;
  children: ReactNode;
  action?: ReactNode;
}) {
  return (
    <section className="rounded-2xl border border-border bg-card shadow-sm">
      <div className="flex flex-col gap-3 border-b border-border/60 px-4 py-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            <Icon className="h-4.5 w-4.5" />
          </div>
          <div className="min-w-0">
            <h2 className="text-base font-semibold text-foreground">{title}</h2>
            {description && <p className="mt-1 text-sm leading-relaxed text-muted-foreground">{description}</p>}
          </div>
        </div>
        {action && <div className="flex shrink-0 flex-wrap gap-2">{action}</div>}
      </div>
      <div>{children}</div>
    </section>
  );
}

function EmptyState({ children }: { children: ReactNode }) {
  return (
    <div className="px-4 py-8 text-center text-sm text-muted-foreground">
      {children}
    </div>
  );
}

function CapabilityPills({ profile }: { profile: types.DeviceProfile }) {
  const { t } = useTranslation();
  const caps = profile.capabilities;
  const items = [
    caps?.supportsReadState ? t('advancedDevices.capabilities.read') : '',
    caps?.supportsSetSpeed ? t('advancedDevices.capabilities.setSpeed') : '',
    caps?.supportsManualGears ? t('fanCurve.manualGear.title') : '',
    caps?.supportsCustomSpeed ? t('controlPanel.fan.customSpeedTitle') : '',
    caps?.supportsDebugFrames ? t('advancedDevices.capabilities.debugFrames') : '',
    caps?.supportsRawCommands ? t('advancedDevices.capabilities.raw') : '',
    ((caps as any)?.supportsGearLight || caps?.supportsLighting) ? t('advancedDevices.capabilities.gearLight') : '',
    caps?.supportsLighting ? t('advancedDevices.capabilities.lighting') : '',
    ((caps as any)?.supportsBrightness || caps?.supportsLighting) ? t('advancedDevices.capabilities.brightness') : '',
    caps?.supportsPowerOnStart ? t('advancedDevices.capabilities.powerOnStart') : '',
    caps?.supportsSmartStartStop ? t('advancedDevices.capabilities.smartStartStop') : '',
    (caps as any)?.supportsScreen ? t('advancedDevices.capabilities.screen') : '',
  ].filter(Boolean);

  if (items.length === 0) {
    return <span className="text-xs text-muted-foreground">{t('advancedDevices.capabilities.none')}</span>;
  }

  return (
    <div className="flex flex-wrap gap-1.5">
      {items.map((item) => (
        <span key={item} className="rounded-full border border-border/70 bg-muted/45 px-2 py-0.5 text-[11px] text-muted-foreground">
          {item}
        </span>
      ))}
    </div>
  );
}

function ProfileRow({
  profile,
  active,
  selectable = true,
  context,
  onSelect,
  onEdit,
  onDelete,
}: {
  profile: types.DeviceProfile;
  active?: boolean;
  selectable?: boolean;
  context?: 'template' | 'device';
  onSelect?: (profileID: string) => void;
  onEdit?: (profile: types.DeviceProfile) => void;
  onDelete?: (profile: types.DeviceProfile) => void;
}) {
  const { t } = useTranslation();
  const Icon = transportIcon(profile.transport);
  const identity = getProfileIdentity(profile);
  const displayName = getProfileDisplayName(profile, t('advancedDevices.status.unnamedDevice'));
  const builtInLabel = context === 'template'
    ? t('advancedDevices.status.template')
    : t('advancedDevices.status.builtin');

  return (
    <div className="flex flex-col gap-3 border-b border-border/60 px-4 py-3 last:border-b-0 lg:flex-row lg:items-center lg:justify-between">
      <div className="flex min-w-0 gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-border/70 bg-background text-muted-foreground">
          <Icon className="h-4.5 w-4.5" />
        </div>
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <div className="truncate text-sm font-semibold text-foreground">{displayName}</div>
            {active && <Badge variant="success">{t('advancedDevices.status.active')}</Badge>}
            {profile.builtIn ? <Badge variant="info">{builtInLabel}</Badge> : <Badge>{t('advancedDevices.status.user')}</Badge>}
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
            <span>{t(`advancedDevices.transport.${normalizeTransport(profile.transport)}`)}</span>
            <span>{formatSpeedRange(profile)}</span>
            {identity && <span className="truncate">{identity}</span>}
          </div>
          <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{summarizeConnection(profile)}</div>
          <div className="mt-2">
            <CapabilityPills profile={profile} />
          </div>
        </div>
      </div>

      <div className="flex shrink-0 flex-wrap gap-2 lg:justify-end">
        {selectable && (
          <Button
            variant={active ? 'secondary' : 'outline'}
            size="sm"
            onClick={() => onSelect?.(profile.id)}
            disabled={active}
            icon={<CheckCircle2 className="h-3.5 w-3.5" />}
          >
            {active ? t('advancedDevices.status.active') : t('advancedDevices.actions.setActive')}
          </Button>
        )}
        {onEdit && !profile.builtIn && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => onEdit(profile)}
            icon={<Pencil className="h-3.5 w-3.5" />}
          >
            {t('advancedDevices.actions.editDevice')}
          </Button>
        )}
        {onDelete && !profile.builtIn && (
          <Button
            variant="danger"
            size="sm"
            onClick={() => onDelete(profile)}
            icon={<Trash2 className="h-3.5 w-3.5" />}
          >
            {t('common.actions.delete')}
          </Button>
        )}
      </div>
    </div>
  );
}

function ConnectionCategoryBanner({
  profiles,
  activeId,
  activeIdsByTransport,
  onSelect,
}: {
  profiles: types.DeviceProfile[];
  activeId: string;
  activeIdsByTransport: Record<string, string>;
  onSelect: (profileID: string) => void;
}) {
  const { t } = useTranslation();

  return (
    <div className="rounded-2xl border border-border bg-card px-4 py-3 shadow-sm">
      <div className="flex flex-col gap-1 pb-3">
        <div className="text-sm font-semibold text-foreground">{t('advancedDevices.connectionBanner.title')}</div>
        <div className="text-xs leading-relaxed text-muted-foreground">{t('advancedDevices.connectionBanner.description')}</div>
      </div>
      <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
        {COMPATIBILITY_TRANSPORT_VALUES.map((transport) => {
          const profile = activeProfileForTransport(profiles, activeIdsByTransport, transport);
          const Icon = transportIcon(transport);
          const active = Boolean(profile?.id && profile.id === activeId);
          const displayName = profile ? getProfileDisplayName(profile, t('advancedDevices.status.unnamedDevice')) : '';

          return (
            <button
              key={transport}
              type="button"
              disabled={!profile}
              onClick={() => profile && onSelect(profile.id)}
              className={`min-h-24 rounded-xl border px-3 py-3 text-left transition-colors ${
                active
                  ? 'border-primary bg-primary/10 text-primary'
                  : 'border-border bg-background hover:border-primary/45 hover:bg-muted/50 disabled:cursor-not-allowed disabled:opacity-65'
              }`}
            >
              <div className="flex items-center justify-between gap-2">
                <div className="flex min-w-0 items-center gap-2">
                  <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-border/70 bg-card text-muted-foreground">
                    <Icon className="h-4 w-4" />
                  </span>
                  <span className="truncate text-sm font-semibold">{t(`advancedDevices.transport.${transport}`)}</span>
                </div>
                {active && <Badge variant="success">{t('advancedDevices.status.active')}</Badge>}
              </div>
              <div className="mt-2 truncate text-sm text-foreground">
                {profile ? displayName : t('advancedDevices.connectionBanner.noDevice')}
              </div>
              <div className="mt-1 truncate text-xs text-muted-foreground">
                {profile ? summarizeConnection(profile) : t('advancedDevices.connectionBanner.addHint')}
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}

function GuardedEntry({
  onFirstConfirm,
}: {
  onFirstConfirm: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="mx-auto flex min-h-[58vh] max-w-2xl flex-col items-center justify-center px-3 py-10 text-center">
      <div className="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl border border-amber-300/50 bg-amber-500/10 text-amber-600 dark:border-amber-700/40 dark:text-amber-300">
        <ShieldAlert className="h-7 w-7" />
      </div>
      <h1 className="text-2xl font-semibold tracking-normal text-foreground">{t('advancedDevices.gate.title')}</h1>
      <p className="mt-3 max-w-xl text-sm leading-7 text-muted-foreground">{t('advancedDevices.gate.description')}</p>
      <div className="mt-5 rounded-xl border border-amber-300/45 bg-amber-50/80 px-4 py-3 text-left text-sm leading-6 text-amber-900 dark:border-amber-700/40 dark:bg-amber-900/15 dark:text-amber-100">
        <div className="font-medium">{t('advancedDevices.gate.warningTitle')}</div>
        <div className="mt-1 text-xs leading-6">{t('advancedDevices.gate.warningBody')}</div>
      </div>
      <Button className="mt-6" onClick={onFirstConfirm} icon={<ShieldAlert className="h-4 w-4" />}>
        {t('advancedDevices.gate.primary')}
      </Button>
    </div>
  );
}

function ImportDevicePanel({
  importCode,
  busy,
  onImportCodeChange,
  onImport,
  onImportFile,
}: {
  importCode: string;
  busy: boolean;
  onImportCodeChange: (value: string) => void;
  onImport: () => void;
  onImportFile: (file: File) => void;
}) {
  const { t } = useTranslation();
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [dragActive, setDragActive] = useState(false);

  const handleFileList = (files: FileList | null) => {
    const file = files?.[0];
    if (file) {
      onImportFile(file);
    }
  };

  const handleDrop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    setDragActive(false);
    handleFileList(event.dataTransfer.files);
  };

  return (
    <div>
      <div className="space-y-3 px-4 py-4">
        <div className="flex items-center justify-between gap-2">
          <div className="text-sm font-medium text-foreground">{t('advancedDevices.importExport.importTitle')}</div>
          <div className="flex flex-wrap justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => fileInputRef.current?.click()}
              disabled={busy}
              icon={<FileInput className="h-3.5 w-3.5" />}
            >
              {t('advancedDevices.importExport.chooseFile')}
            </Button>
            <Button variant="secondary" size="sm" onClick={onImport} loading={busy} icon={<FileInput className="h-3.5 w-3.5" />}>
              {t('common.actions.import')}
            </Button>
          </div>
        </div>
        <input
          ref={fileInputRef}
          type="file"
          accept=".fcdp,.txt,.json,application/json,text/plain"
          className="hidden"
          onChange={(event) => {
            handleFileList(event.target.files);
            event.target.value = '';
          }}
        />
        <div
          role="button"
          tabIndex={0}
          onClick={() => fileInputRef.current?.click()}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ' ') {
              event.preventDefault();
              fileInputRef.current?.click();
            }
          }}
          onDragOver={(event) => {
            event.preventDefault();
            setDragActive(true);
          }}
          onDragLeave={() => setDragActive(false)}
          onDrop={handleDrop}
          className={`rounded-xl border border-dashed px-3 py-3 text-sm transition-colors ${
            dragActive
              ? 'border-primary bg-primary/10 text-primary'
              : 'border-border bg-muted/25 text-muted-foreground hover:border-primary/45 hover:bg-muted/45'
          }`}
        >
          <div className="font-medium text-foreground">{t('advancedDevices.importExport.dropTitle')}</div>
          <div className="mt-1 text-xs leading-relaxed">{t('advancedDevices.importExport.dropHint')}</div>
        </div>
        <textarea
          value={importCode}
          onChange={(event) => onImportCodeChange(event.target.value)}
          rows={5}
          className="w-full resize-none rounded-lg border border-border bg-background px-3 py-2 font-mono text-xs leading-relaxed outline-none ring-offset-background transition-colors focus-visible:ring-2 focus-visible:ring-ring"
          placeholder={t('advancedDevices.importExport.importPlaceholder')}
        />
      </div>
    </div>
  );
}

function DebugProfilePanel({
  isConnected,
  activeProfile,
  config,
  onSaveDraft,
}: {
  isConnected: boolean;
  activeProfile: types.DeviceProfile | null;
  config: types.AppConfig;
  onSaveDraft: (draft: DeviceProfileDraft) => void;
}) {
  const { t } = useTranslation();
  const [rawCommand, setRawCommand] = useState('27');
  const [waitMs, setWaitMs] = useState(800);
  const [result, setResult] = useState<DeviceDebugCommandResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [pendingCommand, setPendingCommand] = useState<{ command: string; waitMs: number } | null>(null);

  const closeConfirm = () => {
    if (loading) return;
    setConfirmOpen(false);
    setPendingCommand(null);
  };

  const requestCommandSend = () => {
    const command = rawCommand.trim();
    if (!command) {
      toast.warning(t('advancedDevices.validation.rawCommandRequired'));
      return;
    }
    if (!isConnected) {
      toast.warning(t('advancedDevices.debug.notConnected'));
      return;
    }
    setPendingCommand({ command, waitMs });
    setConfirmOpen(true);
  };

  const sendCommand = async () => {
    const command = pendingCommand?.command ?? rawCommand.trim();
    const commandWaitMs = pendingCommand?.waitMs ?? waitMs;
    if (!command) {
      toast.warning(t('advancedDevices.validation.rawCommandRequired'));
      return;
    }
    setLoading(true);
    try {
      const response = await apiService.sendDeviceDebugCommand(command, commandWaitMs);
      setResult(response);
      toast.success(t('advancedDevices.toast.debugSent'));
      setConfirmOpen(false);
      setPendingCommand(null);
    } catch (error) {
      toast.error(t('advancedDevices.toast.debugFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-4 px-4 py-4">
      <div className="rounded-xl border border-amber-300/40 bg-amber-500/10 px-3 py-2 text-xs leading-6 text-amber-900 dark:border-amber-700/40 dark:text-amber-100">
        {t('advancedDevices.debug.warning')}
      </div>
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-[1fr_140px_auto] lg:items-end">
        <label className="space-y-1.5">
          <span className="text-xs font-medium text-muted-foreground">{t('advancedDevices.fields.rawCommand')}</span>
          <input
            value={rawCommand}
            onChange={(event) => setRawCommand(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') requestCommandSend();
            }}
            className="h-10 w-full rounded-md border border-input bg-background px-3 font-mono text-xs text-foreground outline-none ring-offset-background transition-colors focus-visible:ring-2 focus-visible:ring-ring"
            placeholder="27 or 5A A5 27 02 29"
          />
        </label>
        <NumberInput
          value={waitMs}
          onChange={setWaitMs}
          min={0}
          max={5000}
          step={50}
          label={t('advancedDevices.fields.waitMs')}
        />
        <Button
          onClick={requestCommandSend}
          loading={loading}
          disabled={!isConnected}
          icon={<Send className="h-4 w-4" />}
        >
          {t('advancedDevices.actions.send')}
        </Button>
      </div>
      {!isConnected && (
        <div className="text-xs text-muted-foreground">{t('advancedDevices.debug.notConnected')}</div>
      )}
      <div className="min-h-32 max-h-72 overflow-auto rounded-xl border border-border bg-background p-3">
        {result ? (
          <pre className="min-w-max whitespace-pre font-mono text-xs leading-5 text-foreground/90">{JSON.stringify(result, null, 2)}</pre>
        ) : (
          <div className="flex h-24 items-center justify-center text-sm text-muted-foreground">
            {t('advancedDevices.debug.resultPlaceholder')}
          </div>
        )}
      </div>
      {result && (
        <div className="flex flex-col gap-2 rounded-xl border border-border bg-muted/25 px-3 py-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="text-xs leading-6 text-muted-foreground">
            {t('advancedDevices.debug.saveProfileHint')}
          </div>
          <Button
            variant="outline"
            size="sm"
            icon={<Save className="h-3.5 w-3.5" />}
            onClick={() => {
              toast.dismiss();
              const transport = normalizeTransport(result.transport || activeProfile?.transport || (config as any).deviceTransport);
              if (!isManualCompatibilityTransport(transport)) {
                toast.info(t('advancedDevices.toast.nativeAutoManaged'));
                return;
              }
              const baseName = activeProfile
                ? getProfileDisplayName(activeProfile, t(`advancedDevices.transport.${transport}`))
                : t(`advancedDevices.transport.${transport}`);
              onSaveDraft(profileDraftFromDebugResult({
                activeProfile,
                config,
                result,
                displayName: t('advancedDevices.debug.profileName', { name: baseName }),
                notes: t('advancedDevices.debug.profileNotes', {
                  command: result.inputHex || result.rawHex,
                  waitMs: result.waitMs,
                }),
              }));
            }}
          >
            {t('advancedDevices.actions.saveDebugProfile')}
          </Button>
        </div>
      )}
      <Dialog
        open={confirmOpen}
        onOpenChange={(open) => {
          if (open) {
            setConfirmOpen(true);
          } else {
            closeConfirm();
          }
        }}
      >
        <DialogContent className="max-h-[calc(100vh-2rem)] w-[calc(100vw-2rem)] max-w-lg overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <ShieldAlert className="h-5 w-5 text-amber-600" />
              {t('advancedDevices.debug.confirmTitle')}
            </DialogTitle>
            <DialogDescription>{t('advancedDevices.debug.confirmDescription')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-3 rounded-lg border border-border bg-muted/35 p-3 text-sm">
            <div className="space-y-1">
              <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                {t('advancedDevices.debug.confirmCommand')}
              </div>
              <pre className="max-h-28 overflow-auto rounded-md bg-background px-3 py-2 font-mono text-xs text-foreground">{pendingCommand?.command ?? ''}</pre>
            </div>
            <div className="flex items-center justify-between gap-4 text-xs text-muted-foreground">
              <span>{t('advancedDevices.debug.confirmWait')}</span>
              <span className="font-mono text-foreground">{pendingCommand?.waitMs ?? waitMs} ms</span>
            </div>
          </div>
          <DialogFooter>
            <Button variant="secondary" onClick={closeConfirm}>
              {t('common.actions.cancel')}
            </Button>
            <Button
              variant="danger"
              loading={loading}
              onClick={() => void sendCommand()}
              icon={<Send className="h-4 w-4" />}
            >
              {t('advancedDevices.debug.confirmSend')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function DeleteDeviceDialog({
  profile,
  busy,
  onOpenChange,
  onConfirm,
}: {
  profile: types.DeviceProfile | null;
  busy: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}) {
  const { t } = useTranslation();
  const Icon = transportIcon(profile?.transport);
  const displayName = profile ? getProfileDisplayName(profile, t('advancedDevices.status.unnamedDevice')) : '';

  return (
    <Dialog open={Boolean(profile)} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Trash2 className="h-5 w-5 text-destructive" />
            {t('advancedDevices.dialog.deleteTitle')}
          </DialogTitle>
          <DialogDescription>{t('advancedDevices.dialog.deleteDescription')}</DialogDescription>
        </DialogHeader>

        {profile && (
          <div className="space-y-3">
            <div className="flex gap-3 rounded-xl border border-border bg-muted/35 p-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-border/70 bg-background text-muted-foreground">
                <Icon className="h-4.5 w-4.5" />
              </div>
              <div className="min-w-0">
                <div className="truncate text-sm font-semibold text-foreground">{displayName}</div>
                <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                  <span>{t(`advancedDevices.transport.${normalizeTransport(profile.transport)}`)}</span>
                  <span>{formatSpeedRange(profile)}</span>
                </div>
                <div className="mt-1 truncate text-xs text-muted-foreground">{summarizeConnection(profile)}</div>
              </div>
            </div>
            <div className="rounded-xl border border-destructive/25 bg-destructive/5 px-3 py-2 text-xs leading-6 text-destructive">
              {t('advancedDevices.dialog.deleteWarning')}
            </div>
          </div>
        )}

        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={busy}>
            {t('common.actions.cancel')}
          </Button>
          <Button
            variant="danger"
            loading={busy}
            onClick={onConfirm}
            icon={<Trash2 className="h-4 w-4" />}
          >
            {t('advancedDevices.dialog.deleteAction')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default function AdvancedDevicesPanel({ config, isConnected, onConfigChange }: AdvancedDevicesPanelProps) {
  const { t } = useTranslation();
  const [unlocked, setUnlocked] = useState(false);
  const [secondWarningOpen, setSecondWarningOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [profiles, setProfiles] = useState<types.DeviceProfile[]>([]);
  const [supportedProfiles, setSupportedProfiles] = useState<types.DeviceProfile[]>([]);
  const [activeId, setActiveId] = useState((config as any).activeDeviceProfileId || '');
  const [activeIdsByTransport, setActiveIdsByTransport] = useState<Record<string, string>>(
    () => activeIdsByTransportFromConfig(config),
  );
  const [addOpen, setAddOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [editProfileDraft, setEditProfileDraft] = useState<DeviceProfileDraft | null>(null);
  const [debugProfileOpen, setDebugProfileOpen] = useState(false);
  const [debugProfileDraft, setDebugProfileDraft] = useState<DeviceProfileDraft | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<types.DeviceProfile | null>(null);
  const [importCode, setImportCode] = useState('');

  const activeProfile = useMemo(
    () => profiles.find((profile) => profile.id === activeId) || profiles[0] || null,
    [activeId, profiles],
  );
  const exportableUserProfiles = useMemo(
    () => profiles.filter((profile) => !profile.builtIn),
    [profiles],
  );
  const deviceProfilesByTransport = useMemo(
    () => DEVICE_TRANSPORT_VALUES
      .map((transport) => ({
        transport,
        profiles: profiles.filter((profile) => normalizeTransport(profile.transport) === transport),
      }))
      .filter((group) => group.profiles.length > 0),
    [profiles],
  );

  const refreshConfig = useCallback(async () => {
    try {
      const latest = await apiService.getConfig();
      onConfigChange(types.AppConfig.createFrom(latest));
    } catch {
      /* Config events also update the store; this is only an eager refresh. */
    }
  }, [onConfigChange]);

  const loadProfiles = useCallback(async () => {
    setLoading(true);
    try {
      const [payload, supported] = await Promise.all([
        apiService.getDeviceProfiles(),
        apiService.getSupportedDeviceProfiles(),
      ]);
      const nextProfiles = Array.isArray(payload?.profiles) ? payload.profiles : [];
      setProfiles(nextProfiles);
      setSupportedProfiles(Array.isArray(supported) ? supported : []);
      setActiveId(payload?.activeId || nextProfiles[0]?.id || '');
      setActiveIdsByTransport(activeIdsByTransportFromPayload(payload));
    } catch (error) {
      toast.error(t('advancedDevices.toast.loadFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    if (unlocked) {
      void loadProfiles();
    }
  }, [loadProfiles, unlocked]);

  useEffect(() => {
    const configActiveId = ((config as any).activeDeviceProfileId || '') as string;
    if (configActiveId) {
      setActiveId(configActiveId);
    }
    setActiveIdsByTransport(activeIdsByTransportFromConfig(config));
  }, [config]);

  const handleSelectProfile = async (profileID: string) => {
    if (!profileID || profileID === activeId) return;
    const current = profiles.find((profile) => profile.id === profileID);
    if (current && !isManualCompatibilityTransport(current.transport)) {
      toast.info(t('advancedDevices.toast.nativeAutoManaged'));
      return;
    }
    setBusy(true);
    try {
      const profile = await apiService.setActiveDeviceProfile(profileID);
      setActiveId(profile.id);
      setActiveIdsByTransport((prev) => ({
        ...prev,
        [normalizeTransport(profile.transport)]: profile.id,
      }));
      await refreshConfig();
      await loadProfiles();
      toast.success(t('advancedDevices.toast.profileActivated'));
    } catch (error) {
      toast.error(t('advancedDevices.toast.activeFailed', { error: getErrorMessage(error) }));
    } finally {
      setBusy(false);
    }
  };

  const handleSaveProfile = async (profile: types.DeviceProfile, setActive: boolean) => {
    const shouldSetActive = setActive && isManualCompatibilityTransport(profile.transport);
    const saved = await apiService.saveDeviceProfile(profile, shouldSetActive);
    if (shouldSetActive) {
      setActiveId(saved.id);
      setActiveIdsByTransport((prev) => ({
        ...prev,
        [normalizeTransport(saved.transport)]: saved.id,
      }));
    }
    await refreshConfig();
    await loadProfiles();
    toast.success(t('advancedDevices.toast.saved'));
  };

  const handleSaveDebugDraft = (draft: DeviceProfileDraft) => {
    setDebugProfileDraft(draft);
    setDebugProfileOpen(true);
  };

  const handleEditProfile = (profile: types.DeviceProfile) => {
    setEditProfileDraft(createDraftFromProfile(profile));
    setEditOpen(true);
  };

  const requestDeleteProfile = (profile: types.DeviceProfile) => {
    setDeleteTarget(profile);
  };

  const confirmDeleteProfile = async () => {
    if (!deleteTarget?.id) return;
    setBusy(true);
    try {
      await apiService.deleteDeviceProfile(deleteTarget.id);
      await refreshConfig();
      await loadProfiles();
      setDeleteTarget(null);
      toast.success(t('advancedDevices.toast.deleted'));
    } catch (error) {
      toast.error(t('advancedDevices.toast.deleteFailed', { error: getErrorMessage(error) }));
    } finally {
      setBusy(false);
    }
  };

  const handleBatchExport = async () => {
    setBusy(true);
    try {
      const path = await apiService.exportDeviceProfilesToFile();
      if (path) {
        toast.success(t('advancedDevices.toast.exportDownloaded'));
      }
    } catch (error) {
      toast.error(t('advancedDevices.toast.exportFailed', { error: getErrorMessage(error) }));
    } finally {
      setBusy(false);
    }
  };

  const handleImport = async (code?: string, rethrow = false) => {
    const source = (code ?? importCode).trim();
    if (!source) {
      toast.warning(t('advancedDevices.toast.importMissing'));
      return;
    }
    setBusy(true);
    try {
      await apiService.importDeviceProfiles(source);
      setImportCode('');
      await refreshConfig();
      await loadProfiles();
      toast.success(t('advancedDevices.toast.imported'));
    } catch (error) {
      toast.error(t('advancedDevices.toast.importFailed', { error: getErrorMessage(error) }));
      if (rethrow) {
        throw error;
      }
    } finally {
      setBusy(false);
    }
  };

  const handleImportFile = async (file: File) => {
    setBusy(true);
    try {
      const code = await file.text();
      setImportCode(code);
      await handleImport(code);
    } catch (error) {
      toast.error(t('advancedDevices.toast.importFileFailed', { error: getErrorMessage(error) }));
    } finally {
      setBusy(false);
    }
  };

  if (!unlocked) {
    return (
      <>
        <GuardedEntry onFirstConfirm={() => setSecondWarningOpen(true)} />
        <Dialog open={secondWarningOpen} onOpenChange={setSecondWarningOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>{t('advancedDevices.gate.dialogTitle')}</DialogTitle>
              <DialogDescription>{t('advancedDevices.gate.dialogDescription')}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="secondary" onClick={() => setSecondWarningOpen(false)}>
                {t('common.actions.cancel')}
              </Button>
              <Button
                onClick={() => {
                  setSecondWarningOpen(false);
                  setUnlocked(true);
                }}
                icon={<ShieldAlert className="h-4 w-4" />}
              >
                {t('advancedDevices.gate.confirm')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </>
    );
  }

  return (
    <div data-page-reveal="cards" className="space-y-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-semibold tracking-normal text-foreground">{t('advancedDevices.title')}</h1>
            {activeProfile && <Badge variant="info">{getProfileDisplayName(activeProfile, t('advancedDevices.status.unnamedDevice'))}</Badge>}
          </div>
          <p className="mt-1 max-w-3xl text-sm leading-6 text-muted-foreground">{t('advancedDevices.subtitle')}</p>
        </div>
        <div className="flex shrink-0 flex-wrap gap-2">
          <Button variant="secondary" onClick={() => void loadProfiles()} loading={loading} icon={<RefreshCw className="h-4 w-4" />}>
            {t('common.actions.refresh')}
          </Button>
          <Button onClick={() => setAddOpen(true)} icon={<Plus className="h-4 w-4" />}>
            {t('advancedDevices.actions.addDevice')}
          </Button>
        </div>
      </div>

      {busy && (
        <div className="flex items-center gap-2 rounded-lg border border-border bg-muted/45 px-3 py-2 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          {t('advancedDevices.status.working')}
        </div>
      )}

      <ConnectionCategoryBanner
        profiles={profiles}
        activeId={activeId}
        activeIdsByTransport={activeIdsByTransport}
        onSelect={handleSelectProfile}
      />

      <Section
        title={t('advancedDevices.sections.supported')}
        description={t('advancedDevices.sections.supportedHint')}
        icon={Library}
      >
        {supportedProfiles.length > 0 ? (
          supportedProfiles.map((profile) => (
            <ProfileRow
              key={profile.id}
              profile={profile}
              context="template"
              selectable={false}
              onSelect={handleSelectProfile}
            />
          ))
        ) : (
          <EmptyState>{t('advancedDevices.sections.emptySupported')}</EmptyState>
        )}
      </Section>

      <Section
        title={t('advancedDevices.sections.user')}
        description={t('advancedDevices.sections.userHint')}
        icon={Boxes}
        action={(
          <Button
            variant="outline"
            size="sm"
            onClick={() => void handleBatchExport()}
            loading={busy}
            disabled={exportableUserProfiles.length === 0}
            icon={<Download className="h-3.5 w-3.5" />}
          >
            {t('advancedDevices.actions.batchExport')}
          </Button>
        )}
      >
        {profiles.length > 0 ? (
          deviceProfilesByTransport.map((group) => (
            <div key={group.transport} className="border-b border-border/60 last:border-b-0">
              <div className="flex items-center justify-between gap-2 bg-muted/25 px-4 py-2">
                <div className="text-xs font-medium text-muted-foreground">{t(`advancedDevices.transport.${group.transport}`)}</div>
                <Badge>
                  {t('advancedDevices.connectionBanner.count', { count: group.profiles.length })}
                </Badge>
              </div>
              {group.profiles.map((profile) => (
                <ProfileRow
                  key={profile.id}
                  profile={profile}
                  context="device"
                  active={profile.id === (activeIdsByTransport[normalizeTransport(profile.transport)] || activeId)}
                  selectable={isManualCompatibilityTransport(profile.transport)}
                  onSelect={handleSelectProfile}
                  onEdit={handleEditProfile}
                  onDelete={requestDeleteProfile}
                />
              ))}
            </div>
          ))
        ) : (
          <EmptyState>{t('advancedDevices.sections.emptyUser')}</EmptyState>
        )}
      </Section>

      <Section
        title={t('advancedDevices.sections.importDevices')}
        description={t('advancedDevices.importExport.description')}
        icon={FileInput}
      >
        <ImportDevicePanel
          importCode={importCode}
          busy={busy}
          onImportCodeChange={setImportCode}
          onImport={() => void handleImport()}
          onImportFile={(file) => void handleImportFile(file)}
        />
      </Section>

      <Section
        title={t('advancedDevices.sections.debug')}
        description={t('advancedDevices.sections.debugHint')}
        icon={ShieldAlert}
      >
        <DebugProfilePanel
          isConnected={isConnected}
          activeProfile={activeProfile}
          config={config}
          onSaveDraft={handleSaveDebugDraft}
        />
      </Section>

      <DeviceProfileEditorDialog
        open={addOpen}
        supportedProfiles={supportedProfiles}
        onOpenChange={setAddOpen}
        onSave={handleSaveProfile}
      />

      <DeviceProfileEditorDialog
        open={editOpen}
        supportedProfiles={supportedProfiles}
        initialDraft={editProfileDraft}
        onOpenChange={(open) => {
          setEditOpen(open);
          if (!open) {
            setEditProfileDraft(null);
          }
        }}
        onSave={handleSaveProfile}
      />

      <DeviceProfileEditorDialog
        open={debugProfileOpen}
        supportedProfiles={supportedProfiles}
        initialDraft={debugProfileDraft}
        onOpenChange={(open) => {
          setDebugProfileOpen(open);
          if (!open) {
            setDebugProfileDraft(null);
          }
        }}
        onSave={handleSaveProfile}
      />

      <DeleteDeviceDialog
        profile={deleteTarget}
        busy={busy}
        onOpenChange={(open) => {
          if (!open && !busy) {
            setDeleteTarget(null);
          }
        }}
        onConfirm={() => void confirmDeleteProfile()}
      />
    </div>
  );
}
