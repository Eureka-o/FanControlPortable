import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import test from 'node:test';

const control = readFileSync(new URL('../src/app/components/ControlPanel.tsx', import.meta.url), 'utf8');
const pluginsSection = readFileSync(new URL('../src/app/components/settings/PluginsSection.tsx', import.meta.url), 'utf8');
const marketplacePanel = readFileSync(new URL('../src/app/components/settings/MarketplacePanel.tsx', import.meta.url), 'utf8');
const systemSection = readFileSync(new URL('../src/app/components/settings/SystemSettingsSection.tsx', import.meta.url), 'utf8');
const themePanel = readFileSync(new URL('../src/app/components/settings/ThemeManagementPanel.tsx', import.meta.url), 'utf8');
const actionMenu = readFileSync(new URL('../src/app/components/settings/ActionMenu.tsx', import.meta.url), 'utf8');
const dialogPrimitive = readFileSync(new URL('../src/components/ui/dialog.tsx', import.meta.url), 'utf8');
const appShell = readFileSync(new URL('../src/app/components/AppShell.tsx', import.meta.url), 'utf8');
const homePage = readFileSync(new URL('../src/app/page.tsx', import.meta.url), 'utf8');
const pageHost = readFileSync(new URL('../src/app/components/PluginPageHost.tsx', import.meta.url), 'utf8');
const pluginHost = readFileSync(new URL('../src/app/lib/plugin-host.ts', import.meta.url), 'utf8');

