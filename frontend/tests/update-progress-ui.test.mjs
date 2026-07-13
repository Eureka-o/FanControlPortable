import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const readSource = (path) => {
  try {
    return readFileSync(new URL(path, import.meta.url), 'utf8');
  } catch {
    return '';
  }
};

const widgetSource = readSource('../src/app/components/UpdateProgressWidget.tsx');
const shellSource = readSource('../src/app/components/AppShell.tsx');
const aboutSource = readSource('../src/app/components/AboutPanel.tsx');
const backendSource = readSource('../../internal/guiapp/update_api.go');

test('mounts update progress outside page content so navigation does not remove it', () => {
  assert.match(shellSource, /<UpdateProgressWidget\s*\/>/);
});

test('uses a draggable top-right compact progress ring with an internal percentage', () => {
  assert.match(widgetSource, /top-14/);
  assert.match(widgetSource, /right-4/);
  assert.match(widgetSource, /onPointerDown/);
  assert.match(widgetSource, /onPointerMove/);
  assert.match(widgetSource, /<svg/);
  assert.match(widgetSource, /<circle/);
  assert.match(widgetSource, /\{percent\}%/);
  assert.match(widgetSource, /data-update-drag-handle/);
});

test('reopens the collapsed progress ring and gives the expanded card more room', () => {
  const pointerDownStart = widgetSource.indexOf('const handlePointerDown');
  const pointerMoveStart = widgetSource.indexOf('const handlePointerMove');
  const pointerDownBlock = widgetSource.slice(pointerDownStart, pointerMoveStart);

  assert.match(pointerDownBlock, /suppressCollapsedClickRef\.current = false/);
  assert.match(widgetSource, /w-\[304px\]/);
});

test('supports pause, resume, and cancel while preserving resumable progress', () => {
  assert.match(widgetSource, /pauseUpdate/);
  assert.match(widgetSource, /resumeUpdate/);
  assert.match(widgetSource, /cancelUpdate/);
  assert.match(widgetSource, /pauseUpdateDownload/);
  assert.match(widgetSource, /resumeUpdateDownload/);
  assert.match(widgetSource, /cancelUpdateDownload/);
  assert.match(widgetSource, /stage === 'paused'/);
  assert.match(widgetSource, /stage === 'canceled'/);
  assert.match(widgetSource, /expectedSHA256/);
});

test('offers a manual retry after a resumable download fails', () => {
  assert.match(widgetSource, /retryUpdate/);
  assert.match(widgetSource, /common\.actions\.retry/);
});

test('uses backend events as the only source of update stages and retry limits', () => {
  const startBlock = widgetSource.slice(widgetSource.indexOf('startUpdate: async'), widgetSource.indexOf('retryUpdate: async'));
  const pauseBlock = widgetSource.slice(widgetSource.indexOf('pauseUpdate: async'), widgetSource.indexOf('resumeUpdate: async'));
  const resumeBlock = widgetSource.slice(widgetSource.indexOf('resumeUpdate: async'), widgetSource.indexOf('cancelUpdate: async'));
  const cancelBlock = widgetSource.slice(widgetSource.indexOf('cancelUpdate: async'), widgetSource.indexOf('dismissUpdate:'));

  assert.doesNotMatch(widgetSource, /maxAttempts: 3/);
  assert.doesNotMatch(startBlock, /stage:/);
  assert.doesNotMatch(pauseBlock, /stage:/);
  assert.doesNotMatch(resumeBlock, /stage:/);
  assert.doesNotMatch(cancelBlock, /stage:/);
  assert.match(widgetSource, /starting: boolean/);
  assert.match(aboutSource, /state\.starting/);
  assert.match(backendSource, /beginUpdateDownload\(\)[\s\S]*?emitUpdateProgress\(updateProgress\{[^}]*Stage: *"downloading"[^}]*Attempt: *1[^}]*MaxAttempts: *updateDownloadAttempts/);
});

test('shows a one-time completion toast after the installer restarts the app', () => {
  assert.match(widgetSource, /updateCompletedOnLaunch/);
  assert.match(widgetSource, /updateComplete/);
  assert.match(widgetSource, /toast\.success/);
});
