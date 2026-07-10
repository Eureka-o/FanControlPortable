(function () {
  "use strict";

  const ns = window.FanControlOmenFanPlugin = window.FanControlOmenFanPlugin || {};

    const PLUGIN_ID = "omen-fan";
    const MOCK_BASE_URL = "http://127.0.0.1:8787";
    const STORAGE_KEY = "fancontrol.omen-fan.preview.v4";
    const RPM_MIN = 800;
    const RPM_MAX = 6000;
    const RPM_STEP = 100;
    const TEMP_MIN = 20;
    const CPU_TEMP_MAX = 105;
    const GPU_TEMP_MAX = 90;
    const SAMPLE_COUNTS = [1, 2, 3, 5, 10];

    const CPU_CURVE = curveFromPairs([
      [20, 1000], [25, 1000], [30, 1000], [35, 1200], [40, 1400], [45, 1600],
      [50, 1800], [55, 2000], [60, 2300], [65, 2600], [70, 2900], [75, 3200],
      [80, 3500], [85, 3800], [90, 4000], [95, 4200], [100, 4400], [105, 4600],
    ]);

    const GPU_CURVE = curveFromPairs([
      [20, 1000], [25, 1000], [30, 1000], [35, 1200], [40, 1400], [45, 1600],
      [50, 1800], [55, 2000], [60, 2300], [65, 2600], [70, 2900], [75, 3200],
      [80, 3500], [85, 3800], [90, 4000],
    ]);

    const QUIET_CPU_CURVE = curveFromPairs([
      [20, 900], [25, 900], [30, 900], [35, 1000], [40, 1100], [45, 1200],
      [50, 1400], [55, 1600], [60, 1800], [65, 2100], [70, 2500], [75, 2900],
      [80, 3400], [85, 3900], [90, 4300], [95, 4700], [100, 5200], [105, 5600],
    ]);

    const QUIET_GPU_CURVE = curveFromPairs([
      [20, 900], [25, 900], [30, 900], [35, 1000], [40, 1100], [45, 1300],
      [50, 1500], [55, 1800], [60, 2100], [65, 2500], [70, 3000], [75, 3500],
      [80, 4100], [85, 4700], [90, 5200],
    ]);

    const COOL_CPU_CURVE = curveFromPairs([
      [20, 1200], [25, 1200], [30, 1300], [35, 1500], [40, 1700], [45, 2000],
      [50, 2300], [55, 2700], [60, 3100], [65, 3500], [70, 3900], [75, 4300],
      [80, 4700], [85, 5100], [90, 5400], [95, 5700], [100, 6000], [105, 6000],
    ]);

    const COOL_GPU_CURVE = curveFromPairs([
      [20, 1200], [25, 1200], [30, 1300], [35, 1500], [40, 1700], [45, 2000],
      [50, 2300], [55, 2700], [60, 3100], [65, 3500], [70, 4000], [75, 4500],
      [80, 5000], [85, 5500], [90, 6000],
    ]);

    const MODE_CARDS = [
      { id: "balanced", title: "均衡", detail: "平衡性能与噪声，适合日常使用。", icon: "Gauge", cpu: 65 },
      { id: "performance", title: "性能", detail: "更高 CPU 功耗限制，优先性能释放。", icon: "Zap", cpu: 95 },
      { id: "quiet", title: "安静", detail: "降低响应强度，优先控制噪声。", icon: "Fan", cpu: 35 },
      { id: "custom", title: "大师", detail: "启用自定义 CPU/GPU 风扇曲线。", icon: "SlidersHorizontal", cpu: 95 },
    ];

    const TABS = [
      ["overview", "概览"],
      ["curve", "风扇曲线"],
      ["settings", "设置"],
    ];

    function curveFromPairs(pairs) {
      return pairs.map(([temperature, rpm]) => ({ temperature, rpm }));
    }

    function cloneCurve(curve) {
      return curve.map((point) => ({ temperature: point.temperature, rpm: point.rpm }));
    }

    function cloneSavedCurveTemplate(template) {
      if (!template || !Array.isArray(template.cpuCurve) || !Array.isArray(template.gpuCurve)) {
        return null;
      }
      return {
        cpuCurve: normalizeCurve(template.cpuCurve, CPU_CURVE, CPU_TEMP_MAX),
        gpuCurve: normalizeCurve(template.gpuCurve, GPU_CURVE, GPU_TEMP_MAX),
        updatedAt: Number(template.updatedAt) || 0,
      };
    }

    function formatTemplateTime(timestamp) {
      const value = Number(timestamp);
      if (!Number.isFinite(value) || value <= 0) return "尚未保存";
      try {
        return new Date(value).toLocaleString("zh-CN", {
          month: "2-digit",
          day: "2-digit",
          hour: "2-digit",
          minute: "2-digit",
        });
      } catch {
        return "已保存";
      }
    }

    // A CurveProfile represents one saved snapshot of the CPU + GPU custom fan curves.
    // Users can manage multiple profiles (like the main app's fan-curve profile list)
    // in addition to the three built-in templates (default / quiet / cool).
    function cloneCurveProfile(profile) {
      if (!profile || !Array.isArray(profile.cpuCurve) || !Array.isArray(profile.gpuCurve)) return null;
      return {
        id: String(profile.id || ""),
        name: (String(profile.name || "").trim()) || "未命名方案",
        cpuCurve: normalizeCurve(profile.cpuCurve, CPU_CURVE, CPU_TEMP_MAX),
        gpuCurve: normalizeCurve(profile.gpuCurve, GPU_CURVE, GPU_TEMP_MAX),
        updatedAt: Number(profile.updatedAt) || 0,
      };
    }

    function normalizeCurveProfiles(list, legacyTemplate) {
      const out = [];
      const seen = new Set();
      if (Array.isArray(list)) {
        for (const raw of list) {
          const cloned = cloneCurveProfile(raw);
          if (!cloned) continue;
          let id = cloned.id;
          if (!id || seen.has(id)) {
            id = "p_" + Date.now().toString(36) + "_" + Math.random().toString(36).slice(2, 8);
          }
          seen.add(id);
          out.push({ ...cloned, id });
        }
      }
      // Migrate legacy single-slot template: earlier builds only allowed one saved template
      // stored in `savedCurveTemplate`. When a stored settings blob still has that field but
      // no `curveProfiles`, we lift it into the new list so users don't lose their saved curve.
      if (legacyTemplate && !out.some((p) => p.id === "legacy")) {
        const legacy = cloneSavedCurveTemplate(legacyTemplate);
        if (legacy) {
          out.push({
            id: "legacy",
            name: "旧模板",
            cpuCurve: legacy.cpuCurve,
            gpuCurve: legacy.gpuCurve,
            updatedAt: legacy.updatedAt || Date.now(),
          });
        }
      }
      return out;
    }

    function createDefaultSettings() {
      return {
        tab: "overview",
        mode: "custom",
        cpuPowerLimit: 95,
        gpuBoost: true,
        gpuMode: "hybrid",
        screenOverdrive: false,
        batteryCap: 100,
        coolerConnected: false,
        responseSpeed: 1,
        learningProfiles: {
          disconnected: createLearningProfile(),
          connected: createLearningProfile(),
        },
  	      jointLearning: false,
  	      learningBias: "quiet",
  	      jointBias: 0,
  	      cpuTarget: 70,
        gpuTarget: 70,
        quietCpuLimit: 95,
        quietGpuLimit: 85,
        risePrediction: false,
        debugMode: false,
        cpuCurve: cloneCurve(CPU_CURVE),
        gpuCurve: cloneCurve(GPU_CURVE),
        savedCurveTemplate: null,
        curveProfiles: [],
        activeCurveProfileId: null,
      };
    }

    function createLearningProfile() {
      return {
        fanLearning: false,
        jointLearning: false,
        revision: 0,
        resetAt: 0,
        learnedAt: 0,
      };
    }

    function normalizeLearningProfile(profile, fallbackEnabled) {
      const fanLearning = Boolean(profile && profile.fanLearning) || Boolean(fallbackEnabled);
      const jointLearning = Boolean(profile && profile.jointLearning) || Boolean(fallbackEnabled);
      const resetAt = Number(profile && profile.resetAt) || 0;
      const learnedAt = Number(profile && profile.learnedAt) || ((fanLearning || jointLearning) && !resetAt ? 1 : 0);
      return {
        fanLearning,
        jointLearning,
        revision: Number(profile && profile.revision) || 0,
        resetAt,
        learnedAt,
      };
    }

    function normalizeLearningProfiles(settings) {
      const legacyEnabled = Boolean(settings && settings.jointLearning);
      const profiles = settings && settings.learningProfiles ? settings.learningProfiles : {};
      return {
        disconnected: normalizeLearningProfile(profiles.disconnected, legacyEnabled),
        connected: normalizeLearningProfile(profiles.connected, legacyEnabled),
      };
    }

    function learningScheme(settings) {
      return settings && settings.coolerConnected ? "connected" : "disconnected";
    }

    function getLearningProfile(settings) {
      const profiles = normalizeLearningProfiles(settings);
      return profiles[learningScheme(settings)];
    }

    function learningPatch(settings, patch) {
      const profiles = normalizeLearningProfiles(settings);
      const scheme = learningScheme(settings);
      const nextProfile = {
        ...profiles[scheme],
        ...patch,
      };
      if (nextProfile.fanLearning || nextProfile.jointLearning) {
        if (!nextProfile.learnedAt || nextProfile.learnedAt <= nextProfile.resetAt) {
          nextProfile.learnedAt = Date.now();
        }
      } else {
        nextProfile.learnedAt = 0;
      }
      return {
        learningProfiles: {
          ...profiles,
          [scheme]: nextProfile,
        },
        jointLearning: Boolean(nextProfile.jointLearning),
      };
    }

    function resetLearningPatch(settings, scope) {
      const profiles = normalizeLearningProfiles(settings);
      const now = Date.now();
      const resetProfile = (profile) => ({
        ...profile,
        revision: (Number(profile.revision) || 0) + 1,
        resetAt: now,
        learnedAt: 0,
      });
      if (scope === "all") {
        return {
          learningProfiles: {
            disconnected: resetProfile(profiles.disconnected),
            connected: resetProfile(profiles.connected),
          },
        };
      }
      const scheme = learningScheme(settings);
      return {
        learningProfiles: {
          ...profiles,
          [scheme]: resetProfile(profiles[scheme]),
        },
      };
    }

    function hasLearnedOffsets(config) {
      const smart = config && config.smartControl;
      if (!smart) return false;
      return [
        "learnedOffsets",
        "learnedOffsetsHeat",
        "learnedOffsetsCool",
        "learnedRateHeat",
        "learnedRateCool",
      ].some((key) => Array.isArray(smart[key]) && smart[key].some((value) => Number(value) !== 0));
    }

    function learnedCurveFromConfig(curve, bias, config) {
      const offsets = config && config.smartControl && Array.isArray(config.smartControl.learnedOffsets)
        ? config.smartControl.learnedOffsets
        : null;
      if (offsets && offsets.some((value) => Number(value) !== 0)) {
        return curve.map((point, index) => ({
          ...point,
          rpm: roundRpm(point.rpm + (Number(offsets[index]) || 0)),
        }));
      }
      return learnedCurveFrom(curve, bias);
    }

    function learningVisible(settings, appConfig) {
      const profile = getLearningProfile(settings);
      return (profile.fanLearning || profile.jointLearning)
        && (hasLearnedOffsets(appConfig) || Number(profile.learnedAt) > Number(profile.resetAt || 0));
    }

    function learningResultState(profile, appConfig) {
      if (!(profile.fanLearning || profile.jointLearning)) {
        return { value: "未启用", detail: "开启学习后生成建议曲线" };
      }
      if (hasLearnedOffsets(appConfig) || Number(profile.learnedAt) > Number(profile.resetAt || 0)) {
        return { value: "已生成", detail: "虚线显示学习后的建议曲线" };
      }
      return { value: "等待学习", detail: profile.resetAt ? "已重置，等待后端重新生成" : "等待稳定温度样本" };
    }

    function normalizeSampleCount(value) {
      const numeric = Number(value);
      return SAMPLE_COUNTS.includes(numeric) ? numeric : 1;
    }

    function sampleCountLabel(value) {
      const count = normalizeSampleCount(value);
      if (count === 1) return "1 · 即时跟随";
      if (count === 2) return "2 · 轻度平滑";
      if (count === 3) return "3 · 默认";
      if (count === 5) return "5 · 更平滑";
      if (count === 10) return "10 · 最平滑";
      return `${count} · 平滑`;
    }

  	  function normalizeLearningBias(value) {
  	    return value === "cooling" || value === "quiet" ? value : "balanced";
  	  }

  	  function normalizeJointBias(value) {
  	    const number = Number(value);
  	    if (!Number.isFinite(number)) return 0;
  	    return Math.max(-100, Math.min(100, Math.round(number)));
  	  }

  	  function jointBiasLabel(value) {
  	    const bias = normalizeJointBias(value);
  	    if (bias > 0) return `偏电脑风扇 ${bias}%`;
  	    if (bias < 0) return `偏散热器 ${Math.abs(bias)}%`;
  	    return "均衡 0%";
  	  }

    function readStoredSettings() {
      try {
        const raw = window.localStorage.getItem(STORAGE_KEY)
          || window.localStorage.getItem("fancontrol.omen-fan.preview.v2")
          || window.localStorage.getItem("fancontrol.omen-fan.preview.v1");
        if (!raw) return createDefaultSettings();
        const parsed = JSON.parse(raw);
        return {
          ...createDefaultSettings(),
          ...parsed,
          learningProfiles: normalizeLearningProfiles(parsed),
          cpuCurve: normalizeCurve(parsed.cpuCurve, CPU_CURVE, CPU_TEMP_MAX),
          gpuCurve: normalizeCurve(parsed.gpuCurve, GPU_CURVE, GPU_TEMP_MAX),
          savedCurveTemplate: cloneSavedCurveTemplate(parsed.savedCurveTemplate),
          curveProfiles: normalizeCurveProfiles(parsed.curveProfiles, parsed.savedCurveTemplate),
          activeCurveProfileId: typeof parsed.activeCurveProfileId === "string" ? parsed.activeCurveProfileId : null,
        };
      } catch {
        return createDefaultSettings();
      }
    }

    function writeStoredSettings(settings) {
      try {
        window.localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
      } catch {
        /* ignore storage failures in restricted WebView contexts */
      }
    }

    function clamp(value, min, max) {
      const number = Number(value);
      if (!Number.isFinite(number)) return min;
      return Math.min(max, Math.max(min, number));
    }

    function roundRpm(value) {
      return Math.round(clamp(value, RPM_MIN, RPM_MAX) / RPM_STEP) * RPM_STEP;
    }

    function normalizeCurve(curve, fallback, maxTemp) {
      const source = Array.isArray(curve) && curve.length ? curve : fallback;
      const byTemp = new Map();
      for (let temp = TEMP_MIN; temp <= maxTemp; temp += 5) {
        const found = source.find((point) => Number(point.temperature) === temp);
        byTemp.set(temp, roundRpm(found ? found.rpm : rpmAtTemp(source, temp)));
      }
      return Array.from(byTemp, ([temperature, rpm]) => ({ temperature, rpm }));
    }

    function rpmAtTemp(curve, temp) {
      const points = [...curve].sort((left, right) => left.temperature - right.temperature);
      if (!points.length) return RPM_MIN;
      if (temp <= points[0].temperature) return points[0].rpm;
      if (temp >= points[points.length - 1].temperature) return points[points.length - 1].rpm;
      for (let index = 1; index < points.length; index += 1) {
        const right = points[index];
        const left = points[index - 1];
        if (temp <= right.temperature) {
          const span = right.temperature - left.temperature || 1;
          const ratio = (temp - left.temperature) / span;
          return roundRpm(left.rpm + (right.rpm - left.rpm) * ratio);
        }
      }
      return roundRpm(points[points.length - 1].rpm);
    }

    function learnedCurveFrom(curve, bias) {
      const direction = bias === "quiet" ? -1 : 1;
      return curve.map((point, index) => ({
        ...point,
        rpm: roundRpm(point.rpm + direction * (index > curve.length * 0.55 ? 180 : 80)),
      }));
    }

    function formatRpm(value) {
      const number = Number(value);
      return Number.isFinite(number) && number > 0 ? `${Math.round(number).toLocaleString()} RPM` : "0 RPM";
    }

    function formatTemp(value) {
      const number = Number(value);
      return Number.isFinite(number) && number > 0 ? `${Math.round(number)}°C` : "--°C";
    }

    function formatWatts(value) {
      const number = Number(value);
      return Number.isFinite(number) && number > 0 ? `${Math.round(number)} W` : "-- W";
    }

    function getStatusValue(status, key, fallback) {
      const value = status && status[key];
      return value === undefined || value === null ? fallback : value;
    }


    function usePersistentSettings(React) {
      const [settings, setSettings] = React.useState(readStoredSettings);
      const update = React.useCallback((patch) => {
        setSettings((current) => {
          const next = typeof patch === "function" ? patch(current) : { ...current, ...patch };
          writeStoredSettings(next);
          return next;
        });
      }, []);
      return [settings, update];
    }

    async function fetchMockStatus(signal) {
      const response = await fetch(`${MOCK_BASE_URL}/status`, { cache: "no-store", signal });
      if (!response.ok) throw new Error(`${response.status} ${response.statusText}`);
      const payload = await response.json();
      return payload && payload.status ? payload.status : payload;
    }

    async function postMock(path, body) {
      const response = await fetch(`${MOCK_BASE_URL}${path}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body || {}),
      });
      if (!response.ok) throw new Error(`${response.status} ${response.statusText}`);
      const payload = await response.json();
      return payload && payload.status ? payload.status : payload;
    }

  Object.assign(ns, {
    id: PLUGIN_ID,
    PLUGIN_ID,
    MOCK_BASE_URL,
    STORAGE_KEY,
    RPM_MIN,
    RPM_MAX,
    RPM_STEP,
    TEMP_MIN,
    CPU_TEMP_MAX,
    GPU_TEMP_MAX,
    SAMPLE_COUNTS,
    CPU_CURVE,
    GPU_CURVE,
    QUIET_CPU_CURVE,
    QUIET_GPU_CURVE,
    COOL_CPU_CURVE,
    COOL_GPU_CURVE,
    MODE_CARDS,
    TABS,
    curveFromPairs,
    cloneCurve,
    cloneSavedCurveTemplate,
    cloneCurveProfile,
    normalizeCurveProfiles,
    formatTemplateTime,
    createDefaultSettings,
    createLearningProfile,
    normalizeLearningProfile,
    normalizeLearningProfiles,
    learningScheme,
    getLearningProfile,
    learningPatch,
    resetLearningPatch,
    hasLearnedOffsets,
    learnedCurveFromConfig,
    learningVisible,
    learningResultState,
    normalizeSampleCount,
    sampleCountLabel,
    normalizeLearningBias,
    normalizeJointBias,
    jointBiasLabel,
    readStoredSettings,
    writeStoredSettings,
    clamp,
    roundRpm,
    normalizeCurve,
    rpmAtTemp,
    learnedCurveFrom,
    formatRpm,
    formatTemp,
    formatWatts,
    getStatusValue,
    usePersistentSettings,
    fetchMockStatus,
    postMock,
  });
})();
