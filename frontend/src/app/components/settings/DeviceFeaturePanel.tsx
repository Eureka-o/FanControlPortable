'use client';

import type { ReactNode } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { Clock3, Lightbulb, Monitor, Power, RotateCw, Sparkles, Zap } from 'lucide-react';
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

function GroupHeader({
  icon,
  title,
  description,
}: {
  icon: ReactNode;
  title: string;
  description?: string;
}) {
  return (
    <div className="flex min-w-0 items-start gap-3 bg-muted/10 px-5 py-3">
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
        {icon}
      </div>
      <div className="min-w-0">
        <div className="text-sm font-semibold text-foreground">{title}</div>
        {description && <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{description}</div>}
      </div>
    </div>
  );
}

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
  supportsScreen,
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
  supportsScreen: boolean;
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
  const hasBasicFeatures = supportsGearLight || supportsPowerOnStart || supportsScreen;
  const hasSmartStartStopFeatures = supportsSmartStartStop;
  const deviceName = deviceProfile?.displayName || deviceProfile?.model || t('deviceStatus.device.unknown');

  if (!hasBasicFeatures && !hasSmartStartStopFeatures && !lightingControls && !refreshing && !children) {
    return null;
  }

  return (
    <Section title={t('controlPanel.device.sectionTitle')} icon={Zap}>
      {children}

      <AnimatePresence initial={false}>
        {refreshing && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            className="overflow-hidden"
          >
            <div className="flex items-center gap-2 border-b border-border/60 bg-primary/5 px-5 py-3 text-xs text-primary">
              <RotateCw className="h-3.5 w-3.5 animate-spin" />
              <span>{t('controlPanel.device.contextRefreshing', { device: deviceName })}</span>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {hasBasicFeatures && (
        <>
          {supportsGearLight && (
            <SettingRow
              icon={<Lightbulb className={clsx('h-4 w-4', config.gearLight ? 'text-yellow-500' : '')} />}
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
              />
            </SettingRow>
          )}

          {supportsPowerOnStart && (
            <SettingRow
              icon={<Power className={clsx('h-4 w-4', config.powerOnStart ? 'text-primary' : '')} />}
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
              />
            </SettingRow>
          )}

          {supportsScreen && (
            <SettingRow
              icon={<Monitor className="h-4 w-4" />}
              title={t('controlPanel.device.screenTitle')}
              description={t('controlPanel.device.screenDescription')}
              disabled
            />
          )}
        </>
      )}

      {hasSmartStartStopFeatures && (
        <>
          {supportsSmartStartStop && (
            <>
              <SettingRow
                icon={<Zap className="h-4 w-4" />}
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
                  <motion.div
                    initial={{ opacity: 0, height: 0 }}
                    animate={{ opacity: 1, height: 'auto' }}
                    exit={{ opacity: 0, height: 0 }}
                    className="overflow-hidden"
                  >
                    <CompatibilitySubmenu>
                      <CompatibilitySubmenuRow
                        icon={<Clock3 className="h-4 w-4 text-primary" />}
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
                  </motion.div>
                )}
              </AnimatePresence>
            </>
          )}
        </>
      )}

      {lightingControls && (
        <>
          <GroupHeader
            icon={<Sparkles className="h-4 w-4" />}
            title={t('controlPanel.device.groups.lighting')}
            description={t('controlPanel.device.groups.lightingDescription')}
          />
          <div className="px-5 py-4">{lightingControls}</div>
        </>
      )}
    </Section>
  );
}
