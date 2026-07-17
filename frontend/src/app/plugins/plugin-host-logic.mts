import type { PluginAppTab } from '../store/app-store-logic.mts';

interface PluginPageLike {
  id?: string;
  iconAsset?: string;
  order?: number;
}

interface PluginEntryLike {
  id?: string;
  name?: string;
  version?: string;
  enabled?: boolean;
  state?: string;
  frontend?: string;
  style?: string;
  page?: PluginPageLike;
}

export function isVisiblePluginPage(plugin: PluginEntryLike): boolean {
  return plugin.enabled === true &&
    plugin.state === 'ready' &&
    Boolean(plugin.id?.trim()) &&
    Boolean(plugin.frontend?.trim()) &&
    Boolean(plugin.page?.id?.trim());
}

export function pluginPageTabId(plugin: PluginEntryLike): PluginAppTab {
  return `plugin:${plugin.id || ''}:${plugin.page?.id || ''}`;
}

export function pluginAssetURL(pluginId: string, relativePath: string, version?: string): string {
  const encodedPath = relativePath
    .replaceAll('\\', '/')
    .split('/')
    .filter(Boolean)
    .map((part) => encodeURIComponent(part))
    .join('/');
  const versionQuery = version?.trim() ? `?v=${encodeURIComponent(version.trim())}` : '';
  return `/plugin-assets/${encodeURIComponent(pluginId)}/${encodedPath}${versionQuery}`;
}

export function pluginFrontendFingerprint(plugin: PluginEntryLike): string {
  return [plugin.id, plugin.version, plugin.frontend, plugin.style, plugin.page?.id, plugin.page?.iconAsset]
    .map((value) => value || '')
    .join('|');
}

export function sortVisiblePluginPages<T extends PluginEntryLike>(plugins: T[]): T[] {
  return plugins
    .filter(isVisiblePluginPage)
    .slice()
    .sort((left, right) => {
      const orderDifference = Number(left.page?.order || 0) - Number(right.page?.order || 0);
      if (orderDifference !== 0) return orderDifference;
      return String(left.name || left.id || '').localeCompare(String(right.name || right.id || ''));
    });
}
