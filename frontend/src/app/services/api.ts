// Wails API 服务封装
import { EventsOn } from '../../../wailsjs/runtime/runtime';
import { 
  ConnectDevice, 
  DisconnectDevice, 
  GetDeviceStatus,
  GetConfig,
  DownloadAndInstallUpdate,
  PauseUpdateDownload,
  ResumeUpdateDownload,
  CancelUpdateDownload,
  UpdateConfig,
  SetFanCurve,
  ResetLearnedOffsets,
  GetFanCurve,
  SetAutoControl,
  GetAppVersion,
  SetManualGear,
  GetAvailableGears,
  SetGearLight,
  SetPowerOnStart,
  SetSmartStartStop,
  SetWiFiSmartStartStopStandbySpeed,
  SetBrightness,
  SetLightStrip,
  GetTemperature,
  GetTemperatureHistory,
  SetTemperatureHistoryEnabled,
  GetCurrentFanData,
  TestTemperatureReading,
  GetDebugInfo,
  ExportDiagnosticsToFile,
  SetDebugMode,
  SetCustomSpeed
  // CheckWindowsAutoStart,
  // SetWindowsAutoStart
} from '../../../wailsjs/go/main/App';

import { types } from '../../../wailsjs/go/models';

import type {
  DeviceInfo,
  DeviceDebugCommandResult,
  DeviceDebugFrame,
  DeviceSettings,
  DebugInfo,
  LegionFnQSupportPayload,
  LegionPowerModePayload,
  ThemeMeta,
} from '../types/app';

export interface AutoScanDeviceInfo {
  manufacturer?: string;
  product?: string;
  model?: string;
  transport?: string;
  endpoint?: string;
  serial?: string;
  productId?: string;
  profileId?: string;
}

export interface AutoScanDevicesResult {
  connected?: boolean;
  matched?: boolean;
  profileId?: string;
  transport?: string;
  deviceInfo?: AutoScanDeviceInfo;
  devices?: AutoScanDeviceInfo[];
  deviceSettings?: DeviceSettings;
  error?: string;
}

export interface WiFiDiscoveredDevice {
  name?: string;
  profileId?: string;
  transport?: string;
  endpoint: string;
  ip?: string;
  port?: string;
  source?: string;
  network?: string;
  speed?: number;
  targetSpeed?: number;
  temperature?: number;
  latencyMs?: number;
  stateEndpoint?: string;
}

export interface WiFiDiscoveryScope {
  source?: string;
  network?: string;
  candidateCount?: number;
}

export interface WiFiDiscoveryResult {
  mode?: string;
  found?: boolean;
  canceled?: boolean;
  devices?: WiFiDiscoveredDevice[];
  scopes?: WiFiDiscoveryScope[];
  candidateCount?: number;
  scannedCount?: number;
  elapsedMs?: number;
  error?: string;
}

export interface DeviceCandidate {
  id: string;
  transport: 'wifi' | 'ble' | 'hid' | 'serial' | string;
  name: string;
  profileId?: string;
  endpoint?: string;
  source?: string;
  network?: string;
  speed?: number;
  targetSpeed?: number;
  temperature?: number;
  latencyMs?: number;
  connected?: boolean;
  connectable?: boolean;
  error?: string;
}

export interface DeviceScanResult {
  mode?: 'normal' | 'deep' | string;
  connected?: boolean;
  devices?: DeviceCandidate[];
  wifiEnabled?: boolean;
  serialEnabled?: boolean;
  showDeepScan?: boolean;
  error?: string;
}

export interface UpdateRelease {
  tag_name?: string;
  html_url?: string;
  body?: string;
  prerelease?: boolean;
  update_available?: boolean;
  draft?: boolean;
  installer_url?: string;
  installer_sha256?: string;
}

export interface UpdateProgressPayload {
  percent: number;
  received: number;
  total: number;
  stage: 'downloading' | 'paused' | 'retrying' | 'installing' | 'error' | 'canceled';
  message: string;
  attempt?: number;
  maxAttempts?: number;
}

class ApiService {
  // 设备连接
  async connectDevice(): Promise<boolean> {
    return await ConnectDevice();
  }

