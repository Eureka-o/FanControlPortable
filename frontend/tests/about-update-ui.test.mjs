import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/components/AboutPanel.tsx', import.meta.url), 'utf8');
const manualCheckSource = source.slice(
  source.indexOf('const handleCheckUpdate'),
  source.indexOf('const handleDownloadInstall'),
);
test('keeps download and check actions visible and delegates progress globally', () => {
  const actions = source.slice(source.indexOf('<div data-about-actions'), source.indexOf('{releaseError'));

  assert.match(actions, /void handleDownloadInstall\(\)/);
  assert.match(actions, /aboutPanel\.version\.downloadAndInstall/);
  assert.match(actions, /void handleCheckUpdate\(\)/);
  assert.match(actions, /aboutPanel\.version\.checkUpdate/);
  assert.doesNotMatch(actions, /\{hasNewVersion \? \(/);
  assert.doesNotMatch(source, /\{updateStage !== 'idle'[\s\S]*?createPortal\(/);
  assert.match(source, /useUpdateStore/);
  assert.match(source, /startUpdate/);
});

test('removes automatic update checking and keeps both manual outcomes', () => {
  assert.doesNotMatch(source, /AutoUpdateNotifier|AUTO_CHECK_UPDATES|autoCheckUpdates/);
  assert.match(manualCheckSource, /toast\.success\(t\('aboutPanel\.version\.upToDate'/);
  assert.match(manualCheckSource, /toast\.info\(t\('aboutPanel\.version\.newVersionFound'/);
});

test('uses backend version comparison as the update source of truth', () => {
  assert.doesNotMatch(source, /function isLatestVersion/);
  assert.match(source, /targetRelease\.update_available/);
  assert.match(manualCheckSource, /release\.update_available/);
});

test('uses a compact segmented action group and no auto-check helper text', () => {
  assert.doesNotMatch(source, /aboutPanel\.version\.autoCheck/);
  assert.match(source, /data-update-actions/);
  assert.match(source, /rounded-l-none/);
  assert.match(source, /rounded-r-none/);
});

test('lets the update action wrap intact when the expanded dock narrows the page', () => {
  assert.match(source, /max-w-\[980px\]/);
  assert.doesNotMatch(source, /data-about-actions className="[^"]*lg:flex-nowrap/);
  assert.match(source, /data-update-actions className="[^"]*shrink-0/);
});
