import { create } from 'zustand';
import { types } from '../../../wailsjs/go/models';
import { apiService } from '../services/api';
import { configService } from '../services/config-service';
import { deviceService, type DeviceStatusPayload } from '../services/device-service';
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
    sensorListEquals(left.cpuSensors, right.cpuSensors) &&
    sensorListEquals(left.gpuSensors, right.gpuSensors) &&
    sensorListEquals(left.cpuPowerSensors, right.cpuPowerSensors) &&
    sensorListEquals(left.gpuPowerSensors, right.gpuPowerSensors) &&
    gpuDeviceListEquals(left.gpuDevices, right.gpuDevices);
};

type ActiveTab = 'status' | 'curve' | 'control' | 'devices' | 'about';
export type CurveFocusTarget = 'curve-editor' | 'history-details';

interface AppStore {
  isConnected: boolean;
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
  curveFocusTarget: CurveFocusTarget | null;
  sessionHistoryPoints: TemperatureHistoryPoint[];

  setActiveTab: (tab: ActiveTab) => void;
  openCurveTab: (target: CurveFocusTarget) => void;
  clearCurveFocusTarget: () => void;
  clearBridgeWarning: () => void;
  handleTemperaturePayload: (data: types.TemperatureData | null) => void;
  appendSessionHistoryPoint: (data: types.TemperatureData | null) => void;

  initializeApp: () => Promise<void>;
  connectDevice: () => Promise<void>;
  disconnectDevice: () => Promise<void>;
  updateConfig: (config: types.AppConfig) => Promise<void>;
  refreshDeviceContext: () => Promise<DeviceStatusPayload | null>;

  startEventListeners: () => () => void;
}

export const useAppStore = create<AppStore>((set, get) => ({
  isConnected: false,
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
  curveFocusTarget: null,
  sessionHistoryPoints: [],

  setActiveTab: (tab) => set({ activeTab: tab, curveFocusTarget: null }),

  openCurveTab: (target) => set({ activeTab: 'curve', curveFocusTarget: target }),

  clearCurveFocusTarget: () => set({ curveFocusTarget: null }),

  clearBridgeWarning: () => set({ bridgeWarning: null }),

  handleTemperaturePayload: (data) => {
    const bridgeMessage = data?.bridgeMessage?.trim() ?? '';
    const bridgeWarning = data?.bridgeOk === false ? bridgeMessage || getBridgeWarningMessage() : null;
    const current = get();
    if (temperatureDataEquals(current.temperature, data) && current.bridgeWarning === bridgeWarning) {
      return;
    }
    set({ temperature: data, bridgeWarning });
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

    set((state) => ({
      sessionHistoryPoints: appendSampledHistoryPoint(state.sessionHistoryPoints, point, {
        retentionMs: SESSION_HISTORY_RETENTION_MS,
        limit: SESSION_HISTORY_LIMIT,
      }),
    }));
  },

  initializeApp: async () => {
    try {
      set({ isLoading: true });

      const [appConfig, deviceStatus, debugInfo] = await Promise.all([
        configService.getConfig(),
        deviceService.getStatus() as Promise<DeviceStatusPayload>,
        apiService.getDebugInfo().catch(() => null),
      ]);
      const coreServiceError = deviceStatus.error ? getCoreServiceErrorMessage(deviceStatus.error) : null;

      set({
        config: appConfig,
        isConnected: deviceStatus.connected || false,
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
      const success = await deviceService.connect();
      if (success) {
        await get().refreshDeviceContext();
      }
    } catch (error) {
      console.error('连接失败:', error);
      set({ error: i18n.t('store.errors.connectDevice') });
    }
  },

  disconnectDevice: async () => {
    try {
      await deviceService.disconnect();
      set({ isConnected: false, deviceProductId: null, deviceModel: null, deviceSettings: null, runtimeDeviceProfile: null, runtimeDeviceCapabilities: null, fanData: null });
    } catch (error) {
      console.error('断开连接失败:', error);
    }
  },

  updateConfig: async (config) => {
    try {
      await configService.updateConfig(config);
      set({ config, error: null });
    } catch (error) {
      console.error('配置更新失败:', error);
      set({ error: i18n.t('store.errors.saveConfig') });
    }
  },

  refreshDeviceContext: async () => {
    try {
      const [appConfig, status] = await Promise.all([
        configService.getConfig().catch(() => null),
        deviceService.getStatus() as Promise<DeviceStatusPayload>,
      ]);
      const coreServiceError = status?.error ? getCoreServiceErrorMessage(status.error) : null;
      set({
        config: appConfig ? types.AppConfig.createFrom(appConfig) : get().config,
        isConnected: status?.connected || false,
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
        clearPendingDisconnect();
        const coreServiceError = getCoreServiceErrorMessage(message);
        set({
          coreServiceError,
          error: coreServiceError,
          isConnected: false,
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
      deviceService.onDeviceConnected((deviceInfo) => {
        console.log('设备已连接:', deviceInfo);
        clearPendingDisconnect();
        const info = deviceInfo as {
          productId?: string;
          model?: string;
          deviceName?: string;
          currentData?: types.FanData | null;
          deviceProfile?: types.DeviceProfile | null;
          deviceCapabilities?: types.DeviceCapabilities | null;
        };
        const settings = (deviceInfo as { deviceSettings?: DeviceSettings | null })?.deviceSettings || null;
        const connectedDeviceName = [
          info.deviceName,
          info.deviceProfile?.displayName,
          info.deviceProfile?.model,
          info.model,
        ].map((value) => (typeof value === 'string' ? value.trim() : '')).find(Boolean);
        set({
          isConnected: true,
          deviceProductId: info.productId || null,
          deviceModel: info.model || null,
          deviceSettings: settings,
          runtimeDeviceProfile: info.deviceProfile || null,
          runtimeDeviceCapabilities: info.deviceCapabilities || info.deviceProfile?.capabilities || null,
          fanData: info.currentData || null,
          coreServiceError: null,
          error: null,
        });
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
      deviceService.onDeviceDisconnected(() => {
        console.log('设备已断开');
        clearPendingDisconnect();
        if (!get().isConnected) {
          set({ isConnected: false, deviceProductId: null, deviceModel: null, deviceSettings: null, runtimeDeviceProfile: null, runtimeDeviceCapabilities: null, fanData: null });
          return;
        }
        pendingDisconnectTimer = window.setTimeout(() => {
          pendingDisconnectTimer = null;
          set({ isConnected: false, deviceProductId: null, deviceModel: null, deviceSettings: null, runtimeDeviceProfile: null, runtimeDeviceCapabilities: null, fanData: null });
        }, 2200);
      })
    );

    unsubscribers.push(
      deviceService.onDeviceSettingsUpdate((settings) => {
        set({ deviceSettings: settings || null });
      })
    );

    unsubscribers.push(
      deviceService.onDeviceError((errorMsg) => {
        console.error('设备错误:', errorMsg);
        set({ error: errorMsg });
      })
    );

    unsubscribers.push(
      deviceService.onFanDataUpdate((data) => {
        const current = get();
        if (fanDataEquals(current.fanData, data)) return;
        set({ fanData: data });
      })
    );

    unsubscribers.push(
      deviceService.onTemperatureUpdate((data) => {
        get().handleTemperaturePayload(data);
        get().appendSessionHistoryPoint(data);
      })
    );

    unsubscribers.push(
      configService.onConfigUpdate((updatedConfig) => {
        set({ config: updatedConfig });
      })
    );

    unsubscribers.push(
      deviceService.onHotkeyTriggered((payload) => {
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
      deviceService.onLegionPowerModeUpdate((payload) => {
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
