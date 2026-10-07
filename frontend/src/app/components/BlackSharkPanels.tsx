// 黑鲨 BRB02 专属面板：设备设置 / 情景 / 屏幕。
// 三块都只对黑鲨有意义，因此合在一个模块里，避免散落在通用组件目录中。
'use client';

import clsx from 'clsx';
import { useCallback, useEffect, useRef, useState } from 'react';
import {
  Activity,
  Archive,
  ChevronDown,
  Cpu,
  Fan,
  Lightbulb,
  Loader2,
  MonitorSmartphone,
  Power,
  RefreshCw,
  RotateCcw,
  Trash2,
  ImageIcon,
  Upload,
  X,
} from 'lucide-react';
import { AnimatePresence, motion } from 'framer-motion';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Input } from '@/components/ui/input';
import { apiService } from '../services/api';
import { useAppStore } from '../store/app-store';
// 复用设置页既有实现（Section / SettingRow），卡片与设置页同形，不手搓样式。
import { Section, SettingRow } from './settings/SettingLayout';
import type {
  BlackSharkInfo,
  BlackSharkRgbLighting,
  BlackSharkRgbMode,
  BlackSharkHostEffects,
  BlackSharkLcdDisplayPayload,
  SceneRulePayload,
  SceneRulesPayload,
  ScreenImageInfoPayload,
  ScreenCropPreviewPayload,
  ScreenPresetPayload,
  ScreenHistoryItemPayload,
  ScreenHistoryListPayload,
} from '../types/app';
import {
  Button,
  ConfirmButton,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  Slider,
  ToggleSwitch,
  Select,
} from './ui/index';

// 情景槽位固定 4 个，与官方 t_Brb02SceneConfig 及后端 types.SceneSlotCount 一致；改动需同步后端。
const SCENE_SLOTS = 4;

/** 色相条轨道渐变：0..360 色相环直接画在滑杆上。 */
export const HUE_TRACK_GRADIENT =
  'linear-gradient(90deg, hsl(0 100% 50%), hsl(60 100% 50%), hsl(120 100% 50%), hsl(180 100% 50%), hsl(240 100% 50%), hsl(300 100% 50%), hsl(360 100% 50%))';

