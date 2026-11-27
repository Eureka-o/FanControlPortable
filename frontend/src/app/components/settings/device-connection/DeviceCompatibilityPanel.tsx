'use client';

import { AnimatePresence, m } from 'framer-motion';
import { CheckCircle2, ChevronDown, ChevronRight, Usb, Wifi } from 'lucide-react';
import clsx from 'clsx';
import { Input } from '@/components/ui/input';
import { Button, ToggleSwitch } from '../../ui';
import { CompatibilitySubmenuRow } from '../SettingLayout';

type TranslationFn = (key: string, options?: Record<string, unknown>) => string;

interface DeviceCompatibilityPanelProps {
  t: TranslationFn;
  loadingKey: string;
  compatibilityOpen: boolean;
  manualAddOpen: boolean;
  wifiCompatibilityEnabled: boolean;
  wifiConnectionPriorityEnabled: boolean;
  wifiDynamicIPCompatibilityEnabled: boolean;
  serialCompatibilityEnabled: boolean;
  deviceIpInput: string;
  onCompatibilityOpenChange: (next: boolean | ((value: boolean) => boolean)) => void;
  onManualAddOpenChange: (next: boolean | ((value: boolean) => boolean)) => void;
  onDeviceIpInputChange: (value: string) => void;
  onWiFiCompatibilityChange: (enabled: boolean) => void;
  onWiFiConnectionPriorityChange: (enabled: boolean) => void;
  onWiFiDynamicIPCompatibilityChange: (enabled: boolean) => void;
  onSerialCompatibilityChange: (enabled: boolean) => void;
  onManualAdd: () => void;
}

