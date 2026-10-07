// 应用类型定义

// 风扇曲线点
export interface FanCurvePoint {
  temperature: number; // 温度 °C
  rpm: number;         // 转速 RPM
}

export interface FanCurveProfile {
  id: string;
  name: string;
  curve: FanCurvePoint[];
}

export interface FlyDigiRuntimeCapability {
  available: boolean;
  gearSettings: number;
  maxGearCode?: number;
  maxGearLabel?: string;
  maxGearIndex?: number;
  maxRpm?: number;
  selectedGearCode?: number;
  selectedGear?: string;
  source?: string;
  reason?: string;
}

// 风扇数据结构
export interface FanData {
  reportId: number;
  magicSync: number;
  command: number;
  status: number;
  gearSettings: number;
  currentMode: number;
  reserved1: number;
  currentRpm: number;
  targetRpm: number;
  maxGear: string;
  setGear: string;
  workMode: string;
  transport?: string;
  speedUnit?: string;
  flyDigiCapability?: FlyDigiRuntimeCapability;
}

export type GPUReadState = 'active' | 'notPolled' | 'unavailable' | 'error' | 'unknown';
export type TelemetryState = 'fresh' | 'delayed' | 'unavailable';

// 温度数据
export interface TemperatureData {
  cpuTemp: number;     // CPU温度
  gpuTemp: number;     // GPU温度
  cpuPowerWatts?: number;
  gpuPowerWatts?: number;
  gpuReadState?: GPUReadState;
  maxTemp: number;     // 最高温度
  controlTemp?: number; // 当前控温基准温度
  controlSource?: 'max' | 'cpu' | 'gpu'; // 当前控温基准来源
  cpuModel?: string;   // 当前识别的 CPU 型号
  gpuModel?: string;   // 当前识别的 GPU 型号
  cpuSensors?: TemperatureSensor[]; // 当前识别的 CPU 温度传感器
  gpuSensors?: TemperatureSensor[]; // 当前识别的 GPU 温度传感器
  cpuPowerSensors?: PowerSensor[]; // 当前识别的 CPU 功耗传感器
  gpuPowerSensors?: PowerSensor[]; // 当前识别的 GPU 功耗传感器
  updateTime: number;  // 更新时间戳
  bridgeOk?: boolean;  // 桥接程序是否正常
  bridgeMessage?: string; // 桥接程序提示
  telemetrySource?: 'bridge' | 'bridge-cache' | 'local' | 'wmi' | 'nvidia' | 'unknown' | string;
  cpuTelemetrySource?: string;
  gpuTelemetrySource?: string;
  telemetryFailureStage?: 'none' | 'starting' | 'transport' | 'empty' | 'enumeration' | 'selection' | 'fallback' | string;
  telemetryState?: TelemetryState;
}

export interface TemperatureSensor {
  key: string;
  name: string;
  value: number;
}

export interface PowerSensor {
  key: string;
  name: string;
  value: number;
}

// 应用配置
export interface AppConfig {
  legionFnQ?: LegionFnQConfig;
  legionFnQSupport?: LegionFnQSupportCache;
  deviceTransport?: string;
  fanControlDeviceIp?: string;
  wifiConnectionPriorityEnabled?: boolean;
  wifiSmartStartStopEnabled?: boolean;
  wifiSmartStartStopStandbySpeed?: number;
  autoControl: boolean;         // 智能变频开关
  curveProfileToggleHotkey?: string; // 切换曲线方案快捷键
  fanCurve: FanCurvePoint[];   // 风扇曲线
  fanCurveProfiles?: FanCurveProfile[];
  activeFanCurveProfileId?: string;
  gearLight: boolean;          // 挡位灯
  powerOnStart: boolean;       // 通电自启动
	windowsAutoStart: boolean;   // Windows开机自启动
	monitorOnly?: boolean;        // 仅监控模式
  // 主题模式：system/light/dark 为内置基础主题；其它字符串为自定义主题 id（如 'fancontrol-classic'）
  themeMode?: string;
  windowBlur?: 'acrylic' | 'mica' | 'tabbed' | 'off';
  smartStartStop: string;      // 智能启停
  brightness: number;          // 亮度
  tempUpdateRate: number;      // 温度更新频率(秒)
	  tempSampleCount?: number;
	  temperatureHistoryRetentionHours?: number;
  tempSource?: 'max' | 'cpu' | 'gpu';
  cpuSensor?: string;
  gpuSensor?: string;
  cpuPowerSensor?: string;
  gpuPowerSensor?: string;
  gpuReadMode?: 'auto' | 'always' | 'never';
  gpuLowPowerProtection?: boolean;
  configPath: string;          // 配置文件路径
  manualGear: string;          // 手动挡位设置
  manualLevel: string;         // 手动挡位级别(低中高)
  debugMode: boolean;          // 调试模式
  guiMonitoring: boolean;      // GUI监控开关
  customSpeedEnabled: boolean; // 自定义转速开关
  customSpeedRPM: number;      // 自定义转速值(无上下限)
  smartControl: SmartControlConfig; // 学习型智能控温
}

