'use client';

import { useEffect, useMemo, useState } from 'react';
import { AlertTriangle, Loader2 } from 'lucide-react';
import { apiService } from '../services/api';
import type { PluginInfo } from '../types/app';
import {
  getPluginPageRegistration,
  installPluginHost,
  usePluginPageRegistration,
} from '../plugins/plugin-host';

interface PluginPageProps {
  plugin: PluginInfo;
}

const loadedPluginScripts = new Map<string, Promise<void>>();

function safeSourceName(pluginID: string) {
  return pluginID.replace(/[^a-z0-9_-]/gi, '_');
}

async function evaluatePluginScript(plugin: PluginInfo, script: string) {
  const key = `${plugin.id}:${plugin.version || ''}:${plugin.frontend || ''}`;
  const existing = loadedPluginScripts.get(key);
  if (existing) {
    return existing;
  }

  const pending = Promise.resolve().then(() => {
    const sourceURL = `fancontrol-plugin-${safeSourceName(plugin.id)}.js`;
    const run = new Function(`${script}\n//# sourceURL=${sourceURL}`);
    return run();
  });
  loadedPluginScripts.set(key, pending);
  return pending;
}

export default function PluginPage({ plugin }: PluginPageProps) {
  const registration = usePluginPageRegistration(plugin.id);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const frontendPath = String(plugin.frontend || '');
  const isInjectionScript = frontendPath.toLowerCase().endsWith('.js');
  const host = useMemo(() => installPluginHost(), []);

  useEffect(() => {
    let disposed = false;
    setError(null);

    if (!plugin.frontend) {
      setError('插件没有配置前端入口。');
      return () => {
        disposed = true;
      };
    }

    if (!isInjectionScript) {
      setError('当前插件前端入口不是 JS 注入文件，请更新插件安装包。');
      return () => {
        disposed = true;
      };
    }

    setLoading(!getPluginPageRegistration(plugin.id));

    apiService.getPluginFrontendAsset(plugin.id)
      .then(async (script) => {
        if (disposed) return;
        if (!script.trim()) {
          throw new Error('插件前端资源为空。');
        }
        await evaluatePluginScript(plugin, script);
        if (!disposed && !getPluginPageRegistration(plugin.id)) {
          throw new Error('插件前端已加载，但没有注册页面。');
        }
      })
      .catch((reason: unknown) => {
        if (disposed) return;
        const message = reason instanceof Error ? reason.message : String(reason);
        setError(message || '插件前端加载失败。');
      })
      .finally(() => {
        if (!disposed) {
          setLoading(false);
        }
      });

    return () => {
      disposed = true;
    };
  }, [plugin, isInjectionScript]);

  if (error) {
    return (
      <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
        <div className="flex items-center gap-2 font-semibold">
          <AlertTriangle className="h-4 w-4" />
          插件页面加载失败
        </div>
        <p className="mt-2 text-destructive/85">{error}</p>
      </div>
    );
  }

  if (loading || !registration || !host) {
    return (
      <div className="flex min-h-[360px] items-center justify-center rounded-lg border border-border bg-card text-muted-foreground">
        <div className="flex items-center gap-2 text-sm">
          <Loader2 className="h-4 w-4 animate-spin" />
          正在加载插件页面...
        </div>
      </div>
    );
  }

  const PageComponent = registration.component;
  return (
    <div data-plugin-page={plugin.id} className="min-w-0">
      <PageComponent plugin={plugin} host={host} />
    </div>
  );
}
