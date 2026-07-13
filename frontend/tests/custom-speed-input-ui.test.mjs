import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/components/settings/FanControlSection.tsx', import.meta.url), 'utf8');
const inputBlock = source.slice(
  source.indexOf('value={customSpeedInput}'),
  source.indexOf("{t('common.actions.apply')}", source.indexOf('value={customSpeedInput}')),
);

test('allows partial custom speed input and validates only when applying', () => {
  assert.doesNotMatch(inputBlock, /min=\{overviewSpeedRange\.min\}/);
  assert.doesNotMatch(inputBlock, /max=\{overviewSpeedRange\.max\}/);
  assert.match(source, /parseCustomSpeedDraft\(speed\)/);
  assert.match(source, /safeSpeed < overviewSpeedRange\.min \|\| safeSpeed > overviewSpeedRange\.max/);
  assert.match(source, /customSpeedInvalid/);
  assert.match(source, /setCustomSpeedInput\(String\(safeSpeed\)\)/);
});

test('shows the raw draft in the confirmation instead of a clamped minimum', () => {
  assert.match(source, /const customSpeedDraftValue = useMemo/);
  assert.match(source, /formatFanSpeedValue\(customSpeedDraftValue\)/);
  assert.doesNotMatch(source, /const customSpeedInputValue = useMemo/);
});
