import { useEffect, useRef } from 'react';
import { API_BASE_URL } from '@/config';
import { useAuthStore } from '@/store/auth';

interface SSEEvent {
  type: string;
  request_id: string;
  status: string;
}

const MAX_RECONNECT_DELAY = 30_000;

/**
 * Subscribes to the SSE event stream. Calls onEvent for each received event.
 * Reconnects with exponential backoff on connection errors.
 * Re-establishes the connection when the access token changes.
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

    function connect() {
      if (closed) return;

      const url = `${API_BASE_URL}/v1/events?token=${encodeURIComponent(token!)}`;
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
        reconnectTimer = setTimeout(connect, delay);
      };
    }

    connect();

    return () => {
      closed = true;
      es?.close();
      if (reconnectTimer) clearTimeout(reconnectTimer);
    };
  }, [token]);
}
