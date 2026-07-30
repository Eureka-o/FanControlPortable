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
  model?: string;
  error?: string;
  runtime?: { state?: string };
}

export function deviceSnapshotFromStatus(
  status?: DeviceStatusPayload | null,
  connectedFallbackState = 'ready',
) {
  const connected = status?.connected === true;
  return {
    isConnected: connected,
    deviceRuntimeState: status?.runtime?.state || (connected ? connectedFallbackState : 'disconnected'),
    deviceProductId: connected ? status?.productId || null : null,
    deviceModel: connected ? status?.model || null : null,
    deviceSettings: connected ? status?.deviceSettings || null : null,
    runtimeDeviceProfile: connected ? status?.deviceProfile || null : null,
    runtimeDeviceCapabilities: connected
      ? status?.deviceCapabilities || status?.deviceProfile?.capabilities || null
      : null,
    fanData: connected ? status?.currentData || null : null,
  };
}
