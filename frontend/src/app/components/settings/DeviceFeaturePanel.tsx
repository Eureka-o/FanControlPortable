'use client';

import type { ReactNode } from 'react';
import { AnimatePresence, m } from 'framer-motion';
import { Clock3, Lightbulb, Power, RotateCw, Sparkles, Zap } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { types } from '../../../../wailsjs/go/models';
import { Select, ToggleSwitch } from '../ui';
import { CompatibilitySubmenu, CompatibilitySubmenuRow, Section, SettingRow } from './SettingLayout';
import { normalizeWiFiSmartStartStopStandbySpeed } from './device-connection-utils';

type SelectOption<T extends string | number> = {
  value: T;
  label: string;
  description?: string;
  disabled?: boolean;
};

export default function DeviceFeaturePanel({
  config,
  isConnected,
  refreshing = false,
  deviceProfile,
  loadingStates,
  supportsGearLight,
  supportsPowerOnStart,
  supportsSmartStartStop,
  supportsWiFiSmartStartStopStandbySpeed,
  lightingControls,
  smartStartStopOptions,
  wifiSmartStartStopStandbySpeedOptions,
  onGearLightChange,
  onPowerOnStartChange,
  onSmartStartStopChange,
  onWiFiSmartStartStopStandbySpeedChange,
  children,
}: {
  config: types.AppConfig;
  isConnected: boolean;
  refreshing?: boolean;
  deviceProfile?: types.DeviceProfile | null;
  loadingStates: Record<string, boolean>;
  supportsGearLight: boolean;
  supportsPowerOnStart: boolean;
  supportsSmartStartStop: boolean;
  supportsWiFiSmartStartStopStandbySpeed: boolean;
  lightingControls?: ReactNode;
  smartStartStopOptions: SelectOption<string>[];
  wifiSmartStartStopStandbySpeedOptions: SelectOption<number>[];
  onGearLightChange: (enabled: boolean) => void;
  onPowerOnStartChange: (enabled: boolean) => void;
  onSmartStartStopChange: (mode: string) => void;
  onWiFiSmartStartStopStandbySpeedChange: (value: string | number) => void;
  children?: ReactNode;
}) {
  const { t } = useTranslation();
  const hasBasicFeatures = supportsGearLight || supportsPowerOnStart;
  const hasSmartStartStopFeatures = supportsSmartStartStop;
  const deviceName = deviceProfile?.displayName || deviceProfile?.model || t('deviceStatus.device.unknown');

  const hasPrimaryContent = Boolean(children || refreshing || hasBasicFeatures || hasSmartStartStopFeatures);

  if (!hasPrimaryContent && !lightingControls) {
    return null;
  }

  return (
    <div className="space-y-5">
      {hasPrimaryContent && (
        <Section title={t('controlPanel.device.sectionTitle')} icon={Zap} iconTone="blue">
          {children}

      <AnimatePresence initial={false}>
        {refreshing && (
          <m.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            className="overflow-hidden"
          >
            <div className="flex items-center gap-2 border-b border-border/60 bg-primary/5 px-5 py-3 text-xs text-primary">
              <RotateCw className="h-3.5 w-3.5 animate-spin" />
              <span>{t('controlPanel.device.contextRefreshing', { device: deviceName })}</span>
            </div>
          </m.div>
        )}
      </AnimatePresence>

      {hasBasicFeatures && (
        <>
          {supportsGearLight && (
            <SettingRow
              icon={<Lightbulb className={clsx('h-4 w-4', config.gearLight ? 'text-yellow-500' : '')} />}
              iconTone="amber"
              title={t('controlPanel.device.gearLightTitle')}
              description={t('controlPanel.device.gearLightDescription')}
              disabled={!isConnected}
            >
              <ToggleSwitch
                enabled={config.gearLight}
                onChange={onGearLightChange}
                disabled={!isConnected}
                loading={loadingStates.gearLight}
                size="sm"
                color="blue"
              />
            </SettingRow>
          )}

          {supportsPowerOnStart && (
            <SettingRow
              icon={<Power className={clsx('h-4 w-4', config.powerOnStart ? 'text-primary' : '')} />}
              iconTone="blue"
              title={t('controlPanel.device.powerOnStartTitle')}
              description={t('controlPanel.device.powerOnStartDescription')}
              disabled={!isConnected}
            >
              <ToggleSwitch
                enabled={config.powerOnStart}
                onChange={onPowerOnStartChange}
                disabled={!isConnected}
                loading={loadingStates.powerOnStart}
                size="sm"
                color="blue"
              />
            </SettingRow>
          )}

        </>
      )}

      {hasSmartStartStopFeatures && (
        <>
          {supportsSmartStartStop && (
            <>
              <SettingRow
                icon={<Zap className="h-4 w-4" />}
                iconTone="emerald"
                title={t('controlPanel.device.smartStartStopTitle')}
                description={t('controlPanel.device.smartStartStopDescription')}
                disabled={!isConnected}
              >
                <div className="w-full sm:w-40">
                  <Select
                    value={config.smartStartStop || 'off'}
                    onChange={onSmartStartStopChange}
                    options={smartStartStopOptions}
                    disabled={!isConnected}
                    size="sm"
                    className="w-full"
                  />
                </div>
              </SettingRow>
              <AnimatePresence initial={false}>
                {supportsWiFiSmartStartStopStandbySpeed && (config.smartStartStop || 'off') === 'delayed' && (
                  <m.div
                    initial={{ opacity: 0, height: 0 }}
                    animate={{ opacity: 1, height: 'auto' }}
                    exit={{ opacity: 0, height: 0 }}
                    className="overflow-hidden"
                  >
                    <CompatibilitySubmenu>
                      <CompatibilitySubmenuRow
                        icon={<Clock3 className="h-4 w-4 text-primary" />}
                        iconTone="emerald"
                        title={t('controlPanel.device.wifiSmartStartStopStandbySpeed')}
                        description={t('controlPanel.device.wifiSmartStartStopStandbySpeedDescription')}
                      >
                        <div className="w-full md:w-32">
                          <Select
                            value={normalizeWiFiSmartStartStopStandbySpeed((config as any).wifiSmartStartStopStandbySpeed)}
                            onChange={onWiFiSmartStartStopStandbySpeedChange}
                            options={wifiSmartStartStopStandbySpeedOptions}
                            disabled={!isConnected || loadingStates.wifiSmartStartStopStandbySpeed}
                            size="sm"
                            className="w-full"
                          />
                        </div>
                      </CompatibilitySubmenuRow>
                    </CompatibilitySubmenu>
                  </m.div>
                )}
              </AnimatePresence>
            </>
          )}
        </>
      )}

        </Section>
      )}

      {lightingControls && (
        <Section
          title={t('controlPanel.device.groups.lighting')}
          description={t('controlPanel.device.groups.lightingDescription')}
          icon={Sparkles}
          iconTone="purple"
        >
          <div className="p-5">{lightingControls}</div>
        </Section>
      )}
    </div>
  );
}
