import { useEffect } from 'react';
import { apiService } from '../services/api';
import { useAppStore } from '../store/app-store';

export function useAppBootstrap() {
  const initializeApp = useAppStore((state) => state.initializeApp);
  const startEventListeners = useAppStore((state) => state.startEventListeners);

  useEffect(() => {
    let stopListening = () => {};
    let disposed = false;
    stopListening = startEventListeners();
    return () => {
      disposed = true;
      stopListening();
    };
  }, [startEventListeners]);

  useEffect(() => {
    let wasHidden = typeof document !== 'undefined' && document.hidden;

    const applyHiddenState = (hidden: boolean, historyDirty = false) => {
      if (typeof document === 'undefined') {
        return;
      }
      const root = document.documentElement;
      if (hidden) {
        root.dataset.windowHidden = 'true';
      } else {
        delete root.dataset.windowHidden;
      }
      void apiService.setWindowHidden(hidden);
      if (wasHidden && !hidden) {
        const store = useAppStore.getState();
        if (historyDirty || store.temperatureHistoryEnabled) {
          void store.loadTemperatureHistory(true).catch(() => undefined);
        }
      }
      wasHidden = hidden;
    };

    const handleVisibilityChange = () => {
      applyHiddenState(document.hidden);
    };

    if (typeof document !== 'undefined') {
      applyHiddenState(document.hidden);
      document.addEventListener('visibilitychange', handleVisibilityChange);
    }

    const unsubscribeVisibility = apiService.onWindowVisibilityChange((payload) => {
      if (typeof payload === 'boolean') {
        applyHiddenState(!payload);
        return;
      }
      const visible = payload?.visible !== false;
      applyHiddenState(!visible, payload?.historyDirty === true);
    });

    return () => {
      if (typeof document !== 'undefined') {
        document.removeEventListener('visibilitychange', handleVisibilityChange);
        delete document.documentElement.dataset.windowHidden;
      }
      unsubscribeVisibility();
    };
  }, []);

  useEffect(() => {
    void initializeApp();
  }, [initializeApp]);
}
