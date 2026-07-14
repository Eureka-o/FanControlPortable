export type AppTab = 'status' | 'curve' | 'control' | 'devices' | 'about';
export type TimelineEventType = 'disconnect' | 'reconnect' | 'resume' | 'profile';

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
