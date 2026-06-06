'use client';

import { types } from '../../../../wailsjs/go/models';

export type DeviceTransport = 'wifi' | 'ble' | 'serial' | 'hid';
export type DeviceSpeedUnit = 'percent' | 'rpm';

export interface DeviceProfileDraft {
  id?: string;
  displayName: string;
  vendor: string;
  model: string;
  notes: string;
  transport: DeviceTransport;
  speedUnit: DeviceSpeedUnit;
  speedMin: number;
  speedMax: number;
  speedStep: number;
  tickScale: number;
  endpoint: string;
  stateEndpoint: string;
  speedEndpoint: string;
  httpMethod: string;
  requestTimeoutMs: number;
  minSendIntervalMs: number;
  maxRetries: number;
  retryBackoffMs: number;
  bleNameFilter: string;
  bleServiceUuid: string;
  bleWriteCharacteristic: string;
  bleNotifyCharacteristic: string;
  bleWriteWithResponse: boolean;
  serialPort: string;
  serialBaudRate: number;
  serialDataBits: number;
  serialStopBits: number;
  serialParity: string;
  serialFrameDelimiter: string;
  commandEncoding: string;
  checksum: string;
  setSpeedCommand: string;
  readStateCommand: string;
  parserType: string;
  parserExpression: string;
}

export function normalizeTransport(value?: string): DeviceTransport {
  switch ((value || '').toLowerCase()) {
    case 'ble':
      return 'ble';
    case 'serial':
      return 'serial';
    case 'hid':
      return 'hid';
    default:
      return 'wifi';
  }
}

export function normalizeSpeedUnit(value?: string): DeviceSpeedUnit {
  return value === 'rpm' ? 'rpm' : 'percent';
}

export function createEmptyProfileDraft(): DeviceProfileDraft {
  return {
    displayName: '',
    vendor: '',
    model: '',
    notes: '',
    transport: 'wifi',
    speedUnit: 'percent',
    speedMin: 0,
    speedMax: 100,
    speedStep: 1,
    tickScale: 10,
    endpoint: '192.168.137.2',
    stateEndpoint: '/api/data',
    speedEndpoint: '/api/speed',
    httpMethod: 'POST',
    requestTimeoutMs: 2000,
    minSendIntervalMs: 100,
    maxRetries: 0,
    retryBackoffMs: 150,
    bleNameFilter: '',
    bleServiceUuid: '',
    bleWriteCharacteristic: '',
    bleNotifyCharacteristic: '',
    bleWriteWithResponse: true,
    serialPort: '',
    serialBaudRate: 115200,
    serialDataBits: 8,
    serialStopBits: 1,
    serialParity: 'none',
    serialFrameDelimiter: '\\n',
    commandEncoding: 'json',
    checksum: 'none',
    setSpeedCommand: '',
    readStateCommand: '',
    parserType: 'jsonpath',
    parserExpression: '',
  };
}

