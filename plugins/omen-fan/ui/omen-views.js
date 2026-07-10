(function () {
  "use strict";

  const ns = window.FanControlOmenFanPlugin = window.FanControlOmenFanPlugin || {};

  function register(host) {
    if (!host || typeof host.registerPage !== "function") {
      throw new Error("FanControl plugin host is not available");
    }
    const React = host.React;
    const ui = ns.createUiKit(host);
    const {
      e,
      Icon,
      OmenMark,
      StatusPill,
      CardShell,
      TabSwitch,
      Button,
      Toggle,
      SelectControl,
      SliderControl,
      FanCurveEditor,
    } = ui;
    const {
      PLUGIN_ID,
      SAMPLE_COUNTS,
      CPU_CURVE,
      GPU_CURVE,
      QUIET_CPU_CURVE,
      QUIET_GPU_CURVE,
      COOL_CPU_CURVE,
      COOL_GPU_CURVE,
      MODE_CARDS,
      TEMP_MIN,
      RPM_MIN,
      RPM_MAX,
      RPM_STEP,
      CPU_TEMP_MAX,
      GPU_TEMP_MAX,
      getStatusValue,
      roundRpm,
      rpmAtTemp,
      normalizeSampleCount,
      sampleCountLabel,
      normalizeLearningBias,
      normalizeJointBias,
      jointBiasLabel,
      getLearningProfile,
      learningPatch,
      resetLearningPatch,
      fetchMockStatus,
      postMock,
      cloneSavedCurveTemplate,
      cloneCurve,
      cloneCurveProfile,
      formatTemplateTime,
      formatRpm,
      formatTemp,
      formatWatts,
      learningVisible,
      learningResultState,
      learnedCurveFromConfig,
      usePersistentSettings,
      installStyle,
    } = ns;
    let toastTimer = 0;

        function OmenFanPage({ plugin }) {
          React.useEffect(() => {
            installStyle();
          }, []);
          const [settings, updateSettings] = usePersistentSettings(React);
          const [appConfig, setAppConfig] = React.useState(null);
          const [status, setStatus] = React.useState(null);
          const [mockConnected, setMockConnected] = React.useState(false);
          const [loading, setLoading] = React.useState(false);
          const [toastText, setToastText] = React.useState("");
          const mountedRef = React.useRef(true);
          const activeControllerRef = React.useRef(null);
          const temperature = host.useAppStore((store) => store.temperature);

          const cpuTemp = Number(getStatusValue(status, "cpuTemp", temperature && temperature.cpuTemp)) || 70;
          const gpuTemp = Number(getStatusValue(status, "gpuTemp", temperature && temperature.gpuTemp)) || 55;
          const cpuPower = Number(getStatusValue(status, "cpuPowerWatts", temperature && temperature.cpuPowerWatts)) || 0;
          const gpuPower = Number(getStatusValue(status, "gpuPowerWatts", temperature && temperature.gpuPowerWatts)) || 0;
    	      const cpuTarget = roundRpm(rpmAtTemp(settings.cpuCurve, cpuTemp));
    	      const gpuTarget = roundRpm(rpmAtTemp(settings.gpuCurve, gpuTemp));
    	      const mode = MODE_CARDS.find((item) => item.id === settings.mode) || MODE_CARDS[3];
    	      const canEditCurve = settings.mode === "custom";
    	      const appSampleCount = appConfig ? normalizeSampleCount(appConfig.tempSampleCount) : normalizeSampleCount(settings.responseSpeed);
    	      const hardwareUnsupported = plugin && plugin.supported === false && !settings.debugMode && !mockConnected;
    	      const unsupportedReason = plugin && plugin.lastError
    	        ? `检测信息：${plugin.lastError}`
    	        : "当前设备没有通过 OMEN WMI 检测，真实硬件控制不会启用。可打开右侧调试模式预览页面和 Mock 后端。";

          const showToast = React.useCallback((message, kind) => {
            if (!mountedRef.current) return;
            setToastText(message);
            if (host.toast) {
              const notify = kind === "error" ? host.toast.error : host.toast.success;
              if (typeof notify === "function") notify(message);
            }
            window.clearTimeout(toastTimer);
            toastTimer = window.setTimeout(() => setToastText(""), 1800);
          }, []);

    	      const updateWithToast = React.useCallback((patch, message) => {
    	        updateSettings(patch);
    	        if (message) showToast(message);
    	      }, [showToast, updateSettings]);

    	      const refreshAppConfig = React.useCallback(async () => {
    	        if (!host.apiService || typeof host.apiService.getConfig !== "function") return;
    	        try {
    	          const config = await host.apiService.getConfig();
    	          if (mountedRef.current) {
    	            setAppConfig(config || null);
    	            const sampleCount = normalizeSampleCount(config && config.tempSampleCount);
    	            const smartControl = config && config.smartControl ? config.smartControl : {};
    	            updateSettings({
    	              responseSpeed: sampleCount,
    	              learningBias: normalizeLearningBias(smartControl.learningBias),
    	              jointBias: normalizeJointBias(smartControl.jointBias),
    	            });
    	          }
    	        } catch {
    	          /* Plugin must keep working when the main config API is unavailable. */
    	        }
    	      }, [updateSettings]);

          const saveAppConfigPatch = React.useCallback(async (patch) => {
            if (!host.apiService || typeof host.apiService.getConfig !== "function" || typeof host.apiService.updateConfig !== "function") {
              return null;
            }
            const current = await host.apiService.getConfig();
            const next = { ...(current || {}), ...(patch || {}) };
            await host.apiService.updateConfig(next);
            if (mountedRef.current) setAppConfig(next);
            return next;
          }, []);

          const patchSmartControl = React.useCallback(async (patch) => {
            if (!host.apiService || typeof host.apiService.getConfig !== "function" || typeof host.apiService.updateConfig !== "function") {
              return null;
            }
            const current = await host.apiService.getConfig();
            const smartControl = {
              ...((current && current.smartControl) || {}),
              ...(patch || {}),
            };
            const next = { ...(current || {}), smartControl };
            await host.apiService.updateConfig(next);
            if (mountedRef.current) setAppConfig(next);
            return next;
          }, []);

    	      const setSampleCount = React.useCallback(async (value) => {
    	        const sampleCount = normalizeSampleCount(value);
    	        updateSettings({ responseSpeed: sampleCount });
    	        try {
    	          await saveAppConfigPatch({ tempSampleCount: sampleCount });
    	          showToast("温度平滑度已保存");
    	        } catch (error) {
    	          const message = error instanceof Error ? error.message : String(error || "保存失败");
    	          showToast(`温度平滑度保存失败：${message}`, "error");
    	        }
    	      }, [saveAppConfigPatch, showToast, updateSettings]);

    	      const setJointBias = React.useCallback(async (value) => {
    	        const jointBias = normalizeJointBias(value);
    	        updateSettings({ jointBias });
    	        try {
    	          await patchSmartControl({ jointBias });
    	          showToast("联合偏置已保存");
    	        } catch (error) {
    	          const failure = error instanceof Error ? error.message : String(error || "保存失败");
    	          showToast(`联合偏置保存失败：${failure}`, "error");
    	        }
    	      }, [patchSmartControl, showToast, updateSettings]);

    	      const setLearningState = React.useCallback(async (patch, message) => {
    	        const nextProfile = {
    	          ...getLearningProfile(settings),
    	          ...(patch || {}),
    	        };
    	        updateSettings((current) => ({
    	          ...current,
    	          ...learningPatch(current, patch),
    	        }));
    	        try {
    	          const smartPatch = {
    	            enabled: Boolean(nextProfile.jointLearning),
    	            learning: Boolean(nextProfile.fanLearning || nextProfile.jointLearning),
    	            learningBias: normalizeLearningBias(settings.learningBias),
    	            jointBias: normalizeJointBias(settings.jointBias),
    	          };
    	          await patchSmartControl(smartPatch);
    	          if (message) showToast(message);
    	        } catch (error) {
    	          const failure = error instanceof Error ? error.message : String(error || "保存失败");
    	          showToast(`学习设置保存失败：${failure}`, "error");
    	        }
    	      }, [patchSmartControl, settings, showToast, updateSettings]);

          const resetLearning = React.useCallback(async (scope) => {
            updateSettings((current) => ({
              ...current,
              ...resetLearningPatch(current, scope),
    	        }));
    	        try {
    	          if (host.apiService && typeof host.apiService.resetLearnedOffsets === "function") {
    	            await host.apiService.resetLearnedOffsets();
    	          }
    	          if (host.apiService && typeof host.apiService.getConfig === "function") {
    	            const config = await host.apiService.getConfig();
    	            if (mountedRef.current) setAppConfig(config || null);
    	          }
    	          showToast(scope === "all" ? "全部学习结果已重置" : "当前方案学习结果已重置");
    	        } catch (error) {
    	          const failure = error instanceof Error ? error.message : String(error || "重置失败");
    	          showToast(`主程序学习重置失败：${failure}`, "error");
    	        }
          }, [showToast, updateSettings]);

          const refresh = React.useCallback(async () => {
            if (activeControllerRef.current) {
              activeControllerRef.current.abort();
            }
            const controller = new AbortController();
            activeControllerRef.current = controller;
            const timeout = window.setTimeout(() => controller.abort(), 1600);
            if (mountedRef.current) setLoading(true);
            try {
              const next = await fetchMockStatus(controller.signal);
              if (!mountedRef.current || controller.signal.aborted) return;
              setStatus(next);
              setMockConnected(true);
            } catch {
              if (!mountedRef.current || controller.signal.aborted) return;
              setMockConnected(false);
            } finally {
              window.clearTimeout(timeout);
              if (activeControllerRef.current === controller) {
                activeControllerRef.current = null;
              }
              if (mountedRef.current) setLoading(false);
            }
          }, []);

          React.useEffect(() => {
            mountedRef.current = true;
            refresh();
            refreshAppConfig();
            const timer = window.setInterval(refresh, 5000);
            return () => {
              mountedRef.current = false;
              window.clearInterval(timer);
              window.clearTimeout(toastTimer);
              if (activeControllerRef.current) {
                activeControllerRef.current.abort();
                activeControllerRef.current = null;
              }
            };
          }, [refresh, refreshAppConfig]);

          const setMode = async (modeID) => {
            const nextMode = MODE_CARDS.find((item) => item.id === modeID) || MODE_CARDS[3];
            updateSettings({ mode: modeID, cpuPowerLimit: nextMode.cpu });
            showToast(`${nextMode.title}模式已保存`);
            if (mockConnected) {
              try {
                const next = await postMock("/mode", { mode: modeID });
                if (mountedRef.current) setStatus(next);
              } catch {
                if (mountedRef.current) setMockConnected(false);
              }
            }
          };

          const setPowerLimit = async (watts) => {
            updateSettings({ cpuPowerLimit: watts });
            showToast(`CPU 功耗限制已设为 ${watts} W`);
            if (mockConnected) {
              try {
                const next = await postMock("/power", { powerLimitWatts: watts, cpuPowerLimitWatts: watts });
                if (mountedRef.current) setStatus(next);
              } catch {
                if (mountedRef.current) setMockConnected(false);
              }
            }
          };

          const saveCurve = async () => {
            if (!canEditCurve) {
              showToast("当前模式不允许编辑曲线，请切换到大师模式。", "error");
              return;
            }
            const payload = { cpuRpm: cpuTarget, gpuRpm: gpuTarget };
            if (mockConnected) {
              try {
                const next = await postMock("/set-fan", payload);
                if (mountedRef.current) setStatus(next);
                showToast("曲线已应用到 Mock 后端");
                return;
    	          } catch {
    	            if (mountedRef.current) setMockConnected(false);
    	          }
    	        }
    	        showToast("曲线已保存为本地预览");
    	      };

          const enablePlugin = async () => {
            try {
              if (host.apiService && host.apiService.enablePlugin) {
                await host.apiService.enablePlugin(PLUGIN_ID);
              }
              if (host.apiService && host.apiService.refreshPluginDiscovery) {
                await host.apiService.refreshPluginDiscovery();
              }
              showToast("已请求启用插件");
            } catch (error) {
    	          const message = error instanceof Error ? error.message : String(error || "启用插件失败");
    	          showToast(`启用插件失败：${message}`, "error");
    	        }
    	      };

    	      const tabContent = settings.tab === "overview"
    	        ? e(OverviewView, {
    	          e, Icon, StatusPill, Toggle, settings, updateWithToast, setMode, setPowerLimit, mode,
    	          status, mockConnected, cpuTemp, gpuTemp, cpuPower, gpuPower, cpuTarget, gpuTarget,
    	        })
    	        : settings.tab === "curve"
    	          ? e(CurveView, {
    	            e, Icon, StatusPill, Button, Toggle, SliderControl, FanCurveEditor, settings, updateSettings, updateWithToast, cpuTemp, gpuTemp,
    	            cpuTarget, gpuTarget, saveCurve, canEditCurve, appConfig, setLearningState, setJointBias, resetLearning,
    	          })
    	          : e(SettingsView, {
    	            e, Icon, Toggle, SelectControl, SliderControl, settings, updateWithToast, appSampleCount, setSampleCount, patchSmartControl,
    	          });

    	      return e("div", { className: "omen-page" },
            toastText ? e("div", { className: "omen-toast" }, e(Icon, { name: "CircleCheck", size: 17 }), toastText) : null,
            e("div", { className: "omen-topbar", "data-theme-card": "curve-header" },
              e("div", null,
                e("div", { className: "omen-title-line" },
                  e("span", { className: "omen-mark" }, e(OmenMark)),
                  e("h1", { className: "omen-title" }, "OMEN 笔记本风扇"),
                  e("span", { className: "omen-pill-row" },
                    e(StatusPill, { tone: plugin.running ? "success" : "warning" }, plugin.running ? "运行中" : "已停止"),
                    e(StatusPill, { tone: mockConnected ? "success" : "warning" }, mockConnected ? "Mock:8787 已连接" : "Mock:8787 离线"),
                    settings.debugMode ? e(StatusPill, { tone: "info" }, "调试模式") : null,
                  ),
                ),
                e("div", { className: "omen-subtle", style: { marginTop: 4 } }, "温度与功耗优先读取 FanControl 主程序传感器设置；Mock 调试端可覆盖预览值。"),
              ),
              e("div", { className: "omen-toolbar" },
                e(TabSwitch, { settings, updateSettings }),
                e(Button, { onClick: refresh, disabled: loading, icon: e(Icon, { name: "RefreshCw", size: 15 }) }, loading ? "刷新中" : "刷新"),
                e(Button, { onClick: () => updateWithToast({ debugMode: !settings.debugMode }, !settings.debugMode ? "调试模式已开启" : "调试模式已关闭"), icon: e(Icon, { name: "Bug", size: 15 }) }, settings.debugMode ? "关闭调试" : "调试模式"),
    	            e(Button, { onClick: enablePlugin, primary: true, icon: e(Icon, { name: "Power", size: 15 }) }, "启用插件"),
    	          ),
    	        ),
    	        hardwareUnsupported ? e("div", { className: "omen-alert warning" },
    	          e(Icon, { name: "AlertTriangle", size: 18 }),
    	          e("div", null,
    	            e("strong", null, "当前机型未通过 OMEN 检测"),
    	            e("p", null, unsupportedReason),
    	          ),
    	        ) : null,
    	        settings.debugMode ? e("div", { className: "omen-alert warning" },
    	          e(Icon, { name: "AlertTriangle", size: 18 }),
    	          e("div", null,
    	            e("strong", null, "非 OMEN 机型调试"),
    	            e("p", null, "当前页面允许在不匹配机型上预览前端与 Mock 后端。真实硬件写入仍需通过驱动检测与机型验证。"),
    	          ),
    	        ) : null,
    	        e("div", { key: settings.tab, className: "omen-tab-panel" }, tabContent),
    	      );
    	    }

        function OverviewView(props) {
          const {
            e, Icon, StatusPill, Toggle, settings, updateWithToast, setMode, setPowerLimit,
            mode, mockConnected, cpuTemp, gpuTemp, cpuPower, gpuPower, cpuTarget, gpuTarget,
          } = props;
          const cpuRpm = getStatusValue(props.status, "cpuRpm", 0);
          const gpuRpm = getStatusValue(props.status, "gpuRpm", 0);

          return e(React.Fragment, null,
            e(CardShell, { className: "glacier-hero-card omen-hero", dataThemeCard: "device-hero", dataThemeSection: "hero" },
              e("span", { className: "omen-hero-mark" }, e(OmenMark)),
              e("div", null,
                e("div", { className: "omen-inline" },
                  e("h2", null, "HP OMEN 笔记本风扇"),
                  e(StatusPill, { tone: "info" }, `${mode.title}模式`),
                  e(StatusPill, null, `CPU ${settings.cpuPowerLimit} W`),
                ),
                e("p", { style: { marginTop: 5 } }, `${mode.title}模式：独立曲线由软件闭环控制 · ${mockConnected ? "Mock 后端已连接" : "本地预览"}`),
              ),
              e(Toggle, {
                checked: settings.gpuBoost,
                onChange: (gpuBoost) => updateWithToast({ gpuBoost }, gpuBoost ? "GPU 动态加速已开启" : "GPU 动态加速已关闭"),
                label: "GPU 动态加速",
              }),
            ),
            e("div", { className: "omen-grid metrics" },
              e(MetricCard, { e, Icon, icon: "Fan", label: "CPU 风扇", value: formatRpm(cpuRpm), detail: `软件目标 ${formatRpm(cpuTarget)} · CPU 曲线`, dataThemeCard: "fan-speed" }),
              e(MetricCard, { e, Icon, icon: "Fan", label: "GPU 风扇", value: formatRpm(gpuRpm), detail: `软件目标 ${formatRpm(gpuTarget)} · GPU 曲线`, dataThemeCard: "fan-speed" }),
              e(MetricCard, { e, Icon, icon: "Cpu", label: "CPU 温度", value: formatTemp(cpuTemp), detail: `CPU 功耗 ${formatWatts(cpuPower)}`, dataThemeCard: "cpu-temperature" }),
              e(MetricCard, { e, Icon, icon: "Monitor", label: "GPU 温度", value: formatTemp(gpuTemp), detail: `GPU 功耗 ${formatWatts(gpuPower)}`, dataThemeCard: "gpu-temperature" }),
            ),
            e(CardShell, { className: "glacier-control-card omen-section", dataThemeSection: "control-protection", dataThemeCard: "omen-performance" },
              e("div", { className: "omen-section-head" },
                e("div", null,
                  e("div", { className: "omen-inline" }, e(Icon, { name: "Sparkles", size: 18, className: "text-primary" }), e("h2", { className: "omen-section-title" }, "性能档位")),
                ),
                e("div", { className: "omen-chip-row" },
                  e("span", { className: "omen-subtle" }, "CPU 功耗限制"),
                  [35, 65, 95].map((watts) => e(Button, {
                    key: watts,
                    primary: settings.cpuPowerLimit === watts,
                    onClick: () => setPowerLimit(watts),
                  }, `CPU ${watts}W`)),
                ),
              ),
              e("div", { className: "omen-grid modes" }, MODE_CARDS.map((card) => (
                e(ModeCard, {
                  e, Icon,
                  key: card.id,
                  card,
                  active: settings.mode === card.id,
                  onClick: () => setMode(card.id),
                })
              ))),
            ),
            e(CardShell, { className: "glacier-control-card omen-section", dataThemeCard: "omen-system" },
              e("div", { className: "omen-section-head" },
                e("div", null,
                  e("div", { className: "omen-inline" }, e(Icon, { name: "Settings2", size: 18, className: "text-primary" }), e("h2", { className: "omen-section-title" }, "系统设置")),
                  e("p", { style: { marginTop: 4 } }, "这些项目先作为插件前端状态预览，真实硬件写入仍需后端能力逐步接入。"),
                ),
              ),
              e("div", { className: "omen-grid system" },
                e(SystemCard, {
                  e, Icon, icon: "Zap", title: "GPU 模式（MUX）",
                  value: `当前：${settings.gpuMode === "hybrid" ? "混合（Optimus）" : "独显"}`,
                  note: "切换后通常需要重启",
                },
                  e("div", { className: "omen-system-actions" },
                    e(SystemButton, { e, active: settings.gpuMode === "hybrid", onClick: () => updateWithToast({ gpuMode: "hybrid" }, "GPU 模式设置已保存") }, "混合"),
                    e(SystemButton, { e, active: settings.gpuMode === "discrete", onClick: () => updateWithToast({ gpuMode: "discrete" }, "GPU 模式设置已保存") }, "独显"),
                  ),
                ),
                e(SystemCard, {
                  e, Icon, icon: "Monitor", title: "屏幕过驱",
                  value: settings.screenOverdrive ? "高刷屏减少拖影" : "关闭",
                  note: "依赖机型支持",
                },
                  e("div", { className: "omen-system-actions single" },
                    e(Toggle, {
                      checked: settings.screenOverdrive,
                      onChange: (screenOverdrive) => updateWithToast({ screenOverdrive }, "屏幕过驱设置已保存"),
                      label: settings.screenOverdrive ? "已开启" : "已关闭",
                    }),
                  ),
                ),
                e(SystemCard, {
                  e, Icon, icon: "BatteryCharging", title: "电池充电上限",
                  value: `限制 ${settings.batteryCap}%`,
                  note: settings.batteryCap === 80 ? "延长电池寿命" : "充满电池",
                },
                  e("div", { className: "omen-system-actions" },
                    e(SystemButton, { e, active: settings.batteryCap === 100, onClick: () => updateWithToast({ batteryCap: 100 }, "电池充电上限已保存") }, "100%"),
                    e(SystemButton, { e, active: settings.batteryCap === 80, onClick: () => updateWithToast({ batteryCap: 80 }, "电池充电上限已保存") }, "80%"),
                  ),
                ),
              ),
            ),
          );
        }

        function MetricCard({ e, Icon, icon, label, value, detail, dataThemeCard }) {
          return e(CardShell, { className: "glacier-stat-tile omen-metric-card", dataThemeCard, padding: "sm" },
            e("div", { className: "omen-metric-head" }, e(Icon, { name: icon, size: 17 }), label),
            e("div", { className: "omen-metric-value" }, value),
            detail ? e("div", { className: "omen-metric-detail" }, detail) : null,
          );
        }

        function ModeCard({ e, Icon, card, active, onClick }) {
          const detail = card.id === "balanced"
            ? "平衡性能与噪声"
            : card.id === "performance"
              ? "更高频率与 CPU 功耗"
              : card.id === "quiet"
                ? "低噪声散热"
                : "自定义风扇曲线";
          return e(CardShell, {
            className: `glacier-stat-tile omen-mode-card${active ? " active" : ""}`,
            dataThemeCard: "control-mode",
            padding: "md",
            hover: true,
            onClick,
          },
            e("span", { className: "omen-card-icon" }, e(Icon, { name: card.icon, size: 18 })),
            active ? e("span", { className: "omen-mode-check" }, e(Icon, { name: "Check", size: 16 })) : null,
            e("div", { className: "omen-mode-title" }, card.title),
            e("div", { className: "omen-mode-detail" }, detail),
          );
        }

        function SystemCard({ e, Icon, icon, title, value, note, children }) {
          return e(CardShell, { className: "glacier-stat-tile omen-system-card", dataThemeCard: "omen-system-card", padding: "md" },
            e("div", { className: "omen-system-title" }, e(Icon, { name: icon, size: 17 }), title),
            e("div", { className: "omen-system-value" }, value),
            note ? e("div", { className: "omen-system-note" }, note) : null,
            children || null,
          );
        }

        function SystemButton({ e, active, onClick, children }) {
          return e("button", {
            type: "button",
            className: `omen-system-button${active ? " active" : ""}`,
            onClick,
          }, children);
        }

        function CurveView(props) {
          const {
            e, Icon, StatusPill, Button, Toggle, SliderControl, FanCurveEditor, settings, updateSettings, updateWithToast,
            cpuTemp, gpuTemp, cpuTarget, gpuTarget, saveCurve, canEditCurve, appConfig, setLearningState, setJointBias,
            resetLearning,
          } = props;
          const learningProfile = getLearningProfile(settings);
          const showLearnedCurve = learningVisible(settings, appConfig);
          const learningState = learningResultState(learningProfile, appConfig);
          const schemeLabel = settings.coolerConnected ? "散热器已连接方案" : "散热器未连接方案";
          const jointBias = normalizeJointBias(settings.jointBias);
          const [jointBiasDraft, setJointBiasDraft] = React.useState(jointBias);
          const savedTemplate = cloneSavedCurveTemplate(settings.savedCurveTemplate);
          const jointBiasDraftValue = normalizeJointBias(jointBiasDraft);
          const jointBiasDirty = jointBiasDraftValue !== jointBias;
          React.useEffect(() => {
            setJointBiasDraft(jointBias);
          }, [jointBias]);
          const saveJointBias = () => {
            const nextBias = normalizeJointBias(jointBiasDraft);
            if (typeof setJointBias === "function") {
              setJointBias(nextBias);
              return;
            }
            updateWithToast({ jointBias: nextBias }, "联合偏置已保存");
          };
          const learningSwitch = ({ icon, title, detail, active, onChange }) => e("div", {
            className: "flex flex-col gap-3 md:flex-row md:items-center md:justify-between rounded-xl border border-border/70 bg-background/45 p-3",
            "data-theme-ui": "setting-row",
          },
            e("div", { className: "flex min-w-0 items-center gap-3" },
              e("div", { className: "flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground", "data-theme-ui": "setting-row-icon" },
                e(Icon, { name: icon, size: 16, className: "text-primary" }),
              ),
              e("div", { className: "min-w-0" },
                e("div", { className: "flex flex-wrap items-center gap-2" },
                  e("div", { className: "text-sm font-medium text-foreground" }, title),
                  e(StatusPill, { tone: active ? "info" : "muted" }, active ? "已开启" : "已关闭"),
                ),
                e("div", { className: "text-xs leading-relaxed text-muted-foreground" }, detail),
              ),
            ),
            e("div", { className: "flex shrink-0 items-center gap-2" },
              e(Toggle, { checked: active, onChange, label: title }),
            ),
          );

          // Multi-profile curve management. Replaces the legacy single-slot template UI
          // (savedCurveTemplate + saveCurveTemplate + applySavedCurveTemplate). Users can now
          // maintain any number of named profiles and switch between them, similar to the
          // main app's fan-curve profile system.
          const curveProfiles = Array.isArray(settings.curveProfiles) ? settings.curveProfiles : [];
          const activeCurveProfile = curveProfiles.find((p) => p && p.id === settings.activeCurveProfileId) || null;

          const applyCurveProfile = (profileId) => {
            const profile = curveProfiles.find((p) => p && p.id === profileId);
            if (!profile) return;
            updateWithToast({
              mode: "custom",
              activeCurveProfileId: profileId,
              cpuCurve: cloneCurve(profile.cpuCurve),
              gpuCurve: cloneCurve(profile.gpuCurve),
            }, `已套用方案「${profile.name}」`);
          };

          const saveAsNewProfile = () => {
            const suggested = `自定义 ${curveProfiles.length + 1}`;
            const raw = window.prompt("为当前曲线方案命名", suggested);
            if (raw === null) return;
            const name = String(raw).trim() || "未命名方案";
            const id = "p_" + Date.now().toString(36) + "_" + Math.random().toString(36).slice(2, 8);
            const next = curveProfiles.concat([{
              id,
              name,
              cpuCurve: cloneCurve(settings.cpuCurve),
              gpuCurve: cloneCurve(settings.gpuCurve),
              updatedAt: Date.now(),
            }]);
            updateWithToast({ curveProfiles: next, activeCurveProfileId: id }, `已保存为方案「${name}」`);
          };

          const overwriteActiveProfile = () => {
            if (!activeCurveProfile) return;
            const next = curveProfiles.map((p) =>
              p && p.id === activeCurveProfile.id
                ? { ...p, cpuCurve: cloneCurve(settings.cpuCurve), gpuCurve: cloneCurve(settings.gpuCurve), updatedAt: Date.now() }
                : p,
            );
            updateWithToast({ curveProfiles: next }, `方案「${activeCurveProfile.name}」已更新`);
          };

          const renameActiveProfile = () => {
            if (!activeCurveProfile) return;
            const raw = window.prompt("重命名方案", activeCurveProfile.name);
            if (raw === null) return;
            const trimmed = String(raw).trim();
            if (!trimmed || trimmed === activeCurveProfile.name) return;
            const next = curveProfiles.map((p) =>
              p && p.id === activeCurveProfile.id ? { ...p, name: trimmed } : p,
            );
            updateWithToast({ curveProfiles: next }, `已重命名为「${trimmed}」`);
          };

          const deleteActiveProfile = () => {
            if (!activeCurveProfile) return;
            if (curveProfiles.length <= 1) return; // 至少保留一个方案，与主软件 FanCurve 行为一致
            if (!window.confirm(`确定删除方案「${activeCurveProfile.name}」？此操作不可撤销。`)) return;
            const next = curveProfiles.filter((p) => p && p.id !== activeCurveProfile.id);
            const fallbackId = next[0] ? next[0].id : null;
            updateWithToast({ curveProfiles: next, activeCurveProfileId: fallbackId }, `已删除方案「${activeCurveProfile.name}」`);
          };

          const applyTemplate = (name) => {
            if (name === "quiet") {
              updateWithToast({ mode: "custom", cpuCurve: cloneCurve(QUIET_CPU_CURVE), gpuCurve: cloneCurve(QUIET_GPU_CURVE) }, "静音模板已载入");
              return;
            }
            if (name === "cool") {
              updateWithToast({ mode: "custom", cpuCurve: cloneCurve(COOL_CPU_CURVE), gpuCurve: cloneCurve(COOL_GPU_CURVE) }, "强散热模板已载入");
              return;
            }
            updateWithToast({ mode: "custom", cpuCurve: cloneCurve(CPU_CURVE), gpuCurve: cloneCurve(GPU_CURVE) }, "默认模板已载入");
          };

          return e(React.Fragment, null,
            e("div", { className: "omen-section-head", "data-theme-card": "curve-header" },
              e("div", null,
                e("div", { className: "omen-inline" },
                  e(Icon, { name: "Spline", size: 18, className: "text-primary" }),
                  e("h2", { className: "omen-section-title" }, "OMEN 风扇曲线"),
                  e(StatusPill, { tone: canEditCurve ? "info" : "warning" }, canEditCurve ? "大师模式可编辑" : "非自定义模式只读"),
                ),
                e("p", { className: "omen-subtle", style: { marginTop: 4 } }, "CPU/GPU 独立控制，5°C 节点；拖动节点预览，应用时按 100 RPM 取整。"),
              ),
              e("div", { className: "omen-curve-toolbar" },
                e(Button, { primary: !settings.coolerConnected, onClick: () => updateWithToast({ coolerConnected: false }, "已切换为散热器未连接方案") }, "散热器未连接"),
                e(Button, { primary: settings.coolerConnected, onClick: () => updateWithToast({ coolerConnected: true }, "已切换为散热器已连接方案") }, "散热器已连接"),
                e(Button, { onClick: () => applyTemplate("default"), icon: e(Icon, { name: "RotateCcw", size: 15 }) }, "还原"),
                e(Button, { onClick: saveCurve, primary: true, disabled: !canEditCurve, icon: e(Icon, { name: "Save", size: 15 }) }, "保存曲线"),
              ),
            ),
            !canEditCurve ? e("div", { className: "omen-alert warning" },
              e(Icon, { name: "AlertTriangle", size: 18 }),
              e("div", null,
                e("strong", null, "当前模式不会应用自定义风扇曲线"),
                e("p", { style: { marginTop: 3 } }, "切换到大师模式后，CPU 与 GPU 曲线才会进入软件闭环控制。"),
              ),
            ) : null,
            e(CardShell, { className: "glacier-control-card omen-section", dataThemeCard: "omen-templates" },
              e("div", { className: "omen-section-head", style: { marginBottom: 0 } },
                e("div", null,
                  e("div", { className: "omen-inline" }, e(Icon, { name: "Sparkles", size: 18, className: "text-primary" }), e("h2", { className: "omen-section-title" }, "曲线方案")),
                ),
                e("div", { className: "omen-chip-row" },
                  e(Button, { onClick: () => applyTemplate("default") }, "默认"),
                  e(Button, { onClick: () => applyTemplate("quiet") }, "静音"),
                  e(Button, { onClick: () => applyTemplate("cool") }, "强散热"),
                ),
              ),
              e("div", { className: "mt-3 flex flex-col gap-3 rounded-xl border border-border/70 bg-background/45 p-3" },
                e("div", { className: "flex flex-col gap-3 md:flex-row md:items-center md:justify-between" },
                  e("div", { className: "min-w-0" },
                    e("div", { className: "text-sm font-medium text-foreground" }, "自定义方案"),
                  ),
                  e(Button, { onClick: saveAsNewProfile, primary: true, icon: e(Icon, { name: "Plus", size: 15 }) }, "另存为新方案"),
                ),
                curveProfiles.length > 0 ? e("div", { className: "flex flex-col gap-2 md:flex-row md:items-center" },
                  e("div", { className: "min-w-0 flex-1" },
                    e(SelectControl, {
                      value: activeCurveProfile ? activeCurveProfile.id : (curveProfiles[0] ? curveProfiles[0].id : ""),
                      onChange: applyCurveProfile,
                      options: curveProfiles.map((p) => ({ value: p.id, label: `${p.name} · ${formatTemplateTime(p.updatedAt)}` })),
                    }),
                  ),
                  e("div", { className: "flex shrink-0 items-center gap-2" },
                    e(Button, { onClick: overwriteActiveProfile, disabled: !activeCurveProfile, icon: e(Icon, { name: "Save", size: 15 }) }, "覆盖"),
                    e(Button, { onClick: renameActiveProfile, disabled: !activeCurveProfile, icon: e(Icon, { name: "Pencil", size: 15 }) }, "重命名"),
                    e(Button, { onClick: deleteActiveProfile, disabled: !activeCurveProfile || curveProfiles.length <= 1, icon: e(Icon, { name: "Trash2", size: 15 }) }, "删除"),
                  ),
                ) : null,
              ),
            ),
            e("div", { className: "omen-grid curves" },
              e(CurveCard, {
                e, Icon, FanCurveEditor,
                title: "CPU 风扇曲线",
                icon: "Cpu",
                maxTemp: CPU_TEMP_MAX,
                curve: settings.cpuCurve,
                fallbackCurve: CPU_CURVE,
                currentTemp: cpuTemp,
                targetRpm: cpuTarget,
    	            detail: `CPU ${Math.round(cpuTemp)}°C → 草稿预览 ${cpuTarget} RPM · 上限 105°C`,
    	            learnedCurve: showLearnedCurve ? learnedCurveFromConfig(settings.cpuCurve, settings.learningBias, appConfig) : null,
                editable: canEditCurve,
                onCurve: (cpuCurve) => updateSettings({ cpuCurve }),
              }),
              e(CurveCard, {
                e, Icon, FanCurveEditor,
                title: "GPU 风扇曲线",
                icon: "Monitor",
                maxTemp: GPU_TEMP_MAX,
                curve: settings.gpuCurve,
                fallbackCurve: GPU_CURVE,
                currentTemp: gpuTemp,
                targetRpm: gpuTarget,
    	            detail: `GPU ${Math.round(gpuTemp)}°C → 草稿预览 ${gpuTarget} RPM · 上限 90°C`,
    	            learnedCurve: showLearnedCurve ? learnedCurveFromConfig(settings.gpuCurve, settings.learningBias, appConfig) : null,
                editable: canEditCurve,
                onCurve: (gpuCurve) => updateSettings({ gpuCurve }),
              }),
            ),
            e(CardShell, { className: "rounded-2xl border border-border/70 bg-card p-4 shadow-sm", padding: "none", dataThemeCard: "curve-learning" },
              // Header — 电脑风扇学习作为母开关 (右上角 Toggle)。关掉时联合学习也一起关，保证子从属于父的语义。
              e("div", { className: "flex flex-col gap-3 md:flex-row md:items-start md:justify-between" },
                e("div", { className: "flex min-w-0 items-center gap-3" },
                  e("div", { className: "flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground" },
                    e(Icon, { name: "Sparkles", size: 16, className: "text-primary" }),
                  ),
                  e("div", { className: "min-w-0" },
                    e("div", { className: "flex flex-wrap items-center gap-2" },
                      e("div", { className: "text-sm font-medium text-foreground" }, "电脑风扇学习"),
                      e(StatusPill, { tone: learningProfile.fanLearning ? "info" : "muted" }, learningProfile.fanLearning ? "已开启" : "已关闭"),
                    ),
                    e("div", { className: "text-xs leading-relaxed text-muted-foreground" }, `${schemeLabel}独立保存学习结果；图中虚线表示学习后的建议曲线。`),
                  ),
                ),
                e("div", { className: "flex shrink-0 items-center gap-2" },
                  e(Toggle, {
                    checked: learningProfile.fanLearning,
                    onChange: (next) => setLearningState(
                      next ? { fanLearning: true } : { fanLearning: false, jointLearning: false },
                      next ? "电脑风扇学习已开启" : "电脑风扇学习已关闭",
                    ),
                    label: "电脑风扇学习",
                  }),
                ),
              ),
              // 母开关开启后才展开子面板 (含联合学习子开关、偏置、学习结果)
              learningProfile.fanLearning ? e("div", { className: "mt-3 flex flex-col gap-3 rounded-xl border border-border/70 bg-background/45 p-3" },
                // 子开关行 — 联合学习 (扁平样式，无独立卡片 border)
                e("div", { className: "flex flex-col gap-3 md:flex-row md:items-center md:justify-between" },
                  e("div", { className: "flex min-w-0 items-center gap-3" },
                    e("div", { className: "flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground" },
                      e(Icon, { name: "Link", size: 16, className: "text-primary" }),
                    ),
                    e("div", { className: "min-w-0" },
                      e("div", { className: "flex flex-wrap items-center gap-2" },
                        e("div", { className: "text-sm font-medium text-foreground" }, "联合学习"),
                        e(StatusPill, { tone: learningProfile.jointLearning ? "info" : "muted" }, learningProfile.jointLearning ? "已开启" : "已关闭"),
                      ),
                      e("div", { className: "text-xs leading-relaxed text-muted-foreground" },
                        settings.coolerConnected ? "协同散热器与电脑风扇分担噪音。" : "未连接散热器，联合学习将回退到独立方案。",
                      ),
                    ),
                  ),
                  e("div", { className: "flex shrink-0 items-center gap-2" },
                    e(Toggle, {
                      checked: learningProfile.jointLearning,
                      onChange: (next) => setLearningState(
                        { jointLearning: next },
                        next ? "联合学习已开启" : "联合学习已关闭",
                      ),
                      label: "联合学习",
                    }),
                  ),
                ),
                // 联合偏置 slider —— 只有子开关开启才显示
                learningProfile.jointLearning ? e("div", { className: "flex flex-col gap-3 rounded-xl border border-border/70 bg-card/55 p-3", "data-theme-ui": "learning-target-temp" },
                  e("div", { className: "min-w-0" },
                    e("div", { className: "text-xs font-medium text-muted-foreground" }, "联合偏置"),
                    e("div", { className: "mt-1 text-xs leading-relaxed text-muted-foreground" }, `${jointBiasLabel(jointBiasDraftValue)}；拖动只修改草稿，保存后才应用。`),
                  ),
                  e("div", { className: "flex flex-col gap-3 md:flex-row md:items-center" },
                    e("div", { className: "min-w-0 flex-1" },
                      e(SliderControl, { title: "风扇控制占比偏置", value: jointBiasDraftValue, min: -100, max: 100, suffix: "%", onChange: setJointBiasDraft }),
                      e("div", { className: "flex justify-between text-[11px] text-muted-foreground" },
                        e("span", null, "偏散热器"),
                        e("span", null, "均衡"),
                        e("span", null, "偏电脑风扇"),
                      ),
                    ),
                    e("div", { className: "flex shrink-0 items-center gap-2" },
                      e(Button, { onClick: () => setJointBiasDraft(jointBias), disabled: !jointBiasDirty }, "还原"),
                      e(Button, { onClick: saveJointBias, primary: jointBiasDirty, disabled: !jointBiasDirty }, "保存偏置"),
                    ),
                  ),
                ) : null,
                // 学习结果 + 重置按钮 —— 母开关开启时才有意义
                e("div", { className: "flex flex-col gap-3 md:flex-row md:items-start md:justify-between" },
                  e("div", { className: "min-w-0" },
                    e("div", { className: "text-xs font-medium text-muted-foreground" }, "学习结果"),
                    e("div", { className: "mt-1 text-xs leading-relaxed text-muted-foreground" }, learningState.detail),
                  ),
                  e("div", { className: "flex shrink-0 items-center gap-2" },
                    e(Button, {
                      onClick: () => resetLearning("current"),
                      icon: e(Icon, { name: "RotateCcw", size: 15 }),
                    }, "重置当前"),
                    e(Button, {
                      onClick: () => resetLearning("all"),
                    }, "重置全部"),
                  ),
                ),
              ) : null,
            ),
          );
        }

        function CurveCard({ e, Icon, FanCurveEditor, title, icon, detail, curve, fallbackCurve, maxTemp, currentTemp, learnedCurve, editable, onCurve }) {
          return e(CardShell, { className: "glacier-chart-card omen-curve-card", dataThemeCard: "omen-curve-card" },
            e("div", { className: "omen-curve-title" },
              e("div", { className: "omen-inline", style: { alignItems: "flex-start" } },
                e("span", { className: "omen-card-icon" }, e(Icon, { name: icon, size: 18 })),
                e("div", null,
                  e("h3", { className: "omen-card-title" }, title),
                  e("p", { className: "omen-subtle", style: { marginTop: 4 } }, detail),
                ),
              ),
              e("span", { className: `omen-pill ${editable ? "info" : "warning"}` }, editable ? "软件闭环" : "只读预览"),
            ),
            e("div", { className: "omen-legend" },
              e("span", null, e("span", { className: "omen-legend-line" }), "基础曲线"),
              learnedCurve ? e("span", null, e("span", { className: "omen-legend-line dashed" }), "学习后曲线") : null,
              e("span", null, "当前温度"),
            ),
            FanCurveEditor
              ? e("div", { className: "omen-chart-host" }, e(FanCurveEditor, {
                curve,
                fallbackCurve,
                learnedCurve,
                currentTemp,
                minTemp: TEMP_MIN,
                maxTemp,
                tempStep: 5,
                minSpeed: RPM_MIN,
                maxSpeed: RPM_MAX,
                speedStep: RPM_STEP,
                speedTicks: [1000, 2000, 3000, 4000, 5000, 6000],
                speedUnit: " RPM",
                editable,
                onCurveChange: onCurve,
                heightClassName: "omen-chart-height",
                labels: {
                  temperatureAxis: "温度",
                  speedAxis: "速度（RPM）",
                  baseCurve: "基础曲线",
                  learnedCurve: "学习曲线",
                  currentTemperature: "当前 {{temperature}}°C",
                },
              }))
              : e("div", { className: "omen-alert warning" }, "当前主程序未暴露曲线组件，请更新 FanControl 主程序。"),
          );
        }

    	    function SettingsView({ e, Icon, Toggle, SelectControl, SliderControl, settings, updateWithToast, appSampleCount, setSampleCount, patchSmartControl }) {
    	      const responseOptions = SAMPLE_COUNTS.map((value) => ({ value: String(value), label: sampleCountLabel(value) }));
    	      const biasOptions = [
    	        { value: "cooling", label: "偏散热" },
    	        { value: "balanced", label: "均衡" },
    	        { value: "quiet", label: "偏静音" },
    	      ];
    	      const saveLearningBias = (learningBias) => {
    	        const nextBias = normalizeLearningBias(learningBias);
    	        updateWithToast({ learningBias: nextBias }, "学习倾向已保存");
    	        if (typeof patchSmartControl === "function") {
    	          Promise.resolve(patchSmartControl({ learningBias: nextBias })).catch(() => {});
    	        }
    	      };
    	      const saveRisePrediction = (risePrediction) => {
    	        updateWithToast({ risePrediction }, "温升预判设置已保存");
    	        if (typeof patchSmartControl === "function") {
    	          Promise.resolve(patchSmartControl({ temperatureRisePrediction: risePrediction })).catch(() => {});
    	        }
    	      };

          return e(CardShell, {
            className: "rounded-[22px] border border-border/70 bg-card/92 shadow-sm shadow-black/5 backdrop-blur-xl",
            padding: "none",
            dataThemeUi: "setting-section",
          },
            e("div", { className: "flex items-center gap-2.5 border-b border-border/50 px-5 py-4", "data-theme-ui": "setting-section-header" },
              e(Icon, { name: "SlidersHorizontal", size: 18, className: "text-muted-foreground" }),
              e("div", null,
                e("h2", { className: "text-base font-semibold text-foreground" }, "电脑风扇设置"),
                e("div", { className: "text-xs text-muted-foreground" }, settings.coolerConnected ? "散热器已连接方案" : "散热器未连接方案"),
              ),
            ),
            e("div", { className: "divide-y divide-border/45" },
    		        e(SettingRow, {
    		          e, Icon, icon: "BarChart3", title: "温度平滑度", detail: "EMA 指数加权：值越大越平滑（更稳）、越小响应越快（更跟手），不确定保持默认。",
    		          control: e("div", { className: "w-full sm:w-64" },
    		            e(SelectControl, {
    		              value: String(appSampleCount),
    		              onChange: setSampleCount,
    		              options: responseOptions,
    		            }),
    		          ),
    		        }),
    	        e(SettingRow, {
    	          e, Icon, icon: "Target", title: "学习倾向", detail: "偏散热会保守增速，偏静音会在忍受上限内优先降噪。",
    		          control: e("div", { className: "w-full sm:w-64" },
    		            e(SelectControl, {
    		              value: settings.learningBias,
    		              onChange: saveLearningBias,
    		              options: biasOptions,
    		            }),
    		          ),
    	        }),
    		        e(SettingRow, {
    		          e, Icon, icon: "Thermometer", title: "目标温度", detail: "学习机制会围绕目标温度微调建议曲线。",
    		          below: e(SettingSubPanel, { e },
    		            e("div", { className: "omen-pop-grid grid w-full min-w-0 grid-cols-1 gap-3 md:grid-cols-2" },
    		              e(TemperatureMiniCard, { e, label: "CPU", title: "目标温度", value: settings.cpuTarget, min: 55, max: 90, onChange: (cpuTarget) => updateWithToast({ cpuTarget }), SliderControl }),
    		              e(TemperatureMiniCard, { e, label: "GPU", title: "目标温度", value: settings.gpuTarget, min: 50, max: 85, onChange: (gpuTarget) => updateWithToast({ gpuTarget }), SliderControl }),
    		            ),
    		          ),
    		        }),
    		        settings.learningBias === "quiet" ? e(SettingRow, {
    		          e, Icon, icon: "Volume2", title: "偏静音温度忍受上限", detail: "达到上限后会优先恢复散热，不继续为了降噪压低转速。",
    		          below: e(SettingSubPanel, { e },
    		            e("div", { className: "omen-pop-grid grid w-full min-w-0 grid-cols-1 gap-3 md:grid-cols-2" },
    		              e(TemperatureMiniCard, { e, label: "CPU", title: "温度上限", value: settings.quietCpuLimit, min: 60, max: 110, onChange: (quietCpuLimit) => updateWithToast({ quietCpuLimit }), SliderControl }),
    		              e(TemperatureMiniCard, { e, label: "GPU", title: "温度上限", value: settings.quietGpuLimit, min: 55, max: 100, onChange: (quietGpuLimit) => updateWithToast({ quietGpuLimit }), SliderControl }),
    		            ),
    		          ),
    		        }) : null,
    	        e(SettingRow, {
    	          e, Icon, icon: "Radar", title: "温升预判", detail: "提前响应温度上升趋势，减少散热滞后。",
    		          control: e("div", { className: "flex w-full justify-start sm:w-auto sm:justify-end" },
    		            e(Toggle, {
    		              checked: settings.risePrediction,
    		              onChange: saveRisePrediction,
    		            }),
    		          ),
    		        }),
            ),
    	      );
    	    }

        function TemperatureMiniCard({ e, label, title, value, min, max, onChange, SliderControl }) {
          // The wrapper header already shows the label and current value; passing an empty title
          // to SliderControl suppresses the duplicated "目标温度 70°C" line above the slider.
          return e("div", {
            className: "omen-pop-card rounded-xl border border-border/70 bg-background/35 p-3 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-primary/30 hover:bg-card/70 hover:shadow-md",
            "data-theme-card": "curve-history-summary",
          },
            e("div", { className: "mb-2 flex items-center justify-between gap-3" },
              e("div", { className: "min-w-0 text-sm font-medium text-foreground" }, `${label} · ${title}`),
              e("div", { className: "shrink-0 text-sm font-semibold tabular-nums text-primary" }, `${value}°C`),
            ),
            e(SliderControl, {
              title: "",
              value,
              min,
              max,
              suffix: "°C",
              onChange,
            }),
          );
        }

        function SettingSubPanel({ e, children }) {
          return e("div", {
            className: "overflow-hidden rounded-2xl border border-border/60 bg-card/86 p-3 shadow-sm shadow-black/5",
            "data-theme-ui": "compatibility-submenu",
          }, children);
        }

        function SettingRow({ e, Icon, icon, title, detail, control, below }) {
          return e("div", {
            className: "px-5 py-4 transition-colors duration-200 hover:bg-muted/18",
            "data-theme-ui": "setting-row",
          },
            e("div", { className: "flex flex-col gap-4 md:flex-row md:items-center md:justify-between" },
              e("div", { className: "flex min-w-0 flex-1 items-center gap-3" },
                icon ? e("div", { className: "flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-muted/70 text-muted-foreground shadow-inner shadow-white/20", "data-theme-ui": "setting-row-icon" },
                  e(Icon, { name: icon, size: 18 }),
                ) : null,
                e("div", { className: "min-w-0" },
                  e("div", { className: "text-base font-medium text-foreground" }, title),
                  detail ? e("div", { className: "text-sm text-muted-foreground line-clamp-2" }, detail) : null,
                ),
              ),
              control ? e("div", {
                className: "flex w-full min-w-0 justify-start md:ml-auto md:w-auto md:max-w-[36rem] md:shrink-0 md:justify-end",
                "data-theme-ui": "setting-row-control",
              }, control) : null,
            ),
            below ? e("div", { className: `mt-3${icon ? " md:pl-12" : ""}`, "data-theme-ui": "setting-row-below" }, below) : null,
          );
        }

        host.registerPage(PLUGIN_ID, {
          title: "OMEN 笔记本风扇",
          component: OmenFanPage,
        });
  }

  Object.assign(ns, { register });
})();
