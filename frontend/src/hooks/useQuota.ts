import { useQuery } from '@tanstack/react-query';
import { paymentAPI } from '@/services/api';

/**
 * Fetches and caches the user's quota info.
 * Refetches on window focus (throttled by the global focusManager) and
 * every 60 seconds, but only while the page is visible — avoids waking up
 * the network on a locked phone.
 */
export function useQuota() {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['quota'],
    queryFn: () => paymentAPI.getQuota(),
    refetchOnWindowFocus: true,
    refetchInterval: 60_000,
    refetchIntervalInBackground: false,
    staleTime: 30_000,
  });

  return {
    quota: data ?? null,
    isLoading,
    error,
    refetch,
  };
}