export interface SmartControlConfig {
  enabled: boolean;
  learning: boolean;
  learningBias: string;
  filterTransientSpike: boolean;
  targetTemp: number;
  aggressiveness: number;
  hysteresis: number;
  minRpmChange: number;
  rampUpLimit: number;
  rampDownLimit: number;
  learnRate: number;
  learnWindow: number;
  learnDelay: number;
  overheatWeight: number;
  rpmDeltaWeight: number;
  noiseWeight: number;
  trendGain: number;
  maxLearnOffset: number;
  temperatureRisePrediction?: boolean;
  temperatureRisePredictionMaxBoost?: number;
  learnedOffsets: number[];
  learnedOffsetsHeat: number[];
  learnedOffsetsCool: number[];
  learnedRateHeat: number[];
  learnedRateCool: number[];
}

// 调试信息
export interface FanGearTarget {
  gear: string;
  level: string;
}

export interface LegionFnQConfig {
  enabled: boolean;
  takeOverFan: boolean;
  modeMapping: Record<string, FanGearTarget>;
}

export interface LegionFnQSupportCache {
  checked: boolean;
  supported: boolean;
}

export interface LegionPowerModePayload {
  raw: number;
  mapped: number;
  mode: string;
  source: string;
  timestamp: number;
}

export interface LegionFnQSupportPayload {
  supported: boolean;
}

// BlackSharkCurveTempRange 黑鲨曲线那 4 个点能被拖到的温度取值域（℃，含两端）。
export interface BlackSharkCurveTempRange {
  minTempC: number;
  maxTempC: number;
}

export interface BlackSharkFirmwareStatus {
  supported: boolean;
  currentVersion?: string;
  latestVersion?: string;
  updateAvailable: boolean;
  minVersion?: string;
  firmwareUrl?: string;
  firmwareMd5?: string;
  checkedAt?: string;
  error?: string;
  /** 恒为 "official-tool"：本工具只检查版本，不刷写固件。 */
  updateMethod?: string;
  officialToolPath?: string;
  officialToolFound: boolean;
  belowMinVersion?: boolean;
}

export interface BlackSharkCurvePointView {
  tempC: number;
  /** 曲线里写入的字段值，不是实际转速。 */
  fieldRpm: number;
  /** 按标定表换算出的近似实际转速，仅供显示。 */
  approxActualRpm: number;
}

export interface BlackSharkCurveCalibrationEntry {
  fieldRpm: number;
  actualRpm: number;
}

export interface BlackSharkGear {
  /** 1..4，对应官方 UI 的低噪 / 均衡 / 强劲 / 极限。 */
  gear: number;
  fixedValue: number;
  fixedKnown: boolean;
  curve?: BlackSharkCurvePointView[];
  curveKnown: boolean;
}

export interface BlackSharkSwitchStates {
  available: boolean;
  lightingEnabled: boolean;
  lightingKnown: boolean;
  lcdScreenEnabled: boolean;
  lcdScreenKnown: boolean;
  coolingSource: number;
  coolingSourceKnown: boolean;
  /**
   * 智能启停 / 通电自启（写 CMD 0x02，读回 CMD 0x03）。
   */
  smartStartStop: boolean;
  powerOnSelfStart: boolean;
  onOffVectorKnown: boolean;
  onOffVectorReadbackSupported: boolean;
  /** 读回失败时的兜底记录，不是设备上报的状态。 */
  lastCommandedSmartStartStop: boolean;
  lastCommandedPowerOnSelfStart: boolean;
}

