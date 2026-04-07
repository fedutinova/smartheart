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

    function connect(currentToken: string) {
      if (closed) return;

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
        if (closed) return;

        const delay = Math.min(1000 * 2 ** attempt, MAX_RECONNECT_DELAY);
        attempt++;
        reconnectTimer = setTimeout(() => {
          // Refresh token before reconnecting — the old one may have expired.
          ensureFreshToken()
            .then((freshToken) => connect(freshToken))
            .catch(() => {
              // Refresh failed — user is logged out, effect will clean up
              // when the token changes to null.
            });
        }, delay);
      };
    }

    connect(token);

    return () => {
      closed = true;
      es?.close();
      if (reconnectTimer) clearTimeout(reconnectTimer);
    };
  }, [token]);
}
