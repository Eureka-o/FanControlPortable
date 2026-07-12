import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/components/AboutPanel.tsx', import.meta.url), 'utf8');

test('keeps the update action visible and leaves progress to the updater window', () => {
  assert.doesNotMatch(source, /\{hasNewVersion && installerUrl && \(/);
  assert.doesNotMatch(source, /\{updateStage !== 'idle'[\s\S]*?createPortal\(/);
  assert.match(source, /void handleDownloadInstall\(\)/);
});

test('keeps automatic update checks off by default and persists the toggle', () => {
  assert.match(source, /AUTO_CHECK_UPDATES_STORAGE_KEY/);
  assert.match(source, /useState\(\(\) => readAutoCheckUpdates\(\)\)/);
  assert.match(source, /window\.localStorage\.setItem\(AUTO_CHECK_UPDATES_STORAGE_KEY/);
  assert.match(source, /enabled=\{autoCheckUpdates\}/);
});

test('runs the automatic check only once after the window first enters the foreground', () => {
  assert.match(source, /AUTO_CHECK_UPDATES_SESSION_KEY/);
  assert.match(source, /document\.visibilityState !== 'visible'/);
  assert.match(source, /document\.addEventListener\('visibilitychange'/);
  assert.match(source, /window\.sessionStorage\.setItem\(AUTO_CHECK_UPDATES_SESSION_KEY/);
});

test('uses a compact segmented action group and no auto-check helper text', () => {
  assert.doesNotMatch(source, /aboutPanel\.version\.autoCheckHint/);
  assert.match(source, /data-update-actions/);
  assert.match(source, /rounded-l-none/);
  assert.match(source, /rounded-r-none/);
});
