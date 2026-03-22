import { useQuery } from '@tanstack/react-query';
import { paymentAPI } from '@/services/api';

/**
 * Fetches and caches the user's quota info.
 * Refetches on window focus and every 30 seconds.
 */
export function useQuota() {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['quota'],
    queryFn: () => paymentAPI.getQuota(),
    refetchOnWindowFocus: true,
    refetchInterval: 30_000,
    staleTime: 10_000,
  });

  return {
    quota: data ?? null,
    isLoading,
    error,
    refetch,
  };
}
