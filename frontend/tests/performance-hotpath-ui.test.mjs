import assert from 'node:assert/strict';
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';

const store = readFileSync(new URL('../src/app/store/app-store.ts', import.meta.url), 'utf8');
const wifiDiscovery = readFileSync(new URL('../src/app/components/settings/useWiFiDiscovery.ts', import.meta.url), 'utf8');
const globalsCss = readFileSync(new URL('../src/app/globals.css', import.meta.url), 'utf8');
const bootstrapHook = readFileSync(new URL('../src/app/hooks/useAppBootstrap.ts', import.meta.url), 'utf8');
const apiSource = readFileSync(new URL('../src/app/services/api.ts', import.meta.url), 'utf8');
const pageSource = readFileSync(new URL('../src/app/page.tsx', import.meta.url), 'utf8');
const layoutSource = readFileSync(new URL('../src/app/layout.tsx', import.meta.url), 'utf8');

function walkFiles(dirUrl) {
  const results = [];
  for (const entry of readdirSync(dirUrl, { withFileTypes: true })) {
    const childUrl = new URL(entry.isDirectory() ? `${entry.name}/` : entry.name, dirUrl);
    if (entry.isDirectory()) {
      results.push(...walkFiles(childUrl));
    } else if (/\.(ts|tsx)$/.test(entry.name)) {
      results.push(childUrl);
    }
  }
  return results;
}

test('skips a store update when session history sampling keeps the same points and coalesces live telemetry ticks', () => {
  assert.match(store, /points === state\.sessionHistoryPoints \? state/);
  assert.match(store, /requestAnimationFrame\(flushTelemetry\)/);
});

test('limits WiFi scan progress rendering to twice per second', () => {
  assert.match(wifiDiscovery, /setInterval\(\(\) => \{\s*setNow\(Date\.now\(\)\);\s*\}, 500\)/);
});

test('app store calls the shared API service without pass-through service modules', () => {
  assert.doesNotMatch(store, /configService|deviceService/);
  assert.match(store, /apiService\.getConfig\(\)/);
  assert.match(store, /apiService\.getDeviceStatus\(\)/);
});

test('pauses animations and syncs backend window visibility when hidden', () => {
  assert.match(globalsCss, /html\[data-window-hidden="true"\]\s*\*[\s\S]*?animation-play-state:\s*paused\s*!important/);
  assert.match(bootstrapHook, /document\.addEventListener\('visibilitychange'/);
  assert.match(bootstrapHook, /apiService\.setWindowHidden\(hidden\)/);
  assert.match(apiSource, /SetWindowHidden\?\.\(hidden\)/);
  assert.match(apiSource, /EventsOn\('window-visibility-change'/);
});

test('isolates card containment for metric and hero cards', () => {
  assert.match(globalsCss, /\.glacier-metric-card,[\s\S]*?contain:\s*layout style;/);
  assert.match(globalsCss, /\.glacier-hero-card,[\s\S]*?contain:\s*layout style paint;/);
});

test('defers secondary tab prefetch until hover/focus and uses LazyMotion domAnimation', () => {
  assert.match(pageSource, /function prefetchTab\(/);
  assert.doesNotMatch(pageSource, /requestIdleCallback\(prefetchSecondaryTabs/);
  assert.match(layoutSource, /<MotionProvider>/);
  const srcFiles = walkFiles(new URL('../src/', import.meta.url));
  for (const fileUrl of srcFiles) {
    const text = readFileSync(fileUrl, 'utf8');
    assert.doesNotMatch(text, /<\/?motion\./, `${join(fileUrl.pathname)} must use m.* via LazyMotion`);
  }
});
