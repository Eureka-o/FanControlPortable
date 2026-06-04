import { useEffect } from 'react';
import { apiService } from '../services/api';
import { useAppStore } from '../store/app-store';

export function useAppBootstrap() {
  const initializeApp = useAppStore((state) => state.initializeApp);
  const startEventListeners = useAppStore((state) => state.startEventListeners);

  useEffect(() => {
    const stopListening = startEventListeners();
    return () => {
      stopListening();
    };
  }, [startEventListeners]);

  useEffect(() => {
    initializeApp();
  }, [initializeApp]);

  useEffect(() => {
    let cancelled = false;
    const pingCore = () => {
      if (cancelled) return;
      apiService.updateGuiResponseTime().catch(() => {
        // 后端会通过 core-service-error 事件把可见错误同步到状态层。
      });
    };

    pingCore();
    const timer = window.setInterval(pingCore, 5000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, []);
}