  async autoScanDevices(): Promise<AutoScanDevicesResult> {
    const result = await (window as any).go?.main?.App?.AutoScanDevices?.();
    return result && typeof result === 'object' ? result as AutoScanDevicesResult : { connected: false };
  }

  async connectNativeDevice(profileID = ''): Promise<boolean> {
    return !!(await (window as any).go?.main?.App?.ConnectNativeDevice?.(profileID));
  }

  async scanDeviceCandidates(mode: 'normal' | 'deep' = 'normal'): Promise<DeviceScanResult> {
    const result = await (window as any).go?.main?.App?.ScanDeviceCandidates?.(mode);
    return result && typeof result === 'object' ? result as DeviceScanResult : { mode, devices: [] };
  }

  async connectDeviceCandidate(candidate: DeviceCandidate): Promise<boolean> {
    return !!(await (window as any).go?.main?.App?.ConnectDeviceCandidate?.({
      id: candidate.id,
      transport: candidate.transport,
      profileId: candidate.profileId || '',
      endpoint: candidate.endpoint || '',
    }));
  }

  async scanWiFiDevices(mode: 'normal' | 'deep' = 'normal'): Promise<WiFiDiscoveryResult> {
    const result = await (window as any).go?.main?.App?.ScanWiFiDevices?.(mode);
    return result && typeof result === 'object' ? result as WiFiDiscoveryResult : { mode, found: false };
  }

  async controlWiFiScan(action: 'pause' | 'resume' | 'cancel'): Promise<boolean> {
    return !!(await (window as any).go?.main?.App?.ControlWiFiScan?.(action));
  }

  async disconnectDevice(): Promise<void> {
    return await DisconnectDevice();
  }

  async getDeviceStatus(): Promise<any> {
    return await GetDeviceStatus();
  }

  async refreshDeviceSettings(): Promise<DeviceSettings | null> {
    return await (window as any).go?.main?.App?.RefreshDeviceSettings?.();
  }

  // 配置管理
  async getConfig(): Promise<types.AppConfig> {
    return await GetConfig();
  }

  async getAppVersion(): Promise<string> {
    return await GetAppVersion();
  }

  async checkLatestRelease(channel: 'stable' | 'prerelease'): Promise<UpdateRelease | null> {
    const release = await (window as any).go?.main?.App?.CheckLatestRelease?.(channel);
    return release && typeof release === 'object' ? release as UpdateRelease : null;
  }

  async updateCompletedOnLaunch(): Promise<boolean> {
    return !!(await (window as any).go?.main?.App?.UpdateCompletedOnLaunch?.());
  }

  async downloadAndInstallUpdate(
    downloadURL: string,
    windowTitle: string,
    windowBody: string,
    windowRestarting: string,
    expectedSHA256: string,
  ): Promise<void> {
    return await DownloadAndInstallUpdate(
      downloadURL,
      windowTitle,
      windowBody,
      windowRestarting,
      expectedSHA256,
    );
  }

  async pauseUpdateDownload(): Promise<boolean> {
    return await PauseUpdateDownload();
  }

  async resumeUpdateDownload(): Promise<boolean> {
    return await ResumeUpdateDownload();
  }

  async cancelUpdateDownload(downloadURL: string): Promise<void> {
    return await CancelUpdateDownload(downloadURL);
  }

  onUpdateDownloadProgress(
    callback: (payload: UpdateProgressPayload) => void,
  ): () => void {
    return EventsOn('update-download-progress', callback);
  }

  async updateConfig(config: types.AppConfig): Promise<void> {
    return await UpdateConfig(config);
  }

  async getDeviceProfiles(): Promise<types.DeviceProfilesPayload> {
    return await (window as any).go?.main?.App?.GetDeviceProfiles();
  }

  async getSupportedDeviceProfiles(): Promise<types.DeviceProfile[]> {
    const profiles = await (window as any).go?.main?.App?.GetSupportedDeviceProfiles?.();
    return Array.isArray(profiles) ? profiles as types.DeviceProfile[] : [];
  }

  async getUserDeviceProfiles(): Promise<types.DeviceProfile[]> {
    const profiles = await (window as any).go?.main?.App?.GetUserDeviceProfiles?.();
    return Array.isArray(profiles) ? profiles as types.DeviceProfile[] : [];
  }

