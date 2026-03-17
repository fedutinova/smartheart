import { useState, useCallback, useRef } from 'react';
import { useEventSource } from './useEventSource';
import { ROUTES } from '@/config';
import type { Toast } from '@/components/Toast';

export function useToastNotifications() {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const nextId = useRef(1);

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const onSSEEvent = useCallback((evt: { type: string; request_id: string; status: string }) => {
    if (evt.type === 'request_completed') {
      setToasts((prev) => [
        ...prev,
        {
          id: nextId.current++,
          type: 'success',
          message: 'Анализ завершён',
          link: ROUTES.RESULTS.replace(':id', evt.request_id),
          linkText: 'Посмотреть результат',
        },
      ]);
    } else if (evt.type === 'request_failed') {
      setToasts((prev) => [
        ...prev,
        {
          id: nextId.current++,
          type: 'error',
          message: 'Ошибка при анализе',
          link: ROUTES.RESULTS.replace(':id', evt.request_id),
          linkText: 'Подробнее',
        },
      ]);
    }
  }, []);

  useEventSource(onSSEEvent);

  return { toasts, dismiss };
}
