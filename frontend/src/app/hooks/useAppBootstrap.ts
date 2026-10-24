import { useEffect } from 'react';
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
    void initializeApp();
  }, [initializeApp]);
}
