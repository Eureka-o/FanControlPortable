import { useEffect } from 'react';
import { apiService } from '../services/api';

const VISIBLE_GUI_HEARTBEAT_MS = 15000;
const HIDDEN_GUI_HEARTBEAT_MS = 30000;

export function useGuiHeartbeat() {
  useEffect(() => {
    let disposed = false;
    let timer: number | null = null;

    const intervalForVisibility = () => (
      document.hidden ? HIDDEN_GUI_HEARTBEAT_MS : VISIBLE_GUI_HEARTBEAT_MS
    );

    const clearTimer = () => {
      if (timer !== null) {
        window.clearTimeout(timer);
        timer = null;
      }
    };

    const scheduleNext = () => {
      clearTimer();
      if (disposed) return;
      timer = window.setTimeout(pingCore, intervalForVisibility());
    };

    const pingCore = () => {
      if (disposed) return;
      apiService.updateGuiResponseTime()
        .catch(() => {
          // 后端会通过 core-service-error 事件把可见错误同步到状态层。
        })
        .finally(scheduleNext);
    };

    const handleVisibilityChange = () => {
      scheduleNext();
    };

    pingCore();
    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => {
      disposed = true;
      clearTimer();
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, []);
}
