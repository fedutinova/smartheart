import { useSessionState } from './useSessionState';
import { useCallback } from 'react';

export interface PendingJob {
  requestId: string;
  submittedAt: string;
}

const KEY = 'pending_jobs';

/**
 * Tracks submitted jobs that haven't completed yet.
 * Persists across refresh so the user can resume watching results.
 */
export function usePendingJobs() {
  const [jobs, setJobs] = useSessionState<PendingJob[]>(KEY, []);

  const addJob = useCallback((requestId: string) => {
    setJobs((prev) => [
      ...prev.filter((j) => j.requestId !== requestId),
      { requestId, submittedAt: new Date().toISOString() },
    ]);
  }, [setJobs]);

  const removeJob = useCallback((requestId: string) => {
    setJobs((prev) => prev.filter((j) => j.requestId !== requestId));
  }, [setJobs]);

  return { jobs, addJob, removeJob };
}