export function createDraftFromProfile(profile?: types.DeviceProfile | null): DeviceProfileDraft {
  const base = createEmptyProfileDraft();
  if (!profile) {
    return base;
  }

  const transport = normalizeTransport(profile.transport);
  const speedUnit = normalizeSpeedUnit(profile.speedUnit);
  const speedRange = profile.speedRange || base;
  const connection = profile.connection || {};
  const setSpeed = (profile.commands || []).find((command) => command.name === 'setSpeed') || profile.commands?.[0];
  const readState = (profile.commands || []).find((command) => command.name === 'readState');
  const parser = (profile.responseParsers || [])[0];
  const commandEncoding = setSpeed?.encoding || base.commandEncoding;
  const checksum = commandEncoding === 'json' ? 'none' : setSpeed?.checksum || base.checksum;

  return {
    ...base,
    id: profile.builtIn ? undefined : profile.id || undefined,
    displayName: profile.builtIn ? `${profile.displayName || base.displayName} Custom` : profile.displayName || base.displayName,
    vendor: profile.vendor || '',
    model: profile.model || '',
    notes: profile.notes || '',
    transport,
    speedUnit,
    speedMin: Number(speedRange.min ?? (speedUnit === 'rpm' ? 0 : 0)),
    speedMax: Number(speedRange.max ?? (speedUnit === 'rpm' ? 4000 : 100)),
    speedStep: Number(speedRange.step ?? 1),
    tickScale: Number(speedRange.tickScale ?? (speedUnit === 'percent' ? 10 : 1)),
    endpoint: transport === 'wifi' ? connection.endpoint || base.endpoint : connection.endpoint || '',
    stateEndpoint: connection.stateEndpoint || base.stateEndpoint,
    speedEndpoint: connection.speedEndpoint || base.speedEndpoint,
    httpMethod: connection.httpMethod || base.httpMethod,
    requestTimeoutMs: Number(connection.requestTimeoutMs || base.requestTimeoutMs),
    minSendIntervalMs: Number(connection.minSendIntervalMs || base.minSendIntervalMs),
    maxRetries: Number(connection.maxRetries ?? base.maxRetries),
    retryBackoffMs: Number(connection.retryBackoffMs || base.retryBackoffMs),
    bleNameFilter: connection.bleNameFilter || '',
    bleServiceUuid: connection.bleServiceUuid || '',
    bleWriteCharacteristic: connection.bleWriteCharacteristic || '',
    bleNotifyCharacteristic: connection.bleNotifyCharacteristic || '',
    bleWriteWithResponse: connection.bleWriteWithResponse ?? true,
    serialPort: connection.serialPort || '',
    serialBaudRate: Number(connection.serialBaudRate || base.serialBaudRate),
    serialDataBits: Number(connection.serialDataBits || base.serialDataBits),
    serialStopBits: Number(connection.serialStopBits || base.serialStopBits),
    serialParity: connection.serialParity || base.serialParity,
    serialFrameDelimiter: connection.serialFrameDelimiter || base.serialFrameDelimiter,
    commandEncoding,
    checksum,
    setSpeedCommand: setSpeed?.command || '',
    readStateCommand: readState?.command || '',
    parserType: parser?.type || base.parserType,
    parserExpression: parser?.expression || '',
  };
}

function trimOptional(value: string) {
  const next = value.trim();
  return next.length > 0 ? next : undefined;
}

function positiveOrDefault(value: number, fallback: number) {
  return Number.isFinite(value) && value > 0 ? Math.round(value) : fallback;
}

