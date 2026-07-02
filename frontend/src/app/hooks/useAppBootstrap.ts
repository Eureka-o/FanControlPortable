import { useEffect } from 'react';
import { useAppStore } from '../store/app-store';
import { useGuiHeartbeat } from './useGuiHeartbeat';

export function useAppBootstrap() {
  const initializeApp = useAppStore((state) => state.initializeApp);
  const startEventListeners = useAppStore((state) => state.startEventListeners);
  useGuiHeartbeat();

  useEffect(() => {
    const stopListening = startEventListeners();
    return () => {
      stopListening();
    };
  }, [startEventListeners]);

  useEffect(() => {
    initializeApp();
  }, [initializeApp]);
}
