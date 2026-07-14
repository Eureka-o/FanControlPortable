import { create } from 'zustand';
import {
  LatestRequestGate,
  appendTimelineEvent,
  cancelPendingTabChange as cancelTabChange,
  completePendingTabChange as completeTabChange,
  requestTabChange,
  type AppTab,
  type TimelineEvent,
} from './app-store-logic.mts';
import { types } from '../../../wailsjs/go/models';
import { apiService } from '../services/api';
import {
  appendSampledHistoryPoint,
  createLiveHistoryPoint,
  SESSION_HISTORY_LIMIT,
  SESSION_HISTORY_RETENTION_MS,
} from '../lib/temperature-history';
import type { TemperatureHistoryPoint } from '../lib/temperature-history';
import { i18n } from '../lib/i18n';
import { toast } from 'sonner';
import type { DeviceSettings } from '../types/app';

interface DeviceStatusPayload {
  connected?: boolean;
  currentData?: types.FanData | null;
  deviceSettings?: DeviceSettings | null;
  deviceProfile?: types.DeviceProfile | null;
  deviceCapabilities?: types.DeviceCapabilities | null;
  temperature?: types.TemperatureData | null;
  productId?: string;
  model?: string;
  error?: string;
  runtime?: { state?: string };
}

const getBridgeWarningMessage = () => i18n.t('store.bridgeWarning.default');

const getCoreServiceErrorMessage = (detail?: string) => {
  const trimmed = detail?.trim();
  if (
    trimmed?.includes(i18n.t('store.coreService.unavailable')) ||
    trimmed?.startsWith('核心服务不可用') ||
    trimmed?.startsWith('Core service is unavailable') ||
    trimmed?.startsWith('Core サービスを利用できません')
  ) {
    return trimmed;
  }
  return trimmed
    ? i18n.t('store.coreService.unavailableWithDetail', { detail: trimmed })
    : i18n.t('store.coreService.unavailable');
};

const isCoreServiceFailureDetail = (detail?: string) => {
  const normalized = detail?.toLowerCase() ?? '';
  return normalized.includes('core') ||
    normalized.includes('核心服务') ||
    normalized.includes('ipc') ||
    normalized.includes('服务器') ||
    normalized.includes('服务');
};

const optionalNumber = (value?: number) => value ?? 0;

const sensorListEquals = (
  left?: Array<{ key?: string; name?: string; value?: number }>,
  right?: Array<{ key?: string; name?: string; value?: number }>,
) => {
  if (left === right) return true;
  if (!Array.isArray(left) || !Array.isArray(right)) return !left && !right;
  if (left.length !== right.length) return false;
  for (let index = 0; index < left.length; index += 1) {
    const leftItem = left[index];
    const rightItem = right[index];
    if (
      leftItem.key !== rightItem.key ||
      leftItem.name !== rightItem.name ||
      optionalNumber(leftItem.value) !== optionalNumber(rightItem.value)
    ) {
      return false;
    }
  }
  return true;
};

const gpuDeviceListEquals = (left?: types.TemperatureGPUDevice[], right?: types.TemperatureGPUDevice[]) => {
  if (left === right) return true;
  if (!Array.isArray(left) || !Array.isArray(right)) return !left && !right;
  if (left.length !== right.length) return false;
  for (let index = 0; index < left.length; index += 1) {
    const leftItem = left[index];
    const rightItem = right[index];
    if (
      leftItem.key !== rightItem.key ||
      leftItem.name !== rightItem.name ||
      leftItem.vendor !== rightItem.vendor ||
      !sensorListEquals(leftItem.sensors, rightItem.sensors) ||
      !sensorListEquals(leftItem.powerSensors, rightItem.powerSensors)
    ) {
      return false;
    }
  }
  return true;
};

