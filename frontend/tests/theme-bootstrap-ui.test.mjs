import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/lib/theme-bootstrap.ts', import.meta.url), 'utf8');

test('does not replay truncated custom theme CSS during startup', () => {
  assert.match(source, /if \(parsed\.cssTruncated\) \{\s*return null;/);
  assert.match(source, /if \(snapshot\.cssTruncated\) \{\s*applyBaseTheme\(base === 'dark'\);\s*return;/);
});
