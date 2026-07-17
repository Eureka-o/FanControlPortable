(function () {
  'use strict';

  var script = document.currentScript;
  var runtime = window.FanControlPluginHost;
  if (!script || !runtime || runtime.version !== 1) {
    throw new Error('FanControlPluginHost v1 is required');
  }

  var React = runtime.React;
  var h = React.createElement;
  var brandURL = new URL('assets/omen.png', script.src).toString();
  var PAGE_KEYS = ['overview', 'performance', 'curves', 'device'];
  var MODE_KEYS = ['eco', 'balanced', 'performance', 'master'];
  var BOOST_POLICIES = [
    'disabled',
    'enabled',
    'aggressive',
    'efficient-enabled',
    'efficient-aggressive',
    'aggressive-guaranteed',
    'efficient-guaranteed'
  ];

  var english = {
    title: 'HP OMEN', subtitle: 'System control', refresh: 'Refresh', retry: 'Retry', loading: 'Reading OMEN status...',
    active: 'Control active', monitoring: 'Monitoring', unsupported: 'Unsupported hardware', connected: 'Connected',
    reconnecting: 'Reconnecting', suspended: 'Paused', unknown: 'Unknown', readFailed: 'Unable to read OMEN status',
    commandFailed: 'Unable to apply the setting', realtimeOverview: 'Realtime overview', cpuTemp: 'CPU temperature', gpuTemp: 'GPU temperature', cpuPower: 'CPU power',
    gpuPower: 'GPU power', cpuFan: 'CPU fan', gpuFan: 'GPU fan', powerMetric: 'Power', currentMode: 'Current mode', estimated: 'Estimated RPM',
    overview: 'Overview', performance: 'Performance tuning', performanceMode: 'Performance', curves: 'Fan curves', device: 'Device features',
    eco: 'Eco', balanced: 'Balanced', master: 'Master', masterMode: 'Master mode', modeSection: 'Performance mode',
    quickActions: 'Quick controls', cpuTuning: 'CPU tuning', cpuTuningSummary: 'Power and boost policy',
    gpuDynamicBoost: 'GPU Dynamic Boost', screenOverdrive: 'Screen overdrive', chargeProtection: 'Charge protection',
    gpuMode: 'GPU mode', customCurve: 'Custom curves', edit: 'Edit', configure: 'Configure', apply: 'Apply', reset: 'Reset',
    enabled: 'Enabled', disabled: 'Disabled', saved: 'Saved', activeCurve: 'Active', savedNotActive: 'Saved, inactive outside Master mode',
    cpuPowerLimits: 'CPU power limits', cpuStrategy: 'CPU boost and bias', gpuPowerLimits: 'GPU power limits',
    pl1: 'PL1 sustained', pl2: 'PL2 boost', pl4: 'PL4 peak', spl: 'SPL / STAPM', sppt: 'sPPT', fppt: 'fPPT',
    tempLimit: 'Temperature limit', boostPolicy: 'CPU boost policy', powerBias: 'Windows power bias',
    efficiency: 'Efficiency', performanceBias: 'Performance', tgp: 'GPU TGP', ppab: 'PPAB / Dynamic Boost',
    cpuCurve: 'CPU fan curve', gpuCurve: 'GPU fan curve', temperature: 'Temperature', fanSpeed: 'Fan speed',
    responseTime: 'Response', loweringDelay: 'Lowering delay', highTempProtection: 'High-temperature protection',
    seconds: 's', jointLearning: 'Joint learning', jointOff: 'Not enabled', jointOn: 'Enabled', jointPaused: 'Paused',
    deviceControls: 'Device controls', hybrid: 'Hybrid', discrete: 'Discrete', integrated: 'Integrated',
    restartConfirm: 'Changing GPU mode requires a restart. Apply this change?', charge80: '80% protection', charge100: '100% full charge',
    omenKey: 'OMEN key', openFanControl: 'Open FanControl', openOmenHub: 'Open OMEN Gaming Hub', ignore: 'No action',
    diagnostics: 'Device diagnostics', exportDiagnostics: 'Export diagnostics', model: 'Model', bios: 'BIOS', hardwareId: 'Hardware ID',
    noControls: 'No controls are available for this device.',
    boostDisabled: 'Disabled', boostEnabled: 'Enabled', boostAggressive: 'Aggressive', boostEfficientEnabled: 'Efficient Enabled',
    boostEfficientAggressive: 'Efficient Aggressive', boostAggressiveGuaranteed: 'Aggressive at Guaranteed',
    boostEfficientGuaranteed: 'Efficient at Guaranteed'
  };
  var messages = {
    'en-US': english,
    'zh-CN': Object.assign({}, english, {
      subtitle: '设备控制', refresh: '刷新', retry: '重试', loading: '正在读取 OMEN 状态...', active: '控制中', monitoring: '监控中',
      unsupported: '硬件不受支持', connected: '已连接', reconnecting: '正在重连', suspended: '已暂停', unknown: '未知',
      readFailed: '无法读取 OMEN 状态', commandFailed: '设置未能应用', realtimeOverview: '实时概览', cpuTemp: 'CPU 温度', gpuTemp: 'GPU 温度', cpuPower: 'CPU 功耗',
      gpuPower: 'GPU 功耗', cpuFan: 'CPU 风扇', gpuFan: 'GPU 风扇', powerMetric: '功耗', currentMode: '当前模式', estimated: '估算转速',
      overview: '概览', performance: '性能调校', performanceMode: '性能', curves: '风扇曲线', device: '设备功能', eco: '节能', balanced: '平衡',
      master: '大师', masterMode: '大师', modeSection: '性能模式', quickActions: '快捷控制', cpuTuning: 'CPU 调校',
      cpuTuningSummary: '功耗与睿频策略', gpuDynamicBoost: 'GPU Dynamic Boost', screenOverdrive: '屏幕过驱',
      chargeProtection: '充电保护', gpuMode: 'GPU / MUX 模式', customCurve: '自定义曲线', edit: '编辑', configure: '设置',
      apply: '应用', reset: '重置', enabled: '已启用', disabled: '已关闭', saved: '已保存', activeCurve: '生效中',
      savedNotActive: '曲线已保存，仅在大师模式下生效', cpuPowerLimits: 'CPU 功耗限制', cpuStrategy: 'CPU 睿频与性能倾向',
      gpuPowerLimits: 'GPU 功耗限制', pl1: 'PL1 持续功耗', pl2: 'PL2 睿频功耗', pl4: 'PL4 峰值功耗',
      spl: 'SPL / STAPM', sppt: 'sPPT', fppt: 'fPPT', tempLimit: '温度上限', boostPolicy: '睿频策略',
      powerBias: 'Windows 性能倾向', efficiency: '能效', performanceBias: '性能', tgp: 'GPU TGP', ppab: 'PPAB / Dynamic Boost',
      cpuCurve: 'CPU 风扇曲线', gpuCurve: 'GPU 风扇曲线', temperature: '温度', fanSpeed: '转速', responseTime: '响应时间',
      loweringDelay: '降速延迟', highTempProtection: '高温保护', seconds: '秒', jointLearning: '联合学习', jointOff: '未启用',
      jointOn: '已启用', jointPaused: '暂停', deviceControls: '设备控制', hybrid: '混合模式', discrete: '独显模式',
      integrated: '集显模式', restartConfirm: '切换 GPU 模式需要重启系统，确认应用此更改吗？', charge80: '80% 保护',
      charge100: '100% 充满', omenKey: 'OMEN 键', openFanControl: '打开 FanControl', openOmenHub: '打开 OMEN Gaming Hub',
      ignore: '不处理', diagnostics: '设备诊断', exportDiagnostics: '导出诊断', model: '机型', bios: 'BIOS',
      hardwareId: '硬件 ID', noControls: '当前设备没有可用的控制项。', boostDisabled: '禁用', boostEnabled: '启用',
      boostAggressive: '积极', boostEfficientEnabled: '高效启用', boostEfficientAggressive: '高效积极',
      boostAggressiveGuaranteed: '保证频率以上积极', boostEfficientGuaranteed: '保证频率以上高效'
    }),
    'ja-JP': Object.assign({}, english, {
      subtitle: 'デバイス制御', refresh: '更新', retry: '再試行', loading: 'OMEN の状態を読み込んでいます...', active: '制御中',
      monitoring: '監視中', unsupported: '未対応のハードウェア', connected: '接続済み', reconnecting: '再接続中', suspended: '一時停止',
      unknown: '不明', readFailed: 'OMEN の状態を読み込めません', overview: '概要', performance: 'パフォーマンス調整',
      curves: 'ファンカーブ', device: 'デバイス機能', eco: '省電力', balanced: 'バランス', performanceMode: 'パフォーマンス', master: 'マスター',
      masterMode: 'マスターモード', modeSection: 'パフォーマンスモード', quickActions: 'クイック操作', apply: '適用', reset: 'リセット',
      realtimeOverview: 'リアルタイム概要', cpuTemp: 'CPU 温度', gpuTemp: 'GPU 温度', cpuPower: 'CPU 電力', gpuPower: 'GPU 電力', cpuFan: 'CPU ファン', gpuFan: 'GPU ファン', powerMetric: '電力',
      savedNotActive: '保存済み。マスターモード以外では無効', diagnostics: 'デバイス診断', exportDiagnostics: '診断をエクスポート'
    })
  };

  function errorMessage(error) {
    if (error && typeof error.message === 'string') return error.message;
    return String(error || 'Unknown error');
  }

  function numberOrNull(value) {
    var parsed = Number(value);
    return Number.isFinite(parsed) && parsed >= 0 ? parsed : null;
  }

  function objectOr(value, fallback) {
    return value && typeof value === 'object' && !Array.isArray(value) ? Object.assign({}, fallback || {}, value) : (fallback || {});
  }

  function normalizeCurve(value, fallback) {
    var source = objectOr(value, fallback);
    var points = Array.isArray(source.points) ? source.points.map(function (point) {
      return { temperature: numberOrNull(point.temperature) || 0, rpm: numberOrNull(point.rpm) || 0 };
    }) : [];
    return {
      points: points,
      responseTime: numberOrNull(source.responseTime) || 0,
      loweringDelay: numberOrNull(source.loweringDelay) || 0,
      highTemperatureProtection: source.highTemperatureProtection !== false
    };
  }

  function normalizeStatus(value, previous) {
    var source = value && typeof value === 'object' ? value : {};
    var base = previous || {};
    var sourceCurves = objectOr(source.curves, base.curves);
    var baseCurves = base.curves || {};
    return {
      supported: source.supported !== undefined ? source.supported !== false : base.supported !== false,
      controlActive: source.controlActive !== undefined ? source.controlActive === true : base.controlActive === true,
      connectionState: typeof source.connectionState === 'string' ? source.connectionState : (base.connectionState || 'connected'),
      deviceName: typeof source.deviceName === 'string' ? source.deviceName : (base.deviceName || 'HP OMEN'),
      cpuModel: typeof source.cpuModel === 'string' ? source.cpuModel : (base.cpuModel || ''),
      gpuModel: typeof source.gpuModel === 'string' ? source.gpuModel : (base.gpuModel || ''),
      cpuTemp: numberOrNull(source.cpuTemp !== undefined ? source.cpuTemp : base.cpuTemp),
      gpuTemp: numberOrNull(source.gpuTemp !== undefined ? source.gpuTemp : base.gpuTemp),
      cpuPower: numberOrNull(source.cpuPower !== undefined ? source.cpuPower : base.cpuPower),
      gpuPower: numberOrNull(source.gpuPower !== undefined ? source.gpuPower : base.gpuPower),
      cpuRpm: numberOrNull(source.cpuRpm !== undefined ? source.cpuRpm : base.cpuRpm),
      gpuRpm: numberOrNull(source.gpuRpm !== undefined ? source.gpuRpm : base.gpuRpm),
      minFanRpm: numberOrNull(source.minFanRpm !== undefined ? source.minFanRpm : base.minFanRpm),
      maxFanRpm: numberOrNull(source.maxFanRpm !== undefined ? source.maxFanRpm : base.maxFanRpm),
      mode: typeof source.mode === 'string' ? source.mode : (base.mode || 'balanced'),
      processorFamily: typeof source.processorFamily === 'string' ? source.processorFamily : (base.processorFamily || 'intel'),
      capabilities: objectOr(source.capabilities, base.capabilities),
      power: objectOr(source.power, base.power),
      features: objectOr(source.features, base.features),
      curves: {
        cpu: normalizeCurve(sourceCurves.cpu, baseCurves.cpu),
        gpu: normalizeCurve(sourceCurves.gpu, baseCurves.gpu)
      },
      jointLearning: objectOr(source.jointLearning, base.jointLearning),
      diagnostics: objectOr(source.diagnostics, base.diagnostics),
      availableGpuModes: Array.isArray(source.availableGpuModes) ? source.availableGpuModes.slice() : (base.availableGpuModes || []),
      availableOmenKeyActions: Array.isArray(source.availableOmenKeyActions) ? source.availableOmenKeyActions.slice() : (base.availableOmenKeyActions || []),
      rpmEstimated: source.rpmEstimated !== undefined ? source.rpmEstimated === true : base.rpmEstimated === true,
      lastUpdated: numberOrNull(source.lastUpdated) || Date.now()
    };
  }

  function cloneCurves(curves) {
    return {
      cpu: normalizeCurve(curves && curves.cpu),
      gpu: normalizeCurve(curves && curves.gpu)
    };
  }

  function OmenPage(props) {
    var plugin = props.host;
    var ui = plugin.ui;
    var icons = plugin.icons;
    var localeState = React.useState(plugin.locale.current());
    var locale = localeState[0];
    var setLocale = localeState[1];
    var pageState = React.useState('overview');
    var activePage = pageState[0];
    var setActivePage = pageState[1];
    var statusState = React.useState(null);
    var status = statusState[0];
    var setStatus = statusState[1];
    var loadingState = React.useState(true);
    var loading = loadingState[0];
    var setLoading = loadingState[1];
    var errorState = React.useState('');
    var error = errorState[0];
    var setError = errorState[1];
    var pendingState = React.useState('');
    var pending = pendingState[0];
    var setPending = pendingState[1];
    var powerDraftState = React.useState(null);
    var powerDraft = powerDraftState[0];
    var setPowerDraft = powerDraftState[1];
    var curveDraftState = React.useState(null);
    var curveDraft = curveDraftState[0];
    var setCurveDraft = curveDraftState[1];
    var text = messages[locale] || english;

    function curveDraftFromStatus(nextStatus) {
      var next = cloneCurves(nextStatus.curves);
      var options = {
        minTemperature: 40,
        maxTemperature: 100,
        temperatureStep: 5,
        minSpeed: nextStatus.minFanRpm || 0,
        maxSpeed: nextStatus.maxFanRpm || 7000,
        speedStep: 100
      };
      next.cpu.points = ui.resampleFanCurve(next.cpu.points, options);
      next.gpu.points = ui.resampleFanCurve(next.gpu.points, options);
      return next;
    }

    function acceptReadback(next) {
      var normalized = normalizeStatus(next, status);
      setStatus(normalized);
      setPowerDraft(Object.assign({}, normalized.power));
      setCurveDraft(curveDraftFromStatus(normalized));
      return normalized;
    }

    var refresh = React.useCallback(function () {
      setLoading(true);
      setError('');
      return plugin.invoke('get-status').then(function (next) {
        var normalized = normalizeStatus(next, null);
        setStatus(normalized);
        setPowerDraft(Object.assign({}, normalized.power));
        setCurveDraft(curveDraftFromStatus(normalized));
      }).catch(function (reason) {
        setError(errorMessage(reason));
      }).finally(function () {
        setLoading(false);
      });
    }, [plugin]);

    React.useEffect(function () {
      return plugin.locale.subscribe(setLocale);
    }, [plugin]);

    React.useEffect(function () {
      void refresh();
      return plugin.subscribe('status-changed', function (next) {
        setStatus(function (previous) { return normalizeStatus(next, previous); });
        setError('');
        setLoading(false);
      });
    }, [plugin, refresh]);

    function runCommand(method, payload) {
      setPending(method);
      setError('');
      return plugin.invoke(method, payload).then(function (next) {
        acceptReadback(next);
      }).catch(function (reason) {
        var message = errorMessage(reason);
        setError(message);
        plugin.toast.error(text.commandFailed, message);
      }).finally(function () {
        setPending('');
      });
    }

    function displayValue(value, suffix) {
      return value === null || value === undefined ? text.unknown : Math.round(value) + suffix;
    }

    function cardHeading(title, Icon, badge) {
      return h('div', { className: 'omen-card-heading' },
        h('span', { className: 'omen-card-icon', 'aria-hidden': 'true' }, h(Icon, { size: 17 })),
        h('h2', null, title),
        badge || null
      );
    }

    function modeButtons() {
      return h('div', { className: 'omen-mode-grid', role: 'group', 'aria-label': text.modeSection },
        MODE_KEYS.map(function (mode) {
          var selected = status && status.mode === mode;
          return h(ui.Button, {
            key: mode,
            type: 'button',
            size: 'sm',
            variant: selected ? 'primary' : 'outline',
            disabled: Boolean(pending),
            'aria-pressed': selected,
            onClick: function () { void runCommand('set-thermal-mode', { mode: mode }); }
          }, mode === 'master' ? text.masterMode : mode === 'performance' ? text.performanceMode : text[mode]);
        })
      );
    }

    function settingRow(label, control, value) {
      return h('div', { className: 'omen-setting-row' },
        h('div', { className: 'omen-setting-copy' }, h('span', null, label), value && h('strong', null, value)),
        h('div', { className: 'omen-setting-control' }, control)
      );
    }

    function quickCard(title, Icon, content, action) {
      return h(ui.Card, { key: title, className: 'omen-quick-card', padding: 'md' },
        cardHeading(title, Icon),
        h('div', { className: 'omen-quick-content' }, content),
        action || null
      );
    }

    function setPowerField(field, value) {
      setPowerDraft(Object.assign({}, powerDraft || {}, (function () { var item = {}; item[field] = value; return item; })()));
    }

    function setCurveField(target, field, value) {
      var next = cloneCurves(curveDraft || {});
      next[target][field] = value;
      setCurveDraft(next);
    }

    function setCurvePoints(target, points) {
      var next = cloneCurves(curveDraft || {});
      next[target].points = points.map(function (point) {
        return { temperature: point.temperature, rpm: point.rpm };
      });
      setCurveDraft(next);
    }

    function overviewPage(capabilities, features, power, isMasterMode) {
      var quick = [];
      if (capabilities.gpuPower) {
        quick.push(quickCard(text.gpuDynamicBoost, icons.zap,
          h('strong', { className: 'omen-quick-value' }, power.dynamicBoost ? text.enabled : text.disabled),
          h(ui.ToggleSwitch, {
            enabled: power.dynamicBoost === true,
            loading: pending === 'set-gpu-power',
            disabled: Boolean(pending),
            srLabel: text.gpuDynamicBoost,
            onChange: function (enabled) { void runCommand('set-gpu-power', Object.assign({}, power, { dynamicBoost: enabled })); }
          })
        ));
      }
      if (capabilities.screenOverdrive) {
        quick.push(quickCard(text.screenOverdrive, icons.laptop,
          h('strong', { className: 'omen-quick-value' }, features.screenOverdrive ? text.enabled : text.disabled),
          h(ui.ToggleSwitch, {
            enabled: features.screenOverdrive === true,
            loading: pending === 'set-screen-overdrive',
            disabled: Boolean(pending),
            srLabel: text.screenOverdrive,
            onChange: function (enabled) { void runCommand('set-screen-overdrive', { enabled: enabled }); }
          })
        ));
      }
      if (capabilities.chargeProtection) {
        quick.push(quickCard(text.chargeProtection, icons.plug,
          h('strong', { className: 'omen-quick-value' }, features.chargeLimit === 80 ? text.charge80 : text.charge100),
          h('div', { className: 'omen-inline-buttons', role: 'group', 'aria-label': text.chargeProtection },
            [80, 100].map(function (limit) {
              return h(ui.Button, {
                key: limit,
                type: 'button',
                size: 'sm',
                variant: features.chargeLimit === limit ? 'primary' : 'outline',
                disabled: Boolean(pending),
                onClick: function () { void runCommand('set-charge-limit', { limit: limit }); }
              }, limit + '%');
            })
          )
        ));
      }
      if (capabilities.gpuMode) {
        var gpuModes = status.availableGpuModes.length > 0 ? status.availableGpuModes : ['hybrid', 'discrete'];
        quick.push(quickCard(text.gpuMode, icons.laptop,
          h('strong', { className: 'omen-quick-value' }, text[features.gpuMode] || features.gpuMode || text.unknown),
          h(ui.Select, {
            className: 'omen-quick-select', value: features.gpuMode || gpuModes[0], disabled: Boolean(pending), size: 'sm',
            options: gpuModes.map(function (mode) { return { value: mode, label: text[mode] || mode }; }),
            onChange: function (mode) {
              if (mode !== features.gpuMode && window.confirm(text.restartConfirm)) void runCommand('set-gpu-mode', { mode: mode });
            }
          })
        ));
      }

      return h('div', { className: 'omen-view', 'data-page': 'overview' },
        capabilities.thermalMode && h(ui.Card, { className: 'omen-mode-card', padding: 'md' },
          cardHeading(text.modeSection, icons.zap, h(ui.Badge, { variant: isMasterMode ? 'warning' : 'info' }, isMasterMode ? text.masterMode : status.mode === 'performance' ? text.performanceMode : text[status.mode])),
          modeButtons()
        ),
        quick.length > 0 && h(React.Fragment, null,
          h('div', { className: 'omen-section-heading' }, h('h2', null, text.quickActions)),
          h('div', { className: 'omen-quick-grid' }, quick)
        )
      );
    }

    function performancePage(capabilities, power) {
      var draft = powerDraft || power;
      var cards = [];

      function tuningControl(field, label, min, max, suffix) {
        var value = Number(draft[field] || 0);
        return h('div', { key: field, className: 'omen-power-control' },
          h(ui.Slider, {
            label: label, value: value, min: min, max: max, step: 1, showValue: false, disabled: Boolean(pending),
            onChange: function (next) { setPowerField(field, next); }
          }),
          h(ui.NumberInput, {
            className: 'omen-power-number', value: value, min: min, max: max, step: 1, suffix: suffix, disabled: Boolean(pending),
            onChange: function (next) { setPowerField(field, next); }
          })
        );
      }

      if (capabilities.cpuPowerLimits) {
        var amd = status.processorFamily.toLowerCase() === 'amd';
        var fields = amd
          ? [['spl', text.spl], ['sppt', text.sppt], ['fppt', text.fppt]]
          : [['pl1', text.pl1], ['pl2', text.pl2]].concat(capabilities.cpuPl4 ? [['pl4', text.pl4]] : []);
        if (capabilities.cpuTempLimit) fields.push(['tempLimit', text.tempLimit]);
        cards.push(h(ui.Card, { key: 'cpu-power', className: 'omen-tuning-card', padding: 'md' },
          cardHeading(text.cpuPowerLimits, icons.gauge),
          h('div', { className: 'omen-power-controls' }, fields.map(function (field) {
            var temperature = field[0] === 'tempLimit';
            var min = temperature ? (amd ? 75 : 0) : (amd ? 5 : 0);
            var max = temperature ? (amd ? 96 : 110) : (amd ? 150 : 250);
            return tuningControl(field[0], field[1], min, max, temperature ? '°C' : 'W');
          })),
          h('div', { className: 'omen-card-actions' },
            h(ui.Button, { type: 'button', size: 'sm', loading: pending === 'set-cpu-power', disabled: Boolean(pending),
              onClick: function () {
                var payload = {};
                fields.forEach(function (field) { payload[field[0]] = draft[field[0]]; });
                void runCommand('set-cpu-power', payload);
              } }, text.apply)
          )
        ));
      }
      if (capabilities.cpuBoostPolicy || capabilities.powerBias) {
        cards.push(h(ui.Card, { key: 'cpu-strategy', className: 'omen-tuning-card', padding: 'md' },
          cardHeading(text.cpuStrategy, icons.zap),
          capabilities.cpuBoostPolicy && settingRow(text.boostPolicy,
            h(ui.Select, {
              value: power.boostPolicy || 'enabled', disabled: Boolean(pending), size: 'sm',
              options: BOOST_POLICIES.map(function (policy) {
                var key = 'boost' + policy.split('-').map(function (part) { return part.charAt(0).toUpperCase() + part.slice(1); }).join('');
                return { value: policy, label: text[key] || policy };
              }),
              onChange: function (policy) { void runCommand('set-cpu-boost-policy', { policy: policy }); }
            }), text['boost' + String(power.boostPolicy || '').split('-').map(function (part) { return part.charAt(0).toUpperCase() + part.slice(1); }).join('')] || power.boostPolicy),
          capabilities.powerBias && settingRow(text.powerBias,
            h(ui.Select, {
              value: power.powerBias || 'balanced', disabled: Boolean(pending), size: 'sm',
              options: [
                { value: 'efficiency', label: text.efficiency },
                { value: 'balanced', label: text.balanced },
                { value: 'performance', label: text.performanceBias }
              ],
              onChange: function (bias) { void runCommand('set-power-bias', { bias: bias }); }
            }), text[power.powerBias] || power.powerBias)
        ));
      }
      if (capabilities.gpuPower) {
        cards.push(h(ui.Card, { key: 'gpu-power', className: 'omen-tuning-card omen-tuning-card-wide', padding: 'md' },
          cardHeading(text.gpuPowerLimits, icons.laptop),
          h('div', { className: 'omen-power-controls' },
            tuningControl('tgp', text.tgp, 0, 250, 'W'),
            tuningControl('ppab', text.ppab, 0, 100, 'W')
          ),
          settingRow(text.gpuDynamicBoost,
            h(ui.ToggleSwitch, { enabled: draft.dynamicBoost === true, disabled: Boolean(pending), srLabel: text.gpuDynamicBoost,
              onChange: function (enabled) { setPowerField('dynamicBoost', enabled); } }), draft.dynamicBoost ? text.enabled : text.disabled),
          h('div', { className: 'omen-card-actions' },
            h(ui.Button, { type: 'button', size: 'sm', loading: pending === 'set-gpu-power', disabled: Boolean(pending),
              onClick: function () { void runCommand('set-gpu-power', { tgp: draft.tgp, ppab: draft.ppab, dynamicBoost: draft.dynamicBoost }); } }, text.apply)
          )
        ));
      }
      return h('div', { className: 'omen-view', 'data-page': 'performance' },
        cards.length > 0 ? h('div', { className: 'omen-tuning-grid' }, cards) : h('div', { className: 'omen-empty' }, text.noControls)
      );
    }

    function curveEditor(target, isMasterMode, capabilities) {
      var curve = curveDraft && curveDraft[target] || { points: [] };
      var title = target === 'cpu' ? text.cpuCurve : text.gpuCurve;
      var minFanRpm = status.minFanRpm || 0;
      var maxFanRpm = status.maxFanRpm || 7000;
      var currentTemperature = target === 'cpu' ? status.cpuTemp : status.gpuTemp;
      return h(ui.Card, { className: 'omen-curve-card', padding: 'md' },
        cardHeading(title, icons.fan, h(ui.Badge, { variant: isMasterMode ? 'success' : 'warning' }, isMasterMode ? text.activeCurve : text.savedNotActive)),
        h(ui.FanCurveEditor, {
          points: curve.points,
          onChange: function (points) { setCurvePoints(target, points); },
          minTemperature: 40,
          maxTemperature: 100,
          minSpeed: minFanRpm,
          maxSpeed: maxFanRpm,
          speedStep: 100,
          speedUnit: ' RPM',
          temperatureAxisLabel: text.temperature + ' (°C)',
          speedAxisLabel: text.fanSpeed + ' (RPM)',
          temperatureLabel: function (value) { return text.temperature + ': ' + value + '°C'; },
          baseCurveLabel: title,
          currentTemperature: target === 'cpu' ? status.cpuTemp : status.gpuTemp,
          currentTemperatureLabel: currentTemperature === null ? undefined : text.temperature + ': ' + Math.round(currentTemperature) + '°C',
          pointAriaLabel: function (point) { return point.temperature + '°C, ' + point.rpm + ' RPM'; },
          disabled: Boolean(pending),
          compact: true
        }),
        capabilities.curveResponse && h('div', { className: 'omen-curve-options' },
          h(ui.NumberInput, { label: text.responseTime, value: curve.responseTime, min: 0, max: 60, suffix: text.seconds, disabled: Boolean(pending),
            onChange: function (value) { setCurveField(target, 'responseTime', value); } }),
          h(ui.NumberInput, { label: text.loweringDelay, value: curve.loweringDelay, min: 0, max: 300, suffix: text.seconds, disabled: Boolean(pending),
            onChange: function (value) { setCurveField(target, 'loweringDelay', value); } }),
          settingRow(text.highTempProtection,
            h(ui.ToggleSwitch, { enabled: curve.highTemperatureProtection, disabled: Boolean(pending), srLabel: text.highTempProtection,
              onChange: function (enabled) { setCurveField(target, 'highTemperatureProtection', enabled); } }),
            curve.highTemperatureProtection ? text.enabled : text.disabled)
        ),
        h('div', { className: 'omen-card-actions omen-curve-actions' },
          h(ui.Button, { type: 'button', size: 'sm', variant: 'outline', disabled: Boolean(pending),
            onClick: function () { void runCommand('reset-fan-curve', { target: target }); } }, text.reset),
          h(ui.Button, { type: 'button', size: 'sm', loading: pending === 'set-fan-curve', disabled: Boolean(pending),
            onClick: function () { void runCommand('set-fan-curve', { target: target, curve: curve }); } }, text.apply)
        )
      );
    }

    function curvesPage(capabilities, isMasterMode) {
      var joint = status.jointLearning || {};
      return h('div', { className: 'omen-view', 'data-page': 'curves' },
        capabilities.fanCurves
          ? h('div', { className: 'omen-curve-stack' }, curveEditor('cpu', isMasterMode, capabilities), curveEditor('gpu', isMasterMode, capabilities))
          : h('div', { className: 'omen-empty' }, text.noControls),
        capabilities.jointLearning && h(ui.Card, { className: 'omen-joint-learning', padding: 'md' },
          cardHeading(text.jointLearning, icons.gauge,
            h(ui.Badge, { variant: joint.paused ? 'warning' : joint.enabled ? 'success' : 'default' }, joint.paused ? text.jointPaused : joint.enabled ? text.jointOn : text.jointOff)),
          h(ui.ToggleSwitch, {
            enabled: joint.enabled === true,
            loading: pending === 'set-joint-learning',
            disabled: Boolean(pending),
            srLabel: text.jointLearning,
            onChange: function (enabled) { void runCommand('set-joint-learning', { enabled: enabled }); }
          })
        )
      );
    }

    function devicePage(capabilities, features) {
      var cards = [];
      if (capabilities.omenKey) {
        var actions = status.availableOmenKeyActions.length > 0 ? status.availableOmenKeyActions : ['fancontrol', 'omen-hub', 'ignore'];
        cards.push(quickCard(text.omenKey, icons.settings,
          h(ui.Select, {
            value: features.omenKeyAction || actions[0], disabled: Boolean(pending), size: 'sm',
            options: actions.map(function (action) {
              var label = action === 'fancontrol' ? text.openFanControl : action === 'omen-hub' ? text.openOmenHub : text.ignore;
              return { value: action, label: label };
            }),
            onChange: function (action) { void runCommand('set-omen-key', { action: action }); }
          })
        ));
      }
      return h('div', { className: 'omen-view', 'data-page': 'device' },
        cards.length > 0 && h('div', { className: 'omen-device-grid' }, cards),
        capabilities.diagnostics && h(ui.Card, { className: 'omen-diagnostics-row', padding: 'md' },
          cardHeading(text.diagnostics, icons.settings),
          h('div', { className: 'omen-diagnostics-data' },
            h('span', null, text.model, h('strong', null, status.diagnostics.model || status.deviceName)),
            h('span', null, text.bios, h('strong', null, status.diagnostics.bios || text.unknown)),
            h('span', null, text.hardwareId, h('strong', null, status.diagnostics.hardwareId || text.unknown))
          ),
          h(ui.Button, { type: 'button', size: 'sm', variant: 'outline', loading: pending === 'export-diagnostics', disabled: Boolean(pending),
            onClick: function () { void runCommand('export-diagnostics'); } }, text.exportDiagnostics)
        ),
        cards.length === 0 && !capabilities.diagnostics && h('div', { className: 'omen-empty' }, text.noControls)
      );
    }

    if (!status && loading) {
      return h('div', { 'aria-live': 'polite', className: 'omen-loading' }, h(icons.gauge, { size: 18 }), h('span', null, text.loading));
    }

    var currentStatus = status || normalizeStatus({}, null);
    var capabilities = currentStatus.capabilities || {};
    var power = currentStatus.power || {};
    var features = currentStatus.features || {};
    var isMasterMode = currentStatus.mode === 'master';
    var connectionKey = currentStatus.supported === false ? 'unsupported' : currentStatus.connectionState;
    var connectionLabel = text[connectionKey] || (currentStatus.controlActive ? text.active : text.monitoring);
    var joint = currentStatus.jointLearning || {};
    var jointLabel = joint.paused ? text.jointPaused : joint.enabled ? text.jointOn : text.jointOff;
    var currentModeLabel = isMasterMode ? text.masterMode : currentStatus.mode === 'performance' ? text.performanceMode : text[currentStatus.mode] || currentStatus.mode;

    return h('section', { className: 'omen-page' },
      h(ui.RealtimeOverview, {
        title: text.realtimeOverview,
        titleIcon: h(icons.settings, { size: 17, 'aria-hidden': 'true' }),
        hardware: [
          {
            id: 'cpu',
            label: 'CPU',
            model: currentStatus.cpuModel,
            icon: h(icons.gauge, { size: 18, 'aria-hidden': 'true' }),
            metrics: [
              { id: 'temperature', icon: h(icons.thermometer, { size: 14 }), label: text.temperature, value: displayValue(currentStatus.cpuTemp, '°C') },
              { id: 'power', icon: h(icons.zap, { size: 14 }), label: text.powerMetric, value: displayValue(currentStatus.cpuPower, ' W') },
              { id: 'fan', icon: h(icons.fan, { size: 14 }), label: text.fanSpeed, value: displayValue(currentStatus.cpuRpm, ' RPM') }
            ]
          },
          {
            id: 'gpu',
            label: 'GPU',
            model: currentStatus.gpuModel,
            icon: h(icons.laptop, { size: 18, 'aria-hidden': 'true' }),
            metrics: [
              { id: 'temperature', icon: h(icons.thermometer, { size: 14 }), label: text.temperature, value: displayValue(currentStatus.gpuTemp, '°C') },
              { id: 'power', icon: h(icons.zap, { size: 14 }), label: text.powerMetric, value: displayValue(currentStatus.gpuPower, ' W') },
              { id: 'fan', icon: h(icons.fan, { size: 14 }), label: text.fanSpeed, value: displayValue(currentStatus.gpuRpm, ' RPM') }
            ]
          }
        ],
        device: {
          icon: h('img', { src: brandURL, alt: 'OMEN by HP', className: 'omen-overview-brand', draggable: false }),
          name: currentStatus.deviceName,
          connected: currentStatus.supported !== false && connectionKey === 'connected',
          connectionLabel: connectionLabel,
          details: [
            { id: 'mode', label: text.currentMode, value: currentModeLabel },
            { id: 'joint-learning', label: text.jointLearning, value: jointLabel }
          ]
        }
      }),
      error && h('div', { role: 'alert', className: 'omen-alert' },
        h('div', null, h('strong', null, text.readFailed), h('p', null, error)),
        h(ui.Button, { type: 'button', size: 'sm', variant: 'outline', loading: loading, onClick: refresh }, text.retry)
      ),
      h('nav', { className: 'omen-tabs', 'aria-label': text.title }, PAGE_KEYS.map(function (page) {
        var Icon = page === 'overview' ? icons.gauge : page === 'performance' ? icons.zap : page === 'curves' ? icons.fan : icons.settings;
        return h('button', { key: page, type: 'button', className: 'omen-tab', 'data-active': activePage === page ? 'true' : 'false',
          'aria-current': activePage === page ? 'page' : undefined, onClick: function () { setActivePage(page); } },
          h(Icon, { size: 16, 'aria-hidden': 'true' }), h('span', null, text[page])
        );
      })),
      activePage === 'overview' && overviewPage(capabilities, features, power, isMasterMode),
      activePage === 'performance' && performancePage(capabilities, power),
      activePage === 'curves' && curvesPage(capabilities, isMasterMode),
      activePage === 'device' && devicePage(capabilities, features)
    );
  }

  runtime.registerPage({ id: 'control', component: OmenPage });
})();