export function buildProfileFromDraft(draft: DeviceProfileDraft): types.DeviceProfile {
  const speedUnit = normalizeSpeedUnit(draft.speedUnit);
  const transport = normalizeTransport(draft.transport);
  const speedMin = Math.max(0, Math.round(draft.speedMin));
  const defaultMax = speedUnit === 'rpm' ? 4000 : 100;
  const speedMax = Math.max(speedMin + 1, Math.round(draft.speedMax || defaultMax));
  const speedStep = positiveOrDefault(draft.speedStep, 1);
  const tickScale = speedUnit === 'percent' ? positiveOrDefault(draft.tickScale, 10) : 1;

  const commands: types.DeviceCommandTemplate[] = [];
  const checksum = draft.commandEncoding === 'json' ? 'none' : draft.checksum;
  if (draft.setSpeedCommand.trim()) {
    commands.push(types.DeviceCommandTemplate.createFrom({
      name: 'setSpeed',
      command: draft.setSpeedCommand.trim(),
      encoding: draft.commandEncoding,
      checksum,
      description: 'Set target speed',
    }));
  }
  if (draft.readStateCommand.trim()) {
    commands.push(types.DeviceCommandTemplate.createFrom({
      name: 'readState',
      command: draft.readStateCommand.trim(),
      encoding: draft.commandEncoding,
      checksum,
      description: 'Read device state',
    }));
  }

  const parserExpression = draft.parserExpression.trim();
  const responseParsers = parserExpression || draft.parserType === 'plain'
    ? [types.DeviceResponseParser.createFrom({
      name: 'state',
      type: draft.parserType,
      expression: parserExpression || undefined,
    })]
    : [];

  const supportsReadState =
    transport === 'wifi'
      ? Boolean(draft.stateEndpoint.trim())
      : transport === 'ble'
        ? Boolean(draft.bleNotifyCharacteristic.trim())
        : transport === 'hid' || Boolean(draft.readStateCommand.trim());
  const supportsSetSpeed =
    transport === 'wifi'
      ? Boolean(draft.speedEndpoint.trim())
      : transport === 'ble'
        ? Boolean(draft.bleWriteCharacteristic.trim())
        : transport === 'hid' || Boolean(draft.serialPort.trim()) || Boolean(draft.setSpeedCommand.trim());

  return types.DeviceProfile.createFrom({
    id: trimOptional(draft.id || ''),
    displayName: draft.displayName.trim(),
    vendor: trimOptional(draft.vendor),
    model: trimOptional(draft.model),
    notes: trimOptional(draft.notes),
    builtIn: false,
    transport,
    speedUnit,
    speedRange: {
      min: speedMin,
      max: speedMax,
      step: speedStep,
      tickScale,
    },
    connection: {
      endpoint: transport === 'wifi' || transport === 'ble' ? trimOptional(draft.endpoint) : undefined,
      stateEndpoint: trimOptional(draft.stateEndpoint),
      speedEndpoint: trimOptional(draft.speedEndpoint),
      httpMethod: trimOptional(draft.httpMethod),
      requestTimeoutMs: transport === 'wifi' ? positiveOrDefault(draft.requestTimeoutMs, 2000) : undefined,
      minSendIntervalMs: transport === 'wifi' ? Math.max(0, Math.round(draft.minSendIntervalMs)) : undefined,
      maxRetries: transport === 'wifi' ? Math.max(0, Math.round(draft.maxRetries)) : undefined,
      retryBackoffMs: transport === 'wifi' ? Math.max(0, Math.round(draft.retryBackoffMs)) : undefined,
      bleNameFilter: trimOptional(draft.bleNameFilter),
      bleServiceUuid: trimOptional(draft.bleServiceUuid),
      bleWriteCharacteristic: trimOptional(draft.bleWriteCharacteristic),
      bleNotifyCharacteristic: trimOptional(draft.bleNotifyCharacteristic),
      bleWriteWithResponse: draft.bleWriteWithResponse,
      serialPort: trimOptional(draft.serialPort),
      serialBaudRate: transport === 'serial' ? positiveOrDefault(draft.serialBaudRate, 115200) : undefined,
      serialDataBits: transport === 'serial' ? positiveOrDefault(draft.serialDataBits, 8) : undefined,
      serialStopBits: transport === 'serial' ? positiveOrDefault(draft.serialStopBits, 1) : undefined,
      serialParity: trimOptional(draft.serialParity),
      serialFrameDelimiter: trimOptional(draft.serialFrameDelimiter),
    },
    commands,
    responseParsers,
    capabilities: {
      displayName: draft.displayName.trim(),
      transport,
      speedUnit,
      speedRange: {
        min: speedMin,
        max: speedMax,
        step: speedStep,
        tickScale,
      },
      supportsReadState,
      supportsSetSpeed,
      supportsManualGears: true,
      supportsCustomSpeed: true,
      supportsDebugFrames: transport === 'hid',
      supportsRawCommands: transport !== 'wifi' || commands.length > 0,
      supportsLighting: transport === 'hid',
      supportsPowerOnStart: transport === 'hid',
      supportsSmartStartStop: transport === 'hid',
    },
  });
}

export function formatSpeedRange(profile: types.DeviceProfile) {
  const unit = normalizeSpeedUnit(profile.speedUnit);
  const range = profile.speedRange;
  if (!range) {
    return unit === 'rpm' ? 'RPM' : '%';
  }
  const suffix = unit === 'rpm' ? ' RPM' : '%';
  return `${range.min}-${range.max}${suffix}`;
}

export function summarizeConnection(profile: types.DeviceProfile) {
  const connection = profile.connection || {};
  switch (normalizeTransport(profile.transport)) {
    case 'ble':
      return [connection.bleNameFilter, connection.bleServiceUuid].filter(Boolean).join(' / ') || 'BLE';
    case 'serial':
      return [connection.serialPort, connection.serialBaudRate ? `${connection.serialBaudRate}` : ''].filter(Boolean).join(' / ') || 'COM';
    case 'hid':
      return 'HID/RPM';
    default:
      return [connection.endpoint, connection.speedEndpoint].filter(Boolean).join(' / ') || 'WiFi';
  }
}

export function getProfileIdentity(profile: types.DeviceProfile) {
  return [profile.vendor, profile.model].filter(Boolean).join(' / ') || profile.id || '';
}
