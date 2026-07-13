'use client';

import { types } from '../../../../../wailsjs/go/models';
import { type DeviceCandidate, type WiFiDiscoveredDevice } from '../../../services/api';
import { normalizeTransport } from '../../devices/profile-utils';
import { profileConnection, wifiDiscoverySourceKey } from '../device-connection-utils';

export function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export function transportLabel(transport: string | undefined, t: (key: string) => string) {
  switch (normalizeTransport(transport)) {
    case 'ble':
      return t('controlPanel.system.deviceConnection.transportBle');
    case 'hid':
      return t('controlPanel.system.deviceConnection.transportHid');
    case 'serial':
      return t('controlPanel.system.deviceConnection.transportSerial');
    default:
      return t('controlPanel.system.deviceConnection.transportWifi');
  }
}

function candidateKey(candidate: DeviceCandidate) {
  return [
    normalizeTransport(candidate.transport),
    candidate.profileId || '',
    candidate.endpoint || '',
    candidate.id || '',
  ].join(':');
}

export function mergeDeviceCandidates(primary: DeviceCandidate[], secondary: DeviceCandidate[]) {
  const seen = new Set<string>();
  const merged: DeviceCandidate[] = [];
  [...primary, ...secondary].forEach((candidate) => {
    const key = candidateKey(candidate);
    if (seen.has(key)) return;
    seen.add(key);
    merged.push(candidate);
  });
  return merged;
}

export function wifiCandidateFromDiscovery(device: WiFiDiscoveredDevice, profile: types.DeviceProfile | null | undefined, fallbackName: string): DeviceCandidate | null {
  const endpoint = (device.endpoint || device.ip || '').trim();
  if (!endpoint) return null;
  const profileId = (device.profileId || profile?.id || '').trim();
  return {
    id: `wifi:${profileId || 'wifi'}:${endpoint}`,
    transport: 'wifi',
    name: (device.name || profile?.displayName || profile?.model || fallbackName).trim(),
    profileId,
    endpoint,
    source: device.source || 'wifi',
    network: device.network,
    speed: device.speed,
    targetSpeed: device.targetSpeed,
    temperature: device.temperature,
    latencyMs: device.latencyMs,
    connectable: true,
  };
}

function candidateSourceLabel(source: string | undefined, t: (key: string) => string) {
  switch ((source || '').trim().toLowerCase()) {
    case 'native':
      return t('controlPanel.system.deviceConnection.candidateSourceNative');
    case 'manual':
      return t('controlPanel.system.deviceConnection.candidateSourceManual');
    case 'saved':
      return t('controlPanel.system.deviceConnection.candidateSourceSaved');
    case 'wifi':
      return t('controlPanel.system.deviceConnection.candidateSourceWiFi');
    case '':
      return '';
    default:
      return t(wifiDiscoverySourceKey(source));
  }
}

function candidateEndpointLabel(candidate: DeviceCandidate) {
  const endpoint = (candidate.endpoint || '').trim();
  if (!endpoint) return '';
  const transport = normalizeTransport(candidate.transport);
  if (transport === 'hid' || transport === 'ble') return '';
  const compact = endpoint.replace(/^https?:\/\//i, '');
  return compact.length > 42 ? `${compact.slice(0, 26)}...${compact.slice(-10)}` : compact;
}

export function candidateBadges(candidate: DeviceCandidate, t: (key: string, options?: Record<string, unknown>) => string) {
  const badges = [candidateSourceLabel(candidate.source, t), transportLabel(candidate.transport, t)].filter(Boolean);
  const endpoint = candidateEndpointLabel(candidate);
  if (endpoint) badges.push(t('controlPanel.system.deviceConnection.candidateEndpoint', { endpoint }));
  if (typeof candidate.speed === 'number') {
    badges.push(t('controlPanel.system.deviceConnection.wifiScanSpeed', { speed: candidate.speed }));
  }
  if (typeof candidate.latencyMs === 'number') {
    badges.push(t('controlPanel.system.deviceConnection.wifiScanLatency', { latency: candidate.latencyMs }));
  }
  return badges;
}

export function profileEndpoint(profile: types.DeviceProfile | null | undefined) {
  return profileConnection(profile).endpoint || '';
}