test('adds a fourth Plugins settings tab with matching 4-column pill math', () => {
  assert.match(control, /type SettingsTab = 'device' \| 'fan' \| 'system' \| 'plugins'/);
  assert.match(control, /\{ id: 'plugins', label: t\('controlPanel\.plugins\.sectionTitle'\) \}/);
  assert.match(control, /className="grid grid-cols-4 gap-1 rounded-\[18px\]/);
  assert.match(control, /w-\[calc\(\(100%-1\.5rem\)\/4\)\]/);
  assert.match(control, /<PluginsSection/);
});

test('removes legacy built-in RTSS fallback and focuses Plugins modal on plugins', () => {
  assert.equal(
    existsSync(new URL('../src/app/components/settings/RTSSPluginCard.tsx', import.meta.url)),
    false,
    'RTSSPluginCard.tsx should be deleted once rtss-osd is marketplace-only',
  );
  assert.doesNotMatch(pluginsSection, /RTSSPluginCard/);
  assert.match(pluginsSection, /apiService\.getPlugins\(\)/);
  assert.match(pluginsSection, /apiService\.refreshPlugins\(\)/);
  assert.match(pluginsSection, /apiService\.openPluginsFolder\(\)/);
  assert.match(pluginsSection, /apiService\.setPluginEnabled\(/);
  assert.match(pluginsSection, /<ActionMenu/);
  assert.match(pluginsSection, /browseMarketButton/);
  assert.match(pluginsSection, /showTechMeta\s*=\s*showDevBox\s*\|\|\s*Boolean\(plugin\.devMode\)/);
  assert.match(pluginsSection, /<Dialog open=\{marketOpen\}/);
  assert.match(pluginsSection, /mode="plugins"/);
  assert.match(pluginsSection, /lenovo-legion-fnq-power-mode/);
});

test('embeds Theme Store and update pulling into unified ThemeManagementPanel dialog', () => {
  assert.match(systemSection, /import ThemeManagementPanel from '\.\/ThemeManagementPanel'/);
  assert.match(systemSection, /<ThemeManagementPanel/);
  assert.match(themePanel, /apiService\.listThemes\(\)/);
  assert.match(themePanel, /apiService\.fetchMarketplaceCatalog\(/);
  assert.match(themePanel, /apiService\.installMarketplaceTheme\(/);
  assert.match(themePanel, /apiService\.uninstallTheme\(/);
  assert.match(themePanel, /apiService\.openThemesFolder\(\)/);
  assert.match(themePanel, /manageButton/);
  assert.match(themePanel, /managerTab === 'installed'/);
  assert.match(themePanel, /managerTab === 'download'/);
  assert.match(themePanel, /<MarketplacePanel[\s\S]*mode="themes"/);
  assert.match(themePanel, /!isBuiltin/);
  assert.match(marketplacePanel, /<ActionMenu/);
  assert.match(marketplacePanel, /updateAllThemes/);
  assert.match(marketplacePanel, /item\.installedBuiltin/);
  assert.match(marketplacePanel, /!isBuiltinTheme/);
});

test('provides complete FanControl-branded Plugins, Themes, and Store copy across all locales', () => {
  for (const locale of ['zh-CN', 'en-US', 'ja-JP']) {
    const messages = JSON.parse(readFileSync(new URL(`../src/app/locales/${locale}/translation.json`, import.meta.url), 'utf8'));
    const pluginsCopy = messages.controlPanel?.plugins;
    const themeCopy = messages.controlPanel?.system?.themeManagement;
    assert.ok(pluginsCopy?.sectionTitle, `${locale} missing plugins.sectionTitle`);
    assert.ok(pluginsCopy?.menuButton, `${locale} missing plugins.menuButton`);
    assert.ok(pluginsCopy?.menuGroups?.store, `${locale} missing plugins.menuGroups.store`);
    assert.ok(pluginsCopy?.browseMarketButton, `${locale} missing plugins.browseMarketButton`);
    assert.ok(pluginsCopy?.marketplace?.pluginDialogTitle, `${locale} missing marketplace.pluginDialogTitle`);
    assert.ok(pluginsCopy?.marketplace?.themeDialogTitle, `${locale} missing marketplace.themeDialogTitle`);
    assert.ok(pluginsCopy?.marketplace?.updateAllThemes, `${locale} missing marketplace.updateAllThemes`);
    assert.ok(pluginsCopy?.marketplace?.openRepository, `${locale} missing marketplace.openRepository`);
    assert.ok(pluginsCopy?.marketplace?.resetSource, `${locale} missing marketplace.resetSource`);
    assert.ok(pluginsCopy?.actions?.openPage, `${locale} missing actions.openPage`);
    assert.ok(pluginsCopy?.actions?.manage, `${locale} missing actions.manage`);
    assert.ok(pluginsCopy?.filters?.sidecar, `${locale} missing filters.sidecar`);
    assert.ok(pluginsCopy?.host?.reconnecting, `${locale} missing host.reconnecting`);
    assert.ok(themeCopy?.manageButton, `${locale} missing themeManagement.manageButton`);
    assert.ok(themeCopy?.dialogTitle, `${locale} missing themeManagement.dialogTitle`);
    assert.ok(themeCopy?.tabs?.installed, `${locale} missing themeManagement.tabs.installed`);
    assert.ok(themeCopy?.tabs?.download, `${locale} missing themeManagement.tabs.download`);
    assert.ok(themeCopy?.openFolderShort, `${locale} missing themeManagement.openFolderShort`);
    assert.ok(themeCopy?.checkUpdatesButton, `${locale} missing themeManagement.checkUpdatesButton`);
    assert.ok(themeCopy?.deleteButton, `${locale} missing themeManagement.deleteButton`);
    assert.ok(themeCopy?.confirmDeleteButton, `${locale} missing themeManagement.confirmDeleteButton`);
    assert.doesNotMatch(JSON.stringify(pluginsCopy), /\bTHRM\b/, `${locale} plugins copy must use FanControl branding`);
    assert.doesNotMatch(
      JSON.stringify(pluginsCopy?.marketplace ?? {}),
      /jsDelivr|国内镜像|GitHub 源站|China mirrors|Eureka-o\/FanControl-Plugins/,
      `${locale} marketplace copy must not mention download mirrors or repo origin`,
    );
  }
  assert.doesNotMatch(themePanel, /t\('controlPanel\.plugins\.actions\.uninstall'\)/);
});

test('renders ActionMenu in a viewport-safe portal, animates plugin settings, and exposes Open Repository', () => {
  assert.match(actionMenu, /createPortal/);
  assert.match(actionMenu, /data-action-menu-portal="true"/);
  assert.match(actionMenu, /getBoundingClientRect/);
  assert.match(actionMenu, /AnimatePresence/);

  assert.match(dialogPrimitive, /\[data-action-menu-portal="true"\]/);
  assert.match(dialogPrimitive, /onPointerDownOutside/);
  assert.match(dialogPrimitive, /onFocusOutside/);
  assert.match(dialogPrimitive, /onInteractOutside/);

  assert.match(pluginsSection, /<AnimatePresence[\s\S]*?<m\.div[\s\S]*?isSettingsExpanded/);
  assert.match(pluginsSection, /plugin\.uiSlot === 'settings-panel'/);
  assert.match(pluginsSection, /openRepository/);
  assert.match(pluginsSection, /BRAND\.pluginsRepositoryUrl/);

  assert.match(marketplacePanel, /openRepository/);
  assert.match(marketplacePanel, /BRAND\.pluginsRepositoryUrl/);
  assert.doesNotMatch(marketplacePanel, /\brouteBadge\b/);
  assert.doesNotMatch(marketplacePanel, /resolvedSource/);
  assert.doesNotMatch(marketplacePanel, /latencyMs/);

  assert.match(themePanel, /openRepository/);
  assert.match(themePanel, /BRAND\.pluginsRepositoryUrl/);
});

test('supports official marketplace dialog, dual runtimes, and dynamic Dock pages without eval', () => {
  assert.match(pluginsSection, /apiService\.restartCore\(false\)/);
  assert.match(pluginsSection, /apiService\.importMarketplacePackage/);
  assert.match(pluginsSection, /setActiveTab\(`plugin:\$\{plugin\.id\}`\)/);
  assert.match(pluginsSection, /onRequestUninstall/);
  assert.match(pluginsSection, /onManageInstalled/);
  assert.match(pluginsSection, /<MarketplacePanel/);
  assert.match(pluginsSection, /<PluginDeclarativeSettings/);
  assert.match(pageHost, /plugin\.devMode && plugin\.version/);
  assert.match(pageHost, /<ActionMenu/);

  assert.match(marketplacePanel, /embedded\?: boolean/);
  assert.match(marketplacePanel, /apiService\.fetchMarketplaceCatalog\(/);
  assert.match(marketplacePanel, /apiService\.installMarketplacePlugin\(/);
  assert.match(marketplacePanel, /apiService\.installMarketplaceTheme\(/);
  assert.match(marketplacePanel, /apiService\.importMarketplacePackage\(/);
  assert.match(marketplacePanel, /fancontrol:themes-updated/);

  assert.match(appShell, /--sidebar-active-offset/);
  assert.match(appShell, /pluginTabs\.map\(/);
  assert.match(homePage, /<PluginPageHost/);
  assert.match(homePage, /isSidebarPluginVisible/);

  assert.doesNotMatch(pluginHost, /\beval\s*\(|new\s+Function\s*\(/);
  assert.match(pluginHost, /FanControlPluginHost/);
  assert.match(pluginHost, /createScopedPluginHost/);
  assert.match(pluginHost, /ui:\s*FanControlUI/);
  assert.match(pluginHost, /icons:\s*PLUGIN_HOST_ICONS/);
});
