import { useEffect, useRef } from 'react';
import { API_BASE_URL, JWT_STORAGE_KEY } from '@/config';

interface SSEEvent {
  type: string;
  request_id: string;
  status: string;
}

/**
 * Subscribes to the SSE event stream. Calls onEvent for each received event.
 * Falls back silently on connection error (polling remains as backup).
 *
 * Uses a stable ref for the callback so the SSE connection is not
 * torn down / recreated on every render.
 */
export function useEventSource(onEvent: (evt: SSEEvent) => void) {
  const callbackRef = useRef(onEvent);
  callbackRef.current = onEvent;

  useEffect(() => {
    const token = localStorage.getItem(JWT_STORAGE_KEY);
    if (!token) return;

    // EventSource doesn't support custom headers, so pass token as query param.
    const url = `${API_BASE_URL}/v1/events?token=${encodeURIComponent(token)}`;
    const es = new EventSource(url);

    es.onmessage = (e) => {
      try {
        const evt: SSEEvent = JSON.parse(e.data);
        callbackRef.current(evt);
      } catch {
        // Ignore malformed events
      }
    };

    es.onerror = () => {
      es.close();
    };

    return () => es.close();
    // Only reconnect when token changes, not on every callback change
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
}
