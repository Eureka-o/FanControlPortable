export type BuiltinTab = 'status' | 'curve' | 'control' | 'devices' | 'about';
export type AppTab = BuiltinTab | `plugin:${string}`;
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

export interface BridgeTelemetryLike {
  bridgeOk?: boolean;
  bridgeMessage?: string | null;
  telemetryFailureStage?: string | null;
}

export function isBridgeStartingTelemetry(temp?: BridgeTelemetryLike | null): boolean {
  if (!temp || temp.bridgeOk !== false) {
    return false;
  }
  const stage = temp.telemetryFailureStage?.trim().toLowerCase() ?? '';
  if (stage === 'starting') {
    return true;
  }
  const message = temp.bridgeMessage?.trim() ?? '';
  return message.includes('桥接程序正在启动中');
}

export function resolveBridgeWarning(
  temp?: BridgeTelemetryLike | null,
  fallbackMessage = '',
): string | null {
  if (!temp || temp.bridgeOk !== false || isBridgeStartingTelemetry(temp)) {
    return null;
  }
  return temp.bridgeMessage?.trim() || fallbackMessage || null;
}
