import type { types } from '../../../wailsjs/go/models';
import type { DeviceSettings } from '../types/app';

export interface DeviceStatusPayload {
  connected?: boolean;
  currentData?: types.FanData | null;
  deviceSettings?: DeviceSettings | null;
  deviceProfile?: types.DeviceProfile | null;
  deviceCapabilities?: types.DeviceCapabilities | null;
  temperature?: types.TemperatureData | null;
  productId?: string;
  deviceName?: string;
  model?: string;
  error?: string;
  runtime?: { state?: string };
}

export type DeviceSnapshotState = {
  isConnected: boolean;
  deviceRuntimeState: string;
  deviceProductId: string | null;
  deviceModel: string | null;
  deviceSettings: DeviceSettings | null;
  runtimeDeviceProfile: types.DeviceProfile | null;
  runtimeDeviceCapabilities: types.DeviceCapabilities | null;
  fanData: types.FanData | null;
};

export type DeviceSnapshotEvent =
  | { type: 'status'; status?: DeviceStatusPayload | null; connectedFallbackState?: string }
  | { type: 'connected'; status?: DeviceStatusPayload | null }
  | { type: 'disconnected' }
  | { type: 'core-error' }
  | { type: 'settings'; settings?: DeviceSettings | null }
  | { type: 'fan-data'; fanData?: types.FanData | null };

export function deviceSnapshotFromStatus(
  status?: DeviceStatusPayload | null,
  connectedFallbackState = 'ready',
): DeviceSnapshotState {
  const connected = status?.connected === true;
  return {
    isConnected: connected,
    deviceRuntimeState: status?.runtime?.state || (connected ? connectedFallbackState : 'disconnected'),
    deviceProductId: connected ? status?.productId || null : null,
    deviceModel: connected ? status?.deviceName || status?.model || null : null,
    deviceSettings: connected ? status?.deviceSettings || null : null,
    runtimeDeviceProfile: connected ? status?.deviceProfile || null : null,
    runtimeDeviceCapabilities: connected
      ? status?.deviceCapabilities || status?.deviceProfile?.capabilities || null
      : null,
    fanData: connected ? status?.currentData || null : null,
  };
}

export function reduceDeviceSnapshot(
  previous: DeviceSnapshotState,
  event: DeviceSnapshotEvent,
): DeviceSnapshotState {
  switch (event.type) {
    case 'status':
      return deviceSnapshotFromStatus(event.status, event.connectedFallbackState);
    case 'connected':
      return deviceSnapshotFromStatus({ ...(event.status || {}), connected: true }, 'capabilities');
    case 'disconnected':
    case 'core-error':
      return deviceSnapshotFromStatus(null);
    case 'settings':
      return previous.isConnected ? { ...previous, deviceSettings: event.settings || null } : previous;
    case 'fan-data':
      return previous.isConnected ? { ...previous, fanData: event.fanData || null } : previous;
    default:
      return previous;
  }
}
