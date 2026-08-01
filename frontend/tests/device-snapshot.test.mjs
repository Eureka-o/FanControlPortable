import assert from 'node:assert/strict';
import test from 'node:test';

import { deviceSnapshotFromStatus, reduceDeviceSnapshot } from '../src/app/store/device-snapshot.ts';

test('merges correlated fields from a connected status', () => {
  const profile = { id: 'serial-1', capabilities: { supportsSetSpeed: true } };
  const snapshot = deviceSnapshotFromStatus({
    connected: true,
    productId: '0x1234',
    deviceName: 'Acme Serial fan',
    model: 'Serial fan',
    deviceSettings: { available: true },
    deviceProfile: profile,
    currentData: { currentRpm: 42 },
  });

  assert.equal(snapshot.isConnected, true);
  assert.equal(snapshot.deviceRuntimeState, 'ready');
  assert.equal(snapshot.deviceModel, 'Acme Serial fan');
  assert.equal(snapshot.runtimeDeviceProfile, profile);
  assert.equal(snapshot.runtimeDeviceCapabilities, profile.capabilities);
  assert.deepEqual(snapshot.fanData, { currentRpm: 42 });
});

test('uses the connection-event fallback while capabilities are settling', () => {
  const snapshot = deviceSnapshotFromStatus({ connected: true }, 'capabilities');
  assert.equal(snapshot.deviceRuntimeState, 'capabilities');
});

test('clears every correlated field for disconnects and core failures', () => {
  const stale = {
    connected: false,
    productId: 'stale',
    model: 'stale',
    deviceSettings: { available: true },
    deviceProfile: { id: 'stale' },
    deviceCapabilities: { supportsSetSpeed: true },
    currentData: { currentRpm: 99 },
  };

  for (const snapshot of [deviceSnapshotFromStatus(stale), deviceSnapshotFromStatus(null)]) {
    assert.deepEqual(snapshot, {
      isConnected: false,
      deviceRuntimeState: 'disconnected',
      deviceProductId: null,
      deviceModel: null,
      deviceSettings: null,
      runtimeDeviceProfile: null,
      runtimeDeviceCapabilities: null,
      fanData: null,
    });
  }
});

test('patches settings and fan data only while connected', () => {
  const connected = deviceSnapshotFromStatus({ connected: true, currentData: { currentRpm: 42 } });
  const withSettings = reduceDeviceSnapshot(connected, { type: 'settings', settings: { available: true } });
  assert.deepEqual(withSettings.deviceSettings, { available: true });

  const withFanData = reduceDeviceSnapshot(withSettings, { type: 'fan-data', fanData: { currentRpm: 900 } });
  assert.deepEqual(withFanData.fanData, { currentRpm: 900 });

  const disconnected = reduceDeviceSnapshot(withFanData, { type: 'disconnected' });
  const lateSettings = reduceDeviceSnapshot(disconnected, { type: 'settings', settings: { available: true } });
  const lateFanData = reduceDeviceSnapshot(lateSettings, { type: 'fan-data', fanData: { currentRpm: 1200 } });
  assert.equal(lateSettings, disconnected);
  assert.equal(lateFanData, disconnected);
});

test('connected events use the capabilities fallback before refresh', () => {
  const snapshot = reduceDeviceSnapshot(deviceSnapshotFromStatus(null), {
    type: 'connected',
    status: { deviceName: 'Acme fan' },
  });
  assert.equal(snapshot.isConnected, true);
  assert.equal(snapshot.deviceRuntimeState, 'capabilities');
  assert.equal(snapshot.deviceModel, 'Acme fan');
});