  async setActiveDeviceProfile(profileID: string): Promise<types.DeviceProfile> {
    return await (window as any).go?.main?.App?.SetActiveDeviceProfile(profileID);
  }

  async saveDeviceProfile(profile: types.DeviceProfile, setActive: boolean): Promise<types.DeviceProfile> {
    return await (window as any).go?.main?.App?.SaveDeviceProfile(profile, setActive);
  }

  async deleteDeviceProfile(profileID: string): Promise<void> {
    return await (window as any).go?.main?.App?.DeleteDeviceProfile(profileID);
  }

  async exportDeviceProfiles(): Promise<string> {
    return await (window as any).go?.main?.App?.ExportDeviceProfiles();
  }

  async exportDeviceProfilesToFile(): Promise<string> {
    return await (window as any).go?.main?.App?.ExportDeviceProfilesToFile?.();
  }

  async importDeviceProfiles(code: string): Promise<void> {
    return await (window as any).go?.main?.App?.ImportDeviceProfiles(code);
  }

  async testDeviceProfile(params: types.DeviceProfileTestParams): Promise<types.DeviceProfileTestResult> {
    return await (window as any).go?.main?.App?.TestDeviceProfile(params);
  }

  async listSerialPorts(): Promise<types.SerialPortInfo[]> {
    const ports = await (window as any).go?.main?.App?.ListSerialPorts?.();
    return Array.isArray(ports) ? ports as types.SerialPortInfo[] : [];
  }

  // 风扇曲线
  async scanBLEDevices(params: types.BLEScanParams): Promise<types.BLEDeviceInfo[]> {
    const devices = await (window as any).go?.main?.App?.ScanBLEDevices?.(params);
    return Array.isArray(devices) ? devices as types.BLEDeviceInfo[] : [];
  }

  async probeBLEGATT(params: types.BLEGATTProbeParams): Promise<types.BLEGATTProbeResult> {
    return await (window as any).go?.main?.App?.ProbeBLEGATT?.(params);
  }

  async setFanCurve(curve: types.FanCurvePoint[]): Promise<void> {
    return await SetFanCurve(curve);
  }

  // 清空学习到的曲线偏移；后端清零所有 LearnedOffsets。
  async resetLearnedOffsets(): Promise<void> {
    return await ResetLearnedOffsets();
  }

  async getFanCurve(): Promise<types.FanCurvePoint[]> {
    return await GetFanCurve();
  }

  async getFanCurveProfiles(): Promise<{ profiles: Array<{ id: string; name: string; curve: types.FanCurvePoint[] }>; activeId: string }> {
    return await (window as any).go?.main?.App?.GetFanCurveProfiles();
  }

  async setActiveFanCurveProfile(profileID: string): Promise<void> {
    return await (window as any).go?.main?.App?.SetActiveFanCurveProfile(profileID);
  }

  async saveFanCurveProfile(profileID: string, name: string, curve: types.FanCurvePoint[], setActive: boolean): Promise<{ id: string; name: string; curve: types.FanCurvePoint[] }> {
    return await (window as any).go?.main?.App?.SaveFanCurveProfile(profileID, name, curve, setActive);
  }

  async deleteFanCurveProfile(profileID: string): Promise<void> {
    return await (window as any).go?.main?.App?.DeleteFanCurveProfile(profileID);
  }

  async exportFanCurveProfiles(): Promise<string> {
    return await (window as any).go?.main?.App?.ExportFanCurveProfiles();
  }

  async exportFanCurveProfilesToFile(): Promise<string> {
    return await (window as any).go?.main?.App?.ExportFanCurveProfilesToFile?.();
  }

  async importFanCurveProfiles(code: string): Promise<void> {
    return await (window as any).go?.main?.App?.ImportFanCurveProfiles(code);
  }

  // 智能变频
  async setAutoControl(enabled: boolean): Promise<void> {
    return await SetAutoControl(enabled);
  }

  // 自定义转速
  async setCustomSpeed(enabled: boolean, rpm: number): Promise<void> {
    return await SetCustomSpeed(enabled, rpm);
  }

  // 手动挡位控制
  async setManualGear(gear: string, level: string): Promise<boolean> {
    return await SetManualGear(gear, level);
  }

