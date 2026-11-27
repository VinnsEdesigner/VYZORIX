import { describe, it, expect } from 'vitest';
import { emptyWhenList, queryView, syncPhase, type QueryLike } from '@/lib/provisioning/query-view';

function query<T>(partial: Partial<QueryLike<T>>): QueryLike<T> {
  return { data: undefined, isPending: false, isError: false, error: undefined, ...partial };
}

describe('queryView', () => {
  it('reports loading while the first fetch is in flight', () => {
    expect(queryView(query({ isPending: true })).state).toBe('loading');
  });

  it('reports error when the query settled with a failure', () => {
    expect(queryView(query({ isError: true, error: new Error('boom') })).state).toBe('error');
  });

  it('lets an error outrank loading, because a failed query has already settled', () => {
    expect(queryView(query({ isError: true, isPending: true })).state).toBe('error');
  });

  it('reports empty when settled with no data', () => {
    expect(queryView(query({})).state).toBe('empty');
  });

  it('reports ready when data is present', () => {
    expect(queryView(query({ data: { total: 1 } })).state).toBe('ready');
  });

  it('uses a caller predicate for emptiness', () => {
    const isEmpty = (data: number[] | undefined) => !data || data.length === 0;

    expect(queryView(query({ data: [] as number[] }), isEmpty).state).toBe('empty');
    expect(queryView(query({ data: [1] }), isEmpty).state).toBe('ready');
  });

  it('does not call an empty array empty without the list predicate', () => {
    expect(queryView(query({ data: [] as number[] })).state).toBe('ready');
  });

  it('passes the error through so the caller can describe it', () => {
    const error = new Error('the remote is unreachable');
    expect(queryView(query({ isError: true, error })).error).toBe(error);
  });
});

describe('emptyWhenList', () => {
  it('treats undefined and an empty array as empty', () => {
    expect(emptyWhenList(undefined)).toBe(true);
    expect(emptyWhenList([])).toBe(true);
  });

  it('treats a populated array as non-empty', () => {
    expect(emptyWhenList([1])).toBe(false);
  });
});

describe('syncPhase', () => {
  it('is idle before anything ran', () => {
    expect(syncPhase(undefined, false)).toBe('idle');
  });

  it('is running while the mutation is in flight', () => {
    expect(syncPhase(undefined, true)).toBe('running');
  });

  it('is running while the server still reports the job working', () => {
    expect(syncPhase({ state: 'working' }, false)).toBe('running');
    expect(syncPhase({ state: 'pending' }, false)).toBe('running');
  });

  it('is succeeded only on a clean success', () => {
    expect(syncPhase({ state: 'success', applied: 3, failed: 0 }, false)).toBe('succeeded');
  });

  it('is partial when something applied and something failed', () => {
    expect(syncPhase({ state: 'warning', applied: 2, failed: 1 }, false)).toBe('partial');
    expect(syncPhase({ state: 'error', applied: 2, failed: 1 }, false)).toBe('partial');
  });

  it('is failed when nothing applied and the run failed', () => {
    expect(syncPhase({ state: 'error', applied: 0, failed: 2 }, false)).toBe('failed');
    expect(syncPhase({ state: 'error', applied: 0, failed: 0 }, false)).toBe('failed');
  });

  it('never calls a run with failures succeeded', () => {
    expect(syncPhase({ state: 'success', applied: 1, failed: 1 }, false)).not.toBe('succeeded');
  });
});
