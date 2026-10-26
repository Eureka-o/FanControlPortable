import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { runInNewContext } from 'node:vm';
import ts from 'typescript';
import { subscribeWithSelector } from 'zustand/middleware';
import { createStore } from 'zustand/vanilla';

const storeSource = readFileSync(new URL('../src/app/store/app-store.ts', import.meta.url), 'utf8');
const selectorSource = storeSource.match(/export const select(?:Temperature|FanData|TemperatureHistoryPoints) = [\s\S]*?;/g);
assert.equal(selectorSource?.length, 3);
const selectors = {};
runInNewContext(ts.transpileModule(selectorSource.join('\n'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS },
}).outputText, { exports: selectors });

const createTelemetryStore = () => createStore(subscribeWithSelector(() => ({
  temperature: { cpuTemp: 60 },
  fanData: { currentRpm: 1500 },
  temperatureHistoryEnabled: true,
  temperatureHistoryPoints: [{ timestamp: 1000, cpuTemp: 60 }],
  sessionHistoryPoints: [{ timestamp: 1000, cpuTemp: 59 }],
})));

test('notifies only the telemetry selector whose value changed', () => {
  const store = createTelemetryStore();
  const changed = [];
  for (const [name, selector] of Object.entries(selectors)) {
    store.subscribe(selector, () => changed.push(name));
  }

  store.setState({ config: {}, activeTab: 'control' });
  assert.deepEqual(changed, []);
  store.setState({ temperature: { cpuTemp: 61 } });
  assert.deepEqual(changed, ['selectTemperature']);
  store.setState({ fanData: { currentRpm: 1600 } });
  assert.deepEqual(changed, ['selectTemperature', 'selectFanData']);
  store.setState({ temperatureHistoryPoints: [{ timestamp: 2000, cpuTemp: 61 }] });
  assert.deepEqual(changed, ['selectTemperature', 'selectFanData', 'selectTemperatureHistoryPoints']);
});

test('history subscribers ignore the inactive source and follow source switches', () => {
  const store = createTelemetryStore();
  const points = [];
  store.subscribe(selectors.selectTemperatureHistoryPoints, (next) => points.push(next));

  store.setState({ sessionHistoryPoints: [{ timestamp: 2000, cpuTemp: 61 }] });
  assert.deepEqual(points, []);
  store.setState({ temperatureHistoryEnabled: false });
  assert.equal(points.length, 1);
  assert.equal(points[0], store.getState().sessionHistoryPoints);
  store.setState({ temperatureHistoryPoints: [] });
  assert.equal(points.length, 1);
  store.setState({ sessionHistoryPoints: [] });
  assert.equal(points.length, 2);
  store.setState({ temperatureHistoryEnabled: true });
  assert.equal(points.length, 3);
  assert.equal(points[2], store.getState().temperatureHistoryPoints);
});

test('keeps page orchestration outside live telemetry subscriptions', () => {
  const page = readFileSync(new URL('../src/app/page.tsx', import.meta.url), 'utf8');
  const historyHook = readFileSync(new URL('../src/app/hooks/useTemperatureHistory.ts', import.meta.url), 'utf8');
  const home = page.slice(page.indexOf('function LiveHome()'));
  assert.doesNotMatch(home, /state\.(?:fanData|temperature)\b|view\.(?:fanData|temperature)\b/);
  assert.match(page, /useAppStore\(selectFanData\)/);
  assert.match(page, /useAppStore\(selectTemperature\)/);
  for (const name of ['AppShell', 'DeviceStatus', 'FanCurve', 'ControlPanel']) {
    assert.match(page, new RegExp(`<${name} \\{\\.\\.\\.props\\} \\{\\.\\.\\.useLiveTelemetry\\(props\\.config\\)\\} />`));
    assert.match(home, new RegExp(`<Live${name}\\b`));
  }
  assert.match(historyHook, /useAppStore\(selectTemperatureHistoryPoints\)/);
  assert.doesNotMatch(historyHook, /state\.(?:sessionHistoryPoints|temperatureHistoryPoints)/);
});
