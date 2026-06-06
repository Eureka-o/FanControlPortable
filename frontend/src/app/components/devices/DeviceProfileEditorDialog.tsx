'use client';

import type { ReactNode } from 'react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { Activity, Bluetooth, CheckCircle2, Gauge, Library, Power, RefreshCw, Save, Signal } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Input } from '@/components/ui/input';
import { apiService } from '../../services/api';
import { types } from '../../../../wailsjs/go/models';
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  NumberInput,
  Select,
  ToggleSwitch,
} from '../ui';
import {
  buildProfileFromDraft,
  createDraftFromProfile,
  createEmptyProfileDraft,
  normalizeSpeedUnit,
  type DeviceProfileDraft,
  type DeviceSpeedUnit,
  type DeviceTransport,
} from './profile-utils';

const SERIAL_PORT_NONE_VALUE = '__serial_port_none__';
type ProfileTestAction = 'connect' | 'readState' | 'setSpeed';

interface DeviceProfileEditorDialogProps {
  open: boolean;
  supportedProfiles: types.DeviceProfile[];
  initialDraft?: DeviceProfileDraft | null;
  onOpenChange: (open: boolean) => void;
  onSave: (profile: types.DeviceProfile, setActive: boolean) => Promise<void>;
}

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="block min-w-0 space-y-1.5">
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      {children}
      {hint && <span className="block text-[11px] leading-relaxed text-muted-foreground">{hint}</span>}
    </label>
  );
}

function FieldGroup({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-xl border border-border/70 bg-muted/25 p-3">
      {children}
    </div>
  );
}

function bleDeviceTitle(device: types.BLEDeviceInfo) {
  return device.name || device.address || 'BLE';
}

function bleDeviceServices(device: types.BLEDeviceInfo) {
  const services = device.serviceUuids || [];
  if (services.length === 0) return '';
  const visible = services.slice(0, 2).join(' / ');
  return services.length > 2 ? `${visible} +${services.length - 2}` : visible;
}

function gattCharacteristicCapabilities(characteristic: types.BLEGATTCharacteristicInfo, t: (key: string, options?: Record<string, unknown>) => string) {
  const properties = characteristic.properties || [];
  if (properties.length > 0) {
    return properties.map((property) => t(`advancedDevices.ble.properties.${property}`, { defaultValue: property })).join(' / ');
  }
  const capabilities = [
    characteristic.canRead ? t('advancedDevices.ble.properties.read') : '',
    characteristic.canWrite ? t('advancedDevices.ble.properties.write') : '',
    characteristic.canWriteWithoutResponse ? t('advancedDevices.ble.properties.writeWithoutResponse') : '',
    characteristic.canNotify ? t('advancedDevices.ble.properties.notify') : '',
    characteristic.canIndicate ? t('advancedDevices.ble.properties.indicate') : '',
  ].filter(Boolean);
  return capabilities.join(' / ');
}

function defaultTestSpeed(draft: DeviceProfileDraft) {
  const min = Number.isFinite(draft.speedMin) ? draft.speedMin : 0;
  const max = Number.isFinite(draft.speedMax) ? draft.speedMax : (draft.speedUnit === 'rpm' ? 4000 : 100);
  if (draft.speedUnit === 'rpm') {
    return Math.round(Math.max(min, Math.min(max, Math.max(1000, Math.round((min + max) / 2)))));
  }
  return Math.max(min, Math.min(max, 50));
}

function formatSpeedValue(value: number | undefined, unit: DeviceSpeedUnit) {
  if (typeof value !== 'number' || Number.isNaN(value)) return '';
  if (unit === 'rpm') return `${Math.round(value)} RPM`;
  return `${value}%`;
}

function useDraftFormDefaults(open: boolean, initialDraft?: DeviceProfileDraft | null) {
  const [draft, setDraft] = useState<DeviceProfileDraft>(() => createEmptyProfileDraft());
  const [libraryProfileId, setLibraryProfileId] = useState('blank');

  useEffect(() => {
    if (open) {
      setDraft(initialDraft ? { ...initialDraft } : createEmptyProfileDraft());
      setLibraryProfileId('blank');
    }
  }, [initialDraft, open]);

  return { draft, setDraft, libraryProfileId, setLibraryProfileId };
}

