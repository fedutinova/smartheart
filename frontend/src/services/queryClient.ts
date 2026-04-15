import { QueryClient, focusManager } from '@tanstack/react-query';
import { AxiosError } from 'axios';

// Throttle focus-triggered refetches: when the user unlocks their phone,
// the browser fires a burst of focus/visibilitychange events.  Without
// throttling, every query with refetchOnWindowFocus=true fires at once
// with a potentially stale token, causing a cascade of 401s and logouts.
// Only report "focused" at most once per 10 seconds.
let lastFocusTime = 0;
focusManager.setEventListener((handleFocus) => {
  const onFocus = () => {
    const now = Date.now();
    if (now - lastFocusTime > 10_000) {
      lastFocusTime = now;
      handleFocus();
    }
  };
  window.addEventListener('visibilitychange', onFocus);
  window.addEventListener('focus', onFocus);
  return () => {
    window.removeEventListener('visibilitychange', onFocus);
    window.removeEventListener('focus', onFocus);
  };
});

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: (failureCount, error) => {
        if (error instanceof AxiosError && error.response?.status === 401) return false;
        return failureCount < 1;
      },
    },
  },
});
