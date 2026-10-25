import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/components/BlackSharkPanels.tsx', import.meta.url), 'utf8');
const zh = readFileSync(new URL('../src/app/locales/zh-CN/translation.json', import.meta.url), 'utf8');

test('allows BlackShark BLE image transfer and warns before the slow path', () => {
  assert.match(source, /builtin\.blackshark\.brb02\.ble\.rpm/);
  assert.match(source, /const isBleTransfer = runtimeDeviceProfile\?\.transport === 'ble'/);
  assert.match(source, /setBleWarningOpen\(true\)/);
  assert.match(source, /bleSlowWarningDescription/);
  assert.match(source, /runImageTransfer\(\(\) => applyPreset/);
  assert.match(source, /runImageTransfer\(\(\) => applyCrop/);
  assert.match(zh, /"bleSlowWarningDescription"/);
});
