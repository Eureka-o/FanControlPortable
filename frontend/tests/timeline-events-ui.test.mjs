import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const curve = readFileSync(new URL('../src/app/components/FanCurve.tsx', import.meta.url), 'utf8');
const store = readFileSync(new URL('../src/app/store/app-store.ts', import.meta.url), 'utf8');
const api = readFileSync(new URL('../src/app/services/api.ts', import.meta.url), 'utf8');
const ipc = readFileSync(new URL('../../internal/ipc/ipc.go', import.meta.url), 'utf8');
const gui = readFileSync(new URL('../../internal/guiapp/ipc_client.go', import.meta.url), 'utf8');
const core = readFileSync(new URL('../../internal/coreapp/system_device.go', import.meta.url), 'utf8');

test('renders real connection, resume, and profile events without heartbeat inference', () => {
  assert.match(ipc, /EventSystemResume\s*=\s*"system-resume"/);
  assert.match(core, /BroadcastEvent\(ipc\.EventSystemResume/);
  assert.match(gui, /case ipc\.EventSystemResume:/);
  assert.match(api, /onSystemResume\(/);

  assert.match(store, /timelineEvents: TimelineEvent\[\]/);
  assert.match(store, /type: 'disconnect'/);
  assert.match(store, /type: 'reconnect'/);
  assert.match(store, /type: 'resume'/);
  assert.match(store, /activeFanCurveProfileId/);
  assert.match(store, /type: 'profile'/);
  assert.doesNotMatch(store, /onHeartbeat\(/);

  assert.match(curve, /ReferenceLine/);
  assert.match(curve, /const timelineEventLayout = useMemo/);
  assert.match(curve, /const historyTimeDomain = useMemo/);
  assert.match(curve, /row \* 12/);
  assert.match(curve, /anchorEnd/);
});
