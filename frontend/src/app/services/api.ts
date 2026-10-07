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
  SetTemperatureHistoryRetentionHours,
  GetCurrentFanData,
  TestTemperatureReading,
  GetDebugInfo,
  ExportDiagnosticsToFile,
  SetDebugMode,
  SetCustomSpeed,
  GetBlackSharkInfo,
  GetBlackSharkManualGearPresets,
  SetBlackSharkLightingEnabled,
  SetBlackSharkOnOffVector,
  SetBlackSharkLcdScreenEnabled,
  GetBlackSharkRgbLighting,
  GetSceneRules,
  SetSceneRules,
  ListSceneProcesses,
  SelectBlackSharkRgbMode,
  SetBlackSharkRgbModeEffects,
  SetBlackSharkRgbModeColor,
  GetBlackSharkLcdDisplay,
  SetBlackSharkLcdDisplay,
  ExportBlackSharkConfigSnapshot,
  RestoreBlackSharkConfigFromFile
  // CheckWindowsAutoStart,
  // SetWindowsAutoStart
} from '../../../wailsjs/go/main/App';

import { types } from '../../../wailsjs/go/models';
import type { AxisNoiseProfile } from '../lib/noise-diagnostic';
// 黑鲨手动挡位派生值类型；唯一所有者在后端 deviceproto 的标定表。
import type { CoreBlackSharkGearRpms } from '../lib/manualGearPresets';

import type {
  DeviceInfo,
  DeviceDebugCommandResult,
  DeviceDebugFrame,
  DeviceSettings,
  DebugInfo,
  LegionFnQSupportPayload,
  LegionPowerModePayload,
  ThemeMeta,
  BlackSharkInfo,
  BlackSharkFirmwareStatus,
  BlackSharkCurveTempRange,
  BlackSharkRgbLighting,
  BlackSharkHostEffects,
  BlackSharkConfigExportResult,
  BlackSharkConfigRestoreResult,
  BlackSharkLcdDisplayPayload,
  BlackSharkLcdDisplayResult,
  SceneRulesPayload,
  SceneRulesResult,
  ScreenPresetListPayload,
  ScreenUploadResultPayload,
  ScreenImageInfoPayload,
  ScreenImageReadPayload,
  ScreenCropPreviewPayload,
  ScreenHistoryListPayload,
} from '../types/app';

let configWriteGeneration = 0;

export function getConfigWriteGeneration() {
	return configWriteGeneration;
}

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

