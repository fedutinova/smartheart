import { useState, useEffect, useCallback } from 'react';

/**
 * Like useState, but persists the value in sessionStorage.
 * Survives page refresh, cleared when the tab closes.
 */
export function useSessionState<T>(key: string, initialValue: T): [T, (value: T | ((prev: T) => T)) => void, () => void] {
  const [state, setState] = useState<T>(() => {
    try {
      const stored = sessionStorage.getItem(key);
      return stored !== null ? JSON.parse(stored) : initialValue;
    } catch {
      return initialValue;
    }
  });

  useEffect(() => {
    try {
      sessionStorage.setItem(key, JSON.stringify(state));
    } catch {
      // quota exceeded or private browsing — ignore
    }
  }, [key, state]);

  const clear = useCallback(() => {
    sessionStorage.removeItem(key);
    setState(initialValue);
  }, [key, initialValue]);

  return [state, setState, clear];
}
