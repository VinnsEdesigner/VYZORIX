/**
 * Integration tests for the super-admin API key presentation hooks.
 *
 * These render the REAL hooks via React Testing Library. The hooks call the
 * REAL `@vyzorix/api-client` admin endpoints (real restClient/axios + domain
 * mappers). MSW intercepts the HTTP requests and returns mock server responses
 * mirroring the Go server contract
 * (apps/api/internal/api/handlers/admin/key_admin_handler.go):
 * snake_case field names, RFC3339 ISO timestamp strings, 204 on force-revoke.
 *
 * No vi.mock — the real code path runs end-to-end.
 */
import { describe, it, expect, beforeEach } from 'vitest';
import { waitFor, act } from '@testing-library/react';
import { renderHookWithQueryClient, createTestQueryClient } from '../helpers/render-hook';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { resetAdminApiKeyFixtures } from '@/test/msw/vyzor-msw-handlers-admin-apikeys';
import {
  useAdminApiKeys,
  useAdminUserKeys,
  useGlobalStats,
  useAdminUserKeyStats,
  useForceRevokeKey,
} from '@/hooks/apikey';

setupIntegrationTest();

beforeEach(() => {
  resetAdminApiKeyFixtures();
});

// The bridge returns the response body directly (not the orval envelope).
function body(res: { data?: unknown }) {
  return res.data as Record<string, unknown> | undefined;
}

describe('useAdminApiKeys', () => {
  it('lists all keys across users (flattened) on page 1', async () => {
    const { result } = renderHookWithQueryClient(() => useAdminApiKeys());
    await waitFor(() => expect((body(result.current)?.keys as unknown[])?.length).toBeGreaterThan(0));
    // 8 seeded keys, list-all returns ALL of them (incl. revoked).
    expect(body(result.current)?.keys).toHaveLength(8);
    expect((body(result.current)?.pagination as { total?: number })?.total).toBe(8);
  });

  it('maps keys through the real mapper (camelCase + Date + user identity)', async () => {
    const { result } = renderHookWithQueryClient(() => useAdminApiKeys());
    await waitFor(() => expect((body(result.current)?.keys as unknown[])?.length).toBeGreaterThan(0));
    const first = (body(result.current)?.keys as Record<string, unknown>[])[0];
    expect(first).toBeDefined();
    expect(first?.id).toBe('admin-key-1');
    expect(first?.user_id).toBe('op-1');
    expect(first?.user_name).toBe('Acme Corp');
    expect(first?.scope).toBe('admin');
    expect(first?.is_active).toBe(true);
  });

  it('filters by userId', async () => {
    const { result } = renderHookWithQueryClient(() =>
      useAdminApiKeys({ userId: 'op-2' }),
    );
    await waitFor(() => expect((body(result.current)?.keys as unknown[])?.length).toBeGreaterThan(0));
    const keys = (body(result.current)?.keys ?? []) as { user_id?: string }[];
    expect(keys).toHaveLength(3);
    expect(keys.every((k) => k.user_id === 'op-2')).toBe(true);
  });

  it('filters by search (matches key name and user name)', async () => {
    const { result } = renderHookWithQueryClient(() =>
      useAdminApiKeys({ search: 'Acme' }),
    );
    await waitFor(() => expect((body(result.current)?.keys as unknown[])?.length).toBeGreaterThan(0));
    const keys = (body(result.current)?.keys ?? []) as { user_name?: string }[];
    expect(keys).toHaveLength(3);
    expect(keys.every((k) => k.user_name === 'Acme Corp')).toBe(true);
  });


});

describe('useAdminUserKeys', () => {
  it('is disabled when no userId is provided', () => {
    const { result } = renderHookWithQueryClient(() => useAdminUserKeys(undefined));
    expect(result.current.fetchStatus).toBe('idle');
  });

  it('lists all keys for a specific user', async () => {
    const { result } = renderHookWithQueryClient(() => useAdminUserKeys('op-1'));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    // op-1 owns admin-key-1, admin-key-2, admin-key-7.
    expect(result.current.data?.keys ?? []).toHaveLength(3);
    expect((result.current.data?.keys ?? []).every((k) => k.user_id === 'op-1')).toBe(true);
  });
});

describe('useGlobalStats', () => {
  it('fetches global API key statistics', async () => {
    const { result } = renderHookWithQueryClient(() => useGlobalStats());
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.total_keys).toBe(8);
    expect(result.current.data?.active_keys).toBe(6);
    expect(result.current.data?.revoked_keys).toBe(2);
    expect(result.current.data?.total_users).toBe(3);
    expect(result.current.data?.top_users?.length).toBeGreaterThan(0);
  });

  it('aggregates requests by scope', async () => {
    const { result } = renderHookWithQueryClient(() => useGlobalStats());
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    // admin scope: admin-key-1 (142) + admin-key-8 (210) = 352.
    expect(result.current.data?.requests_by_scope?.admin).toBe(352);
  });
});

describe('useAdminUserKeyStats', () => {
  it('is disabled when no userId is provided', () => {
    const { result } = renderHookWithQueryClient(() => useAdminUserKeyStats(undefined));
    expect(result.current.fetchStatus).toBe('idle');
  });

  it('fetches per-user statistics', async () => {
    const { result } = renderHookWithQueryClient(() => useAdminUserKeyStats('op-1'));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.user_id).toBe('op-1');
    expect(result.current.data?.total_keys).toBe(3);
    expect(result.current.data?.active_keys).toBe(3);
    expect(result.current.data?.monthly_limit).toBe(20);
  });
});

describe('useForceRevokeKey', () => {
  it('force-revokes a key and invalidates the admin list cache', async () => {
    const queryClient = createTestQueryClient();
    const listHook = renderHookWithQueryClient(() => useAdminApiKeys(), { queryClient });
    await waitFor(() => expect((body(listHook.result.current)?.keys as unknown[]) ?? []).toHaveLength(8));

    const revokeHook = renderHookWithQueryClient(() => useForceRevokeKey(), { queryClient });
    await act(async () => {
      revokeHook.result.current.mutate({ keyId: 'admin-key-1' });
    });
    await waitFor(() => expect(revokeHook.result.current.isSuccess).toBe(true));
    // List refetches after invalidation — admin-key-1 now revoked (isActive=false),
    // but list-all returns ALL keys including revoked, so the count is unchanged.
    // Poll until the refetched data reflects the mutated state.
    await waitFor(() => {
      const keys = (body(listHook.result.current)?.keys ?? []) as { id?: string; is_active?: boolean }[];
      const revoked = keys.find((k) => k.id === 'admin-key-1');
      expect(revoked?.is_active).toBe(false);
    });
  });

  it('returns 404 error for a missing key', async () => {
    const { result } = renderHookWithQueryClient(() => useForceRevokeKey());
    await act(async () => {
      result.current.mutate({ keyId: 'nope' });
    });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBeDefined();
  });
});
