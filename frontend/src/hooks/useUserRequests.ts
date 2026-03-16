import { useQuery } from '@tanstack/react-query';
import { requestAPI } from '@/services/api';
import type { PaginatedResponse, Request } from '@/types';

const INTERNAL_MARKER = 'Analyze this ECG/EKG image';
const PAGE_SIZE = 20;

function filterInternalRequests(requests: Request[]): Request[] {
  return requests.filter(
    (r) => !r.text_query?.includes(INTERNAL_MARKER)
  );
}

export function useUserRequests(limit = PAGE_SIZE, offset = 0) {
  const query = useQuery<PaginatedResponse<Request>>({
    queryKey: ['requests', limit, offset],
    queryFn: () => requestAPI.getUserRequests(limit, offset),
  });

  const filtered = query.data?.data
    ? filterInternalRequests(query.data.data)
    : [];

  return {
    ...query,
    requests: filtered,
    total: query.data?.total ?? 0,
    pageSize: limit,
  };
}
