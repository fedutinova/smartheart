import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import { vi } from 'vitest';
import type { QuotaInfo } from '@/types';
import { useQuota } from './useQuota';

const { mockGetQuota } = vi.hoisted(() => ({
  mockGetQuota: vi.fn(),
}));

vi.mock('@/services/api', () => ({
  paymentAPI: {
    getQuota: mockGetQuota,
  },
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);
}

describe('useQuota', () => {
  beforeEach(() => {
    mockGetQuota.mockReset();
  });

  it('returns quota data on success', async () => {
    const quotaData: QuotaInfo = {
      daily_limit: 50,
      used_today: 45,
      free_remaining: 5,
      paid_analyses_remaining: 10,
      needs_payment: false,
      price_per_analysis_kopecks: 4900,
      subscription_price_kopecks: 199900,
    };
    mockGetQuota.mockResolvedValue(quotaData);

    const { result } = renderHook(() => useQuota(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.quota).toEqual(quotaData);
    });
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('returns null and error on failure', async () => {
    const error = new Error('Failed to fetch quota');
    mockGetQuota.mockRejectedValue(error);

    const { result } = renderHook(() => useQuota(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.quota).toBeNull();
      expect(result.current.error).toBeDefined();
      expect(result.current.isLoading).toBe(false);
    });
  });

  it('refetches quota on demand via refetch', async () => {
    const quotaData: QuotaInfo = {
      daily_limit: 50,
      used_today: 30,
      free_remaining: 20,
      paid_analyses_remaining: 15,
      needs_payment: false,
      price_per_analysis_kopecks: 4900,
      subscription_price_kopecks: 199900,
    };
    mockGetQuota.mockResolvedValue(quotaData);

    const { result } = renderHook(() => useQuota(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.quota).toEqual(quotaData);
    });

    // Call refetch
    result.current.refetch();

    await waitFor(() => {
      expect(mockGetQuota).toHaveBeenCalledTimes(2);
    });
  });
});