export interface DeviceImageTransferRequest {
  dataBase64: string;
  fileName: string;
  mimeType: string;
  format: 'rgb565-be';
  width: number;
  height: number;
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
		configWriteGeneration += 1;
		return await UpdateConfig(config);
	}

	async restartCore(monitorOnlySession = false): Promise<void> {
		const restart = (window as any).go?.main?.App?.RestartCore;
		if (typeof restart !== 'function') {
			throw new Error('核心服务重启接口不可用');
		}
		await restart(monitorOnlySession);
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

  // 蓝牙扫描 / 风扇曲线
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

  async exportFanCurveProfiles(profileIDs: string[]): Promise<string> {
    return await (window as any).go?.main?.App?.ExportFanCurveProfiles(profileIDs);
  }

  async exportFanCurveProfilesToFile(profileIDs: string[]): Promise<string> {
    return await (window as any).go?.main?.App?.ExportFanCurveProfilesToFile?.(profileIDs);
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

  async transferDeviceImage(request: DeviceImageTransferRequest): Promise<unknown> {
    const transfer = (window as any).go?.main?.App?.TransferDeviceImage;
    if (typeof transfer !== 'function') {
      throw new Error('Device image transfer is not available yet');
    }
    return await transfer(request);
  }

  // Windows自启动相关
  async checkWindowsAutoStart(): Promise<boolean> {
    // 走 window.go 动态代理（未走 wailsjs 强类型绑定）。
    return await (window as any).go?.main?.App?.CheckWindowsAutoStart();
  }

  async setWindowsAutoStart(enabled: boolean): Promise<void> {
    // 走 window.go 动态代理（未走 wailsjs 强类型绑定）。
    return await (window as any).go?.main?.App?.SetWindowsAutoStart(enabled);
  }

  async getAutoStartMethod(): Promise<string> {
    return await (window as any).go?.main?.App?.GetAutoStartMethod();
  }

  async setAutoStartWithMethod(enabled: boolean, method: string): Promise<void> {
    return await (window as any).go?.main?.App?.SetAutoStartWithMethod(enabled, method);
  }

  async isRunningAsAdmin(): Promise<boolean> {
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

  async setTemperatureHistoryRetentionHours(hours: number): Promise<void> {
    return await SetTemperatureHistoryRetentionHours(hours);
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

  async saveAxisNoiseProfile(profile: AxisNoiseProfile): Promise<AxisNoiseProfile> {
    return await (window as any).go?.main?.App?.SaveAxisNoiseProfile(profile);
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

  /**
   * 订阅主机侧灯效状态事件（槽 6「响应」/ 槽 7「音频同步」）。
   */
  onBlackSharkHostEffects(callback: (data: BlackSharkHostEffects) => void): () => void {
    return EventsOn('blackshark-host-effects', callback);
  }

  /** 屏幕图片传输进度：{sent, total} 帧数（设备层每 16 帧推一次）。 */
  onScreenImageTransferProgress(callback: (data: { sent: number; total: number }) => void): () => void {
    return EventsOn('screen-image-transfer-progress', callback);
  }

  /** 取消进行中的屏幕图片传输（上传调用会随即以失败返回）。 */
  async cancelScreenImageTransfer(): Promise<boolean> {
    const r = await (window as any).go?.main?.App?.CancelScreenImageTransfer?.();
    return !!r;
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

  // Legion 事件监听
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

  // 自定义主题
  // 以下方法走 Wails 运行时暴露的 window.go.main.App 代理，无需重新生成强类型绑定。

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

  onCoreResynced(callback: () => void): () => void {
    return EventsOn('core-resynced', callback);
  }

  // 黑鲨（BlackShark）BRB02

  /**
   * 读取黑鲨聚合信息：固件状态、开关、档位曲线与量程；走一次设备查询。
   */
  async getBlackSharkInfo(): Promise<BlackSharkInfo> {
    return (await GetBlackSharkInfo()) as unknown as BlackSharkInfo;
  }

  /**
   * 主动检查一次固件版本（读设备 CMD 0x01 + 拉官方版本清单）；设备 IO + 联网。
   */
  async checkBlackSharkFirmwareUpdate(): Promise<BlackSharkFirmwareStatus> {
    const call = (window as any).go?.main?.App?.CheckBlackSharkFirmwareUpdate;
    if (typeof call !== 'function') {
      throw new Error('CheckBlackSharkFirmwareUpdate 绑定缺失（请重新生成 wails 绑定）');
    }
    return (await call()) as unknown as BlackSharkFirmwareStatus;
  }

  /**
   * 读取已缓存的固件检查结果；纯读内存，不发设备查询、不联网。
   */
  async getBlackSharkFirmwareStatus(): Promise<BlackSharkFirmwareStatus> {
    const call = (window as any).go?.main?.App?.GetBlackSharkFirmwareStatus;
    if (typeof call !== 'function') {
      throw new Error('GetBlackSharkFirmwareStatus 绑定缺失（请重新生成 wails 绑定）');
    }
    return (await call()) as unknown as BlackSharkFirmwareStatus;
  }

  /**
   * 手动挡位面板所需数值：四档 × 三档转速 + RPM 量程 + 步进；取后端标定表，无设备 IO。
   */
  async getBlackSharkManualGearPresets(): Promise<CoreBlackSharkGearRpms> {
    return (await GetBlackSharkManualGearPresets()) as unknown as CoreBlackSharkGearRpms;
  }

  /**
   * 黑鲨曲线那 4 个点能被拖到的温度取值域（℃，含两端）。
   * 曲线图的横向拖动据此取范围，界面上不再另写一份区间；取值域的所有者在 deviceproto。
   * 纯计算、不碰设备，没连设备也能取。
   */
  async getBlackSharkCurveTempRange(): Promise<BlackSharkCurveTempRange> {
    const call = (window as any).go?.main?.App?.GetBlackSharkCurveTempRange;
    if (typeof call !== 'function') {
      throw new Error('GetBlackSharkCurveTempRange 绑定缺失（请重新生成 wails 绑定）');
    }
    return (await call()) as unknown as BlackSharkCurveTempRange;
  }

  /**
   * 写入「智能启停 + 通电自启」两路开关（CMD 0x02）；打设备。
   */
  async setBlackSharkOnOffVector(smartStartStop: boolean, powerOnSelfStart: boolean): Promise<boolean> {
    return await SetBlackSharkOnOffVector(smartStartStop, powerOnSelfStart);
  }

  /**
   * 读取灯效页聚合信息：开关 + 当前模式 + 全部 8 个模式的参数；走一次设备查询。
   */
  async getBlackSharkRgbLighting(): Promise<BlackSharkRgbLighting> {
    return (await GetBlackSharkRgbLighting()) as unknown as BlackSharkRgbLighting;
  }

  /**
   * 只取本机缓存的灯效状态（"上次读到的那份"）；零设备 IO。
   */
  async getBlackSharkRgbLightingCached(): Promise<BlackSharkRgbLighting> {
    const call = (window as any).go?.main?.App?.GetBlackSharkRgbLightingCached;
    if (typeof call !== 'function') return { available: false, switchEnabled: false, switchKnown: false, count: 0, currentIndex: 0, colorSettable: true };
    const r = await call();
    return r && typeof r === 'object' ? (r as BlackSharkRgbLighting) : { available: false, switchEnabled: false, switchKnown: false, count: 0, currentIndex: 0, colorSettable: true };
  }

  /**
   * 只取本机缓存的屏图（"上次读到的那份"）；零设备 IO。
   */
  async getScreenImageCached(): Promise<ScreenImageReadPayload> {
    const call = (window as any).go?.main?.App?.GetScreenImageCached;
    if (typeof call !== 'function') return { ok: false, bytes: 0, crc: 0 };
    const r = await call();
    return r && typeof r === 'object' ? (r as ScreenImageReadPayload) : { ok: false, bytes: 0, crc: 0 };
  }

  /** 切换当前生效的灯效模式（0x14 写、0x13 回读确认）；打设备。 */
  async selectBlackSharkRgbMode(index: number): Promise<boolean> {
    return await SelectBlackSharkRgbMode(index);
  }

  /**
   * 修改某灯效模式的速度与亮度（0x12，设备层做读-改-写 + 回读校验）；打设备。
   */
  async setBlackSharkRgbModeEffects(index: number, speed: number, brightness: number): Promise<boolean> {
    return await SetBlackSharkRgbModeEffects(index, speed, brightness);
  }

  /**
   * 设置某灯效模式的颜色（0x12 的 payload[5..7]，大端 R,G,B）；打设备。
   */
  async setBlackSharkRgbModeColor(index: number, hue: number, staticColor: boolean): Promise<boolean> {
    return await SetBlackSharkRgbModeColor(index, hue, staticColor);
  }

  /**
   * 写颜色下拉选项（0x12 的 payload[0] 高半字节，取值 0..4）；打设备。
   */
  async setBlackSharkRgbColorOption(index: number, option: number): Promise<boolean> {
    const call = (window as any).go?.main?.App?.SetBlackSharkRgbColorOption;
    if (typeof call !== 'function') {
      throw new Error('SetBlackSharkRgbColorOption 绑定缺失（请重新生成 wails 绑定）');
    }
    return !!(await call(index, option));
  }

  /**
   * 读取主机侧灯效驱动的状态（槽位 6「响应」/ 槽位 7「音频同步」）；读设备。
   */
  async getBlackSharkHostEffects(): Promise<BlackSharkHostEffects> {
    const call = (window as any).go?.main?.App?.GetBlackSharkHostEffects;
    if (typeof call !== 'function') {
      throw new Error('GetBlackSharkHostEffects 绑定缺失（请重新生成 wails 绑定）');
    }
    return (await call()) as unknown as BlackSharkHostEffects;
  }

  /**
   * 读取屏幕显示参数（0xC2）：选中的三项 + 左右顺序 + pos；读设备。
   */
  async getBlackSharkLcdDisplay(): Promise<BlackSharkLcdDisplayPayload> {
    return (await GetBlackSharkLcdDisplay()) as unknown as BlackSharkLcdDisplayPayload;
  }

  /**
   * 保存并下发屏幕显示参数（0xC2）；items = 按屏上左右顺序排的项 id，恰好 3 项。打设备。
   */
  async setBlackSharkLcdDisplay(pos: number, items: number[]): Promise<BlackSharkLcdDisplayResult> {
    return (await SetBlackSharkLcdDisplay(pos, items)) as unknown as BlackSharkLcdDisplayResult;
  }

  /**
   * 导出当前设备配置快照到 JSON 文件（用户选路径；取消则 saved=false 且无 error）。
   */
  async exportBlackSharkConfigSnapshot(): Promise<BlackSharkConfigExportResult> {
    return (await ExportBlackSharkConfigSnapshot()) as unknown as BlackSharkConfigExportResult;
  }

  /**
   * 选一个快照文件并写回设备（Go 侧先弹确认框；取消则返回空结果）；打设备。
   */
  async restoreBlackSharkConfigFromFile(): Promise<BlackSharkConfigRestoreResult> {
    return (await RestoreBlackSharkConfigFromFile()) as unknown as BlackSharkConfigRestoreResult;
  }

  // 情景（按前台进程自动切换档位/灯效）

  /**
   * 读取情景规则与基准档位。
   */
  async getSceneRules(): Promise<SceneRulesPayload> {
    return (await GetSceneRules()) as unknown as SceneRulesPayload;
  }

  /** 保存情景规则与基准档位；返回逐字段的校验提示（可能非空但已保存）。 */
  async setSceneRules(payload: SceneRulesPayload): Promise<SceneRulesResult> {
    // Wails 生成的 models.SceneRulesPayload 带 convertValues 辅助方法，而这里传的是同构的普通对象，
    // 因此只做一次影响类型检查的 cast；Wails 按 JSON 序列化，运行时无差别。
    const arg = payload as unknown as Parameters<typeof SetSceneRules>[0];
    return (await SetSceneRules(arg)) as unknown as SceneRulesResult;
  }

  /** 可选作触发者的进程名（去重升序）；平台不支持时为空列表。 */
  async listSceneProcesses(): Promise<string[]> {
    return await ListSceneProcesses();
  }

  /** 内置屏保精选列表（对应官方 UI 的 10 个位置）。 */
  // ---- 屏幕/屏保图像 ----
  // 这些方法走 window.go 动态代理，不依赖 wailsjs 生成的强类型绑定。

  async listScreenPresets(): Promise<ScreenPresetListPayload> {
    const r = await (window as any).go?.main?.App?.ListScreenPresets?.();
    return r && typeof r === 'object' ? (r as ScreenPresetListPayload) : { presets: [] };
  }

  /** 按需取单张缩略图（data URL）；十张原图共约 870KB，一次全传会拖重首屏。 */
  async screenPresetThumbnail(officialPosition: number): Promise<string> {
    const r = await (window as any).go?.main?.App?.ScreenPresetThumbnail?.(officialPosition);
    return typeof r === 'string' ? r : '';
  }

  // 上传某张内置屏保到设备（整包重传 + 回读校验）；打设备。
  async uploadScreenPreset(officialPosition: number): Promise<ScreenUploadResultPayload> {
    const r = await (window as any).go?.main?.App?.UploadScreenPreset?.(officialPosition);
    return r && typeof r === 'object'
      ? (r as ScreenUploadResultPayload)
      : { success: false, error: 'no response', reports: 0, short: 0, timestamp: 0, size: 0, crc: 0, verified: false };
  }

  /**
   * 上传本地图片到设备屏保（可带缩放/平移）；打设备且耗时。
   */
  async uploadScreenImageFile(
    path: string,
    zoom = 0,
    offsetX = 0,
    offsetY = 0,
  ): Promise<ScreenUploadResultPayload> {
    const r = await (window as any).go?.main?.App?.UploadScreenImageFile?.(path, zoom, offsetX, offsetY);
    return r && typeof r === 'object'
      ? (r as ScreenUploadResultPayload)
      : { success: false, error: 'no response', reports: 0, short: 0, timestamp: 0, size: 0, crc: 0, verified: false };
  }

  /**
   * 列出历史图片（官方装备箱缓存的画布）：排除"设备当前那张"，取最新 limit 张。
   * 纯本机 IO，不打设备；excludeCrc 直接传 0xC5 回读到的那个。
   */
  async listScreenHistoryImages(excludeCrc = 0, limit = 2): Promise<ScreenHistoryListPayload> {
    const call = (window as any).go?.main?.App?.ListScreenHistoryImages;
    if (typeof call !== 'function') return { dir: '', items: [], error: 'no binding' };
    const r = await call(excludeCrc, limit);
    return r && typeof r === 'object' ? (r as ScreenHistoryListPayload) : { dir: '', items: [], error: 'no response' };
  }

  /** 把一张历史画布**原样直传**上屏（`*.bin` 免解码、免裁剪）；打设备。 */
  async uploadScreenHistoryImage(path: string): Promise<ScreenUploadResultPayload> {
    const r = await (window as any).go?.main?.App?.UploadScreenHistoryImage?.(path);
    return r && typeof r === 'object'
      ? (r as ScreenUploadResultPayload)
      : { success: false, error: 'no response', reports: 0, short: 0, timestamp: 0, size: 0, crc: 0, verified: false };
  }

  /** 删除官方缓存目录里的一张历史画布；true 只表示删除成功。 */
  async deleteScreenHistoryImage(path: string): Promise<boolean> {
    const r = await (window as any).go?.main?.App?.DeleteScreenHistoryImage?.(path);
    return r === true;
  }

  /**
   * 取当前裁剪参数下的预览（data URL）；纯本机运算，不需要设备在线。
   */
  async previewScreenImageCrop(
    path: string,
    zoom: number,
    offsetX: number,
    offsetY: number,
  ): Promise<ScreenCropPreviewPayload> {
    const empty: ScreenCropPreviewPayload = {
      dataUrl: '', width: 0, height: 0, rawWidth: 0, rawHeight: 0, slackX: 0, slackY: 0,
    };
    const r = await (window as any).go?.main?.App?.PreviewScreenImageCrop?.(path, zoom, offsetX, offsetY);
    return r && typeof r === 'object' ? (r as ScreenCropPreviewPayload) : empty;
  }

  /** 打开系统文件选择框，返回路径；取消返回空串。 */
  async pickScreenImageFile(): Promise<string> {
    const r = await (window as any).go?.main?.App?.PickScreenImageFile?.();
    return typeof r === 'string' ? r : '';
  }

  async getScreenImageInfo(): Promise<ScreenImageInfoPayload> {
    const r = await (window as any).go?.main?.App?.GetScreenImageInfo?.();
    return r && typeof r === 'object'
      ? (r as ScreenImageInfoPayload)
      : { hasImage: false, timestamp: 0, size: 0, crc: 0 };
  }

  /**
   * 从设备读回当前屏上那张图（0xC7 + 反复 0xC8，约 2096 帧）；重设备 IO。
   */
  async readScreenImageFromDevice(): Promise<ScreenImageReadPayload> {
    const call = (window as any).go?.main?.App?.ReadScreenImageFromDevice;
    if (typeof call !== 'function') {
      throw new Error('ReadScreenImageFromDevice 绑定缺失（请重新生成 wails 绑定）');
    }
    const r = await call();
    return r && typeof r === 'object' ? (r as ScreenImageReadPayload) : { ok: false, bytes: 0, crc: 0 };
  }

  // 开关黑鲨 RGB 灯效（0x10/0x11）；打设备。
  async setBlackSharkLightingEnabled(enabled: boolean): Promise<boolean> {
    return await SetBlackSharkLightingEnabled(enabled);
  }

  /** 开关黑鲨 LCD 小屏（0xC0/0xC1）；打设备。 */
  async setBlackSharkLcdScreenEnabled(enabled: boolean): Promise<boolean> {
    return await SetBlackSharkLcdScreenEnabled(enabled);
  }

  /**
   * 官方「灯效页 · 重置」：把 8 个灯效槽位全部恢复为出厂灯效；打设备。
   */
  async resetBlackSharkRgb(): Promise<boolean> {
    const call = (window as any).go?.main?.App?.ResetBlackSharkRgb;
    if (typeof call !== 'function') {
      throw new Error('ResetBlackSharkRgb 绑定缺失（请重新生成 wails 绑定）');
    }
    return !!(await call());
  }

  /**
   * 官方「散热页 · 重置」：把 4 个档位的冷却配置恢复为出厂值；打设备。
   */
  async resetBlackSharkCooling(): Promise<boolean> {
    const call = (window as any).go?.main?.App?.ResetBlackSharkCooling;
    if (typeof call !== 'function') {
      throw new Error('ResetBlackSharkCooling 绑定缺失（请重新生成 wails 绑定）');
    }
    return !!(await call());
  }
}

export const apiService = new ApiService();
