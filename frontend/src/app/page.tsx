'use client';

import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { types } from '../../wailsjs/go/models';
import { useShallow } from 'zustand/react/shallow';
import AppFatalError from './components/AppFatalError';
import AppLoadingSkeleton from './components/AppLoadingSkeleton';
import AboutPanel from './components/AboutPanel';
import AdvancedDevicesPanel from './components/AdvancedDevicesPanel';
import AppShell from './components/AppShell';
import ControlPanel from './components/ControlPanel';
import DeviceStatus from './components/DeviceStatus';
import FanCurve from './components/FanCurve';
import PluginPage from './components/PluginPage';
import { useAppBootstrap } from './hooks/useAppBootstrap';
import { apiService } from './services/api';
import { useAppStore } from './store/app-store';

function getErrorMessage(error: unknown) {
  if (error instanceof Error) return error.message;
  if (typeof error === 'string') return error;
  return String(error ?? 'Unknown error');
}

export default function Home() {
  useAppBootstrap();
  const { t } = useTranslation();
  const [diagnosticsExporting, setDiagnosticsExporting] = useState(false);

  const view = useAppStore(
    useShallow((state) => ({
      isConnected: state.isConnected,
      deviceProductId: state.deviceProductId,
      deviceModel: state.deviceModel,
      deviceSettings: state.deviceSettings,
      runtimeDeviceProfile: state.runtimeDeviceProfile,
      runtimeDeviceCapabilities: state.runtimeDeviceCapabilities,
      config: state.config,
      fanData: state.fanData,
      temperature: state.temperature,
      bridgeWarning: state.bridgeWarning,
      coreServiceError: state.coreServiceError,
      isLoading: state.isLoading,
	      error: state.error,
	      activeTab: state.activeTab,
	      curveFocusTarget: state.curveFocusTarget,
	      availablePlugins: state.availablePlugins,
	    })),
	  );

  const initializeApp = useAppStore((state) => state.initializeApp);
  const connectDevice = useAppStore((state) => state.connectDevice);
  const disconnectDevice = useAppStore((state) => state.disconnectDevice);
  const updateConfig = useAppStore((state) => state.updateConfig);
  const refreshDeviceContext = useAppStore((state) => state.refreshDeviceContext);
  const setActiveTab = useAppStore((state) => state.setActiveTab);
  const openCurveTab = useAppStore((state) => state.openCurveTab);
  const clearCurveFocusTarget = useAppStore((state) => state.clearCurveFocusTarget);
  const clearBridgeWarning = useAppStore((state) => state.clearBridgeWarning);

  const safeConfig = useMemo(
    () => view.config || new types.AppConfig(),
    [view.config],
  );
  const pluginTabs = useMemo(() => (
    view.availablePlugins
      .filter((plugin) => plugin.installed && plugin.frontend)
		      .map((plugin) => ({
		        id: `plugin:${plugin.id}` as const,
		        title: plugin.name,
		        icon: plugin.icon,
		        content: <PluginPage plugin={plugin} />,
		      }))
  ), [view.availablePlugins]);

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

  if (view.isLoading) {
    return <AppLoadingSkeleton />;
  }

  if (view.error && !view.config) {
    return <AppFatalError message={view.error} onRetry={initializeApp} />;
  }

  return (
    <AppShell
      activeTab={view.activeTab}
      onTabChange={setActiveTab}
      isConnected={view.isConnected}
      fanData={view.fanData}
      temperature={view.temperature}
      runtimeDeviceProfile={view.runtimeDeviceProfile}
      config={safeConfig}
      autoControl={safeConfig.autoControl}
      error={view.error}
      bridgeWarning={view.bridgeWarning}
      diagnosticsExporting={diagnosticsExporting}
      onExportDiagnostics={exportDiagnostics}
      onDismissBridgeWarning={clearBridgeWarning}
      statusContent={
        <DeviceStatus
          isConnected={view.isConnected}
          deviceProductId={view.deviceProductId}
          deviceModel={view.deviceModel}
          deviceSettings={view.deviceSettings}
          fanData={view.fanData}
          temperature={view.temperature}
          runtimeDeviceProfile={view.runtimeDeviceProfile}
          config={safeConfig}
          coreServiceError={view.coreServiceError}
          onConnect={connectDevice}
          onDisconnect={disconnectDevice}
          onConfigChange={updateConfig}
          onOpenCurveEditor={() => openCurveTab('curve-editor')}
          onOpenHistoryDetails={() => openCurveTab('history-details')}
          diagnosticsExporting={diagnosticsExporting}
          onExportDiagnostics={exportDiagnostics}
        />
      }
      curveContent={
        <FanCurve
          config={safeConfig}
          onConfigChange={updateConfig}
          isConnected={view.isConnected}
          fanData={view.fanData}
          temperature={view.temperature}
          runtimeDeviceProfile={view.runtimeDeviceProfile}
          runtimeDeviceCapabilities={view.runtimeDeviceCapabilities}
          deviceModel={view.deviceModel}
          focusTarget={view.curveFocusTarget}
          onFocusHandled={clearCurveFocusTarget}
        />
      }
      controlContent={
        <ControlPanel
          config={safeConfig}
          onConfigChange={updateConfig}
          isConnected={view.isConnected}
          fanData={view.fanData}
          temperature={view.temperature}
          runtimeDeviceProfile={view.runtimeDeviceProfile}
          runtimeDeviceCapabilities={view.runtimeDeviceCapabilities}
          onDeviceContextRefresh={refreshDeviceContext}
        />
      }
      devicesContent={
        <AdvancedDevicesPanel
          config={safeConfig}
          isConnected={view.isConnected}
          onConfigChange={updateConfig}
        />
      }
	      pluginTabs={pluginTabs}
	      aboutContent={<AboutPanel />}
    />
  );
}
