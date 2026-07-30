import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const api = readFileSync(new URL('../src/app/services/api.ts', import.meta.url), 'utf8');
const store = readFileSync(new URL('../src/app/store/app-store.ts', import.meta.url), 'utf8');

test('resyncs core state once after IPC recovery', () => {
  assert.match(api, /onCoreResynced\(/);
  assert.match(store, /resyncCore:\s*async/);
  assert.match(store, /resyncInFlight/);
  assert.match(store, /onCoreResynced\(/);
  assert.match(store, /recovering[\s\S]*resyncCore\(\)/);
});
