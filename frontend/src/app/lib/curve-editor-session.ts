export type CurveEditorSession = {
  dirty: boolean;
  pendingProfileId: string;
  pendingTab: boolean;
};

export const createCurveEditorSession = (): CurveEditorSession => ({
  dirty: false,
  pendingProfileId: '',
  pendingTab: false,
});

export const markCurveEditorDirty = (session: CurveEditorSession): CurveEditorSession => (
  session.dirty ? session : { ...session, dirty: true }
);

export const markCurveEditorSaved = (session: CurveEditorSession, succeeded: boolean): CurveEditorSession => (
  !succeeded || !session.dirty ? session : { ...session, dirty: false }
);

export function requestCurveProfileSwitch(session: CurveEditorSession, profileId: string, activeProfileId: string) {
  if (!profileId || profileId === activeProfileId) {
    return { session, requiresConfirmation: false, nextProfileId: '' };
  }
  if (!session.dirty) {
    return { session, requiresConfirmation: false, nextProfileId: profileId };
  }
  return {
    session: { ...session, pendingProfileId: profileId, pendingTab: false },
    requiresConfirmation: true,
    nextProfileId: '',
  };
}

export function requestCurveTabSwitch(session: CurveEditorSession) {
  if (!session.dirty) {
    return { session, requiresConfirmation: false, completeTab: true };
  }
  return {
    session: { ...session, pendingProfileId: '', pendingTab: true },
    requiresConfirmation: true,
    completeTab: false,
  };
}

export const cancelCurveEditorSwitch = (session: CurveEditorSession): CurveEditorSession => (
  !session.pendingProfileId && !session.pendingTab
    ? session
    : { ...session, pendingProfileId: '', pendingTab: false }
);

export function resolveCurveEditorSwitch(
  session: CurveEditorSession,
  action: 'save' | 'discard',
  saveSucceeded: boolean,
) {
  if (action === 'save' && !saveSucceeded) {
    return { session, nextProfileId: '', completeTab: false };
  }
  return {
    session: createCurveEditorSession(),
    nextProfileId: session.pendingProfileId,
    completeTab: session.pendingTab,
  };
}
