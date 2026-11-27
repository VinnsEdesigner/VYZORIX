import type { UseQueryResult } from '@tanstack/react-query';

export type QueryViewState = 'loading' | 'error' | 'empty' | 'ready';

export interface QueryView<T> {
  state: QueryViewState;
  data: T | undefined;
  error: unknown;
}

export interface QueryLike<T> {
  data: T | undefined;
  isPending: boolean;
  isError: boolean;
  error: unknown;
}

export function queryView<T>(
  query: QueryLike<T>,
  isEmpty: (data: T | undefined) => boolean = (data) => data === undefined,
): QueryView<T> {
  if (query.isError) {
    return { state: 'error', data: query.data, error: query.error };
  }
  if (query.isPending) {
    return { state: 'loading', data: query.data, error: undefined };
  }
  if (isEmpty(query.data)) {
    return { state: 'empty', data: query.data, error: undefined };
  }

  return { state: 'ready', data: query.data, error: undefined };
}

export function emptyWhenList<T>(data: readonly T[] | undefined): boolean {
  return !data || data.length === 0;
}

export function asQueryLike<T, TError = unknown>(
  query: Pick<UseQueryResult<T, TError>, 'data' | 'isPending' | 'isError' | 'error'>,
): QueryLike<T> {
  return {
    data: query.data,
    isPending: query.isPending,
    isError: query.isError,
    error: query.error,
  };
}

export type SyncPhase = 'idle' | 'running' | 'succeeded' | 'partial' | 'failed';

export interface SyncOutcomeLike {
  state?: string;
  applied?: number;
  failed?: number;
  error?: string;
}

export function syncPhase(job: SyncOutcomeLike | undefined, isRunning: boolean): SyncPhase {
  if (isRunning) return 'running';
  if (!job) return 'idle';
  if (job.state === 'working' || job.state === 'pending') return 'running';
  if (job.state === 'error' || (job.failed ?? 0) > 0) {
    return (job.applied ?? 0) > 0 ? 'partial' : 'failed';
  }
  if (job.state === 'warning') return 'partial';
  if (job.state === 'success') return 'succeeded';

  return 'idle';
}
