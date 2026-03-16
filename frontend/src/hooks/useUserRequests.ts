import { useQuery } from '@tanstack/react-query';
import { requestAPI } from '@/services/api';
import type { PaginatedResponse, Request } from '@/types';

const INTERNAL_MARKER = 'Analyze this ECG/EKG image';

function filterInternalRequests(requests: Request[]): Request[] {
  return requests.filter(
    (r) => !r.text_query?.includes(INTERNAL_MARKER)
  );
}

export function useUserRequests() {
  const query = useQuery<PaginatedResponse<Request>>({
    queryKey: ['requests'],
    queryFn: () => requestAPI.getUserRequests(),
  });

  const filtered = query.data?.data
    ? filterInternalRequests(query.data.data)
    : [];

  return {
    ...query,
    requests: filtered,
  };
}
