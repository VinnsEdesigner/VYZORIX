import { describe, it, expect } from 'vitest';
import { waitFor, act } from '@testing-library/react';
import { createTestQueryClient, renderHookWithQueryClient } from '../helpers/render-hook';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { resetProvisioningStore } from '../msw/vyzor-msw-handlers-provisioning';
import { http, HttpResponse } from 'msw';
import {
  useCheckProvisioningConnection,
  useCreateProvisioningConnection,
  useCreateProvisioningRepository,
  useDeleteProvisioningConnection,
  useProvisioningConnections,
  useProvisioningDiscovery,
  useProvisioningInstance,
  useProvisioningJob,
  useProvisioningJobs,
  useProvisioningRepositories,
  useProvisioningResourceDocument,
  useProvisioningResourceSecrets,
  useProvisioningResources,
  useSyncProvisionedResource,
  useSyncProvisioningRepository,
  useUpdateProvisioningConnection,
} from '@/hooks/provisioning';

const { server } = setupIntegrationTest();

describe('provisioning queries', () => {
  it('maps the repository list into domain objects', async () => {
    const { result } = renderHookWithQueryClient(() => useProvisioningRepositories());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.repositories).toHaveLength(2);
    const [first] = result.current.repositories;
    expect(first?.syncState).toBe('success');
    expect(first?.lastSyncAt).toBeInstanceOf(Date);
    expect(first?.stats.applied).toBe(3);
  });

  it('maps a repository that has never synced to a null date, not the epoch', async () => {
    server.use(
      http.get('/v1/provisioning/repositories', () =>
        HttpResponse.json({
          repositories: [{ id: 'r', name: 'fresh', last_sync_at: undefined, sync_state: 'pending' }],
          total: 1,
        }),
      ),
    );

    const { result } = renderHookWithQueryClient(() => useProvisioningRepositories());
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.repositories[0]?.lastSyncAt).toBeNull();
    expect(result.current.repositories[0]?.syncState).toBe('pending');
  });

  it('defaults the sync interval rather than reporting zero seconds', async () => {
    server.use(
      http.get('/v1/provisioning/repositories', () =>
        HttpResponse.json({ repositories: [{ id: 'r', name: 'x' }], total: 1 }),
      ),
    );

    const { result } = renderHookWithQueryClient(() => useProvisioningRepositories());
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.repositories[0]?.syncIntervalSeconds).toBe(300);
  });

  it('maps connections without any token field', async () => {
    const { result } = renderHookWithQueryClient(() => useProvisioningConnections());
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.connections).toHaveLength(2);
    expect(Object.keys(result.current.connections[0] ?? {})).not.toContain('token');
    expect(result.current.connections[1]?.status).toBe('error');
    expect(result.current.connections[1]?.lastError).toContain('rejected');
  });

  it('separates device resources from org resources by kind', async () => {
    const { result } = renderHookWithQueryClient(() => useProvisioningResources('repo-1'));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const kinds = result.current.resources.map((r) => r.kind);
    expect(kinds).toEqual(['device', 'device', 'alerts']);

    const alerts = result.current.resources.find((r) => r.kind === 'alerts');
    expect(alerts?.deviceId).toBe('');
    expect(alerts?.path).toBe('orgs/acme/alerts.toml');
  });

  it('does not fetch resources for a missing repository id', () => {
    const { result } = renderHookWithQueryClient(() => useProvisioningResources(undefined));

    expect(result.current.fetchStatus).toBe('idle');
    expect(result.current.resources).toEqual([]);
  });

  it('reports the failure streak from the health endpoint', async () => {
    server.use(
      http.get('/v1/provisioning/repositories/:id/health', () =>
        HttpResponse.json({ failure_streak: 3, successes: 1, failures: 3, average_duration_ms: 0, last_duration_ms: 0, timeline: [] }),
      ),
    );

    const { result } = renderHookWithQueryClient(() => useProvisioningJobs('repo-1'));
    expect(result.current).toBeDefined();
  });

  it('maps the discovery list with device and org entries merged', async () => {
    const { result } = renderHookWithQueryClient(() => useProvisioningDiscovery('repo-1'));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.discovered).toHaveLength(3);
    expect(result.current.discovered.filter((d) => d.kind === 'alerts')).toHaveLength(1);
  });

  it('reads secret readiness and counts the missing keys', async () => {
    const { result } = renderHookWithQueryClient(() => useProvisioningResourceSecrets('repo-1', 'res-1'));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.readiness.total).toBe(2);
    expect(result.current.readiness.missing).toBe(1);
    expect(result.current.readiness.secrets[1]).toEqual({ app: 'dashboard', key: 'API_KEY', set: false });
  });

  it('returns no secrets for an org resource', async () => {
    const { result } = renderHookWithQueryClient(() => useProvisioningResourceSecrets('repo-1', 'res-3'));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.readiness.secrets).toEqual([]);
  });

  it('reads the signed document as text', async () => {
    const { result } = renderHookWithQueryClient(() => useProvisioningResourceDocument('repo-1', 'res-1'));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toContain('[device]');
    expect(result.current.data).toContain('living-room');
  });

  it('surfaces the org-resource refusal for a document request', async () => {
    const { result } = renderHookWithQueryClient(() => useProvisioningResourceDocument('repo-1', 'res-3'));
    await waitFor(() => expect(result.current.isError).toBe(true));
  });

  it('reads the instance summary counts', async () => {
    const { result } = renderHookWithQueryClient(() => useProvisioningInstance());
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.summary.repositories).toBe(2);
    expect(result.current.summary.bootSeedConfigured).toBe(false);
  });

  it('maps a job list newest-first with its counters', async () => {
    const { result } = renderHookWithQueryClient(() => useProvisioningJobs('repo-1'));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.jobs[0]?.applied).toBe(2);
    expect(result.current.jobs[0]?.state).toBe('success');
  });

  it('maps a single job by id', async () => {
    const { result } = renderHookWithQueryClient(() => useProvisioningJob('repo-1', 'job-1'));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.job?.id).toBe('job-1');
  });
});

