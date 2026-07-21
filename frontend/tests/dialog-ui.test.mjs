import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const dialog = readFileSync(new URL('../src/components/ui/dialog.tsx', import.meta.url), 'utf8');
const styles = readFileSync(new URL('../src/app/globals.css', import.meta.url), 'utf8');
const axisNoise = readFileSync(new URL('../src/app/components/AxisNoiseScan.tsx', import.meta.url), 'utf8');
const fanControl = readFileSync(new URL('../src/app/components/settings/FanControlSection.tsx', import.meta.url), 'utf8');

test('keeps every dialog inside the viewport', () => {
  assert.match(dialog, /w-\[calc\(100vw-2rem\)\]/);
  assert.match(dialog, /max-h-\[calc\(100vh-2rem\)\]/);
  assert.match(dialog, /overflow-y-auto/);
  assert.match(axisNoise, /grid-cols-1[^\n]*min-\[560px\]:grid-cols-3/);
  assert.match(fanControl, /<Dialog open=\{showCustomSpeedWarning\}/);
  assert.doesNotMatch(fanControl, /fixed inset-0 z-50/);
});

test('uses opacity-only dialog animations to avoid WebView compositing seams', () => {
  assert.doesNotMatch(dialog, /data-\[state=open\]:animate-in|data-\[state=closed\]:animate-out/);

  const animationStart = styles.indexOf('@keyframes dialog-fade-in');
  const animationEnd = styles.indexOf('[data-slot="dialog-overlay"]', animationStart);
  assert.ok(animationStart >= 0 && animationEnd > animationStart);

  const animationStyles = styles.slice(animationStart, animationEnd);
  assert.match(animationStyles, /opacity:\s*0/);
  assert.doesNotMatch(animationStyles, /transform|filter/);
});
