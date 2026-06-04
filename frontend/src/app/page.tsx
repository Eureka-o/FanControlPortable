'use client';

import { useMemo } from 'react';
import { types } from '../../wailsjs/go/models';
import { useShallow } from 'zustand/react/shallow';
import AppFatalError from './components/AppFatalError';
import AppLoadingSkeleton from './components/AppLoadingSkeleton';
import AboutPanel from './components/AboutPanel';
import AppShell from './components/AppShell';
import ControlPanel from './components/ControlPanel';
import DeviceStatus from './components/DeviceStatus';
import FanCurve from './components/FanCurve';
import { useAppBootstrap } from './hooks/useAppBootstrap';
import { useAppStore } from './store/app-store';

export default function Home() {
  useAppBootstrap();

  const view = useAppStore(
    useShallow((state) => ({
      isConnected: state.isConnected,
      deviceProductId: state.deviceProductId,
      deviceModel: state.deviceModel,
      deviceSettings: state.deviceSettings,
      config: state.config,
      fanData: state.fanData,
      temperature: state.temperature,
      legionFnQSupported: state.legionFnQSupported,
      bridgeWarning: state.bridgeWarning,
      coreServiceError: state.coreServiceError,
      isLoading: state.isLoading,
      error: state.error,
      activeTab: state.activeTab,
      curveFocusTarget: state.curveFocusTarget,
    })),
  );

  const initializeApp = useAppStore((state) => state.initializeApp);
  const connectDevice = useAppStore((state) => state.connectDevice);
  const disconnectDevice = useAppStore((state) => state.disconnectDevice);
  const updateConfig = useAppStore((state) => state.updateConfig);
  const setActiveTab = useAppStore((state) => state.setActiveTab);
  const openCurveTab = useAppStore((state) => state.openCurveTab);
  const clearCurveFocusTarget = useAppStore((state) => state.clearCurveFocusTarget);
  const clearBridgeWarning = useAppStore((state) => state.clearBridgeWarning);

  const safeConfig = useMemo(
    () => view.config || new types.AppConfig(),
    [view.config],
  );

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
      autoControl={safeConfig.autoControl}
      error={view.error}
      bridgeWarning={view.bridgeWarning}
      onDismissBridgeWarning={clearBridgeWarning}
      statusContent={
        <DeviceStatus
          isConnected={view.isConnected}
          deviceProductId={view.deviceProductId}
          deviceModel={view.deviceModel}
          deviceSettings={view.deviceSettings}
          fanData={view.fanData}
          temperature={view.temperature}
          config={safeConfig}
          coreServiceError={view.coreServiceError}
          onConnect={connectDevice}
          onDisconnect={disconnectDevice}
          onConfigChange={updateConfig}
          onOpenCurveEditor={() => openCurveTab('curve-editor')}
          onOpenHistoryDetails={() => openCurveTab('history-details')}
        />
      }
      curveContent={
        <FanCurve
          config={safeConfig}
          onConfigChange={updateConfig}
          isConnected={view.isConnected}
          fanData={view.fanData}
          temperature={view.temperature}
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
          legionFnQSupported={view.legionFnQSupported}
          deviceModel={view.deviceModel}
          deviceSettings={view.deviceSettings}
        />
      }
      aboutContent={<AboutPanel />}
    />
  );
}
