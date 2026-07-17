import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const pluginRoot = new URL('../../plugins/omen-fan/src/', import.meta.url);

test('OMEN plugin injects its brand icon and keeps a host icon fallback', async () => {
  const manifest = JSON.parse(await readFile(new URL('plugin.json', pluginRoot), 'utf8'));
  const icon = await readFile(new URL(manifest.page.iconAsset, pluginRoot));

  assert.equal(manifest.page.icon, 'fan');
  assert.equal(manifest.page.iconAsset, 'ui/assets/omen.png');
  assert.ok(icon.length > 1_000);
});

test('OMEN frontend exposes the approved four-page information architecture', async () => {
  const source = await readFile(new URL('ui/index.js', pluginRoot), 'utf8');

  for (const page of ['overview', 'performance', 'curves', 'device']) {
    assert.match(source, new RegExp(`['"]${page}['"]`));
  }
  for (const label of ['概览', '性能调校', '风扇曲线', '设备功能']) {
    assert.match(source, new RegExp(label));
  }
  for (const mode of ['eco', 'balanced', 'performance', 'master']) {
    assert.match(source, new RegExp(`['"]${mode}['"]`));
  }
  assert.match(source, /大师模式/);
  assert.doesNotMatch(source, /set-fan-mode|set-manual-speed/i);
});

test('OMEN controls are capability-gated and update from backend readback', async () => {
  const source = await readFile(new URL('ui/index.js', pluginRoot), 'utf8');

  for (const token of [
    'get-status',
    'status-changed',
    'set-thermal-mode',
    'set-cpu-power',
    'set-cpu-boost-policy',
    'set-power-bias',
    'set-gpu-power',
    'set-screen-overdrive',
    'set-charge-limit',
    'set-gpu-mode',
    'set-fan-curve',
    'set-joint-learning',
    'export-diagnostics',
  ]) {
    assert.match(source, new RegExp(`['"]${token}['"]`));
  }
  for (const capability of [
    'thermalMode',
    'cpuPowerLimits',
    'cpuBoostPolicy',
    'powerBias',
    'gpuPower',
    'screenOverdrive',
    'chargeProtection',
    'gpuMode',
    'fanCurves',
    'jointLearning',
    'diagnostics',
  ]) {
    assert.match(source, new RegExp(`capabilities\\.${capability}`));
  }
  assert.match(source, /normalizeStatus\(next, previous\)/);
  assert.doesNotMatch(source, /\beval\s*\(|new Function/);
});

test('OMEN overview reuses the host realtime overview component', async () => {
  const source = await readFile(new URL('ui/index.js', pluginRoot), 'utf8');
  const style = await readFile(new URL('ui/index.css', pluginRoot), 'utf8');

  assert.match(source, /ui\.RealtimeOverview/);
  assert.match(source, /cpuModel/);
  assert.match(source, /gpuModel/);
  assert.match(source, /id:\s*'cpu'/);
  assert.match(source, /id:\s*'gpu'/);
  assert.match(source, /id:\s*'fan'/);
  assert.match(source, /id:\s*'mode'/);
  assert.match(source, /id:\s*'joint-learning'/);
  assert.doesNotMatch(source, /omen-device-card|omen-metrics|omen-metric/);
  assert.doesNotMatch(style, /omen-device-card|omen-metrics|omen-metric/);
});

test('OMEN overview keeps simple controls direct and tuning values readable', async () => {
  const source = await readFile(new URL('ui/index.js', pluginRoot), 'utf8');
  const overview = source.slice(source.indexOf('function overviewPage'), source.indexOf('function performancePage'));
  const performance = source.slice(source.indexOf('function performancePage'), source.indexOf('function curveChart'));
  const device = source.slice(source.indexOf('function devicePage'), source.indexOf('if (!status && loading)'));

  assert.doesNotMatch(overview, /setActivePage|text\.cpuTuning|text\.customCurve/);
  for (const method of ['set-gpu-power', 'set-screen-overdrive', 'set-charge-limit', 'set-gpu-mode']) {
    assert.match(overview, new RegExp(`['"]${method}['"]`));
  }
  assert.match(overview, /ui\.Select/);
  assert.match(performance, /ui\.Slider/);
  assert.match(performance, /omen-power-control/);
  for (const method of ['set-screen-overdrive', 'set-charge-limit', 'set-gpu-mode']) {
    assert.doesNotMatch(device, new RegExp(`['"]${method}['"]`));
  }
  assert.match(source, /masterMode:\s*'大师'/);
});

test('OMEN AMD power controls use the G-Helper 8945HX tuning ranges', async () => {
  const source = await readFile(new URL('ui/index.js', pluginRoot), 'utf8');
  const performance = source.slice(source.indexOf('function performancePage'), source.indexOf('function curveChart'));

  assert.match(performance, /amd \? 5 : 0/);
  assert.match(performance, /amd \? 150 : 250/);
  assert.match(performance, /amd \? 75 : 0/);
  assert.match(performance, /amd \? 96 : 110/);
});

test('OMEN curve and device pages preserve the agreed safety and layout rules', async () => {
  const source = await readFile(new URL('ui/index.js', pluginRoot), 'utf8');
  const style = await readFile(new URL('ui/index.css', pluginRoot), 'utf8');

  assert.match(source, /isMasterMode/);
  assert.match(source, /savedNotActive/);
  assert.ok(source.indexOf('omen-joint-learning') > source.indexOf('omen-curve-stack'));
  assert.doesNotMatch(source, /keyboard.{0,24}(light|lighting)|lighting.{0,24}keyboard/i);

  assert.match(style, /\.omen-quick-grid\s*\{[^}]*repeat\(2,/s);
  assert.match(style, /\.omen-diagnostics-row\s*\{[^}]*grid-column:\s*1\s*\/\s*-1/s);
  assert.match(style, /@media \(max-width: 980px\)[\s\S]*\.omen-quick-grid\s*\{[^}]*repeat\(2,/);
  assert.match(style, /@media \(max-width: 620px\)[\s\S]*\.omen-quick-grid\s*\{[^}]*grid-template-columns:\s*1fr/);

  const selectorLines = style.split(/\r?\n/).filter((line) => line.includes('{') && !line.trimStart().startsWith('@'));
  for (const selector of selectorLines) {
    assert.match(selector, /\[data-plugin-id="omen-fan"\]/);
  }
});