export interface BlackSharkInfo {
  available: boolean;
  firmware: BlackSharkFirmwareStatus;
  switches: BlackSharkSwitchStates;
  gears?: BlackSharkGear[];
  curveMinRpm: number;
  curveMaxRpm: number;
  curveCalibration?: BlackSharkCurveCalibrationEntry[];
  fixedMinRpm: number;
  fixedMaxRpm: number;
}

/** 一个滑块的量程（官方 UI 上的上下限）。 */
export interface ValueRangeView {
  min: number;
  max: number;
}

/**
 * 一条「情景」规则：按前台进程自动施加档位/灯效模式。
 */
export interface SceneRulePayload {
  enabled: boolean;
  match: string;
  gear: number;
  rgbMode: number;
  /**
   * 槽位名（可重命名，官方软件也是四个可改名槽位）。空 = 界面显示「情景 N」。
   */
  name?: string;
}

/** 情景规则与「无匹配时回落到的档位」（0=不改变，显式配置而非隐式记忆）。 */
export interface SceneRulesPayload {
  rules: SceneRulePayload[];
  baselineGear: number;
}

/** 保存结果：saved 表示已落盘；issues 是逐字段的校验提示（可能非空但已保存）。 */
export interface SceneRulesResult {
  saved: boolean;
  issues: string[];
}

/**
 * 一个灯效模式的参数。
 */
export interface BlackSharkRgbMode {
  index: number;
  name?: string;
  speed: number;
  brightness: number;
  red: number;
  green: number;
  blue: number;
  current: boolean;
  speedRange: ValueRangeView;
  brightnessRange: ValueRangeView;
  known: boolean;
  /**
   * 非空 = 设备有这个模式，但要主机持续喂数据才会动；文案说明怎么驱动。
   */
  needsHostData?: string;
  /**
   * true = 官方 UI 里该模式的速度滑块置灰、不可操作（值恒 0）。
   */
  speedDisabled?: boolean;
  /**
   * true = 设备当前该模式吃静态颜色（`0x12` 的 `payload[4]` = 0x01，官方 = 选中「单色」）。
   */
  staticColor: boolean;
  /**
   * 颜色下拉的选项号（`0x12` 的 `payload[0]` 高半字节，0..4）与它的名字。
   */
  colorOption: number;
  colorOptionName?: string;
  /**
   * 该灯效页实际显示哪些颜色控件（官方按页 hide/show）。
   */
  colorControls: { colorMode: boolean; singleColor: boolean; hue: 'none' | 'disabled' | 'singleColorOnly' | 'always' };
}

export interface BlackSharkRgbLighting {
  available: boolean;
  switchEnabled: boolean;
  switchKnown: boolean;
  modes?: BlackSharkRgbMode[];
  count: number;
  currentIndex: number;
  /** 恒为 true：颜色确实在写（色相 → RGB、单色/彩色、颜色下拉）；与 Go 侧一致。 */
  colorSettable: boolean;
  error?: string;
  /** 颜色下拉的全部选项（序号 + 名称，顺序即官方下拉顺序）。 */
  colorOptions?: { index: number; name: string }[];
  /**
   * true = 这份是缓存快照（上次读到的那份），不是刚刚读的。
   */
  fromCache?: boolean;
  /** `fromCache` 为真时，这份快照的读取时间（Unix 秒）。 */
  cachedAtUnix?: number;
}


/**
 * 主机侧灯效驱动的状态（槽位 6「响应」/ 槽位 7「音频同步」）。
 */
