import { types } from '../../../../wailsjs/go/models';
import type { WiFiDiscoveryResult } from '../../services/api';
import { getProfileDisplayName, normalizeTransport, type DeviceTransport } from '../devices/profile-utils';

export const EMPTY_PROFILE_SELECT_VALUE = '__fancontrol_no_profile__';

export function configuredDeviceProfiles(config: types.AppConfig): types.DeviceProfile[] {
  return Array.isArray((config as any).deviceProfiles) ? ((config as any).deviceProfiles as types.DeviceProfile[]) : [];
}

export function activeProfileIdsByTransportFromConfig(config: types.AppConfig): Record<string, string> {
  const raw = (config as any).activeDeviceProfileIdsByTransport;
  return raw && typeof raw === 'object' ? raw as Record<string, string> : {};
}

export function profileLabel(profile: types.DeviceProfile) {
  return getProfileDisplayName(profile);
}

export function profileConnection(profile?: types.DeviceProfile | null): types.DeviceConnectionSettings {
  return (profile?.connection || {}) as types.DeviceConnectionSettings;
}

export function profileForTransport(profiles: types.DeviceProfile[], transport: DeviceTransport) {
  return profiles.find((profile) => normalizeTransport(profile.transport) === transport) || null;
}

export function activeProfileForTransport(
  profiles: types.DeviceProfile[],
  activeIdsByTransport: Record<string, string>,
  transport: DeviceTransport,
) {
  const activeId = activeIdsByTransport[transport] || '';
  return profiles.find((profile) => profile.id === activeId && normalizeTransport(profile.transport) === transport)
    || profileForTransport(profiles, transport);
}

export function profilesForTransport(profiles: types.DeviceProfile[], transport: DeviceTransport) {
  return profiles.filter((profile) => normalizeTransport(profile.transport) === transport);
}

export function profileSelectOptions(
  profiles: types.DeviceProfile[],
  emptyLabel: string,
) {
  if (profiles.length === 0) {
    return [{ value: EMPTY_PROFILE_SELECT_VALUE, label: emptyLabel, disabled: true }];
  }
  return profiles.map((profile) => ({ value: profile.id, label: profileLabel(profile) }));
}

export function isEmptyProfileSelectValue(value: string | number) {
  return String(value || '').trim() === EMPTY_PROFILE_SELECT_VALUE;
}

export function isUserVisibleNativeProfile(profile: types.DeviceProfile) {
  const transport = normalizeTransport(profile.transport);
  return (transport === 'ble' || transport === 'hid') && !profile.builtIn;
}

// WiFi compatibility stays on for upgraded users that already have a legacy IP.
export function isWiFiCompatibilityEnabled(config: types.AppConfig) {
  const explicit = (config as any).wifiCompatibilityEnabled;
  if (typeof explicit === 'boolean') {
    return explicit;
  }
  return normalizeTransport((config as any).deviceTransport) === 'wifi' || Boolean(((config as any).fanControlDeviceIp || '').trim());
}

export function isSerialCompatibilityEnabled(config: types.AppConfig) {
  const explicit = (config as any).serialCompatibilityEnabled;
  if (typeof explicit === 'boolean') {
    return explicit;
  }
  return normalizeTransport((config as any).deviceTransport) === 'serial';
}

export function isWiFiDynamicIPCompatibilityEnabled(config: types.AppConfig) {
  return Boolean((config as any).wifiDynamicIpCompatibilityEnabled);
}

export function normalizeWiFiSmartStartStopStandbySpeed(value: unknown) {
  const numeric = typeof value === 'number' ? value : Number(value);
  if (!Number.isFinite(numeric)) return 1;
  return Math.min(100, Math.max(1, Math.round(numeric)));
}

export function wifiDiscoveryDevices(result: WiFiDiscoveryResult | null) {
  return Array.isArray(result?.devices) ? result.devices.filter((device) => !!device.endpoint) : [];
}

// Elapsed labels are shared by live scan progress and final scan results.
export function wifiDiscoveryElapsedLabel(elapsedMs?: number) {
  if (typeof elapsedMs !== 'number' || Number.isNaN(elapsedMs) || elapsedMs < 0) return '';
  if (elapsedMs < 1000) return `${Math.round(elapsedMs)}ms`;
  return `${(elapsedMs / 1000).toFixed(1)}s`;
}

export function wifiDiscoverySourceKey(source?: string) {
  switch (source) {
    case 'exact':
    case 'savedSubnet':
    case 'adapterSubnet':
    case 'windowsHotspot':
    case 'deviceAP':
    case 'commonSubnet':
    case 'expandedSubnet':
      return `controlPanel.system.deviceConnection.wifiScanSources.${source}`;
    default:
      return 'controlPanel.system.deviceConnection.wifiScanSources.unknown';
  }
}