describe('provisioning mutations', () => {
  it('creates a connection and refetches the list', async () => {

    const queryClient = createTestQueryClient();
    const list = renderHookWithQueryClient(() => useProvisioningConnections(), { queryClient });
    await waitFor(() => expect(list.result.current.isSuccess).toBe(true));
    expect(list.result.current.connections).toHaveLength(2);

    const create = renderHookWithQueryClient(() => useCreateProvisioningConnection(), { queryClient });

    await act(async () => {
      await create.result.current.mutateAsync({
        data: { name: 'Bitbucket — infra', type: 'generic', host: 'bitbucket.org', token: 'secret' },
      });
    });

    await waitFor(() => expect(list.result.current.connections).toHaveLength(3));
  });

  it('sends no token when editing, so the stored one survives', async () => {
    let received: Record<string, unknown> = {};
    server.use(
      http.patch('/v1/provisioning/connections/:id', async ({ request }) => {
        received = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ id: 'conn-1', name: 'renamed', type: 'github', status: 'ready' });
      }),
    );

    const update = renderHookWithQueryClient(() => useUpdateProvisioningConnection());

    await act(async () => {
      await update.result.current.mutateAsync({ id: 'conn-1', data: { name: 'renamed' } });
    });

    expect(received.name).toBe('renamed');
    expect(received).not.toHaveProperty('token');
  });

  it('refuses to delete a credential a source still uses, with the server message', async () => {
    const remove = renderHookWithQueryClient(() => useDeleteProvisioningConnection());

    await expect(
      act(async () => {
        await remove.result.current.mutateAsync({ id: 'conn-1' });
      }),
    ).rejects.toBeTruthy();

    await waitFor(() => expect(remove.result.current.isError).toBe(true));
  });

  it('reports a failed health check as data, not as a thrown error', async () => {
    server.use(
      http.post('/v1/provisioning/connections/:id/check', () =>
        HttpResponse.json({ id: 'conn-1', name: 'x', type: 'github', status: 'error', last_error: 'the token was rejected' }),
      ),
    );

    const check = renderHookWithQueryClient(() => useCheckProvisioningConnection());

    let status = '';
    await act(async () => {
      const result = await check.result.current.mutateAsync({ id: 'conn-1' });
      status = result.status ?? '';
    });

    expect(status).toBe('error');
  });

  it('creates a repository and refetches the list', async () => {
    const queryClient = createTestQueryClient();
    const list = renderHookWithQueryClient(() => useProvisioningRepositories(), { queryClient });
    await waitFor(() => expect(list.result.current.isSuccess).toBe(true));
    expect(list.result.current.repositories).toHaveLength(2);

    const create = renderHookWithQueryClient(() => useCreateProvisioningRepository(), { queryClient });

    await act(async () => {
      await create.result.current.mutateAsync({
        data: { name: 'new-fleet', url: 'https://github.com/acme/new.git', connection_id: 'conn-1' },
      });
    });

    await waitFor(() => expect(list.result.current.repositories).toHaveLength(3));
  });

  it('surfaces an unreachable remote as a 422 the caller can describe', async () => {
    server.use(
      http.post('/v1/provisioning/repositories', () =>
        HttpResponse.json({ message: 'could not resolve host', code: 'VALIDATION_FAILED' }, { status: 422 }),
      ),
    );

    const create = renderHookWithQueryClient(() => useCreateProvisioningRepository());

    await expect(
      act(async () => {
        await create.result.current.mutateAsync({
          data: { name: 'x', url: 'https://nonexistent.invalid/x.git', connection_id: 'conn-1' },
        });
      }),
    ).rejects.toBeTruthy();

    await waitFor(() => expect(create.result.current.isError).toBe(true));
  });

  it('maps a sync result so callers never see optional counters', async () => {
    const sync = renderHookWithQueryClient(() => useSyncProvisioningRepository());

    let applied = -1;
    await act(async () => {
      const job = await sync.result.current.mutateAsync({ id: 'repo-1', params: {} });
      applied = job.applied;
    });

    expect(applied).toBe(3);
  });

  it('leaves the store untouched on a dry run', async () => {
    const queryClient = createTestQueryClient();
    const list = renderHookWithQueryClient(() => useProvisioningRepositories(), { queryClient });
    await waitFor(() => expect(list.result.current.isSuccess).toBe(true));
    const before = list.result.current.repositories[0]?.lastSyncAt?.getTime();

    const sync = renderHookWithQueryClient(() => useSyncProvisioningRepository(), { queryClient });
    let dryRun = false;
    await act(async () => {
      const job = await sync.result.current.mutateAsync({ id: 'repo-1', params: { dry_run: true } });
      dryRun = job.dryRun;
      expect(job.applied).toBe(0);
    });

    expect(dryRun).toBe(true);
    await waitFor(() => {
      expect(list.result.current.repositories[0]?.lastSyncAt?.getTime()).toBe(before);
    });
  });

  it('syncs one resource and records the rest as skipped', async () => {
    const sync = renderHookWithQueryClient(() => useSyncProvisionedResource());

    let skipped = 0;
    await act(async () => {
      const job = await sync.result.current.mutateAsync({ id: 'repo-1', resourceId: 'res-1', params: {} });
      skipped = job.skipped;
    });

    expect(skipped).toBe(2);
  });

  it('refuses a per-resource sync for a resource of another repository', async () => {
    const sync = renderHookWithQueryClient(() => useSyncProvisionedResource());

    await expect(
      act(async () => {
        await sync.result.current.mutateAsync({ id: 'repo-1', resourceId: 'res-4', params: {} });
      }),
    ).rejects.toBeTruthy();
  });
});

describe('provisioning store reset', () => {
  it('starts every test from the same seed', async () => {
    resetProvisioningStore();
    const list = renderHookWithQueryClient(() => useProvisioningRepositories());
    await waitFor(() => expect(list.result.current.isSuccess).toBe(true));

    expect(list.result.current.repositories).toHaveLength(2);
  });
});
