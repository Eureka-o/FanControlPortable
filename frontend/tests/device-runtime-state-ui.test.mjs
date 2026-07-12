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
