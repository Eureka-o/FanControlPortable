import type { ComponentType } from 'react';
import type { LucideIcon } from 'lucide-react';

export type PluginIconName = 'fan' | 'gauge' | 'laptop' | 'plug' | 'settings' | 'thermometer' | 'zap';

export interface PluginThemeSnapshot {
  mode: string;
  layer: string;
  dark: boolean;
}

export interface PluginThemeAPI {
  current: () => PluginThemeSnapshot;
  subscribe: (callback: (theme: PluginThemeSnapshot) => void) => () => void;
}

export interface PluginLocaleAPI {
  current: () => string;
  t: (key: string, options?: Record<string, unknown>) => string;
  subscribe: (callback: (locale: string) => void) => () => void;
}

export interface PluginToastAPI {
  success: (message: string, description?: string) => void;
  error: (message: string, description?: string) => void;
  info: (message: string, description?: string) => void;
}

export interface PluginScopedHost {
  pluginId: string;
  invoke: <T = unknown>(method: string, payload?: unknown) => Promise<T>;
  subscribe: (event: string, callback: (payload: unknown) => void) => () => void;
  ui: typeof import('../components/ui');
  icons: Readonly<Record<PluginIconName, LucideIcon>>;
  theme: PluginThemeAPI;
  locale: PluginLocaleAPI;
  toast: PluginToastAPI;
}

export interface PluginPageProps {
  host: PluginScopedHost;
}

export interface PluginPageRegistration {
  id: string;
  component: ComponentType<PluginPageProps>;
  dispose?: () => void;
}

export interface RegisteredPluginPage extends PluginPageRegistration {
  pluginId: string;
}

export interface FanControlPluginHostV1 {
  version: 1;
  React: typeof import('react');
  registerPage: (registration: PluginPageRegistration) => void;
  invoke: <T = unknown>(pluginId: string, method: string, payload?: unknown) => Promise<T>;
  subscribe: (pluginId: string, event: string, callback: (payload: unknown) => void) => () => void;
  ui: typeof import('../components/ui');
  icons: Readonly<Record<PluginIconName, LucideIcon>>;
  theme: {
    current: () => PluginThemeSnapshot;
    subscribe: (pluginId: string, callback: (theme: PluginThemeSnapshot) => void) => () => void;
  };
  locale: {
    current: () => string;
    t: (key: string, options?: Record<string, unknown>) => string;
    subscribe: (pluginId: string, callback: (locale: string) => void) => () => void;
  };
  toast: PluginToastAPI;
}

declare global {
  interface Window {
    FanControlPluginHost?: FanControlPluginHostV1;
  }
}
