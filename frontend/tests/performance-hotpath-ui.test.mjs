import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const store = readFileSync(new URL('../src/app/store/app-store.ts', import.meta.url), 'utf8');
const wifiDiscovery = readFileSync(new URL('../src/app/components/settings/useWiFiDiscovery.ts', import.meta.url), 'utf8');

test('skips a store update when session history sampling keeps the same points', () => {
  assert.match(store, /points === state\.sessionHistoryPoints \? state/);
});

test('limits WiFi scan progress rendering to twice per second', () => {
  assert.match(wifiDiscovery, /setInterval\(\(\) => \{\s*setNow\(Date\.now\(\)\);\s*\}, 500\)/);
});
