'use client';

import React, { useSyncExternalStore, type ComponentType } from 'react';
import * as Icons from 'lucide-react';
import { toast } from 'sonner';
import * as UI from '../components/ui';
import { apiService } from '../services/api';
import { useAppStore } from '../store/app-store';
import type { PluginInfo } from '../types/app';

export interface PluginPageProps {
  plugin: PluginInfo;
  host: FanControlPluginHost;
}

export interface PluginPageRegistration {
  id?: string;
  title?: string;
  component?: ComponentType<PluginPageProps>;
  Component?: ComponentType<PluginPageProps>;
}

export interface FanControlPluginHost {
  version: 1;
  React: typeof React;
  components: typeof UI;
  icons: typeof Icons;
  apiService: typeof apiService;
  useAppStore: typeof useAppStore;
  toast: typeof toast;
  registerPage: (pluginID: string, registration: PluginPageRegistration) => void;
  getPage: (pluginID: string) => NormalizedPluginPageRegistration | undefined;
  subscribe: (listener: () => void) => () => void;
}

export interface NormalizedPluginPageRegistration {
  id: string;
  title?: string;
  component: ComponentType<PluginPageProps>;
}

declare global {
  interface Window {
    FanControlPluginHost?: FanControlPluginHost;
  }
}

const pageRegistrations = new Map<string, NormalizedPluginPageRegistration>();
const listeners = new Set<() => void>();

function emitPluginHostChange() {
  listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function getPluginPageRegistration(pluginID: string) {
  return pageRegistrations.get(pluginID);
}

export function usePluginPageRegistration(pluginID: string) {
  return useSyncExternalStore(
    subscribe,
    () => getPluginPageRegistration(pluginID),
    () => undefined,
  );
}

export function installPluginHost() {
  if (typeof window === 'undefined') {
    return undefined;
  }

  const host: FanControlPluginHost = {
    version: 1,
    React,
    components: UI,
    icons: Icons,
    apiService,
    useAppStore,
    toast,
    registerPage(pluginID, registration) {
      const id = String(pluginID || '').trim();
      const component = registration?.component || registration?.Component;
      if (!id) {
        throw new Error('plugin id is required');
      }
      if (typeof component !== 'function') {
        throw new Error(`plugin ${id} did not provide a React page component`);
      }

      pageRegistrations.set(id, {
        id,
        title: registration.title,
        component,
      });
      emitPluginHostChange();
    },
    getPage: getPluginPageRegistration,
    subscribe,
  };

  window.FanControlPluginHost = host;
  return host;
}