export interface BlackSharkHostEffects {
  /** 设备当前生效的灯效槽位（1..8）；0 = 未知或未连接。 */
  currentSlot: number;
  /** true：正在采集系统播放声音并按 ≈4.9 Hz 推 0x15。 */
  audioSyncRunning: boolean;
  /** 非空 = 采集启动失败及原因（用户看到"没反应"时的唯一线索）。 */
  audioError?: string;
  /** 最近一次算出的主频（Hz，0 = 静音）与据此折出的档位（0 = 静音未下发）。 */
  audioFrequencyHz: number;
  audioLevel: number;
  /** 本次运行累计下发的 0x15 帧数。 */
  audioPushCount: number;
  /** true：已装上全局键鼠低层钩子，按键/鼠标键按下会推 0x16。 */
  reactiveRunning: boolean;
  reactiveError?: string;
  /** 本次运行累计下发的 0x16 帧数。 */
  reactivePushCount: number;
}

export interface DebugInfo {
  debugMode: boolean;
  trayReady: boolean;
  trayInitialized: boolean;
  isConnected: boolean;
  autoReconnectSuppressed?: boolean;
  legionFnQSupported?: boolean;
  guiLastResponse: string;
  monitoringTemp: boolean;
  autoStartLaunch: boolean;
  pawnIOInstallerPath?: string;
  plugins?: Array<{ id: string; name: string; running: boolean; lastError?: string }>;
  blackSharkFirmware?: BlackSharkFirmwareStatus;
}

export interface DeviceDebugFrame {
  id: number;
  direction: string;
  transport: string;
  timestamp: string;
  rawHex: string;
  frameHex: string;
  command: string;
  length: number;
  payloadHex: string;
  checksumOk: boolean;
  checksumRule?: string;
  checksumExpected?: string;
  description: string;
  decoded?: string;
  parsed?: unknown;
}

export interface DeviceDebugCommandResult {
  transport: string;
  inputHex: string;
  frameHex: string;
  rawHex: string;
  waitMs: number;
  frames: DeviceDebugFrame[];
}

export interface DeviceGearRPM {
  gear: number;
  label: string;
  rpm: number;
}

export interface DeviceStatusRead {
  gearSetting?: string;
  maxGear?: string;
  selected?: string;
  mode?: string;
  modeName?: string;
  smartStartStop?: string;
  smartStartStopName?: string;
  currentRpm?: number;
  targetRpm?: number;
}

export interface DeviceSettings {
  available: boolean;
  source: string;
  readAt: string;
  model?: string;
  gearRpmTable?: DeviceGearRPM[];
  workMode?: string;
  workModeName?: string;
  rgbState?: string;
  rgbStateName?: string;
  status?: DeviceStatusRead;
  flyDigiCapability?: FlyDigiRuntimeCapability;
  rawFrames?: DeviceDebugFrame[];
}

// 自启动方式
export type AutoStartMethod = 'none' | 'task_scheduler' | 'registry';

// 自启动信息
export interface AutoStartInfo {
  enabled: boolean;
  method: AutoStartMethod;
  isAdmin: boolean;
}

// 挡位命令
export interface GearCommand {
  name: string;    // 挡位名称
  command: number[]; // 命令字节
  rpm: number;     // 对应转速
}

// 设备状态
export interface DeviceStatus {
  connected: boolean;
  monitoring: boolean;
  currentData: FanData | null;
  temperature: TemperatureData;
  productId?: string;
  model?: string;
  deviceName?: string;
}

// 自定义主题元数据（由后端 ListThemes 返回）
export interface ThemeMeta {
  id: string;
  name: string;
  base: string;        // light | dark
  author?: string;
  version?: string;
  description?: string;
  layer?: 'basic' | 'advanced' | string; // basic | advanced
  contract?: string;
  source: string;      // user | install | builtin
}

// 设备信息
export interface DeviceInfo {
  manufacturer: string;
  product: string;
  serial: string;
  model?: string;
  deviceName?: string;
  productId?: string;
  transport?: string;
  endpoint?: string;
  currentData?: import('../../../wailsjs/go/models').types.FanData | null;
  deviceSettings?: DeviceSettings | null;
  deviceProfile?: import('../../../wailsjs/go/models').types.DeviceProfile | null;
  deviceCapabilities?: import('../../../wailsjs/go/models').types.DeviceCapabilities | null;
  runtime?: { state?: string };
}

/** 一张内置屏保精选（officialPosition 是官方 UI 里的位置 1..10）。 */
export interface ScreenPresetPayload {
  officialPosition: number;
  assetIndex: number;
  name: string;
}

