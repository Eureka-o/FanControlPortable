import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/components/AboutPanel.tsx', import.meta.url), 'utf8');
const manualCheckSource = source.slice(
  source.indexOf('const handleCheckUpdate'),
  source.indexOf('const handleDownloadInstall'),
);

test('keeps the update action visible and delegates progress to the global update task', () => {
  assert.doesNotMatch(source, /\{hasNewVersion && installerUrl && \(/);
  assert.doesNotMatch(source, /\{updateStage !== 'idle'[\s\S]*?createPortal\(/);
  assert.match(source, /useUpdateStore/);
  assert.match(source, /startUpdate/);
  assert.match(source, /void handleDownloadInstall\(\)/);
});

test('removes automatic update checking and keeps both manual outcomes', () => {
  assert.doesNotMatch(source, /AutoUpdateNotifier|AUTO_CHECK_UPDATES|autoCheckUpdates/);
  assert.match(manualCheckSource, /toast\.success\(t\('aboutPanel\.version\.upToDate'/);
  assert.match(manualCheckSource, /toast\.info\(t\('aboutPanel\.version\.newVersionFound'/);
});

test('uses a compact segmented action group and no auto-check helper text', () => {
  assert.doesNotMatch(source, /aboutPanel\.version\.autoCheck/);
  assert.match(source, /data-update-actions/);
  assert.match(source, /rounded-l-none/);
  assert.match(source, /rounded-r-none/);
});

test('keeps all about-page actions on one row in the desktop two-column layout', () => {
  assert.match(source, /max-w-\[980px\]/);
  assert.match(source, /data-about-actions className="[^"]*lg:flex-nowrap/);
});
