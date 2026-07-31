import assert from 'node:assert/strict';
import test from 'node:test';

import { normalizeHistoryDisplayPreferences } from '../src/app/hooks/useHistoryDisplayPreferences.ts';

test('defaults all history timeline events on and preserves selective choices', () => {
  const defaults = normalizeHistoryDisplayPreferences();
  assert.equal(defaults.showTimelineEvents, true);
  assert.deepEqual(defaults.timelineEventVisible, {
    disconnect: true,
    reconnect: true,
    resume: true,
    profile: true,
  });

  const customized = normalizeHistoryDisplayPreferences({
    showTimelineEvents: false,
    timelineEventVisible: { disconnect: false },
  });
  assert.equal(customized.showTimelineEvents, false);
  assert.equal(customized.timelineEventVisible.disconnect, false);
  assert.equal(customized.timelineEventVisible.reconnect, true);
});