export interface ScreenPresetListPayload {
  presets: ScreenPresetPayload[];
}

/**
 * 一张历史画布：官方装备箱自己缓存的 `*.bin`。
 * 内容恒为 121552 字节（428×142 RGB565 大端）——与本项目的画布格式一致，所以能原样直传。
 */
export interface ScreenHistoryItemPayload {
  name: string;
  path: string;
  /** 文件名里的 Unix 秒（= 官方那次上屏的时刻）。 */
  unixTime: number;
  modified: string;
  size: number;
  /** `data:image/png;base64,…`；只对要显示的那几张生成。 */
  thumb?: string;
}

/** 历史图片列表。error 非空表示这一轮没读到（还没缓存过 / 没权限），此时 items 为空。 */
export interface ScreenHistoryListPayload {
  /** 扫描的目录（界面要如实告诉用户图片来自哪里）。 */
  dir: string;
  items: ScreenHistoryItemPayload[];
  /**
   * "设备当前正在用那张"在本机缓存里的对应项（CRC 相同才给），带 thumb。
   * 顶部预览用它，省掉几十秒的"从设备读回"；对不上就是 undefined。
   */
  current?: ScreenHistoryItemPayload;
  error?: string;
}

/** 设备当前保存的屏保图像信息（0xC5 回读）。 */
export interface ScreenImageInfoPayload {
  hasImage: boolean;
  timestamp: number;
  size: number;
  crc: number;
  /**
   * true = 设备当前这张图命中某个内置精选（认图结果）。
   */
  presetMatched?: boolean;
  /** 官方位次 1..10，给缩略图接口用。 */
  presetOfficialPosition?: number;
  presetName?: string;
}

/** 从设备读回当前屏图的结果（data URL 供界面直接显示）。 */
export interface ScreenImageReadPayload {
  ok: boolean;
  dataUrl?: string;
  /** 读回的原始字节数（应为 121552）。 */
  bytes: number;
  /** 本机对读回内容重算的 CRC（可与 info 里"上传者声称的"那个对照）。 */
  crc: number;
  error?: string;
  /**
   * true = 这是缓存快照（上次读到的那份），不是刚刚读的。
   */
  fromCache?: boolean;
  /** `fromCache` 为真时，这份快照的读取时间（Unix 秒）。 */
  cachedAtUnix?: number;
}

/**
 * 手动裁剪的预览。
 */
export interface ScreenCropPreviewPayload {
  /** 按将上传的那份像素编出的 PNG data URL（所见即所传）。 */
  dataUrl: string;
  width: number;
  height: number;
  rawWidth: number;
  rawHeight: number;
  /**
   * 该方向上"还有多少余量"（0 = 这个方向没有可平移的空间）。
   */
  slackX: number;
  slackY: number;
}

/** 一次屏保图像上传的结果。 */
export interface ScreenUploadResultPayload {
  success: boolean;
  error?: string;
  reports: number;
  short: number;
  timestamp: number;
  size: number;
  crc: number;
  verified: boolean;
}

/** 一次配置快照导出的结果（saved=false 且无 error 表示用户取消）。 */
export interface BlackSharkConfigExportResult {
  saved: boolean;
  path?: string;
  error?: string;
}

/**
 * 从快照还原的结果。
 */
export interface BlackSharkConfigRestoreResult {
  restored: string[];
  issues: string[];
}

/**
 * 屏幕显示参数（CMD 0xC2）。
 */
export interface BlackSharkLcdDisplayPayload {
  pos: number;
  items: number[];
  /** 可选项的有序 id 列表（协议事实，由 Go 侧给出；前端只把 id 翻成文案）。 */
  options: number[];
  /**
   * 官方「重置屏幕设置」的效果 = 写回这一组（由 Go 侧 DefaultBlackSharkLcdItems() 给出）。
   */
  defaultItems: number[];
}

/**
 * 屏幕显示参数的下发结果。
 */
export interface BlackSharkLcdDisplayResult {
  saved: boolean;
  applied: boolean;
  pos: number;
  items: number[];
  error?: string;
}
