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

test('supports pause, resume, and cancel while preserving resumable progress', () => {
  assert.match(widgetSource, /pauseUpdate/);
  assert.match(widgetSource, /resumeUpdate/);
  assert.match(widgetSource, /cancelUpdate/);
  assert.match(widgetSource, /pauseUpdateDownload/);
  assert.match(widgetSource, /resumeUpdateDownload/);
  assert.match(widgetSource, /cancelUpdateDownload/);
  assert.match(widgetSource, /stage === 'paused'/);
  assert.match(widgetSource, /stage === 'canceled'/);
});

test('offers a manual retry after a resumable download fails', () => {
  assert.match(widgetSource, /retryUpdate/);
  assert.match(widgetSource, /common\.actions\.retry/);
});

test('shows a one-time completion toast after the installer restarts the app', () => {
  assert.match(widgetSource, /updateCompletedOnLaunch/);
  assert.match(widgetSource, /updateComplete/);
  assert.match(widgetSource, /toast\.success/);
});
