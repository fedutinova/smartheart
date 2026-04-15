import { QueryClient, focusManager } from '@tanstack/react-query';
import { AxiosError } from 'axios';

// When the user unlocks their phone the browser fires a burst of
// focus / visibilitychange events.  Without throttling, every query
// with refetchOnWindowFocus=true fires at once with a potentially
// stale token, causing a cascade of 401s.
//
// Strategy: always let React Query know about visibility changes (so
// refetchIntervalInBackground works correctly), but throttle the
// "became visible" notification so refetchOnWindowFocus queries only
// fire once per 10-second window.
let lastVisibleTime = 0;
focusManager.setEventListener((handleFocus) => {
  const onVisibility = () => {
    if (document.hidden) {
      // Always notify "hidden" immediately — pauses background intervals.
      handleFocus(false);
    } else {
      const now = Date.now();
      if (now - lastVisibleTime > 10_000) {
        lastVisibleTime = now;
        handleFocus(true);
      }
    }
  };
  const onFocus = () => {
    // Window focus (tab switch) — same throttle as visibility.
    if (!document.hidden) {
      const now = Date.now();
      if (now - lastVisibleTime > 10_000) {
        lastVisibleTime = now;
        handleFocus(true);
      }
    }
  };
  window.addEventListener('visibilitychange', onVisibility);
  window.addEventListener('focus', onFocus);
  return () => {
    window.removeEventListener('visibilitychange', onVisibility);
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
