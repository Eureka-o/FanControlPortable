import { Fan, Gauge, Laptop, Plug, Settings, Thermometer, Zap } from 'lucide-react';
import type { PluginIconName } from './plugin-host-types';

export const PLUGIN_ICONS = Object.freeze({
  fan: Fan,
  gauge: Gauge,
  laptop: Laptop,
  plug: Plug,
  settings: Settings,
  thermometer: Thermometer,
  zap: Zap,
}) satisfies Readonly<Record<PluginIconName, typeof Fan>>;
