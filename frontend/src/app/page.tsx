'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import dynamic from 'next/dynamic';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { AlertTriangle } from 'lucide-react';
import { types } from '../../wailsjs/go/models';
import { useShallow } from 'zustand/react/shallow';
import AppFatalError from './components/AppFatalError';
import AppLoadingSkeleton from './components/AppLoadingSkeleton';
import AppShell from './components/AppShell';
import DeviceStatus from './components/DeviceStatus';
import FanCurve from './components/FanCurve';
import PluginPageHost from './components/PluginPageHost';
import { buildPluginAssetURL, isSidebarPluginVisible } from './lib/plugin-host';
import { useAppBootstrap } from './hooks/useAppBootstrap';
import { apiService } from './services/api';
import { useAppStore } from './store/app-store';
import type { PluginInfo } from './types/app';
import { Button, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from './components/ui';

const loadControlPanel = () => import('./components/ControlPanel');
const loadAdvancedDevicesPanel = () => import('./components/AdvancedDevicesPanel');
const loadAboutPanel = () => import('./components/AboutPanel');
const ControlPanel = dynamic(loadControlPanel, { ssr: false, loading: () => null });
const AdvancedDevicesPanel = dynamic(loadAdvancedDevicesPanel, { ssr: false, loading: () => null });
const AboutPanel = dynamic(loadAboutPanel, { ssr: false, loading: () => null });

const prefetchedTabs = new Set<string>();

function prefetchTab(tab: string) {
  if (prefetchedTabs.has(tab)) return;
  prefetchedTabs.add(tab);
  if (tab === 'curve') {
    void import('./components/fan-curve-charts').catch(() => {
      prefetchedTabs.delete(tab);
    });
  } else if (tab === 'control') {
    void loadControlPanel().catch(() => {
      prefetchedTabs.delete(tab);
    });
  } else if (tab === 'devices') {
    void loadAdvancedDevicesPanel().catch(() => {
      prefetchedTabs.delete(tab);
    });
  } else if (tab === 'about') {
    void loadAboutPanel().catch(() => {
      prefetchedTabs.delete(tab);
    });
  }
}

function getErrorMessage(error: unknown) {
  if (error instanceof Error) return error.message;
  if (typeof error === 'string') return error;
  return String(error ?? 'Unknown error');
}

// AppShell / ControlPanel / DeviceStatus / FanCurve 根部不直接订阅 tick 级遥测：
// 各自真正跳数字或绘点的叶子组件通过精确 selector 订阅 store，避免整页随遥测节拍重渲。
function LiveHome() {
  useAppBootstrap();
  const { t } = useTranslation();
  const [diagnosticsExporting, setDiagnosticsExporting] = useState(false);
  const [monitorOnlyDialogOpen, setMonitorOnlyDialogOpen] = useState(false);
  const [monitorOnlyRestarting, setMonitorOnlyRestarting] = useState(false);
  const [plugins, setPlugins] = useState<PluginInfo[]>([]);

  const view = useAppStore(
    useShallow((state) => ({
      isConnected: state.isConnected,
      deviceRuntimeState: state.deviceRuntimeState,
      deviceProductId: state.deviceProductId,
      deviceModel: state.deviceModel,
      deviceSettings: state.deviceSettings,
      runtimeDeviceProfile: state.runtimeDeviceProfile,
      runtimeDeviceCapabilities: state.runtimeDeviceCapabilities,
      config: state.config,
      bridgeWarning: state.bridgeWarning,
      coreServiceError: state.coreServiceError,
      isLoading: state.isLoading,
      error: state.error,
      activeTab: state.activeTab,
      curveFocusTarget: state.curveFocusTarget,
      monitorOnlyActive: state.monitorOnlyActive,
    })),
  );

  const initializeApp = useAppStore((state) => state.initializeApp);
  const connectDevice = useAppStore((state) => state.connectDevice);
  const disconnectDevice = useAppStore((state) => state.disconnectDevice);
  const setConfig = useAppStore((state) => state.setConfig);
  const refreshDeviceContext = useAppStore((state) => state.refreshDeviceContext);
  const setActiveTab = useAppStore((state) => state.setActiveTab);
  const openCurveTab = useAppStore((state) => state.openCurveTab);
  const clearCurveFocusTarget = useAppStore((state) => state.clearCurveFocusTarget);
  const clearBridgeWarning = useAppStore((state) => state.clearBridgeWarning);

  useEffect(() => {
    let active = true;
    void apiService.getPlugins()
      .then((list) => {
        if (active) setPlugins(list);
      })
      .catch(() => {});
    const unsubscribe = apiService.onPluginsUpdate((list) => {
      if (active && Array.isArray(list)) {
        setPlugins(list);
      }
    });
    return () => {
      active = false;
      unsubscribe();
    };
  }, []);

  const sidebarPlugins = useMemo(
    () => plugins.filter((item) => isSidebarPluginVisible(item)),
    [plugins],
  );
  const sidebarPluginTabs = useMemo(
    () =>
      sidebarPlugins.map((item) => {
        const hasAssetIcon = Boolean(
          item.icon && /\.(png|svg|webp|jpe?g)$/i.test(item.icon.trim()),
        );
        return {
          id: item.id,
          label: item.name,
          icon: item.icon,
          iconAssetUrl: hasAssetIcon
            ? buildPluginAssetURL(item.id, item.icon!, item.version)
            : undefined,
        };
      }),
    [sidebarPlugins],
  );

  useEffect(() => {
    if (!view.activeTab.startsWith('plugin:')) return;
    const targetId = view.activeTab.slice('plugin:'.length);
    if (!sidebarPlugins.some((p) => p.id === targetId)) {
      setActiveTab('status');
    }
  }, [setActiveTab, sidebarPlugins, view.activeTab]);

  const renderPluginContent = useCallback(
    (pluginId: string) => {
      const target = sidebarPlugins.find((p) => p.id === pluginId);
      return target ? <PluginPageHost plugin={target} /> : null;
    },
    [sidebarPlugins],
  );

  const enableMonitorOnlyForSession = useCallback(async () => {
    if (monitorOnlyRestarting) return;
    setMonitorOnlyRestarting(true);
    try {
      await apiService.restartCore(true);
    } catch (error) {
      toast.error(t('deviceStatus.monitorOnly.restartFailed', { error: getErrorMessage(error) }));
      setMonitorOnlyRestarting(false);
    }
  }, [monitorOnlyRestarting, t]);

  const safeConfig = useMemo(
    () => view.config || new types.AppConfig(),
    [view.config],
  );

  const exportDiagnostics = useCallback(async () => {
    if (diagnosticsExporting) return;
    setDiagnosticsExporting(true);
    try {
      const path = await apiService.exportDiagnosticsToFile();
      if (path) {
        toast.success(t('appShell.diagnostics.exportSuccess'), { description: path });
      }
    } catch (error) {
      toast.error(t('appShell.diagnostics.exportFailed', { error: getErrorMessage(error) }));
    } finally {
      setDiagnosticsExporting(false);
    }
  }, [diagnosticsExporting, t]);

  const curveContent = (
    <FanCurve
      config={safeConfig}
      onConfigChange={setConfig}
      isConnected={view.isConnected}
      runtimeDeviceProfile={view.runtimeDeviceProfile}
      runtimeDeviceCapabilities={view.runtimeDeviceCapabilities}
      deviceModel={view.deviceModel}
      focusTarget={view.curveFocusTarget}
      onFocusHandled={clearCurveFocusTarget}
    />
  );

  const statusContent = (
    <DeviceStatus
      isConnected={view.isConnected}
      runtimeState={view.deviceRuntimeState}
      deviceProductId={view.deviceProductId}
      deviceModel={view.deviceModel}
      deviceSettings={view.deviceSettings}
      runtimeDeviceProfile={view.runtimeDeviceProfile}
      config={safeConfig}
      coreServiceError={view.coreServiceError}
      monitorOnlyActive={view.monitorOnlyActive}
      onConnect={connectDevice}
      onDisconnect={disconnectDevice}
      onConfigChange={setConfig}
      onOpenCurveEditor={() => setActiveTab('curve')}
      onOpenHistoryDetails={() => openCurveTab('history-details')}
      diagnosticsExporting={diagnosticsExporting}
      onExportDiagnostics={exportDiagnostics}
      onEnableMonitorOnly={() => setMonitorOnlyDialogOpen(true)}
    />
  );

  const effectiveActiveTab = view.monitorOnlyActive && view.activeTab === 'curve'
    ? 'status'
    : view.activeTab;

  if (view.isLoading) {
    return <AppLoadingSkeleton />;
  }

  if (view.error && !view.config) {
    return <AppFatalError message={view.error} onRetry={initializeApp} />;
  }

  return (
    <>
      <AppShell
        activeTab={effectiveActiveTab}
        onTabChange={setActiveTab}
        onTabPrefetch={prefetchTab}
        isConnected={view.isConnected}
        monitorOnlyActive={view.monitorOnlyActive}
        runtimeDeviceProfile={view.runtimeDeviceProfile}
        config={safeConfig}
        autoControl={safeConfig.autoControl}
        error={view.error}
        bridgeWarning={view.bridgeWarning}
        diagnosticsExporting={diagnosticsExporting}
        onExportDiagnostics={exportDiagnostics}
        onDismissBridgeWarning={clearBridgeWarning}
        statusContent={statusContent}
        curveContent={curveContent}
        controlContent={
          <ControlPanel
            config={safeConfig}
            onConfigChange={setConfig}
            isConnected={view.isConnected}
            monitorOnlyActive={view.monitorOnlyActive}
            runtimeDeviceProfile={view.runtimeDeviceProfile}
            runtimeDeviceCapabilities={view.runtimeDeviceCapabilities}
            onDeviceContextRefresh={refreshDeviceContext}
          />
        }
        devicesContent={
          <AdvancedDevicesPanel
            config={safeConfig}
            isConnected={view.isConnected}
            onConfigChange={setConfig}
          />
        }
        aboutContent={<AboutPanel />}
        pluginTabs={sidebarPluginTabs}
        renderPluginContent={renderPluginContent}
      />
      <Dialog open={monitorOnlyDialogOpen} onOpenChange={(open) => !monitorOnlyRestarting && setMonitorOnlyDialogOpen(open)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-amber-600" />
              {t('deviceStatus.monitorOnly.dialogTitle')}
            </DialogTitle>
            <DialogDescription>{t('deviceStatus.monitorOnly.dialogDescription')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-2 rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-3 text-sm leading-relaxed text-amber-800 dark:text-amber-200">
            <p>{t('deviceStatus.monitorOnly.dialogTemporary')}</p>
            <p>{t('deviceStatus.monitorOnly.dialogPersistentHint')}</p>
          </div>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setMonitorOnlyDialogOpen(false)} disabled={monitorOnlyRestarting}>
              {t('deviceStatus.monitorOnly.cancel')}
            </Button>
            <Button onClick={() => void enableMonitorOnlyForSession()} loading={monitorOnlyRestarting}>
              {t('deviceStatus.monitorOnly.confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

export default function Home() {
  return <LiveHome />;
}
