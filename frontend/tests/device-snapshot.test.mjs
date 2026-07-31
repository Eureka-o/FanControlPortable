import assert from 'node:assert/strict';
import test from 'node:test';

import { deviceSnapshotFromStatus } from '../src/app/store/device-snapshot.ts';

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
