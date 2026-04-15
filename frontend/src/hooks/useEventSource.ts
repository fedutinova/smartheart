import { useEffect, useRef } from 'react';
import { API_BASE_URL } from '@/config';
import { useAuthStore } from '@/store/auth';
import { ensureFreshToken } from '@/services/api';

interface SSEEvent {
  type: string;
  request_id: string;
  status: string;
}

const MAX_RECONNECT_DELAY = 30_000;

/**
 * Subscribes to the SSE event stream. Calls onEvent for each received event.
 * Reconnects with exponential backoff on connection errors, refreshing the
 * access token before each reconnect attempt so a stale JWT doesn't cause
 * an infinite reconnect loop.
 *
 * Pauses the connection when the page is hidden (e.g. phone locked) and
 * reconnects when it becomes visible again, avoiding wasted reconnect
 * attempts and spurious logouts caused by token refresh failures while
 * the device is asleep.
 */
export function useEventSource(onEvent: (evt: SSEEvent) => void) {
  const callbackRef = useRef(onEvent);
  callbackRef.current = onEvent;

  const token = useAuthStore((s) => s.accessToken);

  useEffect(() => {
    if (!token) return;

    let es: EventSource | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let attempt = 0;
    let closed = false;
    let paused = false;

    function disconnect() {
      es?.close();
      es = null;
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
    }

    function connect(currentToken: string) {
      if (closed || paused) return;

      const url = `${API_BASE_URL}/v1/events?token=${encodeURIComponent(currentToken)}`;
      es = new EventSource(url);

      es.onopen = () => {
        attempt = 0;
      };

      es.onmessage = (e) => {
        try {
          const evt: SSEEvent = JSON.parse(e.data);
          callbackRef.current(evt);
        } catch {
          // Ignore malformed events
        }
      };

      es.onerror = () => {
        es?.close();
        es = null;
        if (closed || paused) return;

        const delay = Math.min(1000 * 2 ** attempt, MAX_RECONNECT_DELAY);
        attempt++;
        reconnectTimer = setTimeout(() => {
          // silent=true: a transient network error (phone waking up, flaky
          // connection) should not force-logout the user. The 401 interceptor
          // will handle a genuinely expired session on the next API call.
          ensureFreshToken(true)
            .then((freshToken) => connect(freshToken))
            .catch(() => {
              // Refresh failed — user will be logged out by the next real
              // API call's 401 interceptor if the session is truly gone.
            });
        }, delay);
      };
    }

    function onVisibilityChange() {
      if (document.hidden) {
        paused = true;
        disconnect();
      } else {
        paused = false;
        attempt = 0;
        // Refresh token before reconnecting after wake-up.
        ensureFreshToken(true)
          .then((freshToken) => connect(freshToken))
          .catch(() => {
            // Session gone — the next navigation / API call will redirect.
          });
      }
    }

    document.addEventListener('visibilitychange', onVisibilityChange);
    connect(token);

    return () => {
      closed = true;
      disconnect();
      document.removeEventListener('visibilitychange', onVisibilityChange);
    };
  }, [token]);
}
