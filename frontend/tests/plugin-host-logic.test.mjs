import test from 'node:test';
import assert from 'node:assert/strict';

import {
  isVisiblePluginPage,
  pluginAssetURL,
  pluginFrontendFingerprint,
  pluginPageTabId,
  sortVisiblePluginPages,
} from '../src/app/plugins/plugin-host-logic.mts';

const readyPlugin = {
  id: 'fake-plugin',
  name: 'Fake Plugin',
  version: '1.0.0',
  enabled: true,
  state: 'ready',
  frontend: 'ui/index.js',
  style: 'ui/index.css',
  page: { id: 'control', iconAsset: 'ui/assets/icon.png', order: 200 },
};

test('only ready enabled plugins expose a stable tab and versioned asset URL', () => {
  assert.equal(isVisiblePluginPage(readyPlugin), true);
  assert.equal(isVisiblePluginPage({ ...readyPlugin, state: 'starting' }), false);
  assert.equal(pluginPageTabId(readyPlugin), 'plugin:fake-plugin:control');
  assert.equal(
    pluginAssetURL('fake-plugin', 'ui/index.js', '1.0.0'),
    '/plugin-assets/fake-plugin/ui/index.js?v=1.0.0',
  );
  assert.equal(
    pluginAssetURL('fake-plugin', readyPlugin.page.iconAsset, '1.0.0'),
    '/plugin-assets/fake-plugin/ui/assets/icon.png?v=1.0.0',
  );
});

test('sorts plugin pages by manifest order and invalidates changed frontend resources', () => {
  const earlier = { ...readyPlugin, id: 'earlier', page: { id: 'control', order: 100 } };
  assert.deepEqual(sortVisiblePluginPages([readyPlugin, earlier]).map((plugin) => plugin.id), ['earlier', 'fake-plugin']);
  assert.notEqual(pluginFrontendFingerprint(readyPlugin), pluginFrontendFingerprint({ ...readyPlugin, version: '1.0.1' }));
  assert.notEqual(
    pluginFrontendFingerprint(readyPlugin),
    pluginFrontendFingerprint({ ...readyPlugin, page: { ...readyPlugin.page, iconAsset: 'ui/assets/new.png' } }),
  );
});
