import { useEffect } from 'react';
import { API_BASE_URL, JWT_STORAGE_KEY } from '@/config';

interface SSEEvent {
  type: string;
  request_id: string;
  status: string;
}

/**
 * Subscribes to the SSE event stream. Calls onEvent for each received event.
 * Falls back silently on connection error (polling remains as backup).
 */
export function useEventSource(onEvent: (evt: SSEEvent) => void) {
  useEffect(() => {
    const token = localStorage.getItem(JWT_STORAGE_KEY);
    if (!token) return;

    // EventSource doesn't support custom headers, so pass token as query param.
    // The backend SSE endpoint also accepts ?token= for EventSource compatibility.
    const url = `${API_BASE_URL}/v1/events?token=${encodeURIComponent(token)}`;
    const es = new EventSource(url);

    es.onmessage = (e) => {
      try {
        const evt: SSEEvent = JSON.parse(e.data);
        onEvent(evt);
      } catch {
        // Ignore malformed events
      }
    };

    es.onerror = () => {
      // SSE connection failed — polling will handle updates as fallback
      es.close();
    };

    return () => es.close();
  }, [onEvent]);
}
