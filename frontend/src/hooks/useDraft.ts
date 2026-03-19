import { useState, useEffect, useRef, useCallback } from 'react';

const DEBOUNCE_MS = 500;

/**
 * Auto-saves a text draft to sessionStorage with debounce.
 * Restores the draft on mount. Call clear() after successful submit.
 */
export function useDraft(key: string): [string, (value: string) => void, () => void] {
  const [value, setValue] = useState<string>(() => {
    try {
      return sessionStorage.getItem(key) ?? '';
    } catch {
      return '';
    }
  });

  const timerRef = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    timerRef.current = setTimeout(() => {
      try {
        if (value) {
          sessionStorage.setItem(key, value);
        } else {
          sessionStorage.removeItem(key);
        }
      } catch {
        // ignore
      }
    }, DEBOUNCE_MS);

    return () => clearTimeout(timerRef.current);
  }, [key, value]);

  const clear = useCallback(() => {
    setValue('');
    sessionStorage.removeItem(key);
  }, [key]);

  return [value, setValue, clear];
}