  // 获取可用挡位
  async getAvailableGears(): Promise<any> {
    return await GetAvailableGears();
  }

  // 设备设置
  async setGearLight(enabled: boolean): Promise<boolean> {
    return await SetGearLight(enabled);
  }

  async setPowerOnStart(enabled: boolean): Promise<boolean> {
    return await SetPowerOnStart(enabled);
  }

  async setSmartStartStop(mode: string): Promise<boolean> {
    return await SetSmartStartStop(mode);
  }

  async setWiFiSmartStartStopStandbySpeed(percent: number): Promise<boolean> {
    return await SetWiFiSmartStartStopStandbySpeed(percent);
  }

  async setBrightness(percentage: number): Promise<boolean> {
    return await SetBrightness(percentage);
  }

  async setLightStrip(config: types.LightStripConfig): Promise<void> {
    return await SetLightStrip(config);
  }

  // Windows自启动相关
  async checkWindowsAutoStart(): Promise<boolean> {
    // 临时使用window对象调用，等Wails生成绑定后更新
    return await (window as any).go?.main?.App?.CheckWindowsAutoStart();
  }

  async setWindowsAutoStart(enabled: boolean): Promise<void> {
    // 临时使用window对象调用，等Wails生成绑定后更新
    return await (window as any).go?.main?.App?.SetWindowsAutoStart(enabled);
  }

  async getAutoStartMethod(): Promise<string> {
    // 获取当前自启动方式
    return await (window as any).go?.main?.App?.GetAutoStartMethod();
  }

  async setAutoStartWithMethod(enabled: boolean, method: string): Promise<void> {
    // 使用指定方式设置自启动
    return await (window as any).go?.main?.App?.SetAutoStartWithMethod(enabled, method);
  }

  async isRunningAsAdmin(): Promise<boolean> {
    // 检查是否以管理员权限运行
    return await (window as any).go?.main?.App?.IsRunningAsAdmin();
  }

  // 数据获取
  async getTemperature(): Promise<types.TemperatureData> {
    return await GetTemperature();
  }

  async getTemperatureHistory(): Promise<types.TemperatureHistoryPayload> {
    return await GetTemperatureHistory();
  }

  async setTemperatureHistoryEnabled(enabled: boolean): Promise<void> {
    return await SetTemperatureHistoryEnabled(enabled);
  }

  async getCurrentFanData(): Promise<types.FanData | null> {
    return await GetCurrentFanData();
  }

  async beginNoiseDiagnostic(request: types.NoiseDiagnosticBeginRequest): Promise<types.NoiseDiagnosticSession> {
    return await (window as any).go?.main?.App?.BeginNoiseDiagnostic(request);
  }

  async setNoiseDiagnosticTarget(sessionID: string, value: number): Promise<types.NoiseDiagnosticTargetResult> {
    return await (window as any).go?.main?.App?.SetNoiseDiagnosticTarget(sessionID, value);
  }

  async endNoiseDiagnostic(sessionID: string): Promise<void> {
    return await (window as any).go?.main?.App?.EndNoiseDiagnostic(sessionID);
  }

  async cancelNoiseDiagnostic(sessionID: string): Promise<void> {
    return await (window as any).go?.main?.App?.CancelNoiseDiagnostic(sessionID);
  }

  async saveNoiseDiagnosticResult(result: types.NoiseDiagnosticResult): Promise<void> {
    return await (window as any).go?.main?.App?.SaveNoiseDiagnosticResult(result);
  }

  async testTemperatureReading(): Promise<types.TemperatureData> {
    return await TestTemperatureReading();
  }

  // 桥接程序相关
  async getBridgeProgramStatus(): Promise<any> {
    return await (window as any).go?.main?.App?.GetBridgeProgramStatus();
  }

  async testBridgeProgram(): Promise<any> {
    return await (window as any).go?.main?.App?.TestBridgeProgram();
  }

  async restartPawnIO(): Promise<any> {
    return await (window as any).go?.main?.App?.RestartPawnIO();
  }

  async reinstallPawnIO(): Promise<any> {
    return await (window as any).go?.main?.App?.ReinstallPawnIO();
  }

