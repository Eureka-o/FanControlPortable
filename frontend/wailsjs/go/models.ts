export namespace deviceproto {
	
	export class BlackSharkManualGearPresetRPM {
	    gear: number;
	    levels: number[];
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkManualGearPresetRPM(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gear = source["gear"];
	        this.levels = source["levels"];
	    }
	}
	export class BlackSharkManualGearPresetsPayload {
	    minRpm: number;
	    maxRpm: number;
	    gears: BlackSharkManualGearPresetRPM[];
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkManualGearPresetsPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.minRpm = source["minRpm"];
	        this.maxRpm = source["maxRpm"];
	        this.gears = this.convertValues(source["gears"], BlackSharkManualGearPresetRPM);
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

export namespace guiapp {
	
	export class BlackSharkLcdDisplayPayload {
	    pos: number;
	    items: number[];
	    options: number[];
	    defaultItems: number[];
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkLcdDisplayPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pos = source["pos"];
	        this.items = source["items"];
	        this.options = source["options"];
	        this.defaultItems = source["defaultItems"];
	    }
	}
	export class BlackSharkLcdDisplayResult {
	    saved: boolean;
	    applied: boolean;
	    pos: number;
	    items: number[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkLcdDisplayResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.saved = source["saved"];
	        this.applied = source["applied"];
	        this.pos = source["pos"];
	        this.items = source["items"];
	        this.error = source["error"];
	    }
	}
	export class ConfigSnapshotExportResult {
	    saved: boolean;
	    path?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigSnapshotExportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.saved = source["saved"];
	        this.path = source["path"];
	        this.error = source["error"];
	    }
	}
	export class SceneRulePayload {
	    name?: string;
	    enabled: boolean;
	    match: string;
	    gear: number;
	    rgbMode: number;
	
	    static createFrom(source: any = {}) {
	        return new SceneRulePayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.match = source["match"];
	        this.gear = source["gear"];
	        this.rgbMode = source["rgbMode"];
	    }
	}
	export class SceneRulesPayload {
	    rules: SceneRulePayload[];
	    baselineGear: number;
	
	    static createFrom(source: any = {}) {
	        return new SceneRulesPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rules = this.convertValues(source["rules"], SceneRulePayload);
	        this.baselineGear = source["baselineGear"];
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
	export class SceneRulesResult {
	    saved: boolean;
	    issues: string[];
	
	    static createFrom(source: any = {}) {
	        return new SceneRulesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.saved = source["saved"];
	        this.issues = source["issues"];
	    }
	}
	export class ScreenCropPreviewPayload {
	    dataUrl: string;
	    width: number;
	    height: number;
	    rawWidth: number;
	    rawHeight: number;
	    slackX: number;
	    slackY: number;
	
	    static createFrom(source: any = {}) {
	        return new ScreenCropPreviewPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dataUrl = source["dataUrl"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.rawWidth = source["rawWidth"];
	        this.rawHeight = source["rawHeight"];
	        this.slackX = source["slackX"];
	        this.slackY = source["slackY"];
	    }
	}
	export class ScreenHistoryItemPayload {
	    name: string;
	    path: string;
	    unixTime: number;
	    modified: string;
	    size: number;
	    thumb?: string;
	
	    static createFrom(source: any = {}) {
	        return new ScreenHistoryItemPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.unixTime = source["unixTime"];
	        this.modified = source["modified"];
	        this.size = source["size"];
	        this.thumb = source["thumb"];
	    }
	}
	export class ScreenHistoryListPayload {
	    dir: string;
	    items: ScreenHistoryItemPayload[];
	    current?: ScreenHistoryItemPayload;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ScreenHistoryListPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.items = this.convertValues(source["items"], ScreenHistoryItemPayload);
	        this.current = this.convertValues(source["current"], ScreenHistoryItemPayload);
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
	export class ScreenImageInfoPayload {
	    hasImage: boolean;
	    timestamp: number;
	    size: number;
	    crc: number;
	    presetMatched: boolean;
	    presetOfficialPosition?: number;
	    presetName?: string;
	
	    static createFrom(source: any = {}) {
	        return new ScreenImageInfoPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasImage = source["hasImage"];
	        this.timestamp = source["timestamp"];
	        this.size = source["size"];
	        this.crc = source["crc"];
	        this.presetMatched = source["presetMatched"];
	        this.presetOfficialPosition = source["presetOfficialPosition"];
	        this.presetName = source["presetName"];
	    }
	}
	export class ScreenImageReadPayload {
	    ok: boolean;
	    dataUrl?: string;
	    bytes: number;
	    crc: number;
	    error?: string;
	    fromCache?: boolean;
	    cachedAtUnix?: number;
	
	    static createFrom(source: any = {}) {
	        return new ScreenImageReadPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.dataUrl = source["dataUrl"];
	        this.bytes = source["bytes"];
	        this.crc = source["crc"];
	        this.error = source["error"];
	        this.fromCache = source["fromCache"];
	        this.cachedAtUnix = source["cachedAtUnix"];
	    }
	}
	export class ScreenPresetPayload {
	    officialPosition: number;
	    assetIndex: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new ScreenPresetPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.officialPosition = source["officialPosition"];
	        this.assetIndex = source["assetIndex"];
	        this.name = source["name"];
	    }
	}
	export class ScreenPresetListPayload {
	    presets: ScreenPresetPayload[];
	
	    static createFrom(source: any = {}) {
	        return new ScreenPresetListPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.presets = this.convertValues(source["presets"], ScreenPresetPayload);
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
	
	export class ScreenUploadResultPayload {
	    success: boolean;
	    error: string;
	    reports: number;
	    short: number;
	    timestamp: number;
	    size: number;
	    crc: number;
	    verified: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScreenUploadResultPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.reports = source["reports"];
	        this.short = source["short"];
	        this.timestamp = source["timestamp"];
	        this.size = source["size"];
	        this.crc = source["crc"];
	        this.verified = source["verified"];
	    }
	}
	export class UpdateRelease {
	    tag_name: string;
	    html_url: string;
	    body: string;
	    prerelease: boolean;
	    update_available: boolean;
	    installer_url: string;
	    installer_sha256: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateRelease(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tag_name = source["tag_name"];
	        this.html_url = source["html_url"];
	        this.body = source["body"];
	        this.prerelease = source["prerelease"];
	        this.update_available = source["update_available"];
	        this.installer_url = source["installer_url"];
	        this.installer_sha256 = source["installer_sha256"];
	    }
	}

}

export namespace ipc {
	
	export class TransferDeviceImageParams {
	    dataBase64: string;
	    fileName?: string;
	    mimeType?: string;
	    format?: string;
	    width?: number;
	    height?: number;
	
	    static createFrom(source: any = {}) {
	        return new TransferDeviceImageParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dataBase64 = source["dataBase64"];
	        this.fileName = source["fileName"];
	        this.mimeType = source["mimeType"];
	        this.format = source["format"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}

}

export namespace theme {
	
	export class Meta {
	    id: string;
	    name: string;
	    base: string;
	    author?: string;
	    version?: string;
	    description?: string;
	    layer?: string;
	    contract?: string;
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
	        this.layer = source["layer"];
	        this.contract = source["contract"];
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
	export class BlackSharkFirmwareStatus {
	    supported: boolean;
	    currentVersion?: string;
	    latestVersion?: string;
	    updateAvailable: boolean;
	    minVersion?: string;
	    firmwareUrl?: string;
	    firmwareMd5?: string;
	    checkedAt?: string;
	    error?: string;
	    updateMethod?: string;
	    officialToolPath?: string;
	    officialToolFound: boolean;
	    belowMinVersion?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkFirmwareStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.supported = source["supported"];
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	        this.minVersion = source["minVersion"];
	        this.firmwareUrl = source["firmwareUrl"];
	        this.firmwareMd5 = source["firmwareMd5"];
	        this.checkedAt = source["checkedAt"];
	        this.error = source["error"];
	        this.updateMethod = source["updateMethod"];
	        this.officialToolPath = source["officialToolPath"];
	        this.officialToolFound = source["officialToolFound"];
	        this.belowMinVersion = source["belowMinVersion"];
	    }
	}
	export class BlackSharkRgbColorOptionView {
	    index: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkRgbColorOptionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.name = source["name"];
	    }
	}
	export class BlackSharkRgbColorControlsView {
	    colorMode: boolean;
	    singleColor: boolean;
	    hue: string;
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkRgbColorControlsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.colorMode = source["colorMode"];
	        this.singleColor = source["singleColor"];
	        this.hue = source["hue"];
	    }
	}
	export class ValueRangeView {
	    min: number;
	    max: number;
	
	    static createFrom(source: any = {}) {
	        return new ValueRangeView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.min = source["min"];
	        this.max = source["max"];
	    }
	}
	export class BlackSharkRgbMode {
	    index: number;
	    name?: string;
	    speed: number;
	    brightness: number;
	    red: number;
	    green: number;
	    blue: number;
	    current: boolean;
	    needsHostData?: string;
	    speedRange: ValueRangeView;
	    brightnessRange: ValueRangeView;
	    speedDisabled: boolean;
	    staticColor: boolean;
	    colorOption: number;
	    colorOptionName?: string;
	    colorControls: BlackSharkRgbColorControlsView;
	    known: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkRgbMode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.name = source["name"];
	        this.speed = source["speed"];
	        this.brightness = source["brightness"];
	        this.red = source["red"];
	        this.green = source["green"];
	        this.blue = source["blue"];
	        this.current = source["current"];
	        this.needsHostData = source["needsHostData"];
	        this.speedRange = this.convertValues(source["speedRange"], ValueRangeView);
	        this.brightnessRange = this.convertValues(source["brightnessRange"], ValueRangeView);
	        this.speedDisabled = source["speedDisabled"];
	        this.staticColor = source["staticColor"];
	        this.colorOption = source["colorOption"];
	        this.colorOptionName = source["colorOptionName"];
	        this.colorControls = this.convertValues(source["colorControls"], BlackSharkRgbColorControlsView);
	        this.known = source["known"];
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
	export class BlackSharkRgbLighting {
	    available: boolean;
	    switchEnabled: boolean;
	    switchKnown: boolean;
	    modes?: BlackSharkRgbMode[];
	    count: number;
	    currentIndex: number;
	    colorOptions?: BlackSharkRgbColorOptionView[];
	    colorSettable: boolean;
	    fromCache?: boolean;
	    cachedAtUnix?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkRgbLighting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.switchEnabled = source["switchEnabled"];
	        this.switchKnown = source["switchKnown"];
	        this.modes = this.convertValues(source["modes"], BlackSharkRgbMode);
	        this.count = source["count"];
	        this.currentIndex = source["currentIndex"];
	        this.colorOptions = this.convertValues(source["colorOptions"], BlackSharkRgbColorOptionView);
	        this.colorSettable = source["colorSettable"];
	        this.fromCache = source["fromCache"];
	        this.cachedAtUnix = source["cachedAtUnix"];
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
	export class SceneRule {
	    name?: string;
	    enabled: boolean;
	    match: string;
	    gear: number;
	    rgbMode: number;
	
	    static createFrom(source: any = {}) {
	        return new SceneRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.match = source["match"];
	        this.gear = source["gear"];
	        this.rgbMode = source["rgbMode"];
	    }
	}
	export class AxisNoiseZone {
	    min: number;
	    max: number;
	    severity: string;
	
	    static createFrom(source: any = {}) {
	        return new AxisNoiseZone(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.min = source["min"];
	        this.max = source["max"];
	        this.severity = source["severity"];
	    }
	}
	export class AxisNoisePoint {
	    requested: number;
	    actual: number;
	    severity: string;
	
	    static createFrom(source: any = {}) {
	        return new AxisNoisePoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requested = source["requested"];
	        this.actual = source["actual"];
	        this.severity = source["severity"];
	    }
	}
	export class NoiseDiagnosticRange {
	    unit: string;
	    min: number;
	    max: number;
	    step: number;
	    minSource: string;
	    maxSource: string;
	
	    static createFrom(source: any = {}) {
	        return new NoiseDiagnosticRange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.unit = source["unit"];
	        this.min = source["min"];
	        this.max = source["max"];
	        this.step = source["step"];
	        this.minSource = source["minSource"];
	        this.maxSource = source["maxSource"];
	    }
	}
	export class AxisNoiseProfile {
	    deviceKey: string;
	    unit: string;
	    enabled: boolean;
	    range: NoiseDiagnosticRange;
	    points: AxisNoisePoint[];
	    zones: AxisNoiseZone[];
	    testedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new AxisNoiseProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceKey = source["deviceKey"];
	        this.unit = source["unit"];
	        this.enabled = source["enabled"];
	        this.range = this.convertValues(source["range"], NoiseDiagnosticRange);
	        this.points = this.convertValues(source["points"], AxisNoisePoint);
	        this.zones = this.convertValues(source["zones"], AxisNoiseZone);
	        this.testedAt = source["testedAt"];
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
	export class NoiseDiagnosticPoint {
	    requested: number;
	    actual: number;
	    levelDb: number;
	    spreadDb: number;
	    valid: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NoiseDiagnosticPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requested = source["requested"];
	        this.actual = source["actual"];
	        this.levelDb = source["levelDb"];
	        this.spreadDb = source["spreadDb"];
	        this.valid = source["valid"];
	    }
	}
	export class NoiseDiagnosticResult {
	    deviceKey: string;
	    unit: string;
	    points: NoiseDiagnosticPoint[];
	    baselineDb: number;
	    baselineDriftDb: number;
	    riseDb: number;
	    knee: number;
	    suspectedPeak?: number;
	    confidence: string;
	    confidenceReason: string;
	    microphone: string;
	    testedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new NoiseDiagnosticResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceKey = source["deviceKey"];
	        this.unit = source["unit"];
	        this.points = this.convertValues(source["points"], NoiseDiagnosticPoint);
	        this.baselineDb = source["baselineDb"];
	        this.baselineDriftDb = source["baselineDriftDb"];
	        this.riseDb = source["riseDb"];
	        this.knee = source["knee"];
	        this.suspectedPeak = source["suspectedPeak"];
	        this.confidence = source["confidence"];
	        this.confidenceReason = source["confidenceReason"];
	        this.microphone = source["microphone"];
	        this.testedAt = source["testedAt"];
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
	export class DeviceFanCurveProfilesState {
	    profiles: FanCurveProfile[];
	    activeId: string;
	    fanCurve?: FanCurvePoint[];
	    manualGearRpm?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new DeviceFanCurveProfilesState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profiles = this.convertValues(source["profiles"], FanCurveProfile);
	        this.activeId = source["activeId"];
	        this.fanCurve = this.convertValues(source["fanCurve"], FanCurvePoint);
	        this.manualGearRpm = source["manualGearRpm"];
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
	    supportsScreenImageTransfer: boolean;
	    supportsPowerOnStart: boolean;
	    supportsSmartStartStop: boolean;
	    supportsSoftwareSmartStartStop: boolean;
	
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
	        this.supportsScreenImageTransfer = source["supportsScreenImageTransfer"];
	        this.supportsPowerOnStart = source["supportsPowerOnStart"];
	        this.supportsSmartStartStop = source["supportsSmartStartStop"];
	        this.supportsSoftwareSmartStartStop = source["supportsSoftwareSmartStartStop"];
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
	    displayFeatures?: string[];
	
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
	        this.displayFeatures = source["displayFeatures"];
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
	    nativeAutoBLEScanUnix?: number;
	    activeDeviceProfileId: string;
	    activeDeviceProfileIdsByTransport?: Record<string, string>;
	    deviceProfiles?: DeviceProfile[];
	    deviceTransport: string;
	    fanControlDeviceIp: string;
	    wifiCompatibilityEnabled: boolean;
	    wifiConnectionPriorityEnabled: boolean;
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
	    fanCurveProfilesByDevice?: Record<string, DeviceFanCurveProfilesState>;
	    noiseDiagnosticsByDevice?: Record<string, NoiseDiagnosticResult>;
	    axisNoiseProfilesByDevice?: Record<string, AxisNoiseProfile>;
	    activeFanCurveProfileId: string;
	    sceneRules?: SceneRule[];
	    sceneBaselineGear: number;
	    blackSharkLcdItems?: number[];
	    blackSharkRgbCache?: BlackSharkRgbLighting;
	    blackSharkRgbCacheAtUnix?: number;
	    blackSharkFirmwareCache?: BlackSharkFirmwareStatus;
	    blackSharkLcdPos: number;
	    gearLight: boolean;
	    powerOnStart: boolean;
	    powerSpoofEnabled: boolean;
	    powerSpoofPercent: number;
	    powerSpoofOffsetWatts: number;
	    cpuPowerSpoofPercent: number;
	    cpuPowerSpoofOffsetWatts: number;
	    gpuPowerSpoofPercent: number;
	    gpuPowerSpoofOffsetWatts: number;
	    windowsAutoStart: boolean;
	    monitorOnly: boolean;
	    themeMode: string;
	    windowBlur: string;
	    smartStartStop: string;
	    brightness: number;
	    tempUpdateRate: number;
	    tempSampleCount: number;
	    temperatureHistoryRetentionHours: number;
	    tempSource: string;
	    gpuDevice: string;
	    cpuSensor: string;
	    gpuSensor: string;
	    cpuPowerSensor: string;
	    gpuPowerSensor: string;
	    gpuReadMode: string;
	    gpuLowPowerProtection: boolean;
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
	        this.nativeAutoBLEScanUnix = source["nativeAutoBLEScanUnix"];
	        this.activeDeviceProfileId = source["activeDeviceProfileId"];
	        this.activeDeviceProfileIdsByTransport = source["activeDeviceProfileIdsByTransport"];
	        this.deviceProfiles = this.convertValues(source["deviceProfiles"], DeviceProfile);
	        this.deviceTransport = source["deviceTransport"];
	        this.fanControlDeviceIp = source["fanControlDeviceIp"];
	        this.wifiCompatibilityEnabled = source["wifiCompatibilityEnabled"];
	        this.wifiConnectionPriorityEnabled = source["wifiConnectionPriorityEnabled"];
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
	        this.fanCurveProfilesByDevice = this.convertValues(source["fanCurveProfilesByDevice"], DeviceFanCurveProfilesState, true);
	        this.noiseDiagnosticsByDevice = this.convertValues(source["noiseDiagnosticsByDevice"], NoiseDiagnosticResult, true);
	        this.axisNoiseProfilesByDevice = this.convertValues(source["axisNoiseProfilesByDevice"], AxisNoiseProfile, true);
	        this.activeFanCurveProfileId = source["activeFanCurveProfileId"];
	        this.sceneRules = this.convertValues(source["sceneRules"], SceneRule);
	        this.sceneBaselineGear = source["sceneBaselineGear"];
	        this.blackSharkLcdItems = source["blackSharkLcdItems"];
	        this.blackSharkRgbCache = this.convertValues(source["blackSharkRgbCache"], BlackSharkRgbLighting);
	        this.blackSharkRgbCacheAtUnix = source["blackSharkRgbCacheAtUnix"];
	        this.blackSharkFirmwareCache = this.convertValues(source["blackSharkFirmwareCache"], BlackSharkFirmwareStatus);
	        this.blackSharkLcdPos = source["blackSharkLcdPos"];
	        this.gearLight = source["gearLight"];
	        this.powerOnStart = source["powerOnStart"];
	        this.powerSpoofEnabled = source["powerSpoofEnabled"];
	        this.powerSpoofPercent = source["powerSpoofPercent"];
	        this.powerSpoofOffsetWatts = source["powerSpoofOffsetWatts"];
	        this.cpuPowerSpoofPercent = source["cpuPowerSpoofPercent"];
	        this.cpuPowerSpoofOffsetWatts = source["cpuPowerSpoofOffsetWatts"];
	        this.gpuPowerSpoofPercent = source["gpuPowerSpoofPercent"];
	        this.gpuPowerSpoofOffsetWatts = source["gpuPowerSpoofOffsetWatts"];
	        this.windowsAutoStart = source["windowsAutoStart"];
	        this.monitorOnly = source["monitorOnly"];
	        this.themeMode = source["themeMode"];
	        this.windowBlur = source["windowBlur"];
	        this.smartStartStop = source["smartStartStop"];
	        this.brightness = source["brightness"];
	        this.tempUpdateRate = source["tempUpdateRate"];
	        this.tempSampleCount = source["tempSampleCount"];
	        this.temperatureHistoryRetentionHours = source["temperatureHistoryRetentionHours"];
	        this.tempSource = source["tempSource"];
	        this.gpuDevice = source["gpuDevice"];
	        this.cpuSensor = source["cpuSensor"];
	        this.gpuSensor = source["gpuSensor"];
	        this.cpuPowerSensor = source["cpuPowerSensor"];
	        this.gpuPowerSensor = source["gpuPowerSensor"];
	        this.gpuReadMode = source["gpuReadMode"];
	        this.gpuLowPowerProtection = source["gpuLowPowerProtection"];
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
	export class BlackSharkConfigRestoreResult {
	    restored: string[];
	    issues: string[];
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkConfigRestoreResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.restored = source["restored"];
	        this.issues = source["issues"];
	    }
	}
	export class BlackSharkCurveCalibrationEntry {
	    fieldRpm: number;
	    actualRpm: number;
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkCurveCalibrationEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fieldRpm = source["fieldRpm"];
	        this.actualRpm = source["actualRpm"];
	    }
	}
	export class BlackSharkCurvePointView {
	    tempC: number;
	    fieldRpm: number;
	    approxActualRpm: number;
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkCurvePointView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tempC = source["tempC"];
	        this.fieldRpm = source["fieldRpm"];
	        this.approxActualRpm = source["approxActualRpm"];
	    }
	}
	export class BlackSharkCurveTempRange {
	    minTempC: number;
	    maxTempC: number;
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkCurveTempRange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.minTempC = source["minTempC"];
	        this.maxTempC = source["maxTempC"];
	    }
	}
	
	export class BlackSharkGear {
	    gear: number;
	    fixedValue: number;
	    fixedKnown: boolean;
	    curve?: BlackSharkCurvePointView[];
	    curveKnown: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkGear(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gear = source["gear"];
	        this.fixedValue = source["fixedValue"];
	        this.fixedKnown = source["fixedKnown"];
	        this.curve = this.convertValues(source["curve"], BlackSharkCurvePointView);
	        this.curveKnown = source["curveKnown"];
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
	export class BlackSharkHostEffects {
	    currentSlot: number;
	    audioSyncRunning: boolean;
	    audioError?: string;
	    audioFrequencyHz: number;
	    audioLevel: number;
	    audioPushCount: number;
	    reactiveRunning: boolean;
	    reactiveError?: string;
	    reactivePushCount: number;
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkHostEffects(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentSlot = source["currentSlot"];
	        this.audioSyncRunning = source["audioSyncRunning"];
	        this.audioError = source["audioError"];
	        this.audioFrequencyHz = source["audioFrequencyHz"];
	        this.audioLevel = source["audioLevel"];
	        this.audioPushCount = source["audioPushCount"];
	        this.reactiveRunning = source["reactiveRunning"];
	        this.reactiveError = source["reactiveError"];
	        this.reactivePushCount = source["reactivePushCount"];
	    }
	}
	export class BlackSharkSwitchStates {
	    available: boolean;
	    lightingEnabled: boolean;
	    lightingKnown: boolean;
	    lcdScreenEnabled: boolean;
	    lcdScreenKnown: boolean;
	    coolingSource: number;
	    coolingSourceKnown: boolean;
	    smartStartStop: boolean;
	    powerOnSelfStart: boolean;
	    onOffVectorKnown: boolean;
	    onOffVectorReadbackSupported: boolean;
	    lastCommandedSmartStartStop: boolean;
	    lastCommandedPowerOnSelfStart: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkSwitchStates(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.lightingEnabled = source["lightingEnabled"];
	        this.lightingKnown = source["lightingKnown"];
	        this.lcdScreenEnabled = source["lcdScreenEnabled"];
	        this.lcdScreenKnown = source["lcdScreenKnown"];
	        this.coolingSource = source["coolingSource"];
	        this.coolingSourceKnown = source["coolingSourceKnown"];
	        this.smartStartStop = source["smartStartStop"];
	        this.powerOnSelfStart = source["powerOnSelfStart"];
	        this.onOffVectorKnown = source["onOffVectorKnown"];
	        this.onOffVectorReadbackSupported = source["onOffVectorReadbackSupported"];
	        this.lastCommandedSmartStartStop = source["lastCommandedSmartStartStop"];
	        this.lastCommandedPowerOnSelfStart = source["lastCommandedPowerOnSelfStart"];
	    }
	}
	export class BlackSharkInfo {
	    available: boolean;
	    firmware: BlackSharkFirmwareStatus;
	    switches: BlackSharkSwitchStates;
	    gears?: BlackSharkGear[];
	    curveMinRpm: number;
	    curveMaxRpm: number;
	    curveCalibration?: BlackSharkCurveCalibrationEntry[];
	    fixedMinRpm: number;
	    fixedMaxRpm: number;
	
	    static createFrom(source: any = {}) {
	        return new BlackSharkInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.firmware = this.convertValues(source["firmware"], BlackSharkFirmwareStatus);
	        this.switches = this.convertValues(source["switches"], BlackSharkSwitchStates);
	        this.gears = this.convertValues(source["gears"], BlackSharkGear);
	        this.curveMinRpm = source["curveMinRpm"];
	        this.curveMaxRpm = source["curveMaxRpm"];
	        this.curveCalibration = this.convertValues(source["curveCalibration"], BlackSharkCurveCalibrationEntry);
	        this.fixedMinRpm = source["fixedMinRpm"];
	        this.fixedMaxRpm = source["fixedMaxRpm"];
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
	    powerSensors: PowerSensor[];
	
	    static createFrom(source: any = {}) {
	        return new TemperatureGPUDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.vendor = source["vendor"];
	        this.sensors = this.convertValues(source["sensors"], TemperatureSensor);
	        this.powerSensors = this.convertValues(source["powerSensors"], PowerSensor);
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
	export class PowerSensor {
	    key: string;
	    name: string;
	    value: number;
	
	    static createFrom(source: any = {}) {
	        return new PowerSensor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.value = source["value"];
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
	    gpuReadState?: string;
	    maxTemp: number;
	    controlTemp: number;
	    controlSource: string;
	    selectedGpuDevice: string;
	    cpuModel: string;
	    gpuModel: string;
	    cpuSensors: TemperatureSensor[];
	    gpuSensors: TemperatureSensor[];
	    cpuPowerSensors: PowerSensor[];
	    gpuPowerSensors: PowerSensor[];
	    gpuDevices: TemperatureGPUDevice[];
	    updateTime: number;
	    success: boolean;
	    error: string;
	    telemetrySource?: string;
	    telemetryFailureStage?: string;
	
	    static createFrom(source: any = {}) {
	        return new BridgeTemperatureData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpuTemp = source["cpuTemp"];
	        this.gpuTemp = source["gpuTemp"];
	        this.cpuPowerWatts = source["cpuPowerWatts"];
	        this.gpuPowerWatts = source["gpuPowerWatts"];
	        this.gpuReadState = source["gpuReadState"];
	        this.maxTemp = source["maxTemp"];
	        this.controlTemp = source["controlTemp"];
	        this.controlSource = source["controlSource"];
	        this.selectedGpuDevice = source["selectedGpuDevice"];
	        this.cpuModel = source["cpuModel"];
	        this.gpuModel = source["gpuModel"];
	        this.cpuSensors = this.convertValues(source["cpuSensors"], TemperatureSensor);
	        this.gpuSensors = this.convertValues(source["gpuSensors"], TemperatureSensor);
	        this.cpuPowerSensors = this.convertValues(source["cpuPowerSensors"], PowerSensor);
	        this.gpuPowerSensors = this.convertValues(source["gpuPowerSensors"], PowerSensor);
	        this.gpuDevices = this.convertValues(source["gpuDevices"], TemperatureGPUDevice);
	        this.updateTime = source["updateTime"];
	        this.success = source["success"];
	        this.error = source["error"];
	        this.telemetrySource = source["telemetrySource"];
	        this.telemetryFailureStage = source["telemetryFailureStage"];
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
	export class DeviceCandidate {
	    id: string;
	    transport: string;
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
	    connectable: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.transport = source["transport"];
	        this.name = source["name"];
	        this.profileId = source["profileId"];
	        this.endpoint = source["endpoint"];
	        this.source = source["source"];
	        this.network = source["network"];
	        this.speed = source["speed"];
	        this.targetSpeed = source["targetSpeed"];
	        this.temperature = source["temperature"];
	        this.latencyMs = source["latencyMs"];
	        this.connected = source["connected"];
	        this.connectable = source["connectable"];
	        this.error = source["error"];
	    }
	}
	
	
	export class DeviceConnectRequest {
	    id?: string;
	    transport?: string;
	    profileId?: string;
	    endpoint?: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceConnectRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.transport = source["transport"];
	        this.profileId = source["profileId"];
	        this.endpoint = source["endpoint"];
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
	    checksumRule?: string;
	    checksumExpected?: string;
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
	        this.checksumRule = source["checksumRule"];
	        this.checksumExpected = source["checksumExpected"];
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
	export class FlyDigiRuntimeCapability {
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
	
	    static createFrom(source: any = {}) {
	        return new FlyDigiRuntimeCapability(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.gearSettings = source["gearSettings"];
	        this.maxGearCode = source["maxGearCode"];
	        this.maxGearLabel = source["maxGearLabel"];
	        this.maxGearIndex = source["maxGearIndex"];
	        this.maxRpm = source["maxRpm"];
	        this.selectedGearCode = source["selectedGearCode"];
	        this.selectedGear = source["selectedGear"];
	        this.source = source["source"];
	        this.reason = source["reason"];
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
	    flyDigiCapability?: FlyDigiRuntimeCapability;
	
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
	        this.flyDigiCapability = this.convertValues(source["flyDigiCapability"], FlyDigiRuntimeCapability);
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
	
	export class DeviceScanResult {
	    mode: string;
	    connected: boolean;
	    devices: DeviceCandidate[];
	    wifiEnabled: boolean;
	    serialEnabled: boolean;
	    showDeepScan?: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceScanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.connected = source["connected"];
	        this.devices = this.convertValues(source["devices"], DeviceCandidate);
	        this.wifiEnabled = source["wifiEnabled"];
	        this.serialEnabled = source["serialEnabled"];
	        this.showDeepScan = source["showDeepScan"];
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
	    flyDigiCapability?: FlyDigiRuntimeCapability;
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
	        this.flyDigiCapability = this.convertValues(source["flyDigiCapability"], FlyDigiRuntimeCapability);
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
	
	
	
	
	
	
	export class NoiseDiagnosticBeginRequest {
	    deviceKey: string;
	    range: NoiseDiagnosticRange;
	
	    static createFrom(source: any = {}) {
	        return new NoiseDiagnosticBeginRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceKey = source["deviceKey"];
	        this.range = this.convertValues(source["range"], NoiseDiagnosticRange);
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
	
	
	
	export class NoiseDiagnosticSession {
	    sessionId: string;
	    deviceKey: string;
	    range: NoiseDiagnosticRange;
	    configRevision: number;
	
	    static createFrom(source: any = {}) {
	        return new NoiseDiagnosticSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.deviceKey = source["deviceKey"];
	        this.range = this.convertValues(source["range"], NoiseDiagnosticRange);
	        this.configRevision = source["configRevision"];
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
	export class NoiseDiagnosticTargetResult {
	    requested: number;
	    actual: number;
	    unit: string;
	
	    static createFrom(source: any = {}) {
	        return new NoiseDiagnosticTargetResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requested = source["requested"];
	        this.actual = source["actual"];
	        this.unit = source["unit"];
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
	    gpuReadState?: string;
	    maxTemp: number;
	    controlTemp: number;
	    controlSource: string;
	    selectedGpuDevice: string;
	    cpuModel: string;
	    gpuModel: string;
	    cpuSensors: TemperatureSensor[];
	    gpuSensors: TemperatureSensor[];
	    cpuPowerSensors: PowerSensor[];
	    gpuPowerSensors: PowerSensor[];
	    gpuDevices: TemperatureGPUDevice[];
	    updateTime: number;
	    bridgeOk: boolean;
	    bridgeMessage: string;
	    telemetrySource?: string;
	    cpuTelemetrySource?: string;
	    gpuTelemetrySource?: string;
	    telemetryFailureStage?: string;
	    telemetryState: string;
	
	    static createFrom(source: any = {}) {
	        return new TemperatureData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpuTemp = source["cpuTemp"];
	        this.gpuTemp = source["gpuTemp"];
	        this.cpuPowerWatts = source["cpuPowerWatts"];
	        this.gpuPowerWatts = source["gpuPowerWatts"];
	        this.gpuReadState = source["gpuReadState"];
	        this.maxTemp = source["maxTemp"];
	        this.controlTemp = source["controlTemp"];
	        this.controlSource = source["controlSource"];
	        this.selectedGpuDevice = source["selectedGpuDevice"];
	        this.cpuModel = source["cpuModel"];
	        this.gpuModel = source["gpuModel"];
	        this.cpuSensors = this.convertValues(source["cpuSensors"], TemperatureSensor);
	        this.gpuSensors = this.convertValues(source["gpuSensors"], TemperatureSensor);
	        this.cpuPowerSensors = this.convertValues(source["cpuPowerSensors"], PowerSensor);
	        this.gpuPowerSensors = this.convertValues(source["gpuPowerSensors"], PowerSensor);
	        this.gpuDevices = this.convertValues(source["gpuDevices"], TemperatureGPUDevice);
	        this.updateTime = source["updateTime"];
	        this.bridgeOk = source["bridgeOk"];
	        this.bridgeMessage = source["bridgeMessage"];
	        this.telemetrySource = source["telemetrySource"];
	        this.cpuTelemetrySource = source["cpuTelemetrySource"];
	        this.gpuTelemetrySource = source["gpuTelemetrySource"];
	        this.telemetryFailureStage = source["telemetryFailureStage"];
	        this.telemetryState = source["telemetryState"];
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
	    gapBeforeSeconds?: number;
	
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
	        this.gapBeforeSeconds = source["gapBeforeSeconds"];
	    }
	}
	export class TemperatureHistoryPayload {
	    enabled: boolean;
	    sampleIntervalSeconds: number;
	    retentionHours: number;
	    points: TemperatureHistoryPoint[];
	
	    static createFrom(source: any = {}) {
	        return new TemperatureHistoryPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.sampleIntervalSeconds = source["sampleIntervalSeconds"];
	        this.retentionHours = source["retentionHours"];
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

