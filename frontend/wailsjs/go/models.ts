export namespace theme {

	export class Meta {
	    id: string;
	    name: string;
	    base: string;
	    author?: string;
	    version?: string;
	    description?: string;
	    source: string;

	    static createFrom(source: any = {}) {
	        return new Meta(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.base = source["base"];
	        this.author = source["author"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.source = source["source"];
	    }
	}

}

export namespace types {

	export class RGBColor {
	    r: number;
	    g: number;
	    b: number;

	    static createFrom(source: any = {}) {
	        return new RGBColor(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.r = source["r"];
	        this.g = source["g"];
	        this.b = source["b"];
	    }
	}
	export class LightStripConfig {
	    mode: string;
	    speed: string;
	    brightness: number;
	    colors: RGBColor[];

	    static createFrom(source: any = {}) {
	        return new LightStripConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.speed = source["speed"];
	        this.brightness = source["brightness"];
	        this.colors = this.convertValues(source["colors"], RGBColor);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SmartControlConfig {
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
	    learnedOffsets: number[];
	    learnedOffsetsHeat: number[];
	    learnedOffsetsCool: number[];
	    learnedRateHeat: number[];
	    learnedRateCool: number[];
	    learnedOffsetsByProfile?: Record<string, Array<number>>;
	    temperatureRisePrediction: boolean;
	    temperatureRisePredictionMaxBoost: number;

	    static createFrom(source: any = {}) {
	        return new SmartControlConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.learning = source["learning"];
	        this.learningBias = source["learningBias"];
	        this.filterTransientSpike = source["filterTransientSpike"];
	        this.targetTemp = source["targetTemp"];
	        this.aggressiveness = source["aggressiveness"];
	        this.hysteresis = source["hysteresis"];
	        this.minRpmChange = source["minRpmChange"];
	        this.rampUpLimit = source["rampUpLimit"];
	        this.rampDownLimit = source["rampDownLimit"];
	        this.learnRate = source["learnRate"];
	        this.learnWindow = source["learnWindow"];
	        this.learnDelay = source["learnDelay"];
	        this.overheatWeight = source["overheatWeight"];
	        this.rpmDeltaWeight = source["rpmDeltaWeight"];
	        this.noiseWeight = source["noiseWeight"];
	        this.trendGain = source["trendGain"];
	        this.maxLearnOffset = source["maxLearnOffset"];
	        this.learnedOffsets = source["learnedOffsets"];
	        this.learnedOffsetsHeat = source["learnedOffsetsHeat"];
	        this.learnedOffsetsCool = source["learnedOffsetsCool"];
	        this.learnedRateHeat = source["learnedRateHeat"];
	        this.learnedRateCool = source["learnedRateCool"];
	        this.learnedOffsetsByProfile = source["learnedOffsetsByProfile"];
	        this.temperatureRisePrediction = source["temperatureRisePrediction"];
	        this.temperatureRisePredictionMaxBoost = source["temperatureRisePredictionMaxBoost"];
	    }
	}
	export class FanCurveProfile {
	    id: string;
	    name: string;
	    curve: FanCurvePoint[];

	    static createFrom(source: any = {}) {
	        return new FanCurveProfile(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.curve = this.convertValues(source["curve"], FanCurvePoint);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FanCurvePoint {
	    temperature: number;
	    rpm: number;

	    static createFrom(source: any = {}) {
	        return new FanCurvePoint(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.temperature = source["temperature"];
	        this.rpm = source["rpm"];
	    }
	}
	export class DeviceCapabilities {
	    profileId?: string;
	    displayName?: string;
	    transport: string;
	    speedUnit: string;
	    speedRange: DeviceSpeedRange;
	    supportsReadState: boolean;
	    supportsSetSpeed: boolean;
	    supportsManualGears: boolean;
	    supportsCustomSpeed: boolean;
	    supportsDebugFrames: boolean;
	    supportsRawCommands: boolean;
	    supportsGearLight: boolean;
	    supportsLighting: boolean;
	    supportsBrightness: boolean;
	    supportsScreen: boolean;
	    supportsPowerOnStart: boolean;
	    supportsSmartStartStop: boolean;

	    static createFrom(source: any = {}) {
	        return new DeviceCapabilities(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.displayName = source["displayName"];
	        this.transport = source["transport"];
	        this.speedUnit = source["speedUnit"];
	        this.speedRange = this.convertValues(source["speedRange"], DeviceSpeedRange);
	        this.supportsReadState = source["supportsReadState"];
	        this.supportsSetSpeed = source["supportsSetSpeed"];
	        this.supportsManualGears = source["supportsManualGears"];
	        this.supportsCustomSpeed = source["supportsCustomSpeed"];
	        this.supportsDebugFrames = source["supportsDebugFrames"];
	        this.supportsRawCommands = source["supportsRawCommands"];
	        this.supportsGearLight = source["supportsGearLight"];
	        this.supportsLighting = source["supportsLighting"];
	        this.supportsBrightness = source["supportsBrightness"];
	        this.supportsScreen = source["supportsScreen"];
	        this.supportsPowerOnStart = source["supportsPowerOnStart"];
	        this.supportsSmartStartStop = source["supportsSmartStartStop"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DeviceSpeedMapPoint {
	    percentTicks: number;
	    rpm: number;

	    static createFrom(source: any = {}) {
	        return new DeviceSpeedMapPoint(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.percentTicks = source["percentTicks"];
	        this.rpm = source["rpm"];
	    }
	}
	export class DeviceResponseParser {
	    name: string;
	    type: string;
	    expression?: string;

	    static createFrom(source: any = {}) {
	        return new DeviceResponseParser(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.expression = source["expression"];
	    }
	}
	export class DeviceCommandTemplate {
	    name: string;
	    command: string;
	    encoding?: string;
	    checksum?: string;
	    description?: string;

	    static createFrom(source: any = {}) {
	        return new DeviceCommandTemplate(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.command = source["command"];
	        this.encoding = source["encoding"];
	        this.checksum = source["checksum"];
	        this.description = source["description"];
	    }
	}
	export class DeviceConnectionSettings {
	    endpoint?: string;
	    stateEndpoint?: string;
	    speedEndpoint?: string;
	    httpMethod?: string;
	    requestTimeoutMs?: number;
	    minSendIntervalMs?: number;
	    maxRetries?: number;
	    retryBackoffMs?: number;
	    bleNameFilter?: string;
	    bleServiceUuid?: string;
	    bleWriteCharacteristic?: string;
	    bleNotifyCharacteristic?: string;
	    bleWriteWithResponse?: boolean;
	    serialPort?: string;
	    serialBaudRate?: number;
	    serialDataBits?: number;
	    serialStopBits?: number;
	    serialParity?: string;
	    serialFrameDelimiter?: string;

	    static createFrom(source: any = {}) {
	        return new DeviceConnectionSettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = source["endpoint"];
	        this.stateEndpoint = source["stateEndpoint"];
	        this.speedEndpoint = source["speedEndpoint"];
	        this.httpMethod = source["httpMethod"];
	        this.requestTimeoutMs = source["requestTimeoutMs"];
	        this.minSendIntervalMs = source["minSendIntervalMs"];
	        this.maxRetries = source["maxRetries"];
	        this.retryBackoffMs = source["retryBackoffMs"];
	        this.bleNameFilter = source["bleNameFilter"];
	        this.bleServiceUuid = source["bleServiceUuid"];
	        this.bleWriteCharacteristic = source["bleWriteCharacteristic"];
	        this.bleNotifyCharacteristic = source["bleNotifyCharacteristic"];
	        this.bleWriteWithResponse = source["bleWriteWithResponse"];
	        this.serialPort = source["serialPort"];
	        this.serialBaudRate = source["serialBaudRate"];
	        this.serialDataBits = source["serialDataBits"];
	        this.serialStopBits = source["serialStopBits"];
	        this.serialParity = source["serialParity"];
	        this.serialFrameDelimiter = source["serialFrameDelimiter"];
	    }
	}
	export class DeviceSpeedRange {
	    min: number;
	    max: number;
	    step?: number;
	    tickScale?: number;

	    static createFrom(source: any = {}) {
	        return new DeviceSpeedRange(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.min = source["min"];
	        this.max = source["max"];
	        this.step = source["step"];
	        this.tickScale = source["tickScale"];
	    }
	}
	export class DeviceProfile {
	    id: string;
	    displayName: string;
	    vendor?: string;
	    model?: string;
	    notes?: string;
	    builtIn?: boolean;
	    transport: string;
	    speedUnit: string;
	    speedRange: DeviceSpeedRange;
	    connection?: DeviceConnectionSettings;
	    commands?: DeviceCommandTemplate[];
	    responseParsers?: DeviceResponseParser[];
	    speedMap?: DeviceSpeedMapPoint[];
	    capabilities: DeviceCapabilities;

	    static createFrom(source: any = {}) {
	        return new DeviceProfile(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	        this.vendor = source["vendor"];
	        this.model = source["model"];
	        this.notes = source["notes"];
	        this.builtIn = source["builtIn"];
	        this.transport = source["transport"];
	        this.speedUnit = source["speedUnit"];
	        this.speedRange = this.convertValues(source["speedRange"], DeviceSpeedRange);
	        this.connection = this.convertValues(source["connection"], DeviceConnectionSettings);
	        this.commands = this.convertValues(source["commands"], DeviceCommandTemplate);
	        this.responseParsers = this.convertValues(source["responseParsers"], DeviceResponseParser);
	        this.speedMap = this.convertValues(source["speedMap"], DeviceSpeedMapPoint);
	        this.capabilities = this.convertValues(source["capabilities"], DeviceCapabilities);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LegionFnQSupportCache {
	    checked: boolean;
	    supported: boolean;

	    static createFrom(source: any = {}) {
	        return new LegionFnQSupportCache(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.checked = source["checked"];
	        this.supported = source["supported"];
	    }
	}
	export class FanGearTarget {
	    gear: string;
	    level: string;

	    static createFrom(source: any = {}) {
	        return new FanGearTarget(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gear = source["gear"];
	        this.level = source["level"];
	    }
	}
	export class LegionFnQConfig {
	    enabled: boolean;
	    takeOverFan: boolean;
	    modeMapping: Record<string, FanGearTarget>;

	    static createFrom(source: any = {}) {
	        return new LegionFnQConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.takeOverFan = source["takeOverFan"];
	        this.modeMapping = this.convertValues(source["modeMapping"], FanGearTarget, true);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AppConfig {
	    legionFnQ: LegionFnQConfig;
	    legionFnQSupport: LegionFnQSupportCache;
	    activeDeviceProfileId: string;
	    activeDeviceProfileIdsByTransport?: Record<string, string>;
	    deviceProfiles?: DeviceProfile[];
	    deviceTransport: string;
	    fanControlDeviceIp: string;
	    wifiCompatibilityEnabled: boolean;
	    wifiDynamicIpCompatibilityEnabled: boolean;
	    wifiSmartStartStopEnabled: boolean;
	    wifiSmartStartStopStandbySpeed: number;
	    serialCompatibilityEnabled: boolean;
	    autoControl: boolean;
	    manualGearToggleHotkey: string;
	    autoControlToggleHotkey: string;
	    curveProfileToggleHotkey: string;
	    manualGearLevels: Record<string, string>;
	    manualGearRpm: Record<string, any>;
	    fanCurve: FanCurvePoint[];
	    fanCurveProfiles: FanCurveProfile[];
	    activeFanCurveProfileId: string;
	    gearLight: boolean;
	    powerOnStart: boolean;
	    windowsAutoStart: boolean;
	    themeMode: string;
	    smartStartStop: string;
	    brightness: number;
	    tempUpdateRate: number;
	    tempSampleCount: number;
	    tempSource: string;
	    gpuDevice: string;
	    cpuSensor: string;
	    gpuSensor: string;
	    configPath: string;
	    manualGear: string;
	    manualLevel: string;
	    debugMode: boolean;
	    guiMonitoring: boolean;
	    customSpeedEnabled: boolean;
	    customSpeedRPM: number;
	    ignoreDeviceOnReconnect: boolean;
	    smartControl: SmartControlConfig;
	    lightStrip: LightStripConfig;

	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.legionFnQ = this.convertValues(source["legionFnQ"], LegionFnQConfig);
	        this.legionFnQSupport = this.convertValues(source["legionFnQSupport"], LegionFnQSupportCache);
	        this.activeDeviceProfileId = source["activeDeviceProfileId"];
	        this.activeDeviceProfileIdsByTransport = source["activeDeviceProfileIdsByTransport"];
	        this.deviceProfiles = this.convertValues(source["deviceProfiles"], DeviceProfile);
	        this.deviceTransport = source["deviceTransport"];
	        this.fanControlDeviceIp = source["fanControlDeviceIp"];
	        this.wifiCompatibilityEnabled = source["wifiCompatibilityEnabled"];
	        this.wifiDynamicIpCompatibilityEnabled = source["wifiDynamicIpCompatibilityEnabled"];
	        this.wifiSmartStartStopEnabled = source["wifiSmartStartStopEnabled"];
	        this.wifiSmartStartStopStandbySpeed = source["wifiSmartStartStopStandbySpeed"];
	        this.serialCompatibilityEnabled = source["serialCompatibilityEnabled"];
	        this.autoControl = source["autoControl"];
	        this.manualGearToggleHotkey = source["manualGearToggleHotkey"];
	        this.autoControlToggleHotkey = source["autoControlToggleHotkey"];
	        this.curveProfileToggleHotkey = source["curveProfileToggleHotkey"];
	        this.manualGearLevels = source["manualGearLevels"];
	        this.manualGearRpm = source["manualGearRpm"];
	        this.fanCurve = this.convertValues(source["fanCurve"], FanCurvePoint);
	        this.fanCurveProfiles = this.convertValues(source["fanCurveProfiles"], FanCurveProfile);
	        this.activeFanCurveProfileId = source["activeFanCurveProfileId"];
	        this.gearLight = source["gearLight"];
	        this.powerOnStart = source["powerOnStart"];
	        this.windowsAutoStart = source["windowsAutoStart"];
	        this.themeMode = source["themeMode"];
	        this.smartStartStop = source["smartStartStop"];
	        this.brightness = source["brightness"];
	        this.tempUpdateRate = source["tempUpdateRate"];
	        this.tempSampleCount = source["tempSampleCount"];
	        this.tempSource = source["tempSource"];
	        this.gpuDevice = source["gpuDevice"];
	        this.cpuSensor = source["cpuSensor"];
	        this.gpuSensor = source["gpuSensor"];
	        this.configPath = source["configPath"];
	        this.manualGear = source["manualGear"];
	        this.manualLevel = source["manualLevel"];
	        this.debugMode = source["debugMode"];
	        this.guiMonitoring = source["guiMonitoring"];
	        this.customSpeedEnabled = source["customSpeedEnabled"];
	        this.customSpeedRPM = source["customSpeedRPM"];
	        this.ignoreDeviceOnReconnect = source["ignoreDeviceOnReconnect"];
	        this.smartControl = this.convertValues(source["smartControl"], SmartControlConfig);
	        this.lightStrip = this.convertValues(source["lightStrip"], LightStripConfig);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BLEManufacturerData {
	    companyId: number;
	    dataHex?: string;

	    static createFrom(source: any = {}) {
	        return new BLEManufacturerData(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.companyId = source["companyId"];
	        this.dataHex = source["dataHex"];
	    }
	}
	export class BLEDeviceInfo {
	    address: string;
	    name?: string;
	    rssi: number;
	    serviceUuids?: string[];
	    manufacturerData?: BLEManufacturerData[];
	    writeCharacteristicUuids?: string[];
	    notifyCharacteristicUuids?: string[];
	    matched: boolean;
	    matchScore?: number;
	    matchReasons?: string[];
	    matchedProfileId?: string;
	    matchedProfileDisplayName?: string;
	    suggestedNameFilter?: string;
	    suggestedServiceUuid?: string;
	    suggestedWriteCharacteristic?: string;
	    suggestedNotifyCharacteristic?: string;

	    static createFrom(source: any = {}) {
	        return new BLEDeviceInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.name = source["name"];
	        this.rssi = source["rssi"];
	        this.serviceUuids = source["serviceUuids"];
	        this.manufacturerData = this.convertValues(source["manufacturerData"], BLEManufacturerData);
	        this.writeCharacteristicUuids = source["writeCharacteristicUuids"];
	        this.notifyCharacteristicUuids = source["notifyCharacteristicUuids"];
	        this.matched = source["matched"];
	        this.matchScore = source["matchScore"];
	        this.matchReasons = source["matchReasons"];
	        this.matchedProfileId = source["matchedProfileId"];
	        this.matchedProfileDisplayName = source["matchedProfileDisplayName"];
	        this.suggestedNameFilter = source["suggestedNameFilter"];
	        this.suggestedServiceUuid = source["suggestedServiceUuid"];
	        this.suggestedWriteCharacteristic = source["suggestedWriteCharacteristic"];
	        this.suggestedNotifyCharacteristic = source["suggestedNotifyCharacteristic"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BLEGATTCharacteristicInfo {
	    uuid: string;
	    properties?: string[];
	    canRead?: boolean;
	    canWrite?: boolean;
	    canWriteWithoutResponse?: boolean;
	    canNotify?: boolean;
	    canIndicate?: boolean;
	    mtu?: number;

	    static createFrom(source: any = {}) {
	        return new BLEGATTCharacteristicInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uuid = source["uuid"];
	        this.properties = source["properties"];
	        this.canRead = source["canRead"];
	        this.canWrite = source["canWrite"];
	        this.canWriteWithoutResponse = source["canWriteWithoutResponse"];
	        this.canNotify = source["canNotify"];
	        this.canIndicate = source["canIndicate"];
	        this.mtu = source["mtu"];
	    }
	}
	export class BLEGATTProbeParams {
	    timeoutMs?: number;
	    address?: string;
	    serviceUuid?: string;
	    profile: DeviceProfile;

	    static createFrom(source: any = {}) {
	        return new BLEGATTProbeParams(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timeoutMs = source["timeoutMs"];
	        this.address = source["address"];
	        this.serviceUuid = source["serviceUuid"];
	        this.profile = this.convertValues(source["profile"], DeviceProfile);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BLEGATTServiceInfo {
	    uuid: string;
	    characteristics?: BLEGATTCharacteristicInfo[];
	    error?: string;

	    static createFrom(source: any = {}) {
	        return new BLEGATTServiceInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uuid = source["uuid"];
	        this.characteristics = this.convertValues(source["characteristics"], BLEGATTCharacteristicInfo);
	        this.error = source["error"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BLEGATTProbeResult {
	    address?: string;
	    name?: string;
	    services?: BLEGATTServiceInfo[];
	    suggestedServiceUuid?: string;
	    suggestedWriteCharacteristic?: string;
	    suggestedNotifyCharacteristic?: string;

	    static createFrom(source: any = {}) {
	        return new BLEGATTProbeResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.name = source["name"];
	        this.services = this.convertValues(source["services"], BLEGATTServiceInfo);
	        this.suggestedServiceUuid = source["suggestedServiceUuid"];
	        this.suggestedWriteCharacteristic = source["suggestedWriteCharacteristic"];
	        this.suggestedNotifyCharacteristic = source["suggestedNotifyCharacteristic"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}


	export class BLEScanParams {
	    timeoutMs?: number;
	    nameFilter?: string;
	    serviceUuid?: string;
	    writeCharacteristicUuid?: string;
	    notifyCharacteristicUuid?: string;
	    onlyMatched?: boolean;
	    profiles?: DeviceProfile[];

	    static createFrom(source: any = {}) {
	        return new BLEScanParams(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timeoutMs = source["timeoutMs"];
	        this.nameFilter = source["nameFilter"];
	        this.serviceUuid = source["serviceUuid"];
	        this.writeCharacteristicUuid = source["writeCharacteristicUuid"];
	        this.notifyCharacteristicUuid = source["notifyCharacteristicUuid"];
	        this.onlyMatched = source["onlyMatched"];
	        this.profiles = this.convertValues(source["profiles"], DeviceProfile);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TemperatureGPUDevice {
	    key: string;
	    name: string;
	    vendor: string;
	    sensors: TemperatureSensor[];

	    static createFrom(source: any = {}) {
	        return new TemperatureGPUDevice(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.vendor = source["vendor"];
	        this.sensors = this.convertValues(source["sensors"], TemperatureSensor);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TemperatureSensor {
	    key: string;
	    name: string;
	    value: number;

	    static createFrom(source: any = {}) {
	        return new TemperatureSensor(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.value = source["value"];
	    }
	}
	export class BridgeTemperatureData {
	    cpuTemp: number;
	    gpuTemp: number;
	    cpuPowerWatts?: number;
	    gpuPowerWatts?: number;
	    maxTemp: number;
	    controlTemp: number;
	    controlSource: string;
	    selectedGpuDevice: string;
	    cpuModel: string;
	    gpuModel: string;
	    cpuSensors: TemperatureSensor[];
	    gpuSensors: TemperatureSensor[];
	    gpuDevices: TemperatureGPUDevice[];
	    updateTime: number;
	    success: boolean;
	    error: string;

	    static createFrom(source: any = {}) {
	        return new BridgeTemperatureData(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpuTemp = source["cpuTemp"];
	        this.gpuTemp = source["gpuTemp"];
	        this.cpuPowerWatts = source["cpuPowerWatts"];
	        this.gpuPowerWatts = source["gpuPowerWatts"];
	        this.maxTemp = source["maxTemp"];
	        this.controlTemp = source["controlTemp"];
	        this.controlSource = source["controlSource"];
	        this.selectedGpuDevice = source["selectedGpuDevice"];
	        this.cpuModel = source["cpuModel"];
	        this.gpuModel = source["gpuModel"];
	        this.cpuSensors = this.convertValues(source["cpuSensors"], TemperatureSensor);
	        this.gpuSensors = this.convertValues(source["gpuSensors"], TemperatureSensor);
	        this.gpuDevices = this.convertValues(source["gpuDevices"], TemperatureGPUDevice);
	        this.updateTime = source["updateTime"];
	        this.success = source["success"];
	        this.error = source["error"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}



	export class DeviceDebugFrame {
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
	    description: string;
	    decoded?: string;
	    parsed?: any;

	    static createFrom(source: any = {}) {
	        return new DeviceDebugFrame(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.direction = source["direction"];
	        this.transport = source["transport"];
	        this.timestamp = source["timestamp"];
	        this.rawHex = source["rawHex"];
	        this.frameHex = source["frameHex"];
	        this.command = source["command"];
	        this.length = source["length"];
	        this.payloadHex = source["payloadHex"];
	        this.checksumOk = source["checksumOk"];
	        this.description = source["description"];
	        this.decoded = source["decoded"];
	        this.parsed = source["parsed"];
	    }
	}
	export class DeviceDebugCommandResult {
	    transport: string;
	    inputHex: string;
	    frameHex: string;
	    rawHex: string;
	    waitMs: number;
	    frames: DeviceDebugFrame[];

	    static createFrom(source: any = {}) {
	        return new DeviceDebugCommandResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.transport = source["transport"];
	        this.inputHex = source["inputHex"];
	        this.frameHex = source["frameHex"];
	        this.rawHex = source["rawHex"];
	        this.waitMs = source["waitMs"];
	        this.frames = this.convertValues(source["frames"], DeviceDebugFrame);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

	export class DeviceGearRPM {
	    gear: number;
	    label: string;
	    rpm: number;

	    static createFrom(source: any = {}) {
	        return new DeviceGearRPM(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gear = source["gear"];
	        this.label = source["label"];
	        this.rpm = source["rpm"];
	    }
	}

	export class DeviceProfileTestParams {
	    profile: DeviceProfile;
	    action: string;
	    speedValue?: number;
	    timeoutMs?: number;

	    static createFrom(source: any = {}) {
	        return new DeviceProfileTestParams(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = this.convertValues(source["profile"], DeviceProfile);
	        this.action = source["action"];
	        this.speedValue = source["speedValue"];
	        this.timeoutMs = source["timeoutMs"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FanData {
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

	    static createFrom(source: any = {}) {
	        return new FanData(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reportId = source["reportId"];
	        this.magicSync = source["magicSync"];
	        this.command = source["command"];
	        this.status = source["status"];
	        this.gearSettings = source["gearSettings"];
	        this.currentMode = source["currentMode"];
	        this.reserved1 = source["reserved1"];
	        this.currentRpm = source["currentRpm"];
	        this.targetRpm = source["targetRpm"];
	        this.maxGear = source["maxGear"];
	        this.setGear = source["setGear"];
	        this.workMode = source["workMode"];
	        this.transport = source["transport"];
	        this.speedUnit = source["speedUnit"];
	    }
	}
	export class DeviceProfileTestResult {
	    action: string;
	    transport: string;
	    speedUnit: string;
	    profileId?: string;
	    displayName?: string;
	    connected: boolean;
	    durationMs: number;
	    message?: string;
	    requestedSpeedValue?: number;
	    fanData?: FanData;

	    static createFrom(source: any = {}) {
	        return new DeviceProfileTestResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.transport = source["transport"];
	        this.speedUnit = source["speedUnit"];
	        this.profileId = source["profileId"];
	        this.displayName = source["displayName"];
	        this.connected = source["connected"];
	        this.durationMs = source["durationMs"];
	        this.message = source["message"];
	        this.requestedSpeedValue = source["requestedSpeedValue"];
	        this.fanData = this.convertValues(source["fanData"], FanData);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DeviceProfilesPayload {
	    profiles: DeviceProfile[];
	    activeId: string;
	    activeIdsByTransport?: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new DeviceProfilesPayload(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profiles = this.convertValues(source["profiles"], DeviceProfile);
	        this.activeId = source["activeId"];
	        this.activeIdsByTransport = source["activeIdsByTransport"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

	export class DeviceStatusRead {
	    gearSetting?: string;
	    maxGear?: string;
	    selected?: string;
	    mode?: string;
	    modeName?: string;
	    smartStartStop?: string;
	    smartStartStopName?: string;
	    currentRpm?: number;
	    targetRpm?: number;

	    static createFrom(source: any = {}) {
	        return new DeviceStatusRead(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gearSetting = source["gearSetting"];
	        this.maxGear = source["maxGear"];
	        this.selected = source["selected"];
	        this.mode = source["mode"];
	        this.modeName = source["modeName"];
	        this.smartStartStop = source["smartStartStop"];
	        this.smartStartStopName = source["smartStartStopName"];
	        this.currentRpm = source["currentRpm"];
	        this.targetRpm = source["targetRpm"];
	    }
	}
	export class DeviceSettings {
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
	    rawFrames?: DeviceDebugFrame[];

	    static createFrom(source: any = {}) {
	        return new DeviceSettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.source = source["source"];
	        this.readAt = source["readAt"];
	        this.model = source["model"];
	        this.gearRpmTable = this.convertValues(source["gearRpmTable"], DeviceGearRPM);
	        this.workMode = source["workMode"];
	        this.workModeName = source["workModeName"];
	        this.rgbState = source["rgbState"];
	        this.rgbStateName = source["rgbStateName"];
	        this.status = this.convertValues(source["status"], DeviceStatusRead);
	        this.rawFrames = this.convertValues(source["rawFrames"], DeviceDebugFrame);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}





	export class FanCurveProfilesPayload {
	    profiles: FanCurveProfile[];
	    activeId: string;

	    static createFrom(source: any = {}) {
	        return new FanCurveProfilesPayload(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profiles = this.convertValues(source["profiles"], FanCurveProfile);
	        this.activeId = source["activeId"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}






	export class SerialPortInfo {
	    name: string;
	    path?: string;
	    displayName?: string;
	    source?: string;

	    static createFrom(source: any = {}) {
	        return new SerialPortInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.displayName = source["displayName"];
	        this.source = source["source"];
	    }
	}

	export class TemperatureData {
	    cpuTemp: number;
	    gpuTemp: number;
	    cpuPowerWatts?: number;
	    gpuPowerWatts?: number;
	    maxTemp: number;
	    controlTemp: number;
	    controlSource: string;
	    selectedGpuDevice: string;
	    cpuModel: string;
	    gpuModel: string;
	    cpuSensors: TemperatureSensor[];
	    gpuSensors: TemperatureSensor[];
	    gpuDevices: TemperatureGPUDevice[];
	    updateTime: number;
	    bridgeOk: boolean;
	    bridgeMessage: string;

	    static createFrom(source: any = {}) {
	        return new TemperatureData(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpuTemp = source["cpuTemp"];
	        this.gpuTemp = source["gpuTemp"];
	        this.cpuPowerWatts = source["cpuPowerWatts"];
	        this.gpuPowerWatts = source["gpuPowerWatts"];
	        this.maxTemp = source["maxTemp"];
	        this.controlTemp = source["controlTemp"];
	        this.controlSource = source["controlSource"];
	        this.selectedGpuDevice = source["selectedGpuDevice"];
	        this.cpuModel = source["cpuModel"];
	        this.gpuModel = source["gpuModel"];
	        this.cpuSensors = this.convertValues(source["cpuSensors"], TemperatureSensor);
	        this.gpuSensors = this.convertValues(source["gpuSensors"], TemperatureSensor);
	        this.gpuDevices = this.convertValues(source["gpuDevices"], TemperatureGPUDevice);
	        this.updateTime = source["updateTime"];
	        this.bridgeOk = source["bridgeOk"];
	        this.bridgeMessage = source["bridgeMessage"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

	export class TemperatureHistoryPoint {
	    timestamp: number;
	    cpuTemp: number;
	    gpuTemp: number;
	    fanRpm: number;
	    cpuPowerWatts?: number;
	    gpuPowerWatts?: number;

	    static createFrom(source: any = {}) {
	        return new TemperatureHistoryPoint(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.cpuTemp = source["cpuTemp"];
	        this.gpuTemp = source["gpuTemp"];
	        this.fanRpm = source["fanRpm"];
	        this.cpuPowerWatts = source["cpuPowerWatts"];
	        this.gpuPowerWatts = source["gpuPowerWatts"];
	    }
	}
	export class TemperatureHistoryPayload {
	    enabled: boolean;
	    sampleIntervalSeconds: number;
	    points: TemperatureHistoryPoint[];

	    static createFrom(source: any = {}) {
	        return new TemperatureHistoryPayload(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.sampleIntervalSeconds = source["sampleIntervalSeconds"];
	        this.points = this.convertValues(source["points"], TemperatureHistoryPoint);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}


	export class WiFiDiscoveredDevice {
	    name: string;
	    profileId?: string;
	    transport: string;
	    endpoint: string;
	    ip: string;
	    port?: string;
	    source: string;
	    network?: string;
	    speed?: number;
	    targetSpeed?: number;
	    temperature?: number;
	    latencyMs?: number;
	    stateEndpoint?: string;

	    static createFrom(source: any = {}) {
	        return new WiFiDiscoveredDevice(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.profileId = source["profileId"];
	        this.transport = source["transport"];
	        this.endpoint = source["endpoint"];
	        this.ip = source["ip"];
	        this.port = source["port"];
	        this.source = source["source"];
	        this.network = source["network"];
	        this.speed = source["speed"];
	        this.targetSpeed = source["targetSpeed"];
	        this.temperature = source["temperature"];
	        this.latencyMs = source["latencyMs"];
	        this.stateEndpoint = source["stateEndpoint"];
	    }
	}
	export class WiFiDiscoveryScope {
	    source: string;
	    network: string;
	    candidateCount: number;

	    static createFrom(source: any = {}) {
	        return new WiFiDiscoveryScope(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.network = source["network"];
	        this.candidateCount = source["candidateCount"];
	    }
	}
	export class WiFiDiscoveryResult {
	    mode: string;
	    found: boolean;
	    canceled?: boolean;
	    devices?: WiFiDiscoveredDevice[];
	    scopes?: WiFiDiscoveryScope[];
	    candidateCount: number;
	    scannedCount: number;
	    elapsedMs: number;
	    error?: string;

	    static createFrom(source: any = {}) {
	        return new WiFiDiscoveryResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.found = source["found"];
	        this.canceled = source["canceled"];
	        this.devices = this.convertValues(source["devices"], WiFiDiscoveredDevice);
	        this.scopes = this.convertValues(source["scopes"], WiFiDiscoveryScope);
	        this.candidateCount = source["candidateCount"];
	        this.scannedCount = source["scannedCount"];
	        this.elapsedMs = source["elapsedMs"];
	        this.error = source["error"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

