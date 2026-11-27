import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const root = new URL('../..', import.meta.url);
const globalsCss = readFileSync(new URL('frontend/src/app/globals.css', root), 'utf8');
const deviceStatusSource = readFileSync(new URL('frontend/src/app/components/DeviceStatus.tsx', root), 'utf8');

test('globals.css derives concentric inner radius for custom themes and stops nested metric tile bounce', () => {
  assert.match(
    globalsCss,
    /html\[data-theme\]\s*\{\s*--theme-card-inner-radius:\s*max\(0\.375rem,\s*calc\(var\(--theme-card-radius\) - 0\.375rem\)\);/,
  );
  assert.match(
    globalsCss,
    /html\[data-theme\]\s+\.glacier-shell\s+\.glacier-metric-card\s+\.glacier-stat-tile:hover\s*\{\s*transform:\s*none;/,
  );
});

test('app keeps semantic theme hooks shared across downloadable themes', () => {
  assert.doesNotMatch(deviceStatusSource, /ice-operator-banner/);
  assert.match(deviceStatusSource, /data-theme-card="connection-empty"/);
  assert.match(globalsCss, /data-theme-card="device-hero"\]::before/);
  assert.match(globalsCss, /data-theme-ui="hero-brand-mark"\]\s*\{/);
  assert.match(globalsCss, /data-theme-card="connection-empty"/);
  assert.doesNotMatch(globalsCss.slice(globalsCss.indexOf('Custom themes share')), /data-theme="dune"/);
});