const flyDigiCapabilityEquals = (left?: types.FlyDigiRuntimeCapability | null, right?: types.FlyDigiRuntimeCapability | null) => {
  if (left === right) return true;
  if (!left || !right) return false;
  return left.available === right.available &&
    left.gearSettings === right.gearSettings &&
    left.maxGearCode === right.maxGearCode &&
    left.maxGearLabel === right.maxGearLabel &&
    left.maxGearIndex === right.maxGearIndex &&
    left.maxRpm === right.maxRpm &&
    left.selectedGearCode === right.selectedGearCode &&
    left.selectedGear === right.selectedGear &&
    left.source === right.source &&
    left.reason === right.reason;
};

const fanDataEquals = (left: types.FanData | null, right: types.FanData | null) => {
  if (left === right) return true;
  if (!left || !right) return false;
  return left.reportId === right.reportId &&
    left.magicSync === right.magicSync &&
    left.command === right.command &&
    left.status === right.status &&
    left.gearSettings === right.gearSettings &&
    left.currentMode === right.currentMode &&
    left.reserved1 === right.reserved1 &&
    left.currentRpm === right.currentRpm &&
    left.targetRpm === right.targetRpm &&
    left.maxGear === right.maxGear &&
    left.setGear === right.setGear &&
    left.workMode === right.workMode &&
    left.transport === right.transport &&
    left.speedUnit === right.speedUnit &&
    flyDigiCapabilityEquals(left.flyDigiCapability, right.flyDigiCapability);
};

const temperatureDataEquals = (left: types.TemperatureData | null, right: types.TemperatureData | null) => {
  if (left === right) return true;
  if (!left || !right) return false;
  return left.cpuTemp === right.cpuTemp &&
    left.gpuTemp === right.gpuTemp &&
    optionalNumber(left.cpuPowerWatts) === optionalNumber(right.cpuPowerWatts) &&
    optionalNumber(left.gpuPowerWatts) === optionalNumber(right.gpuPowerWatts) &&
    left.gpuReadState === right.gpuReadState &&
    left.maxTemp === right.maxTemp &&
    left.controlTemp === right.controlTemp &&
    left.controlSource === right.controlSource &&
    left.selectedGpuDevice === right.selectedGpuDevice &&
    left.cpuModel === right.cpuModel &&
    left.gpuModel === right.gpuModel &&
    left.bridgeOk === right.bridgeOk &&
    left.bridgeMessage === right.bridgeMessage &&
    (left as types.TemperatureData & { telemetryState?: string }).telemetryState === (right as types.TemperatureData & { telemetryState?: string }).telemetryState &&
    sensorListEquals(left.cpuSensors, right.cpuSensors) &&
    sensorListEquals(left.gpuSensors, right.gpuSensors) &&
    sensorListEquals(left.cpuPowerSensors, right.cpuPowerSensors) &&
    sensorListEquals(left.gpuPowerSensors, right.gpuPowerSensors) &&
    gpuDeviceListEquals(left.gpuDevices, right.gpuDevices);
};

const runtimeStateFromStatus = (status?: DeviceStatusPayload | null) =>
  status?.runtime?.state || (status?.connected ? 'ready' : 'disconnected');

type ActiveTab = AppTab;
export type CurveFocusTarget = 'curve-editor' | 'history-details';

const deviceContextRequestGate = new LatestRequestGate();

interface AppStore {
  isConnected: boolean;
  deviceRuntimeState: string;
  deviceProductId: string | null;
  deviceModel: string | null;
  deviceSettings: DeviceSettings | null;
  runtimeDeviceProfile: types.DeviceProfile | null;
  runtimeDeviceCapabilities: types.DeviceCapabilities | null;
  config: types.AppConfig | null;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  legionFnQSupported: boolean;
  bridgeWarning: string | null;
  coreServiceError: string | null;
  isLoading: boolean;
  error: string | null;
  activeTab: ActiveTab;
  curveDraftDirty: boolean;
  pendingTab: ActiveTab | null;
  curveFocusTarget: CurveFocusTarget | null;
  sessionHistoryPoints: TemperatureHistoryPoint[];
  timelineEvents: TimelineEvent[];

