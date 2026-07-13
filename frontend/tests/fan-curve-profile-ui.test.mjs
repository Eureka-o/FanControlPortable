import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/components/FanCurve.tsx', import.meta.url), 'utf8');
const styles = readFileSync(new URL('../src/app/globals.css', import.meta.url), 'utf8');
const pageSource = readFileSync(new URL('../src/app/page.tsx', import.meta.url), 'utf8');
const historyHookSource = readFileSync(new URL('../src/app/hooks/useTemperatureHistory.ts', import.meta.url), 'utf8');

test('keeps profile selection, creation, and management in the curve header', () => {
  assert.match(source, /data-curve-profile-toolbar/);
  assert.match(source, /data-curve-profile-row/);
  assert.match(source, /data-curve-profile-list/);
  assert.match(source, /px-4 text-center text-xs font-medium/);
  assert.match(source, /\[&::\-webkit-scrollbar\]:hidden/);
  assert.match(source, /group relative flex shrink-0 hover:z-10 focus-within:z-10/);
  assert.match(source, /absolute -right-\[6px\] -top-\[1px\] z-10 flex h-\[13px\] w-\[13px\]/);
  assert.match(source, /opacity-0[^']*group-hover:opacity-100/);
  assert.match(source, /border-destructive\/50 bg-card text-destructive/);
  assert.match(source, /border-border bg-card text-muted-foreground hover:border-destructive\/50 hover:text-destructive/);
  assert.doesNotMatch(source, /bg-destructive\/10|opacity-100 hover:bg/);
  assert.doesNotMatch(source, /-ml-px flex h-\[13px\]/);
  assert.match(source, /<X className="h-2 w-2" \/>/);
  assert.match(source, /setCreateProfileDialogOpen\(true\)/);
  assert.match(source, /setManageProfilesDialogOpen\(true\)/);
  assert.doesNotMatch(source, /<FanCurveProfileSelect/);
  assert.doesNotMatch(source, /data-theme-card="curve-profiles"/);
  assert.doesNotMatch(source, /data-theme-card="curve-import-export"/);
});

test('stacks export and import sections in the profile manager', () => {
  assert.match(source, /data-profile-export-section/);
  assert.match(source, /data-profile-import-section/);
  assert.doesNotMatch(source, /grid-cols-1 gap-3 md:grid-cols-2/);
});

test('keeps profile rename and transfer actions compact and aligned', () => {
  assert.match(source, /DialogContent className="max-w-\[620px\]"/);
  assert.match(source, /data-profile-rename-row/);
  assert.match(source, /sm:grid-cols-\[minmax\(0,1fr\)_auto\]/);
  assert.match(source, /data-profile-transfer-header className="space-y-1"/);
  assert.match(source, /data-profile-export-actions className="grid grid-cols-1 gap-2 sm:grid-cols-2"/);
  assert.match(source, /rows=\{2\}/);
});

test('guards profile switches when the current curve has unsaved changes', () => {
  assert.match(source, /if \(hasUnsavedChanges\) \{\s*setPendingProfileId\(id\);\s*setProfileSwitchDialogOpen\(true\);\s*return;/);
  assert.match(source, /confirmProfileSwitch\('save'\)/);
  assert.match(source, /confirmProfileSwitch\('discard'\)/);
});

test('confirms deletion and keeps creating a profile from the current curve', () => {
  assert.match(source, /setDeleteProfileDialogOpen\(true\)/);
  assert.match(source, /setPendingDeleteProfileId\(profile\.id\)/);
  assert.match(source, /group-hover:opacity-100/);
  assert.match(source, /saveFanCurveProfile\('', safeName, localCurve, true\)/);
});

test('exports by clipboard or file and imports pasted, selected, or dropped content', () => {
  assert.match(source, /navigator\.clipboard\.writeText\(code\)/);
  assert.match(source, /apiService\.exportFanCurveProfilesToFile\(\)/);
  assert.doesNotMatch(source, /new Blob\(\[code\]/);
  assert.doesNotMatch(source, /anchor\.download/);
  assert.match(source, /type="file"/);
  assert.match(source, /onDrop=\{handleProfileFileDrop\}/);
  assert.match(source, /await file\.text\(\)/);
  assert.match(source, /value=\{importCode\}/);
});

test('reveals history upward and keeps focused-entry layout stable', () => {
  assert.match(source, /const initialFocusTarget = useRef\(focusTarget\)\.current/);
  assert.match(source, /initialFocusTarget === 'history-details' \? 'cards-reverse' : initialFocusTarget \? undefined : 'cards'/);
  assert.match(source, /initial=\{initialFocusTarget \? false : \{ opacity: 0, y: 8 \}\}/);
  assert.match(source, /initial=\{initialFocusTarget \? false : \{ opacity: 0, height: 0 \}\}/);
  assert.match(styles, /\[data-page-reveal="cards-reverse"\] > :nth-last-child\(2\)/);
  assert.match(styles, /\[data-page-reveal="cards-reverse"\] > :nth-last-child\(7\)/);
});

test('uses normal curve navigation and keeps history hydration out of the entry animation', () => {
  assert.match(pageSource, /onOpenCurveEditor=\{\(\) => setActiveTab\('curve'\)\}/);
  assert.match(pageSource, /onOpenHistoryDetails=\{\(\) => openCurveTab\('history-details'\)\}/);
  assert.match(historyHookSource, /startTransition\(\(\) => \{/);
  assert.match(styles, /\[data-theme-card="curve-history"\] \[data-theme-ui="switch-thumb"\][^}]+transition: none;/s);
});