/** 黑鲨（BlackShark）BRB02 散热器的专属控制区。 */
export function BlackSharkPanel() {
  const { t } = useTranslation();
  // 数据加载盯 isConnected 上升沿查一次，同一次连接内不重复，而非每次进页面。
  const isConnected = useAppStore((state) => state.isConnected);
  const [info, setInfo] = useState<BlackSharkInfo | null>(null);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      setInfo(await apiService.getBlackSharkInfo());
    } catch (error) {
      toast.error(t('controlPanel.blackShark.refreshFailed', { error: String(error) }));
    } finally {
      setLoading(false);
    }
  }, [t]);

  // firmware 取自 CachedBlackSharkFirmwareStatus()（上次检查的缓存），需显式触发检查才会填充。
  const checkFirmware = useCallback(async () => {
    setLoading(true);
    try {
      const status = await apiService.checkBlackSharkFirmwareUpdate();
      await refresh();
      if (status.error) {
        toast.error(t('controlPanel.blackShark.firmwareCheckFailed', { error: status.error }));
      } else if (status.updateAvailable) {
        toast.success(t('controlPanel.blackShark.firmwareUpdateAvailable', {
          current: status.currentVersion ?? '?',
          latest: status.latestVersion ?? '?',
        }));
      } else {
        toast.success(t('controlPanel.blackShark.firmwareCheckUpToDate', {
          version: status.currentVersion ?? '?',
        }));
      }
    } catch (error) {
      toast.error(t('controlPanel.blackShark.firmwareCheckFailed', { error: String(error) }));
    } finally {
      setLoading(false);
    }
  }, [refresh, t]);

  // 用本机缓存填充灯效页（零设备 IO）；依赖数组留空，回调只调用 setState。
  const loadRgbFromCache = useCallback(async () => {
    try {
      const cached = await apiService.getBlackSharkRgbLightingCached();
      if (!cached?.modes?.length) {
        return; // 没有缓存就别动界面（保留「点读取灯效加载」的提示）
      }
      setRgb(cached);
      const current = cached.modes.find((m) => m.current) ?? cached.modes.find((m) => m.known);
      setDraft(
        current?.known ? { index: current.index, speed: current.speed, brightness: current.brightness } : null,
      );
      if (current?.known) setStaticColor(!!current.staticColor);
    } catch {
      // 缓存读不到当没有即可：可选优化，失败不影响功能。
    }
  }, []);

  // 连接成功后查询一次，同一次连接内不重复。
  const deviceQueriedRef = useRef(false);
  useEffect(() => {
    if (!isConnected) {
      deviceQueriedRef.current = false;
      return;
    }
    if (deviceQueriedRef.current) return;
    deviceQueriedRef.current = true;
    void refresh();
    void loadRgbFromCache();
  }, [isConnected, refresh, loadRgbFromCache]);

  const run = useCallback(
    async (action: () => Promise<boolean>, successKey: string) => {
      setBusy(true);
      try {
        const ok = await action();
        if (ok) {
          toast.success(t(successKey));
        } else {
          toast.error(t('controlPanel.blackShark.actionFailed'));
        }
        await refresh();
      } catch (error) {
        toast.error(t('controlPanel.blackShark.actionFailed') + ' ' + String(error));
      } finally {
        setBusy(false);
      }
    },
    [refresh, t],
  );

  // runQuiet：提示由动作自己给出，这里只管 busy 与异常兜底，避免 toast 重复。
  const runQuiet = useCallback(
    async (action: () => Promise<void>) => {
      setBusy(true);
      try {
        await action();
        await refresh();
      } catch (error) {
        toast.error(t('controlPanel.blackShark.actionFailed') + ' ' + String(error));
      } finally {
        setBusy(false);
      }
    },
    [refresh, t],
  );

  const switches = info?.switches;
  const gears = info?.gears ?? [];

  // 灯效模式单独一块、按需加载（1+1+8 次设备查询），不随主信息一起拉。
  const [rgb, setRgb] = useState<BlackSharkRgbLighting | null>(null);
  const [rgbLoading, setRgbLoading] = useState(false);
  // 灯效模式折叠卡片默认收起。
  const [rgbOpen, setRgbOpen] = useState(false);
  const [draft, setDraft] = useState<{ index: number; speed: number; brightness: number } | null>(null);
  // 官方 UI 是一根 0..360 的色相滑块（QColor::setHsv(h,255,255)）。
  // hueDraft 只存将要设置的色相，不从设备颜色反推（那需要另做一份 RGB→色相）。
  const [hueDraft, setHueDraft] = useState(0);
  const [staticColor, setStaticColor] = useState(true);

  const loadRgb = useCallback(async () => {
    setRgbLoading(true);
    try {
      const next = await apiService.getBlackSharkRgbLighting();
      setRgb(next);
      const current = next.modes?.find((m) => m.current) ?? next.modes?.find((m) => m.known);
      setDraft(current?.known ? { index: current.index, speed: current.speed, brightness: current.brightness } : null);
      // staticColor 必须取设备回读值：用本地默认值会把彩色模式写成单色。
      if (current?.known) setStaticColor(!!current.staticColor);
    } catch (error) {
      toast.error(t('controlPanel.blackShark.actionFailed') + ' ' + String(error));
    } finally {
      setRgbLoading(false);
    }
  }, [t]);

  const modeLabel = useCallback(
    (mode: BlackSharkRgbMode) => mode.name || t('controlPanel.blackShark.rgbModeN', { n: mode.index }),
    [t],
  );

  // 主机侧灯效状态：槽位 6「响应」/ 7「音频同步」需要主机持续喂数据才会动。
  const [hostFx, setHostFx] = useState<BlackSharkHostEffects | null>(null);

  const loadHostFx = useCallback(async () => {
    try {
      setHostFx(await apiService.getBlackSharkHostEffects());
    } catch {
      // 拿不到状态就不显示状态行 —— 不要用一个错误弹窗打断"看一眼灯效"这件事。
      setHostFx(null);
    }
  }, []);

  useEffect(() => {
    // 订阅核心推送的灯效事件，不做轮询。
    if (!isConnected || !rgbOpen) {
      setHostFx(null);
      return;
    }
    // 订阅前先拉一次：推送有 1 秒节流，否则展开瞬间像没反应。
    void loadHostFx();
    const unsubscribe = apiService.onBlackSharkHostEffects((next) => setHostFx(next));
    return () => unsubscribe();
  }, [isConnected, loadHostFx, rgbOpen]);

  // 当前选中/生效的灯效：draft 优先，否则取设备上报的 current。
  const selectedMode = (rgb?.modes ?? []).find(
    (m) => m.index === (draft?.index ?? rgb?.modes?.find((x) => x.current)?.index),
  );

  // 主机侧状态行：音频同步(7)显示主频/电平/帧数，响应(6)显示帧数，失败原样给出原因。
  const hostFxLine = (() => {
    if (!selectedMode || !hostFx) return null;
    if (selectedMode.index === 7) {
      if (hostFx.audioError) return t('controlPanel.blackShark.hostFxAudioFailed', { error: hostFx.audioError });
      if (!hostFx.audioSyncRunning) return t('controlPanel.blackShark.hostFxInactive', { slot: hostFx.currentSlot });
      if (hostFx.audioLevel === 0) {
        return t('controlPanel.blackShark.hostFxAudioSilent', { count: hostFx.audioPushCount });
      }
      return t('controlPanel.blackShark.hostFxAudioRunning', {
        hz: Math.round(hostFx.audioFrequencyHz),
        level: hostFx.audioLevel,
        count: hostFx.audioPushCount,
      });
    }
    if (selectedMode.index === 6) {
      if (hostFx.reactiveError) return t('controlPanel.blackShark.hostFxReactiveFailed', { error: hostFx.reactiveError });
      if (!hostFx.reactiveRunning) return t('controlPanel.blackShark.hostFxInactive', { slot: hostFx.currentSlot });
      return t('controlPanel.blackShark.hostFxReactiveRunning', { count: hostFx.reactivePushCount });
    }
    return null;
  })();

  // 0x02 一次写两路向量，没有「只改一路」的形式，下发必须带上另一路当前值。
  const onOffKnown = !!switches?.onOffVectorKnown;
  const smartStartStop = onOffKnown
    ? !!switches?.smartStartStop
    : (switches?.lastCommandedSmartStartStop ?? true);
  const powerOnSelfStart = onOffKnown
    ? !!switches?.powerOnSelfStart
    : (switches?.lastCommandedPowerOnSelfStart ?? true);

  return (
    // 用项目既有的 Section 组件，保证与设置页卡片同形。
    <Section title={t('controlPanel.blackShark.panelTitle')} icon={Fan}>
      {/* 不再包一层容器，让行占满宽度、与其他卡片对齐。 */}
        {info && !info.available ? (
          <div className="rounded-xl border border-border/60 bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
            {t('controlPanel.blackShark.notConnected')}
          </div>
        ) : null}

        {info?.available ? (
          <>
            {/* 固件只检查不刷写；缓存此前无触发点恒为空，此按钮即入口。 */}
            <SettingRow
              icon={<Cpu className="h-4 w-4" />}
              title={t('controlPanel.blackShark.firmware', {
                current: info.firmware?.currentVersion || '-',
                latest: info.firmware?.latestVersion || '-',
              })}
              description={t('controlPanel.blackShark.firmwareOfficialOnly')}
              tip={info.firmware?.updateAvailable
                ? t('controlPanel.blackShark.firmwareUpdateAvailable', {
                    current: info.firmware.currentVersion ?? '?',
                    latest: info.firmware.latestVersion ?? '?',
                  })
                : undefined}
            >
              <Button variant="secondary" size="sm" className="shrink-0" onClick={() => void checkFirmware()} loading={loading} disabled={busy}>
                <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
                {t('controlPanel.blackShark.checkUpdate')}
              </Button>
            </SettingRow>

            {/* 风扇开关已移除：0x20 无回读命令，无法确认状态且与 LCD 显示重复。 */}

                  {/* 智能启停/通电自启：写 0x02（一次写两路向量）、读回 0x03，显示设备回读状态。 */}

                  <SettingRow
                    icon={<Activity className="h-4 w-4" />}
                    title={t('controlPanel.blackShark.smartStartStop')}
                    // 描述与「无读回」提示合成一条，避免出现两个 description。
                    description={
                      !onOffKnown
                        ? `${t('controlPanel.blackShark.smartStartStopDesc')}（${t('controlPanel.blackShark.noReadback')}）`
                        : t('controlPanel.blackShark.smartStartStopDesc')
                    }
                  >
                    <ToggleSwitch
                      size="sm"
                      enabled={smartStartStop}
                      disabled={busy}
                      onChange={(next) =>
                        void run(
                          () => apiService.setBlackSharkOnOffVector(next, powerOnSelfStart),
                          'controlPanel.blackShark.smartStartStopOk',
                        )
                      }
                    />
                  </SettingRow>

                  <SettingRow
                    icon={<Power className="h-4 w-4" />}
                    title={t('controlPanel.blackShark.powerOnSelfStart')}
                    // 描述与「无读回」提示合成一条，避免出现两个 description。
                    description={
                      !onOffKnown
                        ? `${t('controlPanel.blackShark.powerOnSelfStartDesc')}（${t('controlPanel.blackShark.noReadback')}）`
                        : t('controlPanel.blackShark.powerOnSelfStartDesc')
                    }
                  >
                    <ToggleSwitch
                      size="sm"
                      enabled={powerOnSelfStart}
                      disabled={busy}
                      onChange={(next) =>
                        void run(
                          () => apiService.setBlackSharkOnOffVector(smartStartStop, next),
                          'controlPanel.blackShark.powerOnSelfStartOk',
                        )
                      }
                    />
                  </SettingRow>

    <SettingRow
      icon={<Lightbulb className="h-4 w-4" />}
      title={t('controlPanel.blackShark.lightingSwitch')}
      description={t('controlPanel.blackShark.lightingSwitchDesc')}
    >
      <ToggleSwitch
        size="sm"
        enabled={!!switches?.lightingEnabled}
        disabled={busy || !switches?.lightingKnown}
        onChange={(next) =>
          void run(
            () => apiService.setBlackSharkLightingEnabled(next),
            'controlPanel.blackShark.lightingSwitchOk',
          )
        }
      />
    </SettingRow>

                  {/* 灯效行与折叠体直接作为 Section 子项，与兼容模式折叠卡同构。 */}
                  <div data-theme-ui="setting-row" className="px-5 py-4 transition-colors duration-200 hover:bg-muted/18">
                    {/* 读取/重置按钮放在行头；行头拆成两半，避免 Button 嵌套在 button 里。 */}
                    <div className="flex w-full flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                      <button
                        type="button"
                        onClick={() => setRgbOpen((v) => !v)}
                        className="flex min-w-0 flex-1 cursor-pointer items-center gap-3 text-left"
                      >
                        <div data-theme-ui="setting-row-icon" className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-muted/70 text-muted-foreground shadow-inner shadow-white/20">
                          <Lightbulb className="h-4 w-4" />
                        </div>
                        <div className="min-w-0">
                          <div className="text-base font-medium text-foreground">{t('controlPanel.blackShark.rgbModes')}</div>
                        </div>
                      </button>
                      <div className="flex shrink-0 items-center gap-2">
                        <Button variant="secondary" size="sm" onClick={() => void loadRgb()} loading={rgbLoading} disabled={busy}>
                          <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
                          {t('controlPanel.blackShark.rgbLoad')}
                        </Button>
                        <ConfirmButton
                          label={t('controlPanel.blackShark.resetRgb')}
                          confirmLabel={t('controlPanel.blackShark.resetRgbConfirm')}
                          disabled={busy || !rgb?.available}
                          icon={<RotateCcw className="mr-1.5 h-3.5 w-3.5" />}
                          onConfirm={() =>
                            run(() => apiService.resetBlackSharkRgb(), 'controlPanel.blackShark.resetRgbOk')
                              .then(() => void loadRgb())
                          }
                        />
                        <button
                          type="button"
                          onClick={() => setRgbOpen((v) => !v)}
                          aria-label={rgbOpen ? t('controlPanel.blackShark.collapse') : t('controlPanel.blackShark.expand')}
                          className="cursor-pointer p-1"
                        >
                          <ChevronDown className={clsx('h-4 w-4 text-muted-foreground transition-transform duration-200', rgbOpen && 'rotate-180')} />
                        </button>
                      </div>
                    </div>

                    <AnimatePresence initial={false}>
                      {rgbOpen && (
                        <motion.div
                          initial={{ opacity: 0, height: 0 }}
                          animate={{ opacity: 1, height: 'auto' }}
                          exit={{ opacity: 0, height: 0 }}
                          data-theme-ui="compatibility-divider"
                          className="mt-3 overflow-hidden border-t border-border/50"
                        >
                          <div className="space-y-3 pt-4">

                  {/* 灯效模式：写 0x12 / 读 0x13 / 选模式 0x14；折叠卡片默认收起，其余选项折入。 */}

                  {rgb?.error ? (
                    <div className="text-xs text-muted-foreground">{rgb.error}</div>
                  ) : null}

                  {/* 缓存来源需显式标注，它不是设备实时状态。 */}
                  {rgb?.fromCache && rgb?.cachedAtUnix ? (
                    <div className="text-[11px] font-medium text-amber-600 dark:text-amber-400">
                      {t('controlPanel.blackShark.rgbFromCache', {
                        time: new Date(rgb.cachedAtUnix * 1000).toLocaleString(),
                      })}
                    </div>
                  ) : null}

                  {rgb?.available && rgb.modes?.length ? (
                    <>
                      <div className="flex flex-wrap gap-1.5">
                        {rgb.modes.map((mode) => (
                          <Button
                            key={mode.index}
                            variant={draft?.index === mode.index ? 'primary' : 'secondary'}
                            size="sm"
                            disabled={busy || !mode.known}
                            onClick={() => {
                              // 不做乐观更新：等设备回读后再刷新状态。
                              setStaticColor(!!mode.staticColor);
                              void run(
                                () => apiService.selectBlackSharkRgbMode(mode.index),
                                'controlPanel.blackShark.rgbModeOk',
                              ).then(() => void loadRgb());
                            }}
                          >
                            {modeLabel(mode)}
                            {mode.current ? ' ●' : ''}
                          </Button>
                        ))}
                      </div>

                      {/* 槽位 6/7 需要主机持续喂数据；显示说明与状态，避免被误判为故障。 */}
                      {selectedMode?.needsHostData ? (
                        <div className="rounded-xl border border-border/60 bg-muted/20 px-3 py-2 text-[11px] leading-relaxed text-muted-foreground">
                          <div>{selectedMode.needsHostData}</div>
                          {hostFxLine ? (
                            <div className="mt-1 font-medium text-foreground/80">{hostFxLine}</div>
                          ) : null}
                        </div>
                      ) : null}

                      {draft ? (
                        <div className="space-y-1.5 pt-1">
                          <Slider
                            label={t('controlPanel.blackShark.rgbSpeed')}
                            // 速度/亮度/色相不显示数值：数值无意义，还挤占轨道宽度。
                            showValue={false}
                            value={draft.speed}
                            min={rgb.modes.find((m) => m.index === draft.index)?.speedRange.min ?? 0}
                            max={rgb.modes.find((m) => m.index === draft.index)?.speedRange.max ?? 65535}
                            step={50}
                            // 常亮(4)/音频同步(7) 的速度滑块在官方 UI 即禁用，后端用 speedDisabled 表示（deviceproto 为准）。
                            disabled={busy || !!rgb.modes.find((m) => m.index === draft.index)?.speedDisabled}
                            // 官方速度滑块左端为最大值；拖到最左读到 payload=4000=max。
                            invert
                            onChange={(v) => setDraft({ ...draft, speed: v })}
                          />
                          <Slider
                            label={t('controlPanel.blackShark.rgbBrightness')}
                            showValue={false}
                            value={draft.brightness}
                            min={rgb.modes.find((m) => m.index === draft.index)?.brightnessRange.min ?? 1}
                            max={rgb.modes.find((m) => m.index === draft.index)?.brightnessRange.max ?? 100}
                            step={1}
                            disabled={busy}
                            onChange={(v) => setDraft({ ...draft, brightness: v })}
                          />
                          <div className="flex items-center justify-end gap-2">
                            <Button
                              variant="secondary"
                              size="sm"
                              disabled={busy}
                              onClick={() =>
                                void run(
                                  () =>
                                    apiService.setBlackSharkRgbModeEffects(
                                      draft.index,
                                      draft.speed,
                                      draft.brightness,
                                    ),
                                  'controlPanel.blackShark.rgbApplyOk',
                                ).then(() => void loadRgb())
                              }
                            >
                              {t('controlPanel.blackShark.rgbApply')}
                            </Button>
                          </div>

                          {/* 颜色控件按模式 gating（表在 deviceproto.BlackSharkRgbColorControlsBySlot）：没有的入口不画，不可用的置灰。 */}
                          {(() => {
                            const mode = rgb.modes.find((m) => m.index === draft.index);
                            const cc = mode?.colorControls;
                            const showColorMode = !!cc?.colorMode;
                            const showSingleColor = !!cc?.singleColor;
                            const hueAvail = cc?.hue ?? 'none';
                            const showHue = hueAvail !== 'none';
                            // 色相可用性照官方：disabled 置灰，singleColorOnly 仅「单色」时可用。
                            const staticForMode = showSingleColor ? staticColor : !!mode?.staticColor;
                            const hueUsable =
                              hueAvail === 'always' ||
                              (hueAvail === 'singleColorOnly' && staticForMode);

                            if (!showColorMode && !showSingleColor && !showHue) {
                              return (
                                <div className="text-[11px] leading-relaxed text-muted-foreground">
                                  {t('controlPanel.blackShark.rgbColorNone')}
                                </div>
                              );
                            }
                            // 该页没有「单色/彩色」开关时，写颜色要带上设备当前字节，否则会改掉官方写死的值。
                            const colorForWrite = showSingleColor ? staticColor : !!mode?.staticColor;

                            return (
                              <>
                                {showColorMode && rgb.colorOptions?.length ? (
                                  <Select
                                    label={t('controlPanel.blackShark.rgbColorMode')}
                                    value={mode?.colorOption ?? 0}
                                    disabled={busy}
                                    options={rgb.colorOptions.map((o) => ({ value: o.index, label: o.name }))}
                                    onChange={(v) =>
                                      void run(
                                        () => apiService.setBlackSharkRgbColorOption(draft.index, v),
                                        'controlPanel.blackShark.rgbColorModeOk',
                                      ).then(() => void loadRgb())
                                    }
                                  />
                                ) : null}

                                {showHue ? (
                                  <Slider
                                    label={t('controlPanel.blackShark.rgbHue')}
                                    showValue={false}
                                    value={hueDraft}
                                    min={0}
                                    max={360}
                                    step={1}
                                    disabled={busy || !hueUsable}
                                    // 色相环直接映射到滑杆；不可用时整条变灰。
                                    trackGradient={HUE_TRACK_GRADIENT}
                                    onChange={setHueDraft}
                                  />
                                ) : null}

                                {showSingleColor ? (
                                  <div className="flex items-center justify-between gap-2">
                                    <span className="flex items-center gap-1.5">
                                      <span className="text-[11px] text-muted-foreground">
                                        {t('controlPanel.blackShark.rgbStaticColor')}
                                      </span>
                                      <ToggleSwitch
                                        size="sm"
                                        enabled={staticColor}
                                        disabled={busy}
                                        onChange={setStaticColor}
                                      />
                                    </span>
                                  </div>
                                ) : null}

                                {showHue ? (
                                  <div className="flex items-center justify-between gap-2">
                                    <span className="text-[11px] leading-relaxed text-muted-foreground">
                                      {t('controlPanel.blackShark.rgbColorNote')}
                                    </span>
                                    <Button
                                      variant="secondary"
                                      size="sm"
                                      disabled={busy || !hueUsable}
                                      onClick={() =>
                                        void run(
                                          () =>
                                            apiService.setBlackSharkRgbModeColor(
                                              draft.index,
                                              hueDraft,
                                              colorForWrite,
                                            ),
                                          'controlPanel.blackShark.rgbColorOk',
                                        ).then(() => void loadRgb())
                                      }
                                    >
                                      {t('controlPanel.blackShark.rgbColorApply')}
                                    </Button>
                                  </div>
                                ) : null}
                              </>
                            );
                          })()}
                        </div>
                      ) : null}
                    </>
                  ) : (
                    <div className="text-xs text-muted-foreground">
                      {t('controlPanel.blackShark.rgbNotLoaded')}
                    </div>
                  )}
                          </div>
                        </motion.div>
                      )}
                    </AnimatePresence>
                  </div>

                {/* 原生变频曲线数据从设备读（0x26）。 */}

                {/* 配置快照：各项写入可反复重写，快照只回到用户已知的好状态，可再取。
                    用标准 SettingRow（title/description + 右侧控件），与温度平滑度行同构。 */}
                <SettingRow
                  icon={<Archive className="h-4 w-4" />}
                  title={t('controlPanel.blackShark.snapshot')}
                  description={t('controlPanel.blackShark.snapshotNote')}
                >
                  <div className="flex flex-wrap items-center justify-end gap-2">
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={busy}
                      onClick={() =>
                        void runQuiet(async () => {
                          const res = await apiService.exportBlackSharkConfigSnapshot();
                          if (res.error) {
                            toast.error(res.error);
                            return;
                          }
                          if (!res.saved) return; // 用户取消
                          toast.success(t('controlPanel.blackShark.snapshotExportOk', { path: res.path ?? '' }));
                        })
                      }
                    >
                      {t('controlPanel.blackShark.snapshotExport')}
                    </Button>
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={busy}
                      onClick={() =>
                        void runQuiet(async () => {
                          const res = await apiService.restoreBlackSharkConfigFromFile();
                          // 取消时两项皆空，不提示。
                          if (!res.restored.length && !res.issues.length) return;
                          if (res.restored.length) {
                            toast.success(t('controlPanel.blackShark.snapshotRestoreDone', { n: res.restored.length }));
                          }
                          res.issues.forEach((issue) => toast.warning(issue));
                        })
                      }
                    >
                      {t('controlPanel.blackShark.snapshotRestore')}
                    </Button>
                  </div>
                </SettingRow>
              </>
            ) : null}
    </Section>
  );
}