  setActiveTab: (tab: ActiveTab) => void;
  setCurveDraftDirty: (dirty: boolean) => void;
  completePendingTabChange: () => void;
  cancelPendingTabChange: () => void;
  openCurveTab: (target: CurveFocusTarget) => void;
  clearCurveFocusTarget: () => void;
  clearBridgeWarning: () => void;
  handleTemperaturePayload: (data: types.TemperatureData | null) => void;
  appendSessionHistoryPoint: (data: types.TemperatureData | null) => void;

  initializeApp: () => Promise<void>;
  connectDevice: () => Promise<void>;
  disconnectDevice: () => Promise<void>;
  setConfig: (config: types.AppConfig) => void;
  refreshDeviceContext: () => Promise<DeviceStatusPayload | null>;

  startEventListeners: () => () => void;
}

export const useAppStore = create<AppStore>((set, get) => ({
  isConnected: false,
  deviceRuntimeState: 'disconnected',
  deviceProductId: null,
  deviceModel: null,
  deviceSettings: null,
  runtimeDeviceProfile: null,
  runtimeDeviceCapabilities: null,
  config: null,
  fanData: null,
  temperature: null,
  legionFnQSupported: false,
  bridgeWarning: null,
  coreServiceError: null,
  isLoading: true,
  error: null,
  activeTab: 'status',
  curveDraftDirty: false,
  pendingTab: null,
  curveFocusTarget: null,
  sessionHistoryPoints: [],
  timelineEvents: [],

  setActiveTab: (tab) => set((state) => ({
    ...requestTabChange(state, tab),
    curveFocusTarget: null,
  })),

  setCurveDraftDirty: (dirty) => set({ curveDraftDirty: dirty }),

  completePendingTabChange: () => set((state) => completeTabChange(state)),

  cancelPendingTabChange: () => set((state) => cancelTabChange(state)),

  openCurveTab: (target) => set({ activeTab: 'curve', curveFocusTarget: target }),

  clearCurveFocusTarget: () => set({ curveFocusTarget: null }),

  clearBridgeWarning: () => set({ bridgeWarning: null }),

  handleTemperaturePayload: (data) => {
    const current = get();
    const merged = data && current.temperature ? {
      ...data,
      cpuSensors: data.cpuSensors ?? current.temperature.cpuSensors,
      gpuSensors: data.gpuSensors ?? current.temperature.gpuSensors,
      cpuPowerSensors: data.cpuPowerSensors ?? current.temperature.cpuPowerSensors,
      gpuPowerSensors: data.gpuPowerSensors ?? current.temperature.gpuPowerSensors,
      gpuDevices: data.gpuDevices ?? current.temperature.gpuDevices,
    } as types.TemperatureData : data;
    const bridgeMessage = merged?.bridgeMessage?.trim() ?? '';
    const bridgeWarning = merged?.bridgeOk === false ? bridgeMessage || getBridgeWarningMessage() : null;
    if (temperatureDataEquals(current.temperature, merged) && current.bridgeWarning === bridgeWarning) {
      return;
    }
    set({ temperature: merged, bridgeWarning });
  },

  appendSessionHistoryPoint: (data) => {
    if (!data) return;

    const point = createLiveHistoryPoint({
      updateTime: data.updateTime,
      cpuTemp: data.cpuTemp,
      gpuTemp: data.gpuReadState === 'notPolled' ? 0 : data.gpuTemp,
      cpuPowerWatts: data.cpuPowerWatts,
      gpuPowerWatts: data.gpuReadState === 'notPolled' ? 0 : data.gpuPowerWatts,
    }, Number(get().fanData?.currentRpm || 0));

    if (!point) return;

    set((state) => {
      const points = appendSampledHistoryPoint(state.sessionHistoryPoints, point, {
        retentionMs: SESSION_HISTORY_RETENTION_MS,
        limit: SESSION_HISTORY_LIMIT,
      });
      return points === state.sessionHistoryPoints ? state : { sessionHistoryPoints: points };
    });
  },

  initializeApp: async () => {
    try {
      set({ isLoading: true });

      const [appConfig, deviceStatus, debugInfo] = await Promise.all([
        apiService.getConfig(),
        apiService.getDeviceStatus() as Promise<DeviceStatusPayload>,
        apiService.getDebugInfo().catch(() => null),
      ]);
      const coreServiceError = deviceStatus.error ? getCoreServiceErrorMessage(deviceStatus.error) : null;

      set({
        config: appConfig,
        isConnected: deviceStatus.connected || false,
        deviceRuntimeState: runtimeStateFromStatus(deviceStatus),
        deviceProductId: deviceStatus.productId || null,
        deviceModel: deviceStatus.model || null,
        deviceSettings: deviceStatus.deviceSettings || null,
        runtimeDeviceProfile: deviceStatus.deviceProfile || null,
        runtimeDeviceCapabilities: deviceStatus.deviceCapabilities || deviceStatus.deviceProfile?.capabilities || null,
        fanData: deviceStatus.currentData || null,
        legionFnQSupported: debugInfo?.legionFnQSupported === true,
        coreServiceError,
        error: coreServiceError,
      });

      get().handleTemperaturePayload(deviceStatus.temperature || null);
    } catch (error) {
      console.error('初始化失败:', error);
      const detail = error instanceof Error ? error.message : undefined;
      const coreServiceError = isCoreServiceFailureDetail(detail) ? getCoreServiceErrorMessage(detail) : null;
      set({ error: coreServiceError || i18n.t('store.errors.initializeApp'), coreServiceError });
    } finally {
      set({ isLoading: false });
    }
  },

  connectDevice: async () => {
    try {
      set({ deviceRuntimeState: 'discovering' });
      await apiService.connectDevice();
      await get().refreshDeviceContext();
    } catch (error) {
      console.error('连接失败:', error);
      set({ deviceRuntimeState: 'disconnected', error: i18n.t('store.errors.connectDevice') });
    }
  },

  disconnectDevice: async () => {
    deviceContextRequestGate.invalidate();
    try {
      await apiService.disconnectDevice();
      set((state) => ({
        isConnected: false,
        deviceRuntimeState: 'disconnected',
        deviceProductId: null,
        deviceModel: null,
        deviceSettings: null,
        runtimeDeviceProfile: null,
        runtimeDeviceCapabilities: null,
        fanData: null,
        timelineEvents: appendTimelineEvent(state.timelineEvents, { timestamp: Date.now(), type: 'disconnect' }),
      }));
    } catch (error) {
      console.error('断开连接失败:', error);
    }
  },

  setConfig: (config) => set((state) => {
    const previousProfileId = ((state.config as any)?.activeFanCurveProfileId || '') as string;
    const nextProfileId = ((config as any)?.activeFanCurveProfileId || '') as string;
    return {
      config,
      timelineEvents: previousProfileId && nextProfileId && previousProfileId !== nextProfileId
        ? appendTimelineEvent(state.timelineEvents, { timestamp: Date.now(), type: 'profile' })
        : state.timelineEvents,
    };
  }),

  refreshDeviceContext: async () => {
    const requestGeneration = deviceContextRequestGate.begin();
    try {
      const [appConfig, status] = await Promise.all([
        apiService.getConfig().catch(() => null),
        apiService.getDeviceStatus() as Promise<DeviceStatusPayload>,
      ]);
      if (!deviceContextRequestGate.isCurrent(requestGeneration)) {
        return status;
      }
      const coreServiceError = status?.error ? getCoreServiceErrorMessage(status.error) : null;
      set({
        config: appConfig ? types.AppConfig.createFrom(appConfig) : get().config,
        isConnected: status?.connected || false,
        deviceRuntimeState: runtimeStateFromStatus(status),
        deviceSettings: status?.deviceSettings || null,
        deviceProductId: status?.productId || null,
        deviceModel: status?.model || null,
        runtimeDeviceProfile: status?.deviceProfile || null,
        runtimeDeviceCapabilities: status?.deviceCapabilities || status?.deviceProfile?.capabilities || null,
        fanData: status?.currentData || null,
        coreServiceError,
        error: coreServiceError,
      });
      if (status?.temperature) {
        get().handleTemperaturePayload(status.temperature);
      }
      return status;
    } catch (error) {
      if (!deviceContextRequestGate.isCurrent(requestGeneration)) {
        return null;
      }
      const detail = error instanceof Error ? error.message : undefined;
      const coreServiceError = isCoreServiceFailureDetail(detail) ? getCoreServiceErrorMessage(detail) : null;
      set({ error: coreServiceError || i18n.t('store.errors.connectDevice'), coreServiceError });
      return null;
    }
  },

  startEventListeners: () => {
    const unsubscribers: Array<() => void> = [];
    let pendingDisconnectTimer: number | null = null;
    const clearPendingDisconnect = () => {
      if (pendingDisconnectTimer !== null) {
        window.clearTimeout(pendingDisconnectTimer);
        pendingDisconnectTimer = null;
      }
    };

    unsubscribers.push(
      apiService.onCoreServiceError((message) => {
        deviceContextRequestGate.invalidate();
        clearPendingDisconnect();
        const coreServiceError = getCoreServiceErrorMessage(message);
        set({
          coreServiceError,
          error: coreServiceError,
          isConnected: false,
          deviceRuntimeState: 'disconnected',
          deviceProductId: null,
          deviceModel: null,
          deviceSettings: null,
          runtimeDeviceProfile: null,
          runtimeDeviceCapabilities: null,
          fanData: null,
        });
      })
    );

    unsubscribers.push(
      apiService.onCoreServiceOK(() => {
        set((state) => ({
          coreServiceError: null,
          error: state.coreServiceError && state.error === state.coreServiceError ? null : state.error,
        }));
      })
    );

    unsubscribers.push(
      apiService.onDeviceConnected((deviceInfo) => {
        console.log('设备已连接:', deviceInfo);
        clearPendingDisconnect();
        const info = deviceInfo as {
          productId?: string;
          model?: string;
          deviceName?: string;
          currentData?: types.FanData | null;
          deviceProfile?: types.DeviceProfile | null;
          deviceCapabilities?: types.DeviceCapabilities | null;
          runtime?: { state?: string };
        };
        const settings = (deviceInfo as { deviceSettings?: DeviceSettings | null })?.deviceSettings || null;
        const connectedDeviceName = [
          info.deviceName,
          info.deviceProfile?.displayName,
          info.deviceProfile?.model,
          info.model,
        ].map((value) => (typeof value === 'string' ? value.trim() : '')).find(Boolean);
        set((state) => ({
          isConnected: true,
          deviceRuntimeState: info.runtime?.state || 'capabilities',
          deviceProductId: info.productId || null,
          deviceModel: info.model || null,
          deviceSettings: settings,
          runtimeDeviceProfile: info.deviceProfile || null,
          runtimeDeviceCapabilities: info.deviceCapabilities || info.deviceProfile?.capabilities || null,
          fanData: info.currentData || null,
          coreServiceError: null,
          error: null,
          timelineEvents: appendTimelineEvent(state.timelineEvents, { timestamp: Date.now(), type: 'reconnect' }),
        }));
        if (connectedDeviceName) {
          toast.success(i18n.t('store.device.connectedTitle'), {
            description: i18n.t('store.device.connectedDescription', { device: connectedDeviceName }),
            duration: 2200,
          });
        }
        void get().refreshDeviceContext();
      })
    );

    unsubscribers.push(
      apiService.onDeviceDisconnected(() => {
        deviceContextRequestGate.invalidate();
        console.log('设备已断开');
        clearPendingDisconnect();
        set((state) => ({
          timelineEvents: appendTimelineEvent(state.timelineEvents, { timestamp: Date.now(), type: 'disconnect' }),
        }));
        if (!get().isConnected) {
          set({ isConnected: false, deviceRuntimeState: 'disconnected', deviceProductId: null, deviceModel: null, deviceSettings: null, runtimeDeviceProfile: null, runtimeDeviceCapabilities: null, fanData: null });
          return;
        }
        pendingDisconnectTimer = window.setTimeout(() => {
          pendingDisconnectTimer = null;
          set({ isConnected: false, deviceRuntimeState: 'disconnected', deviceProductId: null, deviceModel: null, deviceSettings: null, runtimeDeviceProfile: null, runtimeDeviceCapabilities: null, fanData: null });
        }, 2200);
      })
    );

    unsubscribers.push(
      apiService.onDeviceSettingsUpdate((settings) => {
        set({ deviceSettings: settings || null });
        void get().refreshDeviceContext();
      })
    );

    unsubscribers.push(
      apiService.onDeviceError((errorMsg) => {
        console.error('设备错误:', errorMsg);
        set({ error: errorMsg });
      })
    );

    unsubscribers.push(
      apiService.onFanDataUpdate((data) => {
        const current = get();
        if (fanDataEquals(current.fanData, data)) return;
        set({ fanData: data });
      })
    );

    unsubscribers.push(
      apiService.onTemperatureUpdate((data) => {
        get().handleTemperaturePayload(data);
        get().appendSessionHistoryPoint(data);
      })
    );

    unsubscribers.push(
      apiService.onConfigUpdate((updatedConfig) => {
        get().setConfig(updatedConfig);
      })
    );

    unsubscribers.push(
      apiService.onSystemResume((payload) => {
        const timestamp = Number(payload?.timestamp || 0) || Date.now();
        set((state) => ({
          timelineEvents: appendTimelineEvent(state.timelineEvents, { timestamp, type: 'resume' }),
        }));
      })
    );

    unsubscribers.push(
      apiService.onHotkeyTriggered((payload) => {
        const message = typeof payload?.message === 'string' ? payload.message : '';
        if (!message) return;
        const ok = payload?.success !== false;
        if (ok) {
          toast.success(i18n.t('store.hotkey.successTitle'), { description: message, duration: 2600 });
        } else {
          toast.error(i18n.t('store.hotkey.failureTitle'), { description: message, duration: 3200 });
        }
      })
    );

    unsubscribers.push(
      apiService.onLegionPowerModeUpdate((payload) => {
        const mode = typeof payload?.mode === 'string' ? payload.mode : '';
        if (!mode) return;
        const modeLabel: Record<string, string> = {
          Quiet: i18n.t('store.legionModes.Quiet'),
          Balance: i18n.t('store.legionModes.Balance'),
          Performance: i18n.t('store.legionModes.Performance'),
          Extreme: i18n.t('store.legionModes.Extreme'),
          GodMode: i18n.t('store.legionModes.GodMode'),
        };
        toast.success(i18n.t('store.legionFnQ.modeChangedTitle'), {
          description: i18n.t('store.legionFnQ.modeDescription', { mode: modeLabel[mode] || mode }),
          duration: 2600,
        });
      })
    );

    unsubscribers.push(
      apiService.onLegionFnQSupportUpdate((payload) => {
        set({ legionFnQSupported: payload?.supported === true });
      })
    );

    return () => {
      clearPendingDisconnect();
      unsubscribers.forEach((unsubscribe) => unsubscribe());
    };
  },
}));
