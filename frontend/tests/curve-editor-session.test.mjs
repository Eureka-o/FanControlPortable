import assert from 'node:assert/strict';
import test from 'node:test';

import {
  createCurveEditorSession,
  markCurveEditorDirty,
  markCurveEditorSaved,
  requestCurveProfileSwitch,
  requestCurveTabSwitch,
  resolveCurveEditorSwitch,
} from '../src/app/lib/curve-editor-session.ts';

test('tracks dirty and successful save state', () => {
  const dirty = markCurveEditorDirty(createCurveEditorSession());
  assert.equal(dirty.dirty, true);
  assert.equal(markCurveEditorSaved(dirty, true).dirty, false);
  assert.equal(markCurveEditorSaved(dirty, false), dirty);
});

test('defers a profile switch until dirty changes are resolved', () => {
  const dirty = markCurveEditorDirty(createCurveEditorSession());
  const requested = requestCurveProfileSwitch(dirty, 'profile-b', 'profile-a');
  assert.equal(requested.requiresConfirmation, true);
  assert.equal(requested.nextProfileId, '');
  assert.equal(requested.session.pendingProfileId, 'profile-b');

  const discarded = resolveCurveEditorSwitch(requested.session, 'discard', true);
  assert.equal(discarded.nextProfileId, 'profile-b');
  assert.equal(discarded.session.dirty, false);
  assert.equal(discarded.session.pendingProfileId, '');
});

test('completes a pending tab switch only after save succeeds', () => {
  const pending = requestCurveTabSwitch(markCurveEditorDirty(createCurveEditorSession())).session;
  assert.equal(pending.pendingTab, true);

  const failed = resolveCurveEditorSwitch(pending, 'save', false);
  assert.equal(failed.completeTab, false);
  assert.equal(failed.session, pending);

  const saved = resolveCurveEditorSwitch(pending, 'save', true);
  assert.equal(saved.completeTab, true);
  assert.equal(saved.session.dirty, false);
  assert.equal(saved.session.pendingTab, false);
});

test('switches immediately when the draft is clean', () => {
  const requested = requestCurveProfileSwitch(createCurveEditorSession(), 'profile-b', 'profile-a');
  assert.equal(requested.requiresConfirmation, false);
  assert.equal(requested.nextProfileId, 'profile-b');
});