/** 「情景」：按前台进程自动施加档位与灯效模式。 */
export function ScenePanel() {
  const { t } = useTranslation();
  const [rules, setRules] = useState<SceneRulePayload[]>([]);
  const [baseline, setBaseline] = useState(0);
  const [processes, setProcesses] = useState<string[]>([]);
  const [issues, setIssues] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);

  // 固定四个槽位（官方 t_Brb02SceneConfig 恒 4 行）：不足补齐，避免槽位数量随配置变化。
  const padRules = useCallback((list: SceneRulePayload[]): SceneRulePayload[] => {
    const out = list.slice(0, SCENE_SLOTS);
    while (out.length < SCENE_SLOTS) {
      out.push({ enabled: false, name: '', match: '', gear: 0, rgbMode: 0 });
    }
    return out;
  }, []);

  const load = useCallback(async () => {
    setBusy(true);
    try {
      const data: SceneRulesPayload = await apiService.getSceneRules();
      setRules(padRules(data.rules ?? []));
      setBaseline(data.baselineGear ?? 0);
      const procs = await apiService.listSceneProcesses();
      setProcesses(procs ?? []);
    } catch (error) {
      toast.error(t('controlPanel.scene.loadFailed', { error: String(error) }));
    } finally {
      setBusy(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  const save = useCallback(
    async (nextRules: SceneRulePayload[], nextBaseline: number) => {
      setBusy(true);
      try {
        const result = await apiService.setSceneRules({ rules: nextRules, baselineGear: nextBaseline });
        setIssues(result.issues ?? []);
        if (result.saved) {
          toast.success(t('controlPanel.scene.saved'));
          // 校验问题要显式提示：越界字段会被报出而非静默纠正。
          if ((result.issues ?? []).length > 0) {
            toast.warning(t('controlPanel.scene.savedWithIssues', { n: result.issues.length }));
          }
        } else {
          toast.error(t('controlPanel.scene.saveFailed'));
        }
      } catch (error) {
        toast.error(t('controlPanel.scene.saveFailed') + ' ' + String(error));
      } finally {
        setBusy(false);
      }
    },
    [t],
  );

  const patch = useCallback(
    (index: number, changes: Partial<SceneRulePayload>) => {
      const next = rules.map((r, i) => (i === index ? { ...r, ...changes } : r));
      setRules(next);
      void save(next, baseline);
    },
    [rules, baseline, save],
  );

  const gearOptions = [
    { value: 0, label: t('controlPanel.scene.keep') },
    { value: 1, label: t('controlPanel.scene.gear1') },
    { value: 2, label: t('controlPanel.scene.gear2') },
    { value: 3, label: t('controlPanel.scene.gear3') },
    { value: 4, label: t('controlPanel.scene.gear4') },
  ];

  const rgbOptions = [
    { value: 0, label: t('controlPanel.scene.keep') },
    ...Array.from({ length: 8 }, (_, i) => ({ value: i + 1, label: `#${i + 1}` })),
  ];

  /** 进程下拉项：把当前值并入，否则 Radix 无法显示不在列表里的值。 */
  const processOptionsFor = (current: string) => {
    const items = processes.slice(0, 300).map((p) => ({ value: p, label: p }));
    if (current && !items.some((o) => o.value === current)) {
      items.unshift({ value: current, label: current });
    }
    return items;
  };

  return (
    // 与「原生控制/设备设置」同用 Section（标题栏 + 卡片），标题复用 controlPanel.scene.title。
    <Section title={t('controlPanel.scene.title')} icon={Activity}>
      {/* 去掉正文包装层，行/内容直接作为 Section 子项以占满宽度。 */}

      <p className="px-5 py-4 text-xs leading-relaxed text-muted-foreground">{t('controlPanel.scene.hint')}</p>

      {rules.map((rule, index) => (
        // Select 的 label 渲染在控件上方；同一行里有的传 label 有的没传会导致高度不齐。
        <div
          key={`slot-${index}`}
          data-theme-ui="setting-row"
          className="flex flex-col gap-4 px-5 py-4 transition-colors duration-200 hover:bg-muted/18 sm:flex-row sm:items-center sm:justify-between"
        >
          {/* 左侧用可重命名的名称输入框替代静态「槽位 N」；右侧只留进程/档位/灯效三个下拉。 */}
          <div className="flex min-w-0 shrink-0 items-center gap-3">
            <ToggleSwitch
              size="sm"
              enabled={rule.enabled}
              disabled={busy}
              onChange={(next) => patch(index, { enabled: next })}
            />
            <Input
              className="h-9 w-40 shrink-0 text-base font-medium"
              value={rule.name ?? ''}
              placeholder={t('controlPanel.scene.namePlaceholder', { n: index + 1 })}
              maxLength={24}
              disabled={busy}
              onChange={(e) => patch(index, { name: e.target.value })}
            />
          </div>
          <div className="flex min-w-0 flex-wrap items-end gap-2">
            <Select
              size="sm"
              className="min-w-[9rem] flex-1"
              label={t('controlPanel.scene.process')}
              value={rule.match}
              disabled={busy}
              placeholder={t('controlPanel.scene.pickProcess')}
              options={processOptionsFor(rule.match)}
              onChange={(value) => patch(index, { match: value })}
            />
          <Select
            size="sm"
            label={t('controlPanel.scene.gear')}
            value={rule.gear}
            disabled={busy}
            options={gearOptions}
            onChange={(value) => patch(index, { gear: value })}
          />
          <Select
            size="sm"
            label={t('controlPanel.scene.rgbMode')}
            value={rule.rgbMode}
            disabled={busy}
            options={rgbOptions}
            onChange={(value) => patch(index, { rgbMode: value })}
          />
          </div>
        </div>
      ))}

      {/* 基准行改用 SettingRow（title/description）+ 右侧窄 Select。 */}
      <SettingRow
        icon={<Activity className="h-4 w-4" />}
        title={t('controlPanel.scene.baseline')}
        description={t('controlPanel.scene.baselineHint')}
      >
        <div className="w-40">
          <Select
            size="sm"
            value={baseline}
            disabled={busy || rules.length === 0}
            options={gearOptions}
            onChange={(value) => {
              setBaseline(value);
              void save(rules, value);
            }}
          />
        </div>
      </SettingRow>

      {issues.length > 0 ? (
        <ul className="list-inside list-disc px-5 pb-4 text-xs text-amber-600 dark:text-amber-400">
          {issues.map((issue) => (
            <li key={issue}>{issue}</li>
          ))}
        </ul>
      ) : null}
    </Section>
  );
}

/** 「LCD 小屏」区：小屏开关 + 屏幕显示参数（0xC2）。 */
function BlackSharkLcdSection() {
  const { t } = useTranslation();
  const isConnected = useAppStore((state) => state.isConnected);
  const [busy, setBusy] = useState(false);
  const [switches, setSwitches] = useState<BlackSharkInfo['switches'] | undefined>(undefined);
  // 屏幕显示参数（0xC2）：屏上三格显示哪三项及顺序。0xC2 只有 Set 没有 Get，
  // 读的是本工具保存的那一份，所以文案一律是「已下发」。
  const [lcd, setLcd] = useState<BlackSharkLcdDisplayPayload | null>(null);
  const [lcdItems, setLcdItems] = useState<number[] | null>(null);
  // 屏幕显示参数折叠状态。
  const [lcdOpen, setLcdOpen] = useState(false);

  const loadLcd = useCallback(async () => {
    try {
      const next = await apiService.getBlackSharkLcdDisplay();
      setLcd(next);
      setLcdItems(next.items.slice(0, 3));
    } catch (error) {
      toast.error(t('controlPanel.blackShark.actionFailed') + ' ' + String(error));
    }
  }, [t]);

  // 小屏开关状态（0xC0/0xC1 有读回）：拿不到保持 undefined，开关置灰，不猜状态。
  const loadSwitches = useCallback(async () => {
    try {
      const info = await apiService.getBlackSharkInfo();
      setSwitches(info?.switches);
    } catch {
      // 忽略：界面会因状态未知而置灰。
    }
  }, []);

  // 连接后查一次，与「原生控制」一致。
  useEffect(() => {
    if (!isConnected) return;
    void loadLcd();
    void loadSwitches();
  }, [isConnected, loadLcd, loadSwitches]);

  const lcdDuplicated = !!lcdItems && new Set(lcdItems).size !== lcdItems.length;

  /** 下发屏幕显示参数（0xC2）。 */
  const applyLcdItems = useCallback(
    async (items: number[]) => {
      setBusy(true);
      try {
        const res = await apiService.setBlackSharkLcdDisplay(0, items);
        if (res.applied) {
          toast.success(t('controlPanel.blackShark.lcdDisplayOk'));
        } else {
          // 保存成功但下发失败时带上 Go 侧原因，否则用户不知该先连设备还是重试。
          toast.error(res.error || t('controlPanel.blackShark.actionFailed'));
        }
        await loadLcd();
      } catch (error) {
        toast.error(t('controlPanel.blackShark.actionFailed') + ' ' + String(error));
      } finally {
        setBusy(false);
      }
    },
    [loadLcd, t],
  );

  const setLcdEnabled = useCallback(
    async (next: boolean) => {
      setBusy(true);
      try {
        if (await apiService.setBlackSharkLcdScreenEnabled(next)) {
          toast.success(t('controlPanel.blackShark.lcdSwitchOk'));
        } else {
          toast.error(t('controlPanel.blackShark.actionFailed'));
        }
        await loadSwitches();
      } catch (error) {
        toast.error(t('controlPanel.blackShark.actionFailed') + ' ' + String(error));
      } finally {
        setBusy(false);
      }
    },
    [loadSwitches, t],
  );

  return (
    // LCD 两行直接作为 Section 子项，放在屏幕图像内容之上。
    <>
      <SettingRow
        icon={<MonitorSmartphone className="h-4 w-4" />}
        title={t('controlPanel.blackShark.lcdSwitch')}
        description={t('controlPanel.blackShark.lcdSwitchDesc')}
      >
        <ToggleSwitch
          size="sm"
          enabled={!!switches?.lcdScreenEnabled}
          disabled={busy || !switches?.lcdScreenKnown}
          onChange={(next) => void setLcdEnabled(next)}
        />
      </SettingRow>

      {/* 与「灯效模式」行同构：AnimatePresence + motion.div（border-t 分隔线），体内放下拉/提示/按钮。 */}
      <div data-theme-ui="setting-row" className="px-5 py-4 transition-colors duration-200 hover:bg-muted/18">
        {/* 重置/下发按钮放在行头；下发即把当前三个下拉的值交给 applyLcdItems。 */}
        <div className="flex w-full flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <button
            type="button"
            onClick={() => setLcdOpen((v) => !v)}
            className="flex min-w-0 flex-1 cursor-pointer items-center gap-3 text-left"
          >
            <div data-theme-ui="setting-row-icon" className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-muted/70 text-muted-foreground shadow-inner shadow-white/20">
              <MonitorSmartphone className="h-4 w-4" />
            </div>
            <div className="min-w-0">
              <div className="text-base font-medium text-foreground">{t('controlPanel.blackShark.lcdDisplay')}</div>
              <div className="text-sm text-muted-foreground line-clamp-2">
                {t('controlPanel.blackShark.noReadback')}
              </div>
            </div>
          </button>
          <div className="flex shrink-0 items-center gap-2">
            <Button
              variant="secondary"
              size="sm"
              disabled={busy || !lcd?.defaultItems?.length}
              onClick={() => void applyLcdItems(lcd?.defaultItems ?? [])}
            >
              {t('controlPanel.blackShark.lcdDisplayReset')}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              disabled={busy || !lcdItems || lcdDuplicated}
              onClick={() => void applyLcdItems(lcdItems ?? [])}
            >
              {t('controlPanel.blackShark.lcdDisplayApply')}
            </Button>
            <button
              type="button"
              onClick={() => setLcdOpen((v) => !v)}
              aria-label={lcdOpen ? t('controlPanel.blackShark.collapse') : t('controlPanel.blackShark.expand')}
              className="cursor-pointer p-1"
            >
              <ChevronDown className={clsx('h-4 w-4 text-muted-foreground transition-transform duration-200', lcdOpen && 'rotate-180')} />
            </button>
          </div>
        </div>

        <AnimatePresence initial={false}>
          {lcdOpen && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              data-theme-ui="compatibility-divider"
              className="mt-3 overflow-hidden border-t border-border/50"
            >
              <div className="space-y-3 pt-4">
                <div className="flex flex-wrap items-center gap-1.5">
        {[0, 1, 2].map((slot) => (
          <Select<number>
            key={slot}
            size="sm"
            value={lcdItems?.[slot] ?? -1}
            disabled={busy || !lcdItems}
            options={(lcd?.options ?? []).map((id) => ({
              value: id,
              label: t(`controlPanel.blackShark.lcdItem.${id}`),
            }))}
            onChange={(value) =>
              setLcdItems((prev) => {
                const next = (prev ?? [0, 1, 2]).slice(0, 3);
                next[slot] = value;
                return next;
              })
            }
          />
        ))}
      </div>

                <div className="text-[11px] leading-relaxed text-muted-foreground">
                  {t('controlPanel.blackShark.lcdOrderHint')}
                </div>
                {lcdDuplicated ? (
                  <div className="text-[11px] font-medium text-amber-600 dark:text-amber-400">
                    {t('controlPanel.blackShark.lcdDuplicate')}
                  </div>
                ) : null}
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </>
  );
}

/** 「屏幕 / 屏保图像」：十张官方精选 + 本地上传。 */
export function ScreenPanel() {
  const { t } = useTranslation();
  const [presets, setPresets] = useState<ScreenPresetPayload[]>([]);
  const [thumbs, setThumbs] = useState<Record<number, string>>({});
  const [current, setCurrent] = useState<ScreenImageInfoPayload | null>(null);
  const [busy, setBusy] = useState(false);
  const [uploadingAt, setUploadingAt] = useState<number | null>(null);
  // 图传进度：由核心的进度事件驱动（弹窗里的进度条），上传结束即清空。
  const [uploadProgress, setUploadProgress] = useState<{ sent: number; total: number } | null>(null);
  useEffect(() => {
    const off = apiService.onScreenImageTransferProgress((p) => setUploadProgress(p));
    return () => {
      if (typeof off === 'function') off();
    };
  }, []);
  useEffect(() => {
    if (uploadingAt === null) setUploadProgress(null);
  }, [uploadingAt]);
  // 进度未知时按 0% 显示（设备还没回第一拍），不编数。
  const uploadPercent =
    uploadProgress && uploadProgress.total > 0
      ? Math.min(100, Math.round((uploadProgress.sent / uploadProgress.total) * 100))
      : 0;
  // 当前图缩略图：预设走 ScreenPresetThumbnail；自定义图需点「从设备读回」取真值（很慢）。
  const [currentThumb, setCurrentThumb] = useState('');
  // 从设备读回的整张图（data URL），读到即以此为准。
  const [deviceImg, setDeviceImg] = useState('');
  // 读取时间（Unix 秒）；null 表示刚读到。
  const [deviceImgCachedAt, setDeviceImgCachedAt] = useState<number | null>(null);
  // true 表示当前图刚由预设/本地上传写入，无需再从设备读回。
  const [deviceImgFromUpload, setDeviceImgFromUpload] = useState(false);
  // deviceImg 那张图的 CRC（上传/读回时得到的那个）。**用来判它有没有过期**：
  // 设备上的图被换过之后，deviceImg 里那张就是"上一张"，不能再当"当前图"显示。
  const [deviceImgCRC, setDeviceImgCRC] = useState<number | null>(null);
  const [readingImg, setReadingImg] = useState(false);

  // 历史图片 = 本机缓存（<安装目录>/screen-images/，只记上传成功过屏的图）。null = 还没加载过。
  const [history, setHistory] = useState<ScreenHistoryListPayload | null>(null);
  // 设备当前那张在本机缓存里的对应项（CRC 对得上才有）：顶部预览用它，免掉几十秒的设备读回。
  const [historyCurrent, setHistoryCurrent] = useState<ScreenHistoryItemPayload | null>(null);
  const [historyBusy, setHistoryBusy] = useState(false);

  // 手动裁剪：选好本地图先进裁剪再上传。三参数为整数百分比（zoom 相对刚填满，offset 0=居中）；
  // 预览与上传必须逐位一致，理由见 ScreenCropPreviewPayload。
  const [cropFile, setCropFile] = useState('');
  const [cropZoom, setCropZoom] = useState(100);
  const [cropOffX, setCropOffX] = useState(0);
  const [cropOffY, setCropOffY] = useState(0);
  const [cropPreview, setCropPreview] = useState<ScreenCropPreviewPayload | null>(null);
  const [cropLoading, setCropLoading] = useState(false);

  // 取当前图缩略图：从设备读回的优先，其次内置预设。
  useEffect(() => {
    if (!current?.hasImage) {
      setCurrentThumb('');
      setDeviceImg('');
      setDeviceImgCRC(null);
      return;
    }
    if (current.presetMatched && current.presetOfficialPosition) {
      let cancelled = false;
      void apiService
        .screenPresetThumbnail(current.presetOfficialPosition)
        .then((url) => {
          if (!cancelled) setCurrentThumb(url || '');
        })
        .catch(() => {
          if (!cancelled) setCurrentThumb('');
        });
      return () => {
        cancelled = true;
      };
    }
    setCurrentThumb('');
    return;
  }, [current]);

  // 挂载时先显示缓存图：deviceImg 是组件 state，切页卸载后会丢失，而重读要几十秒。
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const cached = await apiService.getScreenImageCached();
        if (cancelled || !cached?.ok || !cached.dataUrl) return;
        setDeviceImg(cached.dataUrl);
        setDeviceImgCachedAt(cached.cachedAtUnix ?? null);
        // 记下这份的 CRC：它可能已经过期（设备上的图换过了），显示时要用它来判。
        setDeviceImgCRC(cached.crc ?? null);
      } catch {
        // 缓存读取失败不阻塞面板。
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const load = useCallback(async () => {
    setBusy(true);
    try {
      const list = await apiService.listScreenPresets();
      setPresets(list.presets ?? []);
      setCurrent(await apiService.getScreenImageInfo());
    } catch (error) {
      toast.error(t('controlPanel.screen.loadFailed', { error: String(error) }));
    } finally {
      setBusy(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  // 从设备读回整张屏图（0xC7 + 反复 0xC8，约 2096 帧），很慢（几十秒），
  // 只由用户显式触发。
  const readDeviceImage = useCallback(async () => {
    setReadingImg(true);
    try {
      const r = await apiService.readScreenImageFromDevice();
      if (r.ok && r.dataUrl) {
        setDeviceImg(r.dataUrl);
        setDeviceImgCRC(r.crc ?? null);
        // 来源变为设备真值，清掉「刚上传」标记。
        setDeviceImgFromUpload(false);
        // 缓存来源要标出读取时间，否则会被当成屏上此刻的内容。
        const cachedAt = r.fromCache ? (r.cachedAtUnix ?? 0) : 0;
        setDeviceImgCachedAt(cachedAt || null);
        if (r.fromCache) {
          // 该文案带 {{time}} 占位符，必须传入时间，否则界面显示字面量。
          toast.info(t('controlPanel.screen.readFromDeviceCached', {
            time: new Date((r.cachedAtUnix ?? 0) * 1000).toLocaleString(),
          }));
        } else {
          toast.success(t('controlPanel.screen.readFromDeviceOk', { bytes: r.bytes }));
        }
      } else {
        toast.error(r.error || t('controlPanel.screen.readFromDeviceFailed'));
      }
    } catch (error) {
      toast.error(t('controlPanel.screen.readFromDeviceFailed') + ' ' + String(error));
    } finally {
      setReadingImg(false);
    }
  }, [t]);

  // 缩略图按需拉：十张原图共约 870KB，全塞进列表会让首屏很重。
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      for (const p of presets) {
        if (cancelled) return;
        if (thumbs[p.officialPosition]) continue;
        const url = await apiService.screenPresetThumbnail(p.officialPosition);
        if (cancelled) return;
        if (url) setThumbs((prev) => ({ ...prev, [p.officialPosition]: url }));
      }
    })();
    return () => {
      cancelled = true;
    };
    // thumbs 故意不入依赖：它在循环里被更新，入依赖会无限触发
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [presets]);

  const reportResult = useCallback(
    (ok: boolean, detail: string) => {
      if (ok) toast.success(detail);
      else toast.error(detail);
    },
    [],
  );

  const applyPreset = useCallback(
    async (position: number) => {
      setBusy(true);
      setUploadingAt(position);
      const presetThumb = thumbs[position];
      try {
        const res = await apiService.uploadScreenPreset(position);
        if (res.success && res.verified) {
          reportResult(
            true,
            t('controlPanel.screen.applied', { crc: res.crc.toString(16).toUpperCase() }),
          );
          const info = await apiService.getScreenImageInfo();
          setCurrent(info);
          // 刚上传的就是这张预设缩略图，直接显示，不走几十秒读回；清掉「上次读到」标记。
          if (presetThumb) {
            setDeviceImg(presetThumb);
            setDeviceImgFromUpload(true);
            setDeviceImgCachedAt(null);
            setDeviceImgCRC(info.crc ?? null);
          }
        } else {
          reportResult(false, t('controlPanel.screen.failed', { error: res.error ?? 'unknown' }));
        }
      } catch (error) {
        reportResult(false, t('controlPanel.screen.failed', { error: String(error) }));
      } finally {
        setUploadingAt(null);
        setBusy(false);
      }
    },
    [reportResult, t, thumbs],
  );

  // 历史图片：只依赖"设备当前那张"的 CRC —— 既用它把正在用的那张从"历史两格"里排除，
  // 也用它在本机缓存里找到那张、当作顶部预览（这样传完本地图切页签回来，不用再点"从设备读回"）。
  // 纯本机 IO，不会跟上传播放设备抢占。
  const loadHistory = useCallback(async (excludeCrc: number) => {
    setHistoryBusy(true);
    try {
      const list = await apiService.listScreenHistoryImages(excludeCrc, 2);
      setHistory(list);
      setHistoryCurrent(list.current ?? null);
    } catch (error) {
      setHistory({ dir: '', items: [], error: String(error) });
      setHistoryCurrent(null);
    } finally {
      setHistoryBusy(false);
    }
  }, []);

  useEffect(() => {
    void loadHistory(current?.crc ?? 0);
  }, [current?.crc, loadHistory]);

  // 历史图点一下就上屏：字节原样直传（免解码、免裁剪），其余流程与预设完全一致。
  const applyHistory = useCallback(
    async (item: ScreenHistoryItemPayload) => {
      setBusy(true);
      setUploadingAt(-1); // 只为点亮进度弹窗；它不是预设位次，没有对应的格子
      try {
        const res = await apiService.uploadScreenHistoryImage(item.path);
        if (res.success && res.verified) {
          reportResult(
            true,
            t('controlPanel.screen.applied', { crc: res.crc.toString(16).toUpperCase() }),
          );
          const info = await apiService.getScreenImageInfo();
          setCurrent(info);
          // 刚上传的就是这张历史图缩略图 ⇒ 直接显示，不走几十秒读回。
          if (item.thumb) {
            setDeviceImg(item.thumb);
            setDeviceImgFromUpload(true);
            setDeviceImgCachedAt(null);
            setDeviceImgCRC(info.crc ?? null);
          }
          // 它现在是"设备当前那张"了 ⇒ 重拉一次，把它自己从历史里移出去。
          void loadHistory(info.crc);
        } else {
          reportResult(false, t('controlPanel.screen.failed', { error: res.error ?? 'unknown' }));
        }
      } catch (error) {
        reportResult(false, t('controlPanel.screen.failed', { error: String(error) }));
      } finally {
        setUploadingAt(null);
        setBusy(false);
      }
    },
    [loadHistory, reportResult, t],
  );

  // 删除的是官方装备箱自己的缓存文件（不可逆），所以按钮是两步确认。
  const deleteHistory = useCallback(
    async (item: ScreenHistoryItemPayload) => {
      const ok = await apiService.deleteScreenHistoryImage(item.path);
      if (ok) {
        toast.success(t('controlPanel.screen.historyDeleted', { name: item.name }));
        void loadHistory(current?.crc ?? 0);
      } else {
        toast.error(t('controlPanel.screen.historyDeleteFailed'));
      }
    },
    [current?.crc, loadHistory, t],
  );

  // 选图后进入裁剪。取消选择时不做处理。
  const pickForCrop = useCallback(async () => {
    const path = await apiService.pickScreenImageFile();
    if (!path) return; // 用户取消
    setCropPreview(null);
    setCropZoom(100); // 默认：填满并居中
    setCropOffX(0);
    setCropOffY(0);
    setCropFile(path);
  }, []);

  // 预览加 120ms 去抖：每次预览都要解码原图 + 双线性重采样，拖动连续触发会卡。
  useEffect(() => {
    if (!cropFile) return;
    let cancelled = false;
    const timer = window.setTimeout(() => {
      void (async () => {
        setCropLoading(true);
        try {
          const p = await apiService.previewScreenImageCrop(cropFile, cropZoom, cropOffX, cropOffY);
          if (!cancelled && p.dataUrl) setCropPreview(p);
        } catch (error) {
          if (!cancelled) {
            toast.error(t('controlPanel.screen.cropPreviewFailed', { error: String(error) }));
          }
        } finally {
          if (!cancelled) setCropLoading(false);
        }
      })();
    }, 120);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [cropFile, cropZoom, cropOffX, cropOffY, t]);

  // 按当前裁剪参数上传；与预览同源，屏上看到的 428×142 即写入设备的那张。
  const applyCrop = useCallback(async () => {
    if (!cropFile) return;
    setBusy(true);
    setUploadingAt(0);
    try {
      const res = await apiService.uploadScreenImageFile(cropFile, cropZoom, cropOffX, cropOffY);
      if (res.success && res.verified) {
        reportResult(
          true,
          t('controlPanel.screen.applied', { crc: res.crc.toString(16).toUpperCase() }),
        );
        const info = await apiService.getScreenImageInfo();
        setCurrent(info);
        // 刚上传的本地图即这张裁剪预览，直接显示；须在 setCropPreview(null) 之前取。
        if (cropPreview?.dataUrl) {
          setDeviceImg(cropPreview.dataUrl);
          setDeviceImgFromUpload(true);
          setDeviceImgCachedAt(null);
          setDeviceImgCRC(info.crc ?? null);
        }
        setCropFile('');
        setCropPreview(null);
      } else {
        reportResult(false, t('controlPanel.screen.failed', { error: res.error ?? 'unknown' }));
      }
    } catch (error) {
      reportResult(false, t('controlPanel.screen.failed', { error: String(error) }));
    } finally {
      setUploadingAt(null);
      setBusy(false);
    }
  }, [cropFile, cropZoom, cropOffX, cropOffY, reportResult, t]);

  // 「当前图」的取值顺序按**新鲜度**排，不按来源：
  //   ① 刚由我们传上去的那张（deviceImgFromUpload）—— 最新的真值；
  //   ② 本机缓存里 CRC 与设备当前一致的那张（historyCurrent）—— 本地瞬时、且与 0xC5 对得上；
  //   ③ 从设备读回的整张图，**但必须仍与设备当前 CRC 一致**；
  //   ④ 内置预设缩略图（currentThumb，兜底）。
  // ★ 以前是"只要有 deviceImg 就优先"，于是设备换过图之后，那份**过期的读回图**会一直压着新图
  //   （症状：切页签回来显示上一张）。现在用 CRC 卡死：对不上就不许上屏。
  const deviceNowCRC = current?.hasImage ? current.crc : null;
  const deviceImgIsCurrent = deviceImg !== '' && deviceNowCRC !== null && deviceImgCRC === deviceNowCRC;
  const visibleDeviceImg =
    deviceImgFromUpload && deviceImg
      ? deviceImg
      : historyCurrent?.thumb || (deviceImgIsCurrent ? deviceImg : '');

  // 「上次读到的那份」提示只在**确实显示的就是它**时给，否则会误导。
  const showReadCacheHint =
    deviceImgCachedAt !== null && !deviceImgFromUpload && !historyCurrent?.thumb && deviceImgIsCurrent;
  // 手上那份读回图已经过期（设备上的图换过了）：说清楚，免得用户以为界面坏了。
  const staleReadBack =
    deviceImg !== '' && !deviceImgFromUpload && !deviceImgIsCurrent &&
    deviceNowCRC !== null && deviceImgCRC !== null;

  return (
    // 与「原生控制/情景」同用 Section；LCD 两行在最上，屏幕图像内容在其下。
    <Section title={t('controlPanel.screen.title')} icon={MonitorSmartphone}>
      {/* 去掉正文包装层，行直接作为 Section 子项；非行内容各自带内边距。 */}
      <BlackSharkLcdSection />

      {/* 预览图、本地上传、十张预设图合并为一张卡片（参考设置页温度基准）。 */}

      {/* 手动裁剪编辑器。参数与上传同源（同一个 CoverToRGBWith），slackY 用于说明可偏移量。 */}
      {cropFile ? (
        <div className="space-y-2 rounded-2xl border border-border/60 bg-card/86 shadow-sm shadow-black/5 px-4 py-3">
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-medium text-foreground">
              {t('controlPanel.screen.cropTitle')}
            </span>
            <span className="text-[11px] text-muted-foreground">
              {cropPreview
                ? t('controlPanel.screen.cropSource', {
                    w: cropPreview.rawWidth,
                    h: cropPreview.rawHeight,
                    tw: cropPreview.width,
                    th: cropPreview.height,
                  })
                : ''}
            </span>
          </div>

          <div className="flex items-start gap-3">
            <div className="flex h-[96px] w-[288px] shrink-0 items-center justify-center overflow-hidden rounded border border-border/70 bg-black/40">
              {cropPreview?.dataUrl ? (
                <img
                  src={cropPreview.dataUrl}
                  alt={t('controlPanel.screen.cropTitle')}
                  className="h-full w-full object-contain"
                />
              ) : (
                <span className="text-[11px] text-muted-foreground">
                  {cropLoading
                    ? t('controlPanel.screen.cropLoading')
                    : t('controlPanel.screen.cropEmpty')}
                </span>
              )}
            </div>

            <div className="min-w-0 flex-1 space-y-1.5">
              <Slider
                label={t('controlPanel.screen.cropZoom')}
                value={cropZoom}
                min={100}
                max={400}
                step={5}
                disabled={busy}
                onChange={setCropZoom}
              />
              <Slider
                label={
                  cropPreview?.slackX === 0
                    ? `${t('controlPanel.screen.cropOffsetX')}（${t('controlPanel.screen.cropNoSlack')}）`
                    : t('controlPanel.screen.cropOffsetX')
                }
                value={cropOffX}
                min={-100}
                max={100}
                step={5}
                disabled={busy || cropPreview?.slackX === 0}
                onChange={setCropOffX}
              />
              <Slider
                label={
                  cropPreview?.slackY === 0
                    ? `${t('controlPanel.screen.cropOffsetY')}（${t('controlPanel.screen.cropNoSlack')}）`
                    : t('controlPanel.screen.cropOffsetY')
                }
                value={cropOffY}
                min={-100}
                max={100}
                step={5}
                disabled={busy || cropPreview?.slackY === 0}
                onChange={setCropOffY}
              />
            </div>
          </div>

          <div className="flex flex-wrap items-center justify-end gap-1.5">
            <Button
              variant="ghost"
              size="sm"
              disabled={busy}
              onClick={() => {
                setCropZoom(100);
                setCropOffX(0);
                setCropOffY(0);
              }}
            >
              {t('controlPanel.screen.cropReset')}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              disabled={busy}
              onClick={() => {
                setCropFile('');
                setCropPreview(null);
              }}
            >
              {t('controlPanel.screen.cropCancel')}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              loading={busy}
              disabled={!cropPreview || cropLoading}
              onClick={() => void applyCrop()}
            >
              {t('controlPanel.screen.cropApply')}
            </Button>
          </div>
        </div>
      ) : null}

      {/* 设备当前正在使用的图：回读 CRC 反查预设，自定义图如实标注，并标出读取时间。 */}
      {/* 大卡片容器（参考设置页温度基准）：头部缩略图/标题/描述 + 右侧本地上传按钮。 */}
      <div className="px-5 py-4">
        <div className="rounded-2xl border border-border/70 bg-muted/25 p-4">
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex h-[50px] w-[150px] shrink-0 items-center justify-center overflow-hidden rounded-lg border border-border/60 bg-muted/30">
                {visibleDeviceImg || currentThumb ? (
                  <img src={visibleDeviceImg || currentThumb} alt="" className="h-full w-full object-cover" />
                ) : (
                  <ImageIcon className="h-4 w-4 text-muted-foreground" />
                )}
              </div>
              <div className="min-w-0 space-y-0.5">
                <div className="text-base font-medium text-foreground">
                  {current?.hasImage
                    ? (current.presetMatched
                        ? t('controlPanel.screen.nowPreset', { name: current.presetName ?? '' })
                        : deviceImgFromUpload && deviceImg
                          ? t('controlPanel.screen.nowJustUploaded')
                          : historyCurrent?.thumb
                            ? t('controlPanel.screen.nowFromCache', {
                                time: new Date(historyCurrent.unixTime * 1000).toLocaleString(),
                              })
                            : deviceImgIsCurrent
                              ? t('controlPanel.screen.nowReadBack')
                              : t('controlPanel.screen.nowUnknown'))
                    : t('controlPanel.screen.none')}
                </div>
                {current?.hasImage ? (
                  <>
                    <div className="text-xs text-muted-foreground">
                      {t('controlPanel.screen.deviceNow', {
                        size: current.size,
                        crc: current.crc.toString(16).toUpperCase().padStart(4, '0'),
                      })}
                    </div>
                    {showReadCacheHint ? (
                      <div className="text-[11px] font-medium text-amber-600 dark:text-amber-400">
                        {t('controlPanel.screen.readFromDeviceCached', {
                          time: new Date((deviceImgCachedAt ?? 0) * 1000).toLocaleString(),
                        })}
                      </div>
                    ) : null}
                    {staleReadBack ? (
                      <div className="text-[11px] font-medium text-amber-600 dark:text-amber-400">
                        {t('controlPanel.screen.readFromDeviceStale')}
                      </div>
                    ) : null}
                    {!current.presetMatched ? (
                      <div className="pt-1">
                        <Button
                          variant="secondary"
                          size="sm"
                          loading={readingImg}
                          disabled={busy}
                          onClick={() => void readDeviceImage()}
                        >
                          {t('controlPanel.screen.readFromDevice')}
                        </Button>
                      </div>
                    ) : null}
                  </>
                ) : null}
              </div>
            </div>
            <Button variant="secondary" size="sm" className="shrink-0" disabled={busy} onClick={() => void pickForCrop()}>
              <Upload className="mr-1 h-3.5 w-3.5" />
              {t('controlPanel.screen.local')}
            </Button>
          </div>

          {/* 说明文案置于预设图上方。 */}
          <div className="mt-4 text-[11px] leading-relaxed text-muted-foreground">
            {current?.hasImage && (current.presetMatched || visibleDeviceImg)
              ? historyCurrent?.thumb && !(deviceImgFromUpload && deviceImg)
                ? t('controlPanel.screen.nowCaveatCache')
                : t('controlPanel.screen.nowCaveat')
              : t('controlPanel.screen.hint')}
          </div>


          <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3">
        {presets.map((p) => (
          <button
            key={p.officialPosition}
            type="button"
            disabled={busy}
            onClick={() => void applyPreset(p.officialPosition)}
            className="group overflow-hidden rounded-md border border-border/70 text-left transition hover:border-foreground/40 disabled:opacity-60"
            title={p.name}
          >
            {thumbs[p.officialPosition] ? (
              <img
                src={thumbs[p.officialPosition]}
                alt={p.name}
                className="h-[52px] w-full object-cover"
              />
            ) : (
              <div className="flex h-[52px] w-full items-center justify-center bg-muted/40">
                <ImageIcon className="h-4 w-4 text-muted-foreground" />
              </div>
            )}
            <div className="flex items-center justify-between px-2 py-1">
              <span className="truncate text-[11px] text-foreground">{p.name}</span>
              <span className="text-[10px] text-muted-foreground">{p.officialPosition}</span>
            </div>
          </button>
        ))}

        {/* 历史图片紧跟「预设十」，固定占两格 —— 与预设共用同一个网格，所以它就在那两格里，
            不另起一列。**没有历史时也渲染占位格**：位置固定，不因有没有历史而跳布局。
            这两格是本机缓存（<安装目录>/screen-images/，只记上传成功过屏的图），点一下即重传。 */}
        {[0, 1].map((slot) => {
          const item = history?.items[slot] ?? null;
          if (!item) {
            return (
              <div
                key={`history-slot-${slot}`}
                className="overflow-hidden rounded-md border border-dashed border-border/70"
              >
                <div className="flex h-[52px] w-full items-center justify-center bg-muted/20">
                  <span className="text-[10px] text-muted-foreground">
                    {historyBusy && !history
                      ? t('controlPanel.screen.historyLoading')
                      : t('controlPanel.screen.historySlotEmpty')}
                  </span>
                </div>
                <div className="flex items-center justify-between px-2 py-1">
                  <span className="text-[10px] text-muted-foreground">
                    {t('controlPanel.screen.historySlotLabel')}
                  </span>
                </div>
              </div>
            );
          }
          return (
            <div
              key={item.path}
              className="relative overflow-hidden rounded-md border border-border/70 transition hover:border-foreground/40"
            >
              <button
                type="button"
                disabled={busy}
                onClick={() => void applyHistory(item)}
                className="block w-full text-left disabled:opacity-60"
                title={t('controlPanel.screen.historyApply')}
              >
                {item.thumb ? (
                  <img src={item.thumb} alt={item.name} className="h-[52px] w-full object-cover" />
                ) : (
                  <div className="flex h-[52px] w-full items-center justify-center bg-muted/40">
                    <ImageIcon className="h-4 w-4 text-muted-foreground" />
                  </div>
                )}
                <div className="flex items-center justify-between px-2 py-1">
                  <span className="truncate text-[11px] text-foreground">
                    {new Date(item.unixTime * 1000).toLocaleString()}
                  </span>
                  <span className="text-[10px] text-muted-foreground">
                    {t('controlPanel.screen.historySlotLabel')}
                  </span>
                </div>
              </button>
              {/* 右上角删除：两步确认按钮（第一下变红进入待确认，第二下才真删）。
                  className 由 cn() 里的 twMerge 合并，能盖掉 size=sm 的默认尺寸。 */}
              <ConfirmButton
                label={<X className="h-3 w-3" />}
                confirmLabel={<Trash2 className="h-3 w-3" />}
                onConfirm={() => void deleteHistory(item)}
                disabled={busy}
                variant="ghost"
                className="absolute top-1 right-1 h-5 w-5 bg-background/70 p-0 text-muted-foreground backdrop-blur-sm hover:bg-destructive hover:text-white"
              />
            </div>
          );
        })}
          </div>

          {/* 说明"最后两格"是什么：历史图没有独立标题，靠这一行把它与上面十张预设区分开。 */}
          <div className="mt-2 text-[11px] leading-relaxed text-muted-foreground">
            {history?.error && !history.items.length
              ? t('controlPanel.screen.historyEmptyWithReason', { reason: history.error })
              : t('controlPanel.screen.historySource')}
          </div>
        </div>
      </div>

      {/* 上传期间弹窗：整包上传要几十秒。只有"取消上传"一个动作 —— 它会真的取消这次
          设备独占任务（不是关掉弹窗就完事），所以不提供关闭按钮，免得设备停在半张图。
          进度来自核心的进度事件（设备层每 16 帧推一次）。 */}
      <Dialog open={uploadingAt !== null}>
        <DialogContent hideClose className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t('controlPanel.screen.uploadingTitle')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
              <div
                className="h-full rounded-full bg-primary transition-[width] duration-200"
                style={{ width: `${uploadPercent}%` }}
              />
            </div>
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>
                {uploadProgress && uploadProgress.total > 0
                  ? t('controlPanel.screen.uploadingProgress', {
                      sent: uploadProgress.sent,
                      total: uploadProgress.total,
                    })
                  : ''}
              </span>
              <span>{uploadPercent}%</span>
            </div>
            <Button
              variant="secondary"
              size="sm"
              className="w-full rounded-lg"
              onClick={() => {
                void apiService.cancelScreenImageTransfer();
              }}
            >
              {t('controlPanel.screen.uploadingCancel')}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </Section>
  );
}
