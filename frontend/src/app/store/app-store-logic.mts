export type BuiltinAppTab = 'status' | 'curve' | 'control' | 'devices' | 'about';
export type PluginAppTab = `plugin:${string}:${string}`;
export type AppTab = BuiltinAppTab | PluginAppTab;
export type TimelineEventType = 'disconnect' | 'reconnect' | 'resume' | 'profile';

export function pluginAppTab(pluginId: string, pageId: string): PluginAppTab {
  return `plugin:${pluginId}:${pageId}`;
}

export function isPluginAppTab(tab: string): tab is PluginAppTab {
  const parts = tab.split(':');
  return parts.length === 3 && parts[0] === 'plugin' && parts[1].length > 0 && parts[2].length > 0;
}

export interface TimelineEvent {
  timestamp: number;
  type: TimelineEventType;
}

export function appendTimelineEvent(events: TimelineEvent[], event: TimelineEvent) {
  const previous = events.at(-1);
  if (previous?.type === event.type && Math.abs(event.timestamp - previous.timestamp) < 1_500) {
    return events;
  }
  return [...events, event].slice(-100);
}

export interface TabNavigationState {
  activeTab: AppTab;
  curveDraftDirty: boolean;
  pendingTab: AppTab | null;
}

export function requestTabChange(state: TabNavigationState, target: AppTab): TabNavigationState {
  if (state.activeTab === 'curve' && target !== 'curve' && state.curveDraftDirty) {
    return { ...state, pendingTab: target };
  }
  return { ...state, activeTab: target, pendingTab: null };
}

export function completePendingTabChange(state: TabNavigationState): TabNavigationState {
  return {
    activeTab: state.pendingTab ?? state.activeTab,
    curveDraftDirty: false,
    pendingTab: null,
  };
}

export function cancelPendingTabChange(state: TabNavigationState): TabNavigationState {
  return { ...state, pendingTab: null };
}

export class LatestRequestGate {
  private generation = 0;

  begin() {
    this.generation += 1;
    return this.generation;
  }

  invalidate() {
    this.generation += 1;
  }

  isCurrent(generation: number) {
    return generation === this.generation;
  }
}
