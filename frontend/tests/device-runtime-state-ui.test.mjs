import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/components/DeviceStatus.tsx', import.meta.url), 'utf8');

test('renders a compact device runtime state beside the existing connection label', () => {
  assert.match(source, /runtimeState/);
  assert.match(source, /deviceStatus\.runtimeState/);
  assert.match(source, /connectStatus\.connected/);
  assert.match(source, /rounded-md px-2 py-0\.5 text-\[11px\]/);
});

test('keeps connection recovery guidance and actions in the existing device status states', () => {
  assert.match(source, /data-device-recovery="connection"/);
  assert.match(source, /deviceStatus\.recovery\.deviceAvailable/);
  assert.match(source, /deviceStatus\.recovery\.releaseCompetingTools/);
  assert.match(source, /deviceStatus\.recovery\.retryOrExport/);
  assert.match(source, /onClick=\{onConnect\}/);
  assert.match(source, /data-device-recovery="temperature"/);
  assert.match(source, /deviceStatus\.bridgeWarning\.recovery\.checkServices/);
  assert.match(source, /apiService\.restartPawnIO\(\)/);
  assert.match(source, /disabled=\{diagnosticsExporting\}/);
  assert.match(source, /onClick=\{onExportDiagnostics\}/);
});
