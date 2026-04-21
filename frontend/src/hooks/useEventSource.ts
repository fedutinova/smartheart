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
 * Uses fetch with Authorization header (instead of EventSource) to avoid
 * exposing the JWT in the query string, which would be logged and leaked
 * through browser history, nginx logs, proxies, etc.
 *
 * The connection stays open while the tab exists — no disconnect on
 * visibility change, so tab switches don't trigger reconnect noise.
 */
export function useEventSource(onEvent: (evt: SSEEvent) => void) {
  const callbackRef = useRef(onEvent);
  callbackRef.current = onEvent;

  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  useEffect(() => {
    if (!isAuthenticated) return;

    let controller: AbortController | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let attempt = 0;
    let closed = false;

    function disconnect() {
      controller?.abort();
      controller = null;
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
    }

    async function connect(currentToken: string) {
      if (closed) return;

      controller = new AbortController();
      const url = `${API_BASE_URL}/v1/events`;

      try {
        const response = await fetch(url, {
          headers: {
            Authorization: `Bearer ${currentToken}`,
          },
          signal: controller.signal,
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        attempt = 0;

        // Read the SSE stream: format is "data: <json>\n\n"
        const reader = response.body!.getReader();
        const decoder = new TextDecoder();
        let buffer = '';

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });

          // Split on double newline (SSE event separator)
          const events = buffer.split('\n\n');
          // Keep the last incomplete event in the buffer
          buffer = events.pop() ?? '';

          for (const event of events) {
            if (!event.startsWith('data: ')) continue;
            try {
              const jsonStr = event.slice(6); // Remove "data: " prefix
              const evt: SSEEvent = JSON.parse(jsonStr);
              callbackRef.current(evt);
            } catch (e) {
              console.warn('Failed to parse SSE event:', event, e);
            }
          }
        }

        // Connection closed normally, schedule reconnect
        if (!closed) {
          scheduleReconnect();
        }
      } catch (e) {
        if (e instanceof Error && e.name === 'AbortError') {
          // Connection was aborted by cleanup, not an error
          return;
        }

        // Connection failed, schedule reconnect with exponential backoff
        if (!closed) {
          scheduleReconnect();
        }
      }
    }

    function scheduleReconnect() {
      if (closed) return;
      const delay = Math.min(1000 * 2 ** attempt, MAX_RECONNECT_DELAY);
      attempt++;
      reconnectTimer = setTimeout(() => {
        if (closed) return;
        ensureFreshToken(true)
          .then((freshToken) => connect(freshToken))
          .catch(() => {
            // Token refresh failed, will retry on next schedule
            if (!closed) scheduleReconnect();
          });
      }, delay);
    }

    const initialToken = useAuthStore.getState().accessToken;
    if (initialToken) {
      connect(initialToken);
    }

    return () => {
      closed = true;
      disconnect();
    };
  }, [isAuthenticated]);
}
