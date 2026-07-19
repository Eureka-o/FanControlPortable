import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/lib/theme-bootstrap.ts', import.meta.url), 'utf8');
const syncSource = readFileSync(new URL('../src/app/components/SystemThemeSync.tsx', import.meta.url), 'utf8');
const settingsSource = readFileSync(new URL('../src/app/components/settings/SystemSettingsSection.tsx', import.meta.url), 'utf8');

test('does not replay truncated custom theme CSS during startup', () => {
  assert.match(source, /if \(parsed\.cssTruncated\) \{\s*return null;/);
  assert.match(source, /if \(snapshot\.cssTruncated\) \{\s*applyBaseTheme\(base === 'dark'\);\s*return;/);
});

test('keeps the current theme when custom theme CSS refresh fails', () => {
  const fallback = syncSource.slice(
    syncSource.indexOf('if (!css) {'),
    syncSource.indexOf('// 先设基底明暗'),
  );

  assert.match(fallback, /if \(!css\) \{/);
  assert.match(fallback, /\breturn;/);
  assert.doesNotMatch(fallback, /clearCustomTheme|writeThemeBootstrapSnapshot/);
});

test('does not change the base theme before custom theme CSS is ready', () => {
  const handler = settingsSource.slice(
    settingsSource.indexOf('const handleThemeModeChange'),
    settingsSource.indexOf('const handleWindowBlurChange'),
  );

  assert.match(handler, /if \(isBuiltin\) \{[\s\S]*?classList\.toggle\('dark', nextThemeIsDark\);[\s\S]*?\}/);
});

test('keeps cached custom theme metadata when theme discovery is temporarily unavailable', () => {
  const applyCustomTheme = syncSource.slice(
    syncSource.indexOf('async function applyCustomTheme'),
    syncSource.indexOf('export default function SystemThemeSync'),
  );

  assert.match(applyCustomTheme, /const cachedSnapshot = readThemeBootstrapSnapshot\(\);/);
  assert.match(applyCustomTheme, /cachedSnapshot\?\.mode === id/);
});
