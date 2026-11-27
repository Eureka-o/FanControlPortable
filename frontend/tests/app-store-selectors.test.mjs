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

test('fan data equality keeps curve target RPM updates', () => {
  assert.match(
    storeSource,
    /left\.targetRpm === right\.targetRpm\s*&&\s*left\.curveTargetRpm === right\.curveTargetRpm/,
  );
});

test('keeps page orchestration and card roots outside live telemetry subscriptions', () => {
  const page = readFileSync(new URL('../src/app/page.tsx', import.meta.url), 'utf8');
  const appShell = readFileSync(new URL('../src/app/components/AppShell.tsx', import.meta.url), 'utf8');
  const controlPanel = readFileSync(new URL('../src/app/components/ControlPanel.tsx', import.meta.url), 'utf8');
  const deviceStatus = readFileSync(new URL('../src/app/components/DeviceStatus.tsx', import.meta.url), 'utf8');
  const fanCurve = readFileSync(new URL('../src/app/components/FanCurve.tsx', import.meta.url), 'utf8');
  const historyHook = readFileSync(new URL('../src/app/hooks/useTemperatureHistory.ts', import.meta.url), 'utf8');
  const home = page.slice(page.indexOf('function LiveHome()'));
  assert.doesNotMatch(home, /state\.(?:fanData|temperature)\b|view\.(?:fanData|temperature)\b/);
  assert.doesNotMatch(page, /useLiveTelemetry/);
  assert.doesNotMatch(page, /useAppStore\(selectFanData\)/);
  assert.doesNotMatch(page, /useAppStore\(selectTemperature\)/);
  assert.doesNotMatch(home, /<Live(?:AppShell|ControlPanel|DeviceStatus|FanCurve)\b/);
  assert.match(home, /<DeviceStatus\b/);
  assert.match(home, /<FanCurve\b/);
  // 遥测叶子（标题栏 StatusBadges、设置页遥测概览条、DeviceStatus/FanCurve 内部叶组件）自己用精确 selector 订阅。
  assert.match(appShell, /const fanData = useAppStore\(selectFanData\)/);
  assert.match(appShell, /const temperature = useAppStore\(selectTemperature\)/);
  assert.match(controlPanel, /useAppStore\(selectFanData\)/);
  assert.match(controlPanel, /useAppStore\(selectTemperature\)/);
  // ControlPanel 对 fanData 只允许窄字段 selector（连接回退/上下文键用的 transport）。
  assert.match(controlPanel, /useAppStore\(\(state\) => state\.fanData\?\.transport/);
  assert.doesNotMatch(controlPanel, /state\.fanData\b(?!\?\.transport)/);
  assert.match(deviceStatus, /const LiveCpuMetricCard = memo/);
  assert.match(deviceStatus, /const LiveFanMetricCard = memo/);
  assert.match(deviceStatus, /const LiveMiniFanCurveCard = memo/);
  assert.match(fanCurve, /const LiveTemperatureIndicator = memo/);
  assert.match(fanCurve, /state\.fanData\?\.transport/);
  assert.match(historyHook, /useAppStore\(selectTemperatureHistoryPoints\)/);
  assert.doesNotMatch(historyHook, /state\.(?:sessionHistoryPoints|temperatureHistoryPoints)/);
});