export default function DeviceProfileEditorDialog({
  open,
  supportedProfiles,
  initialDraft,
  onOpenChange,
  onSave,
}: DeviceProfileEditorDialogProps) {
  const { t } = useTranslation();
  const { draft, setDraft, libraryProfileId, setLibraryProfileId } = useDraftFormDefaults(open, initialDraft);
  const isEditing = Boolean(draft.id);
  const [setActive, setSetActive] = useState(true);
  const [formError, setFormError] = useState('');
  const [loading, setLoading] = useState(false);
  const [serialPorts, setSerialPorts] = useState<types.SerialPortInfo[]>([]);
  const [serialPortsLoading, setSerialPortsLoading] = useState(false);
  const [serialPortsError, setSerialPortsError] = useState('');
  const [bleDevices, setBLEDevices] = useState<types.BLEDeviceInfo[]>([]);
  const [bleScanLoading, setBLEScanLoading] = useState(false);
  const [bleScanError, setBLEScanError] = useState('');
  const [bleScanCompleted, setBLEScanCompleted] = useState(false);
  const [bleGATTLoading, setBLEGATTLoading] = useState(false);
  const [bleGATTError, setBLEGATTError] = useState('');
  const [bleGATTResult, setBLEGATTResult] = useState<types.BLEGATTProbeResult | null>(null);
  const [profileTestLoading, setProfileTestLoading] = useState<ProfileTestAction | null>(null);
  const [profileTestResult, setProfileTestResult] = useState<types.DeviceProfileTestResult | null>(null);
  const [profileTestError, setProfileTestError] = useState('');
  const [testSpeed, setTestSpeed] = useState(50);

  useEffect(() => {
    if (!open) return;
    setSetActive(!initialDraft?.id);
    setFormError('');
    setLoading(false);
    setBLEDevices([]);
    setBLEScanError('');
    setBLEScanCompleted(false);
    setBLEScanLoading(false);
    setBLEGATTLoading(false);
    setBLEGATTError('');
    setBLEGATTResult(null);
    setProfileTestLoading(null);
    setProfileTestResult(null);
    setProfileTestError('');
  }, [open]);

  useEffect(() => {
    if (open) {
      setTestSpeed(defaultTestSpeed(draft));
    }
  }, [draft.speedMax, draft.speedMin, draft.speedUnit, open]);

  const libraryOptions = useMemo(() => [
    { value: 'blank', label: t('advancedDevices.dialog.blankProfile') },
    ...supportedProfiles.map((profile) => ({
      value: profile.id,
      label: profile.displayName || profile.id,
    })),
  ], [supportedProfiles, t]);

  const serialPortOptions = useMemo(() => {
    const current = draft.serialPort.trim();
    const options = serialPorts.map((port) => ({
      value: port.name,
      label: port.displayName || port.name,
    }));
    if (current && !options.some((option) => option.value.toLowerCase() === current.toLowerCase())) {
      options.unshift({
        value: current,
        label: t('advancedDevices.placeholders.serialPortCustom', { port: current }),
      });
    }
    return [
      {
        value: SERIAL_PORT_NONE_VALUE,
        label: t('advancedDevices.placeholders.serialPortSelect'),
        disabled: true,
      },
      ...options,
    ];
  }, [draft.serialPort, serialPorts, t]);

  const checksumOptions = useMemo(() => {
    const values = draft.commandEncoding === 'json'
      ? ['none']
      : ['none', 'sum8', 'xor8', 'crc16'];
    return values.map((value) => ({ value, label: value }));
  }, [draft.commandEncoding]);

  const loadSerialPorts = useCallback(async () => {
    setSerialPortsLoading(true);
    setSerialPortsError('');
    try {
      const ports = await apiService.listSerialPorts();
      setSerialPorts(ports);
    } catch (error) {
      setSerialPorts([]);
      setSerialPortsError(error instanceof Error ? error.message : String(error));
    } finally {
      setSerialPortsLoading(false);
    }
  }, []);

  useEffect(() => {
    if (open && draft.transport === 'serial') {
      void loadSerialPorts();
    }
  }, [draft.transport, loadSerialPorts, open]);

  const bleMatchProfiles = useMemo(
    () => supportedProfiles.filter((profile) => profile.transport === 'ble'),
    [supportedProfiles],
  );

  const scanBLEDevices = useCallback(async () => {
    setBLEScanLoading(true);
    setBLEScanError('');
    setBLEScanCompleted(false);
    setBLEGATTError('');
    setBLEGATTResult(null);
    try {
      const devices = await apiService.scanBLEDevices(types.BLEScanParams.createFrom({
        timeoutMs: 5000,
        nameFilter: draft.bleNameFilter,
        serviceUuid: draft.bleServiceUuid,
        writeCharacteristicUuid: draft.bleWriteCharacteristic,
        notifyCharacteristicUuid: draft.bleNotifyCharacteristic,
        profiles: bleMatchProfiles,
      }));
      setBLEDevices(devices);
      setBLEScanCompleted(true);
    } catch (error) {
      setBLEDevices([]);
      setBLEScanCompleted(true);
      setBLEScanError(error instanceof Error ? error.message : String(error));
    } finally {
      setBLEScanLoading(false);
    }
  }, [
    bleMatchProfiles,
    draft.bleNameFilter,
    draft.bleNotifyCharacteristic,
    draft.bleServiceUuid,
    draft.bleWriteCharacteristic,
  ]);

  const applyBLEDevice = useCallback((device: types.BLEDeviceInfo) => {
    setDraft((current) => ({
      ...current,
      endpoint: device.address || current.endpoint,
      bleNameFilter: device.suggestedNameFilter || device.name || current.bleNameFilter,
      bleServiceUuid: device.suggestedServiceUuid || device.serviceUuids?.[0] || current.bleServiceUuid,
      bleWriteCharacteristic: device.suggestedWriteCharacteristic || device.writeCharacteristicUuids?.[0] || current.bleWriteCharacteristic,
      bleNotifyCharacteristic: device.suggestedNotifyCharacteristic || device.notifyCharacteristicUuids?.[0] || current.bleNotifyCharacteristic,
    }));
  }, [setDraft]);

  const probeBLEGATT = useCallback(async () => {
    setBLEGATTLoading(true);
    setBLEGATTError('');
    setBLEGATTResult(null);
    setFormError('');
    try {
      const profile = buildProfileFromDraft({
        ...draft,
        displayName: draft.displayName.trim() || t('advancedDevices.profileTest.draftName'),
      });
      const result = await apiService.probeBLEGATT(types.BLEGATTProbeParams.createFrom({
        timeoutMs: 10000,
        address: draft.endpoint.trim() || undefined,
        serviceUuid: draft.bleServiceUuid.trim() || undefined,
        profile,
      }));
      setBLEGATTResult(types.BLEGATTProbeResult.createFrom(result));
    } catch (error) {
      setBLEGATTError(error instanceof Error ? error.message : String(error));
    } finally {
      setBLEGATTLoading(false);
    }
  }, [draft, t]);

  const applyBLEGATTSuggestions = useCallback((result: types.BLEGATTProbeResult) => {
    setDraft((current) => ({
      ...current,
      endpoint: result.address || current.endpoint,
      bleServiceUuid: result.suggestedServiceUuid || current.bleServiceUuid,
      bleWriteCharacteristic: result.suggestedWriteCharacteristic || current.bleWriteCharacteristic,
      bleNotifyCharacteristic: result.suggestedNotifyCharacteristic || current.bleNotifyCharacteristic,
    }));
  }, [setDraft]);

  const updateDraft = <K extends keyof DeviceProfileDraft>(key: K, value: DeviceProfileDraft[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const handleTransportChange = (transport: DeviceTransport) => {
    setDraft((current) => {
      const next = { ...current, transport };
      if (transport === 'hid') {
        next.speedUnit = 'rpm';
        next.speedMin = 0;
        next.speedMax = 4000;
        next.speedStep = 1;
        next.tickScale = 1;
        next.endpoint = '';
      } else if (transport === 'ble') {
        next.endpoint = current.transport === 'ble' ? current.endpoint : '';
        if (current.transport === 'hid') {
          next.speedUnit = 'percent';
          next.speedMin = 0;
          next.speedMax = 100;
          next.speedStep = 1;
          next.tickScale = 10;
        }
      } else if (current.transport === 'hid') {
        next.speedUnit = 'percent';
        next.speedMin = 0;
        next.speedMax = 100;
        next.speedStep = 1;
        next.tickScale = 10;
      }
      if (transport === 'wifi' && !next.endpoint) {
        next.endpoint = '192.168.137.2';
      }
      if (transport === 'serial') {
        next.endpoint = '';
      }
      return next;
    });
    setBLEGATTError('');
    setBLEGATTResult(null);
  };

  const handleSpeedUnitChange = (speedUnit: DeviceSpeedUnit) => {
    setDraft((current) => ({
      ...current,
      speedUnit,
      speedMin: 0,
      speedMax: speedUnit === 'rpm' ? 4000 : 100,
      speedStep: 1,
      tickScale: speedUnit === 'rpm' ? 1 : 10,
    }));
  };

  const handleLibraryChange = (profileID: string) => {
    setLibraryProfileId(profileID);
    const profile = supportedProfiles.find((item) => item.id === profileID);
    setDraft(profile ? createDraftFromProfile(profile) : createEmptyProfileDraft());
    setProfileTestResult(null);
    setProfileTestError('');
  };

  const runProfileTest = useCallback(async (action: ProfileTestAction) => {
    setProfileTestLoading(action);
    setProfileTestResult(null);
    setProfileTestError('');
    setFormError('');
    try {
      const profile = buildProfileFromDraft({
        ...draft,
        displayName: draft.displayName.trim() || t('advancedDevices.profileTest.draftName'),
      });
      const result = await apiService.testDeviceProfile(types.DeviceProfileTestParams.createFrom({
        profile,
        action,
        speedValue: action === 'setSpeed' ? testSpeed : undefined,
        timeoutMs: 10000,
      }));
      setProfileTestResult(types.DeviceProfileTestResult.createFrom(result));
    } catch (error) {
      setProfileTestError(error instanceof Error ? error.message : String(error));
    } finally {
      setProfileTestLoading(null);
    }
  }, [draft, t, testSpeed]);

  const handleSave = async () => {
    const displayName = draft.displayName.trim();
    if (!displayName) {
      setFormError(t('advancedDevices.validation.nameRequired'));
      return;
    }
    setLoading(true);
    setFormError('');
    try {
      await onSave(buildProfileFromDraft(draft), setActive);
      onOpenChange(false);
    } catch (error) {
      setFormError(error instanceof Error ? error.message : String(error));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[86vh] max-w-3xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {t(isEditing ? 'advancedDevices.dialog.editTitle' : 'advancedDevices.dialog.addTitle')}
          </DialogTitle>
          <DialogDescription>
            {t(isEditing ? 'advancedDevices.dialog.editDescription' : 'advancedDevices.dialog.addDescription')}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {!isEditing && (
            <FieldGroup>
              <div className="mb-3 flex items-center gap-2 text-sm font-medium text-foreground">
                <Library className="h-4 w-4 text-muted-foreground" />
                {t('advancedDevices.dialog.libraryProfile')}
              </div>
              <Select
                value={libraryProfileId}
                onChange={(value) => handleLibraryChange(String(value))}
                options={libraryOptions}
                size="sm"
              />
            </FieldGroup>
          )}

            <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
              <Field label={t('advancedDevices.fields.name')}>
                <Input
                  value={draft.displayName}
                  onChange={(event) => updateDraft('displayName', event.target.value)}
                  placeholder={t('advancedDevices.placeholders.name')}
                />
              </Field>
              <Field label={t('advancedDevices.fields.vendor')}>
                <Input value={draft.vendor} onChange={(event) => updateDraft('vendor', event.target.value)} />
              </Field>
              <Field label={t('advancedDevices.fields.model')}>
                <Input value={draft.model} onChange={(event) => updateDraft('model', event.target.value)} />
              </Field>
              <Field label={t('advancedDevices.fields.transport')}>
                <Select
                  value={draft.transport}
                  onChange={(value) => handleTransportChange(String(value) as DeviceTransport)}
                  options={[
                    { value: 'wifi', label: t('advancedDevices.transport.wifi') },
                    { value: 'ble', label: t('advancedDevices.transport.ble') },
                    { value: 'serial', label: t('advancedDevices.transport.serial') },
                    { value: 'hid', label: t('advancedDevices.transport.hid') },
                  ]}
                  size="sm"
                />
              </Field>
              <Field label={t('advancedDevices.fields.speedUnit')}>
                <Select
                  value={draft.speedUnit}
                  onChange={(value) => handleSpeedUnitChange(String(value) as DeviceSpeedUnit)}
                  options={[
                    { value: 'percent', label: t('advancedDevices.speedUnit.percent') },
                    { value: 'rpm', label: t('advancedDevices.speedUnit.rpm') },
                  ]}
                  disabled={draft.transport === 'hid'}
                  size="sm"
                />
              </Field>
              <Field label={t('advancedDevices.fields.speedMin')}>
                <NumberInput value={draft.speedMin} onChange={(value) => updateDraft('speedMin', value)} min={0} max={draft.speedMax - 1} />
              </Field>
              <Field label={t('advancedDevices.fields.speedMax')}>
                <NumberInput value={draft.speedMax} onChange={(value) => updateDraft('speedMax', value)} min={draft.speedMin + 1} max={draft.speedUnit === 'rpm' ? 12000 : 100} />
              </Field>
              <Field label={t('advancedDevices.fields.speedStep')}>
                <NumberInput value={draft.speedStep} onChange={(value) => updateDraft('speedStep', value)} min={1} max={draft.speedUnit === 'rpm' ? 1000 : 100} />
              </Field>
              {draft.speedUnit === 'percent' && (
                <Field label={t('advancedDevices.fields.tickScale')} hint={t('advancedDevices.hints.tickScale')}>
                  <NumberInput value={draft.tickScale} onChange={(value) => updateDraft('tickScale', value)} min={1} max={100} />
                </Field>
              )}
            </div>

            <Field label={t('advancedDevices.fields.notes')}>
              <textarea
                value={draft.notes}
                onChange={(event) => updateDraft('notes', event.target.value)}
                rows={2}
                className="w-full resize-none rounded-md border border-input bg-background px-3 py-2 text-sm outline-none ring-offset-background transition-colors focus-visible:ring-2 focus-visible:ring-ring"
              />
            </Field>

            {draft.transport === 'wifi' && (
              <FieldGroup>
                <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                  <Field label={t('advancedDevices.fields.endpoint')}>
                    <Input value={draft.endpoint} onChange={(event) => updateDraft('endpoint', event.target.value)} placeholder="192.168.137.2" />
                  </Field>
                  <Field label={t('advancedDevices.fields.httpMethod')}>
                    <Select
                      value={draft.httpMethod}
                      onChange={(value) => updateDraft('httpMethod', String(value))}
                      options={['GET', 'POST', 'PUT', 'PATCH'].map((method) => ({ value: method, label: method }))}
                      size="sm"
                    />
                  </Field>
                  <Field label={t('advancedDevices.fields.stateEndpoint')}>
                    <Input value={draft.stateEndpoint} onChange={(event) => updateDraft('stateEndpoint', event.target.value)} placeholder="/api/data" />
                  </Field>
                  <Field label={t('advancedDevices.fields.speedEndpoint')}>
                    <Input value={draft.speedEndpoint} onChange={(event) => updateDraft('speedEndpoint', event.target.value)} placeholder="/api/speed" />
                  </Field>
                  <Field label={t('advancedDevices.fields.requestTimeoutMs')}>
                    <NumberInput value={draft.requestTimeoutMs} onChange={(value) => updateDraft('requestTimeoutMs', value)} min={100} max={30000} />
                  </Field>
                  <Field label={t('advancedDevices.fields.minSendIntervalMs')}>
                    <NumberInput value={draft.minSendIntervalMs} onChange={(value) => updateDraft('minSendIntervalMs', value)} min={0} max={60000} />
                  </Field>
                  <Field label={t('advancedDevices.fields.maxRetries')}>
                    <NumberInput value={draft.maxRetries} onChange={(value) => updateDraft('maxRetries', value)} min={0} max={5} />
                  </Field>
                  <Field label={t('advancedDevices.fields.retryBackoffMs')}>
                    <NumberInput value={draft.retryBackoffMs} onChange={(value) => updateDraft('retryBackoffMs', value)} min={0} max={10000} />
                  </Field>
                </div>
              </FieldGroup>
            )}

            {draft.transport === 'ble' && (
              <FieldGroup>
                <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                  <div className="flex items-center gap-2 text-sm font-medium text-foreground">
                    <Bluetooth className="h-4 w-4 text-muted-foreground" />
                    {t('advancedDevices.ble.scanTitle')}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      type="button"
                      variant="secondary"
                      size="sm"
                      loading={bleScanLoading}
                      disabled={bleGATTLoading}
                      icon={<RefreshCw className="h-4 w-4" />}
                      title={t('advancedDevices.actions.scanBle')}
                      onClick={() => void scanBLEDevices()}
                    >
                      {t('advancedDevices.actions.scanBle')}
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      loading={bleGATTLoading}
                      disabled={bleScanLoading}
                      icon={<Activity className="h-4 w-4" />}
                      title={t('advancedDevices.actions.probeGatt')}
                      onClick={() => void probeBLEGATT()}
                    >
                      {t('advancedDevices.actions.probeGatt')}
                    </Button>
                  </div>
                </div>
                <p className="mb-3 text-[11px] leading-relaxed text-muted-foreground">
                  {t('advancedDevices.hints.bleScan')}
                </p>
                {bleScanError && (
                  <div className="mb-3 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                    {t('advancedDevices.validation.bleScanFailed', { error: bleScanError })}
                  </div>
                )}
                {bleDevices.length > 0 && (
                  <div className="mb-3 max-h-44 space-y-2 overflow-y-auto pr-1">
                    {bleDevices.map((device) => {
                      const services = bleDeviceServices(device);
                      const profileMatch = device.matchedProfileDisplayName || device.matchedProfileId || '';
                      return (
                        <button
                          key={device.address}
                          type="button"
                          className="w-full rounded-lg border border-border bg-background px-3 py-2 text-left transition-colors hover:border-primary/40 hover:bg-muted/50"
                          onClick={() => applyBLEDevice(device)}
                        >
                          <div className="flex min-w-0 items-center justify-between gap-2">
                            <span className="truncate text-sm font-medium text-foreground">{bleDeviceTitle(device)}</span>
                            {device.matched && (
                              <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-[11px] text-primary">
                                <CheckCircle2 className="h-3 w-3" />
                                {t('advancedDevices.ble.matched')}
                              </span>
                            )}
                          </div>
                          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted-foreground">
                            <span>{device.address}</span>
                            <span className="inline-flex items-center gap-1">
                              <Signal className="h-3 w-3" />
                              {device.rssi} dBm
                            </span>
                          </div>
                          {services && (
                            <div className="mt-1 truncate text-[11px] text-muted-foreground">
                              {t('advancedDevices.ble.services')}: {services}
                            </div>
                          )}
                          {profileMatch && (
                            <div className="mt-1 truncate text-[11px] text-primary">
                              {t('advancedDevices.ble.profileMatch', { profile: profileMatch })}
                            </div>
                          )}
                        </button>
                      );
                    })}
                  </div>
                )}
                {bleScanCompleted && !bleScanLoading && !bleScanError && bleDevices.length === 0 && (
                  <div className="mb-3 rounded-lg border border-border/70 bg-background px-3 py-2 text-xs text-muted-foreground">
                    {t('advancedDevices.validation.bleScanEmpty')}
                  </div>
                )}
                {bleGATTError && (
                  <div className="mb-3 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                    {t('advancedDevices.validation.gattProbeFailed', { error: bleGATTError })}
                  </div>
                )}
                {bleGATTResult && (
                  <div className="mb-3 rounded-lg border border-border/70 bg-background px-3 py-2">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="min-w-0 text-xs text-muted-foreground">
                        <div className="font-medium text-foreground">{t('advancedDevices.ble.gattResult')}</div>
                        <div className="mt-0.5 break-all">
                          {[bleGATTResult.name, bleGATTResult.address].filter(Boolean).join(' / ') || t('advancedDevices.ble.gattUnknownDevice')}
                        </div>
                      </div>
                      {(bleGATTResult.suggestedServiceUuid || bleGATTResult.suggestedWriteCharacteristic || bleGATTResult.suggestedNotifyCharacteristic) && (
                        <Button
                          type="button"
                          variant="secondary"
                          size="sm"
                          icon={<CheckCircle2 className="h-4 w-4" />}
                          onClick={() => applyBLEGATTSuggestions(bleGATTResult)}
                        >
                          {t('advancedDevices.actions.applyGatt')}
                        </Button>
                      )}
                    </div>
                    {(bleGATTResult.services || []).length > 0 ? (
                      <div className="mt-2 max-h-48 space-y-2 overflow-y-auto pr-1">
                        {(bleGATTResult.services || []).map((service) => (
                          <div key={service.uuid} className="rounded-md border border-border/70 bg-muted/20 px-2 py-1.5">
                            <div className="break-all font-mono text-[11px] text-foreground">
                              {t('advancedDevices.ble.service')}: {service.uuid}
                            </div>
                            {service.error ? (
                              <div className="mt-1 text-[11px] text-destructive">{service.error}</div>
                            ) : (service.characteristics || []).length > 0 ? (
                              <div className="mt-1 space-y-1">
                                {(service.characteristics || []).map((characteristic) => {
                                  const capabilities = gattCharacteristicCapabilities(characteristic, t);
                                  return (
                                    <div key={characteristic.uuid} className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[11px]">
                                      <span className="break-all font-mono text-muted-foreground">{characteristic.uuid}</span>
                                      {capabilities && (
                                        <span className="rounded-full bg-primary/10 px-2 py-0.5 text-primary">{capabilities}</span>
                                      )}
                                      {characteristic.mtu ? (
                                        <span className="text-muted-foreground">MTU {characteristic.mtu}</span>
                                      ) : null}
                                    </div>
                                  );
                                })}
                              </div>
                            ) : (
                              <div className="mt-1 text-[11px] text-muted-foreground">{t('advancedDevices.ble.gattNoCharacteristics')}</div>
                            )}
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="mt-2 text-xs text-muted-foreground">{t('advancedDevices.validation.gattProbeEmpty')}</div>
                    )}
                  </div>
                )}
                <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                  <Field label={t('advancedDevices.fields.bleAddress')} hint={t('advancedDevices.hints.bleAddress')}>
                    <Input value={draft.endpoint} onChange={(event) => updateDraft('endpoint', event.target.value)} placeholder="AA:BB:CC:DD:EE:FF" />
                  </Field>
                  <Field label={t('advancedDevices.fields.bleNameFilter')}>
                    <Input value={draft.bleNameFilter} onChange={(event) => updateDraft('bleNameFilter', event.target.value)} />
                  </Field>
                  <Field label={t('advancedDevices.fields.bleService')}>
                    <Input value={draft.bleServiceUuid} onChange={(event) => updateDraft('bleServiceUuid', event.target.value)} />
                  </Field>
                  <Field label={t('advancedDevices.fields.bleWrite')}>
                    <Input value={draft.bleWriteCharacteristic} onChange={(event) => updateDraft('bleWriteCharacteristic', event.target.value)} />
                  </Field>
                  <Field label={t('advancedDevices.fields.bleNotify')}>
                    <Input value={draft.bleNotifyCharacteristic} onChange={(event) => updateDraft('bleNotifyCharacteristic', event.target.value)} />
                  </Field>
                  <div className="md:col-span-2">
                    <ToggleSwitch
                      enabled={draft.bleWriteWithResponse}
                      onChange={(enabled) => updateDraft('bleWriteWithResponse', enabled)}
                      label={t('advancedDevices.fields.bleWriteWithResponse')}
                      size="sm"
                    />
                  </div>
                </div>
              </FieldGroup>
            )}

            {draft.transport === 'serial' && (
              <FieldGroup>
                <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                  <div className="md:col-span-3">
                    <Field label={t('advancedDevices.fields.serialPort')} hint={t('advancedDevices.hints.serialPorts')}>
                      <div className="space-y-2">
                        <div className="grid grid-cols-[minmax(0,1fr)_auto] gap-2">
                          <Select
                            value={draft.serialPort.trim() || SERIAL_PORT_NONE_VALUE}
                            onChange={(value) => {
                              const next = String(value);
                              if (next !== SERIAL_PORT_NONE_VALUE) {
                                updateDraft('serialPort', next);
                              }
                            }}
                            options={serialPortOptions}
                            size="sm"
                            className="min-w-0"
                          />
                          <Button
                            type="button"
                            variant="secondary"
                            size="sm"
                            className="h-10 px-3"
                            loading={serialPortsLoading}
                            icon={<RefreshCw className="h-4 w-4" />}
                            title={t('advancedDevices.actions.refreshPorts')}
                            onClick={() => void loadSerialPorts()}
                          >
                            <span className="sr-only">{t('advancedDevices.actions.refreshPorts')}</span>
                          </Button>
                        </div>
                        <Input
                          value={draft.serialPort}
                          onChange={(event) => updateDraft('serialPort', event.target.value)}
                          placeholder={t('advancedDevices.placeholders.serialPortManual')}
                        />
                        {serialPortsError ? (
                          <span className="block text-[11px] leading-relaxed text-destructive">{t('advancedDevices.validation.serialPortsFailed', { error: serialPortsError })}</span>
                        ) : !serialPortsLoading && serialPorts.length === 0 ? (
                          <span className="block text-[11px] leading-relaxed text-muted-foreground">{t('advancedDevices.validation.serialPortsEmpty')}</span>
                        ) : null}
                      </div>
                    </Field>
                  </div>
                  <Field label={t('advancedDevices.fields.serialBaud')}>
                    <NumberInput value={draft.serialBaudRate} onChange={(value) => updateDraft('serialBaudRate', value)} min={1} />
                  </Field>
                  <Field label={t('advancedDevices.fields.serialDelimiter')}>
                    <Input value={draft.serialFrameDelimiter} onChange={(event) => updateDraft('serialFrameDelimiter', event.target.value)} />
                  </Field>
                  <Field label={t('advancedDevices.fields.serialDataBits')}>
                    <NumberInput value={draft.serialDataBits} onChange={(value) => updateDraft('serialDataBits', value)} min={5} max={8} />
                  </Field>
                  <Field label={t('advancedDevices.fields.serialStopBits')}>
                    <NumberInput value={draft.serialStopBits} onChange={(value) => updateDraft('serialStopBits', value)} min={1} max={2} />
                  </Field>
                  <Field label={t('advancedDevices.fields.serialParity')}>
                    <Select
                      value={draft.serialParity}
                      onChange={(value) => updateDraft('serialParity', String(value))}
                      options={['none', 'odd', 'even'].map((value) => ({ value, label: value }))}
                      size="sm"
                    />
                  </Field>
                </div>
              </FieldGroup>
            )}

            <FieldGroup>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                <Field label={t('advancedDevices.fields.commandEncoding')}>
                  <Select
                    value={draft.commandEncoding}
                    onChange={(value) => {
                      const nextEncoding = String(value);
                      setDraft((current) => ({
                        ...current,
                        commandEncoding: nextEncoding,
                        checksum: nextEncoding === 'json' ? 'none' : current.checksum,
                      }));
                    }}
                    options={['json', 'hex', 'ascii', 'raw'].map((value) => ({ value, label: value }))}
                    size="sm"
                  />
                </Field>
                <Field label={t('advancedDevices.fields.checksum')}>
                  <Select
                    value={draft.commandEncoding === 'json' ? 'none' : draft.checksum}
                    onChange={(value) => updateDraft('checksum', String(value))}
                    options={checksumOptions}
                    disabled={draft.commandEncoding === 'json'}
                    size="sm"
                  />
                </Field>
                <Field label={t('advancedDevices.fields.setSpeedCommand')}>
                  <Input value={draft.setSpeedCommand} onChange={(event) => updateDraft('setSpeedCommand', event.target.value)} />
                </Field>
                <Field label={t('advancedDevices.fields.readStateCommand')}>
                  <Input value={draft.readStateCommand} onChange={(event) => updateDraft('readStateCommand', event.target.value)} />
                </Field>
                <Field label={t('advancedDevices.fields.parserType')}>
                  <Select
                    value={draft.parserType}
                    onChange={(value) => updateDraft('parserType', String(value))}
                    options={['jsonpath', 'byteoffset', 'regex', 'plain'].map((value) => ({ value, label: value }))}
                    size="sm"
                  />
                </Field>
                <Field label={t('advancedDevices.fields.parserExpression')}>
                  <Input value={draft.parserExpression} onChange={(event) => updateDraft('parserExpression', event.target.value)} />
                </Field>
              </div>
            </FieldGroup>

            <FieldGroup>
              <div className="mb-3 flex flex-wrap items-start justify-between gap-2">
                <div className="min-w-0">
                  <div className="flex items-center gap-2 text-sm font-medium text-foreground">
                    <Activity className="h-4 w-4 text-muted-foreground" />
                    {t('advancedDevices.profileTest.title')}
                  </div>
                  <p className="mt-1 text-[11px] leading-relaxed text-muted-foreground">
                    {t('advancedDevices.profileTest.hint')}
                  </p>
                </div>
                <span className="rounded-full border border-border/70 bg-background px-2 py-0.5 text-[11px] text-muted-foreground">
                  {t(`advancedDevices.transport.${draft.transport}`)} / {t(`advancedDevices.speedUnit.${draft.speedUnit}`)}
                </span>
              </div>
              <div className="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_auto_auto_auto] lg:items-end">
                <NumberInput
                  value={testSpeed}
                  onChange={setTestSpeed}
                  min={draft.speedMin}
                  max={draft.speedMax}
                  step={draft.speedUnit === 'rpm' ? 1 : 0.1}
                  label={t('advancedDevices.profileTest.testSpeed')}
                  suffix={draft.speedUnit === 'rpm' ? 'RPM' : '%'}
                />
                <Button
                  type="button"
                  variant="secondary"
                  loading={profileTestLoading === 'connect'}
                  disabled={profileTestLoading !== null}
                  icon={<Power className="h-4 w-4" />}
                  onClick={() => void runProfileTest('connect')}
                >
                  {t('advancedDevices.profileTest.action.connect')}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  loading={profileTestLoading === 'readState'}
                  disabled={profileTestLoading !== null}
                  icon={<Activity className="h-4 w-4" />}
                  onClick={() => void runProfileTest('readState')}
                >
                  {t('advancedDevices.profileTest.action.readState')}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  loading={profileTestLoading === 'setSpeed'}
                  disabled={profileTestLoading !== null}
                  icon={<Gauge className="h-4 w-4" />}
                  onClick={() => void runProfileTest('setSpeed')}
                >
                  {t('advancedDevices.profileTest.action.setSpeed')}
                </Button>
              </div>
              {profileTestError && (
                <div className="mt-3 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs leading-6 text-destructive">
                  {t('advancedDevices.profileTest.error', { error: profileTestError })}
                </div>
              )}
              {profileTestResult && (
                <div className="mt-3 rounded-lg border border-emerald-500/25 bg-emerald-500/10 px-3 py-2 text-xs leading-6 text-emerald-900 dark:text-emerald-100">
                  <div className="flex flex-wrap items-center gap-x-3 gap-y-1 font-medium">
                    <span className="inline-flex items-center gap-1">
                      <CheckCircle2 className="h-3.5 w-3.5" />
                      {t('advancedDevices.profileTest.success', {
                        action: t(`advancedDevices.profileTest.action.${profileTestResult.action}`),
                      })}
                    </span>
                    <span className="font-mono">{profileTestResult.durationMs} ms</span>
                  </div>
                  {profileTestResult.fanData ? (
                    <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-muted-foreground">
                      <span>
                        {t('advancedDevices.profileTest.current')}: {formatSpeedValue(profileTestResult.fanData.currentRpm, normalizeSpeedUnit(profileTestResult.speedUnit))}
                      </span>
                      <span>
                        {t('advancedDevices.profileTest.target')}: {formatSpeedValue(profileTestResult.fanData.targetRpm, normalizeSpeedUnit(profileTestResult.speedUnit))}
                      </span>
                    </div>
                  ) : (
                    <div className="mt-1 text-muted-foreground">{t('advancedDevices.profileTest.noData')}</div>
                  )}
                </div>
              )}
            </FieldGroup>

            <ToggleSwitch
              enabled={setActive}
              onChange={setSetActive}
              label={t('advancedDevices.dialog.setActiveAfterSave')}
              size="sm"
            />
        </div>

        {formError && (
          <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
            {formError}
          </div>
        )}

        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            {t('common.actions.cancel')}
          </Button>
          <Button onClick={handleSave} loading={loading} icon={<Save className="h-4 w-4" />}>
            {t('advancedDevices.actions.saveDevice')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