  // 事件监听
  onDeviceConnected(callback: (data: DeviceInfo) => void): () => void {
    return EventsOn('device-connected', callback);
  }

  onDeviceDisconnected(callback: () => void): () => void {
    return EventsOn('device-disconnected', callback);
  }

  onDeviceError(callback: (error: string) => void): () => void {
    return EventsOn('device-error', callback);
  }

  onDeviceSettingsUpdate(callback: (data: DeviceSettings) => void): () => void {
    return EventsOn('device-settings-update', callback);
  }

  onFanDataUpdate(callback: (data: types.FanData) => void): () => void {
    return EventsOn('fan-data-update', callback);
  }

  onTemperatureUpdate(callback: (data: types.TemperatureData) => void): () => void {
    return EventsOn('temperature-update', callback);
  }

  onTemperatureHistoryUpdate(callback: (data: { timestamp: number; cpuTemp: number; gpuTemp: number; fanRpm?: number; cpuPowerWatts?: number; gpuPowerWatts?: number }) => void): () => void {
    return EventsOn('temperature-history-update', callback);
  }

  onConfigUpdate(callback: (config: types.AppConfig) => void): () => void {
    return EventsOn('config-update', callback);
  }

  onSystemResume(callback: (payload: { timestamp?: number; source?: string }) => void): () => void {
    return EventsOn('system-resume', callback);
  }

  onHotkeyTriggered(callback: (payload: { action: string; shortcut: string; success: boolean; message: string }) => void): () => void {
    return EventsOn('hotkey-triggered', callback);
  }

  // 调试相关方法
  onLegionPowerModeUpdate(callback: (payload: LegionPowerModePayload) => void): () => void {
    return EventsOn('legion-power-mode-update', callback);
  }

  onLegionFnQSupportUpdate(callback: (payload: LegionFnQSupportPayload) => void): () => void {
    return EventsOn('legion-fnq-support-update', callback);
  }

  async getDebugInfo(): Promise<DebugInfo> {
    return await GetDebugInfo() as DebugInfo;
  }

  async exportDiagnosticsToFile(): Promise<string> {
    return await ExportDiagnosticsToFile();
  }

  async setDebugMode(enabled: boolean): Promise<void> {
    return await SetDebugMode(enabled);
  }

  async sendDeviceDebugCommand(hexCommand: string, waitMs = 800): Promise<DeviceDebugCommandResult> {
    return await (window as any).go?.main?.App?.SendDeviceDebugCommand(hexCommand, waitMs);
  }

  async getDeviceDebugFrames(): Promise<DeviceDebugFrame[]> {
    const frames = await (window as any).go?.main?.App?.GetDeviceDebugFrames();
    return Array.isArray(frames) ? frames as DeviceDebugFrame[] : [];
  }

  // ── 自定义主题 ──
  // 注：以下方法走 Wails 运行时自动暴露的 window.go.main.App 代理，
  // 因此无需重新生成强类型绑定即可调用（与上面若干方法同理）。

  // 列出安装目录/用户目录下发现的全部自定义主题。
  async listThemes(): Promise<ThemeMeta[]> {
    const list = await (window as any).go?.main?.App?.ListThemes?.();
    return Array.isArray(list) ? (list as ThemeMeta[]) : [];
  }

  // 读取指定主题的 CSS 文本（用于注入页面）。
  async getThemeCSS(id: string): Promise<string> {
    const css = await (window as any).go?.main?.App?.GetThemeCSS?.(id);
    return typeof css === 'string' ? css : '';
  }

  // 在系统文件管理器中打开主题文件夹，便于用户编辑/新增主题。
  async openThemesFolder(): Promise<void> {
    return await (window as any).go?.main?.App?.OpenThemesFolder?.();
  }

  // 调试事件监听
  onHealthPing(callback: (timestamp: number) => void): () => void {
    return EventsOn('health-ping', callback);
  }

  onHeartbeat(callback: (timestamp: number) => void): () => void {
    return EventsOn('heartbeat', callback);
  }

  onCoreServiceError(callback: (message: string) => void): () => void {
    return EventsOn('core-service-error', callback);
  }

  onCoreServiceOK(callback: () => void): () => void {
    return EventsOn('core-service-ok', callback);
  }
}

export const apiService = new ApiService();