export function DeviceCompatibilityPanel({
  t,
  loadingKey,
  compatibilityOpen,
  manualAddOpen,
  wifiCompatibilityEnabled,
  wifiConnectionPriorityEnabled,
  wifiDynamicIPCompatibilityEnabled,
  serialCompatibilityEnabled,
  deviceIpInput,
  onCompatibilityOpenChange,
  onManualAddOpenChange,
  onDeviceIpInputChange,
  onWiFiCompatibilityChange,
  onWiFiConnectionPriorityChange,
  onWiFiDynamicIPCompatibilityChange,
  onSerialCompatibilityChange,
  onManualAdd,
}: DeviceCompatibilityPanelProps) {
  return (
    <div data-theme-ui="setting-row" className="px-5 py-4 transition-colors duration-200 hover:bg-muted/18">
      <button
        type="button"
        onMouseDown={(event) => event.preventDefault()}
        onClick={() => onCompatibilityOpenChange((value) => !value)}
        className="flex w-full cursor-pointer flex-col gap-4 rounded-xl text-left outline-none focus:outline-none focus-visible:ring-2 focus-visible:ring-ring/50 sm:flex-row sm:items-center sm:justify-between"
      >
        <div className="flex min-w-0 flex-1 items-center gap-3">
          <div data-theme-ui="setting-row-icon" data-icon-tone="purple" className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[11px] bg-muted/70 text-muted-foreground shadow-inner shadow-white/20">
            <Wifi className={clsx('h-4 w-4', (wifiCompatibilityEnabled || serialCompatibilityEnabled) ? 'text-emerald-500' : 'text-primary')} />
          </div>
          <div className="min-w-0">
            <div className="text-[15px] font-semibold tracking-[-0.01em] text-foreground">{t('controlPanel.system.deviceConnection.compatibilityTitle')}</div>
            <div className="mt-0.5 text-[13px] leading-[1.45] text-muted-foreground line-clamp-2">{t('controlPanel.system.deviceConnection.compatibilityDescription')}</div>
          </div>
        </div>
        <ChevronDown className={clsx('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', compatibilityOpen && 'rotate-180')} />
      </button>

      <AnimatePresence initial={false}>
        {compatibilityOpen && (
          <m.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            data-theme-ui="compatibility-divider"
            className="mt-3 overflow-hidden border-t border-border/50"
          >
            <div className="space-y-3 pt-4">
              <CompatibilitySubmenuRow
                icon={<Wifi className={clsx('h-4 w-4', wifiCompatibilityEnabled ? 'text-emerald-500' : 'text-muted-foreground')} />}
                title={t('controlPanel.system.deviceConnection.wifiCompatibilityTitle')}
                description={t('controlPanel.system.deviceConnection.wifiCompatibilityDescription')}
                below={(
                  <AnimatePresence initial={false}>
                    {wifiCompatibilityEnabled && (
                      <m.div
                        initial={{ opacity: 0, height: 0 }}
                        animate={{ opacity: 1, height: 'auto' }}
                        exit={{ opacity: 0, height: 0 }}
                        className="overflow-hidden"
                      >
                        <div data-theme-ui="compatibility-nested" className="mt-3 space-y-2">
                          <div data-theme-ui="compatibility-nested-row" className="rounded-xl border border-border/55 bg-background/35 px-4 py-3">
                            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                              <div className="min-w-0">
                                <div className="text-sm font-medium text-foreground">{t('controlPanel.system.deviceConnection.wifiConnectionPriorityTitle')}</div>
                                <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{t('controlPanel.system.deviceConnection.wifiConnectionPriorityDescription')}</div>
                              </div>
                              <ToggleSwitch
                                enabled={wifiConnectionPriorityEnabled}
                                onChange={(enabled) => void onWiFiConnectionPriorityChange(enabled)}
                                loading={loadingKey === 'wifiConnectionPriority'}
                                size="sm"
                                color="blue"
                              />
                            </div>
                          </div>

                          <div data-theme-ui="compatibility-nested-row" className="rounded-xl border border-border/55 bg-background/35 px-4 py-3">
                            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                              <div className="min-w-0">
                                <div className="text-sm font-medium text-foreground">{t('controlPanel.system.deviceConnection.wifiManualAddTitle')}</div>
                                <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{t('controlPanel.system.deviceConnection.wifiManualAddHint')}</div>
                              </div>
                              <Button
                                variant="outline"
                                size="sm"
                                icon={<ChevronRight className={clsx('h-4 w-4 transition-transform', manualAddOpen && 'rotate-90')} />}
                                onClick={() => onManualAddOpenChange((value) => !value)}
                                className="shrink-0"
                              >
                                {t('controlPanel.system.deviceConnection.wifiManualAddAction')}
                              </Button>
                            </div>

                            <AnimatePresence initial={false}>
                              {manualAddOpen && (
                                <m.div
                                  initial={{ opacity: 0, height: 0 }}
                                  animate={{ opacity: 1, height: 'auto' }}
                                  exit={{ opacity: 0, height: 0 }}
                                  className="overflow-hidden"
                                >
                                  <div data-theme-ui="compatibility-nested-panel" className="mt-3 flex w-full flex-col gap-2 rounded-lg border border-border/50 bg-background/45 p-3 sm:flex-row sm:items-center">
                                    <CheckCircle2 className="hidden h-4 w-4 shrink-0 text-primary sm:block" />
                                    <Input
                                      value={deviceIpInput}
                                      onChange={(event) => onDeviceIpInputChange(event.target.value)}
                                      onKeyDown={(event) => {
                                        if (event.key === 'Enter') void onManualAdd();
                                      }}
                                      placeholder={t('controlPanel.system.deviceConnection.addressPlaceholder')}
                                      aria-label={t('controlPanel.system.deviceConnection.addressPlaceholder')}
                                      className="min-w-0 flex-1"
                                    />
                                    <Button
                                      variant="primary"
                                      size="sm"
                                      onClick={() => void onManualAdd()}
                                      loading={loadingKey === 'manualAdd'}
                                      className="shrink-0"
                                    >
                                      {t('controlPanel.system.deviceConnection.wifiManualSaveAction')}
                                    </Button>
                                  </div>
                                </m.div>
                              )}
                            </AnimatePresence>
                          </div>

                          <div data-theme-ui="compatibility-nested-row" className="rounded-xl border border-border/55 bg-background/35 px-4 py-3">
                            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                              <div className="min-w-0">
                                <div className="text-sm font-medium text-foreground">{t('controlPanel.system.deviceConnection.wifiDynamicIPTitle')}</div>
                                <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{t('controlPanel.system.deviceConnection.wifiDynamicIPDescription')}</div>
                              </div>
                              <ToggleSwitch
                                enabled={wifiDynamicIPCompatibilityEnabled}
                                onChange={(enabled) => void onWiFiDynamicIPCompatibilityChange(enabled)}
                                loading={loadingKey === 'wifiDynamicIPCompatibility'}
                                size="sm"
                                color="blue"
                              />
                            </div>
                          </div>
                        </div>
                      </m.div>
                    )}
                  </AnimatePresence>
                )}
              >
                <ToggleSwitch
                  enabled={wifiCompatibilityEnabled}
                  onChange={(enabled) => void onWiFiCompatibilityChange(enabled)}
                  loading={loadingKey === 'wifiCompatibility'}
                  size="sm"
                  color="blue"
                />
              </CompatibilitySubmenuRow>

              <CompatibilitySubmenuRow
                icon={<Usb className={clsx('h-4 w-4', serialCompatibilityEnabled ? 'text-emerald-500' : 'text-muted-foreground')} />}
                title={t('controlPanel.system.deviceConnection.serialCompatibilityTitle')}
                description={t('controlPanel.system.deviceConnection.serialCompatibilityDescription')}
              >
                <ToggleSwitch
                  enabled={serialCompatibilityEnabled}
                  onChange={(enabled) => void onSerialCompatibilityChange(enabled)}
                  loading={loadingKey === 'serialCompatibility'}
                  size="sm"
                  color="blue"
                />
              </CompatibilitySubmenuRow>
            </div>
          </m.div>
        )}
      </AnimatePresence>
    </div>
  );
}
