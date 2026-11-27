import { http, HttpResponse, delay } from 'msw';
import type {
  ConnectionJSON,
  RepositoryJSON,
  ResourceJSON,
} from '@vyzorix/api-client';
import {
  buildConnection,
  buildConnectionList,
  buildDiscovery,
  buildEventList,
  buildHealth,
  buildInstanceSummary,
  buildJob,
  buildJobList,
  buildRepository,
  buildRepositoryList,
  buildResource,
  buildResourceList,
  buildSecrets,
  buildStats,
} from '../fixtures/provisioning-fixtures';

const BASE = '/v1/provisioning';

interface Store {
  connections: ConnectionJSON[];
  repositories: RepositoryJSON[];
  resources: Record<string, ResourceJSON[]>;
  jobs: Record<string, ReturnType<typeof buildJob>[]>;

  remoteUnreachable: boolean;

  checkFails: boolean;
}

let store: Store;

function seed(): Store {
  const connections = [buildConnection(), buildConnection({ id: 'conn-2', name: 'GitLab — apps', type: 'gitlab', host: 'gitlab.com', status: 'error', last_error: 'the token was rejected' })];
  const repositories = [
    buildRepository(),
    buildRepository({
      id: 'repo-2',
      name: 'staging-fleet',
      title: 'Staging fleet',
      url: 'https://github.com/acme/staging.git',
      sync_state: 'warning',
      stats: buildStats({ applied: 2, drifted: 1, total: 3 }),
    }),
  ];
  const resources: Record<string, ResourceJSON[]> = {
    'repo-1': [
      buildResource(),
      buildResource({ id: 'res-2', device_id: 'workshop', path: 'devices/workshop/vyzorix.toml', state: 'drifted', config_version: 7 }),
      buildResource({ id: 'res-3', kind: 'alerts', device_id: '', path: 'orgs/acme/alerts.toml', state: 'applied', config_version: 1 }),
    ],
    'repo-2': [
      buildResource({ id: 'res-4', device_id: 'scanner-01', path: 'devices/scanner-01/vyzorix.toml', state: 'error', last_error: 'parse error at line 4' }),
    ],
  };

  return {
    connections,
    repositories,
    resources,
    jobs: {
      'repo-1': [buildJob()],
      'repo-2': [buildJob({ id: 'job-2', repo_id: 'repo-2', state: 'warning', applied: 1, failed: 1 })],
    },
    remoteUnreachable: false,
    checkFails: false,
  };
}

export function resetProvisioningStore(): void {
  store = seed();
}

function repoResources(repoId: string): ResourceJSON[] {
  store.resources[repoId] ??= [];
  return store.resources[repoId]!;
}

function repoJobs(repoId: string) {
  store.jobs[repoId] ??= [];
  return store.jobs[repoId]!;
}

export function createProvisioningHandlers() {
  resetProvisioningStore();

  return [

    http.get(`${BASE}/connections`, async () => {
      await delay(10);
      return HttpResponse.json(buildConnectionList(store.connections));
    }),

    http.post(`${BASE}/connections`, async ({ request }) => {
      await delay(10);
      const body = (await request.json()) as Partial<ConnectionJSON> & { token?: string };
      if (!body.name?.trim()) {
        return HttpResponse.json({ message: 'name is required', code: 'VALIDATION_FAILED' }, { status: 400 });
      }
      const created = buildConnection({
        id: `conn-${store.connections.length + 1}`,
        name: body.name,
        type: body.type ?? 'github',
        host: body.host ?? 'github.com',
        status: store.checkFails ? 'error' : 'ready',
        last_error: store.checkFails ? 'the token was rejected' : undefined,
        write_enabled: body.write_enabled ?? false,
      });
      store.connections = [...store.connections, created];
      return HttpResponse.json(created, { status: 201 });
    }),

    http.get(`${BASE}/connections/:id`, async ({ params }) => {
      const found = store.connections.find((c) => c.id === params.id);
      return found
        ? HttpResponse.json(found)
        : HttpResponse.json({ message: 'connection not found', code: 'NOT_FOUND' }, { status: 404 });
    }),

    http.patch(`${BASE}/connections/:id`, async ({ params, request }) => {
      const body = (await request.json()) as Partial<ConnectionJSON> & { token?: string };
      const index = store.connections.findIndex((c) => c.id === params.id);
      if (index < 0) {
        return HttpResponse.json({ message: 'connection not found', code: 'NOT_FOUND' }, { status: 404 });
      }

      const updated = { ...store.connections[index]!, ...body, updated_at: Date.now() };
      store.connections = store.connections.map((c, i) => (i === index ? updated : c));
      return HttpResponse.json(updated);
    }),

    http.delete(`${BASE}/connections/:id`, async ({ params }) => {
      const inUse = store.repositories.some((r) => r.connection_id === params.id);
      if (inUse) {
        return HttpResponse.json(
          { message: 'connection is still in use by 1 repository', code: 'RESOURCE_CONFLICT' },
          { status: 409 },
        );
      }
      store.connections = store.connections.filter((c) => c.id !== params.id);
      return HttpResponse.json({ success: true, message: 'connection deleted' });
    }),

    http.post(`${BASE}/connections/:id/check`, async ({ params }) => {
      const index = store.connections.findIndex((c) => c.id === params.id);
      if (index < 0) {
        return HttpResponse.json({ message: 'connection not found', code: 'NOT_FOUND' }, { status: 404 });
      }
      const checked: ConnectionJSON = {
        ...store.connections[index]!,
        status: store.checkFails ? 'error' : 'ready',
        last_error: store.checkFails ? 'the token was rejected' : undefined,
        checked_at: Date.now(),
      };
      store.connections = store.connections.map((c, i) => (i === index ? checked : c));
      return HttpResponse.json(checked);
    }),

    http.get(`${BASE}/repositories`, async () => {
      await delay(10);
      return HttpResponse.json(buildRepositoryList(store.repositories));
    }),

    http.post(`${BASE}/repositories`, async ({ request }) => {
      await delay(10);
      const body = (await request.json()) as Partial<RepositoryJSON> & { connection_id?: string };
      if (!body.name?.trim() || !body.url?.trim() || !body.connection_id) {
        return HttpResponse.json({ message: 'name, url and connection are required', code: 'VALIDATION_FAILED' }, { status: 400 });
      }
      if (store.remoteUnreachable) {
        return HttpResponse.json(
          { message: 'could not resolve host: no such remote', code: 'VALIDATION_FAILED' },
          { status: 422 },
        );
      }
      const created = buildRepository({
        id: `repo-${store.repositories.length + 1}`,
        name: body.name,
        url: body.url,
        connection_id: body.connection_id,
        sync_state: 'pending',
        include: body.include ?? [],
        exclude: body.exclude ?? [],
        last_sync_at: undefined,
        stats: buildStats(),
      });
      store.repositories = [...store.repositories, created];
      return HttpResponse.json(created, { status: 201 });
    }),

    http.get(`${BASE}/repositories/:id`, async ({ params }) => {
      const found = store.repositories.find((r) => r.id === params.id);
      return found
        ? HttpResponse.json(found)
        : HttpResponse.json({ message: 'repository not found', code: 'NOT_FOUND' }, { status: 404 });
    }),

    http.patch(`${BASE}/repositories/:id`, async ({ params, request }) => {
      const body = (await request.json()) as Partial<RepositoryJSON>;
      const index = store.repositories.findIndex((r) => r.id === params.id);
      if (index < 0) {
        return HttpResponse.json({ message: 'repository not found', code: 'NOT_FOUND' }, { status: 404 });
      }
      const updated = { ...store.repositories[index]!, ...body, updated_at: Date.now() };
      store.repositories = store.repositories.map((r, i) => (i === index ? updated : r));
      return HttpResponse.json(updated);
    }),

    http.delete(`${BASE}/repositories/:id`, async ({ params }) => {
      store.repositories = store.repositories.filter((r) => r.id !== params.id);
      return HttpResponse.json({ success: true, message: 'repository deleted' });
    }),

    http.post(`${BASE}/repositories/:id/sync`, async ({ params, request }) => {
      const url = new URL(request.url);
      const dryRun = url.searchParams.get('dry_run') === 'true';
      const repoId = String(params.id);
      const count = repoResources(repoId).length;

      const job = buildJob({
        id: `job-${repoJobs(repoId).length + 1}`,
        repo_id: repoId,
        dry_run: dryRun,
        state: 'success',
        applied: dryRun ? 0 : count,
        skipped: dryRun ? count : 0,
        results: [],
      });

      if (!dryRun) {
        store.jobs[repoId] = [job, ...repoJobs(repoId)];
        store.repositories = store.repositories.map((r) =>
          r.id === repoId ? { ...r, sync_state: 'success', last_sync_at: Date.now() } : r,
        );
      }
      return HttpResponse.json(job);
    }),

    http.post(`${BASE}/repositories/:id/resources/:resourceId/sync`, async ({ params }) => {
      const repoId = String(params.id);
      const resourceId = String(params.resourceId);
      const resource = repoResources(repoId).find((r) => r.id === resourceId);
      if (!resource) {
        return HttpResponse.json({ message: 'not found', code: 'NOT_FOUND' }, { status: 404 });
      }
      const job = buildJob({
        id: `job-${repoJobs(repoId).length + 1}`,
        repo_id: repoId,
        applied: 1,
        skipped: repoResources(repoId).length - 1,
        results: [{ path: resource.path ?? '', outcome: 'applied' }],
      });
      store.jobs[repoId] = [job, ...repoJobs(repoId)];
      return HttpResponse.json(job);
    }),

    http.get(`${BASE}/repositories/:id/resources`, async ({ params }) => {
      await delay(10);
      return HttpResponse.json(buildResourceList(repoResources(String(params.id))));
    }),

    http.get(`${BASE}/repositories/:id/resources/:resourceId/document`, async ({ params }) => {
      const resource = repoResources(String(params.id)).find((r) => r.id === params.resourceId);
      if (!resource) {
        return HttpResponse.json({ message: 'not found', code: 'NOT_FOUND' }, { status: 404 });
      }
      if (resource.kind !== 'device') {
        return HttpResponse.json(
          { message: 'this resource is not a device manifest, so it has no signed document', code: 'VALIDATION_FAILED' },
          { status: 400 },
        );
      }
      return HttpResponse.text(`# signed v${resource.config_version}\n[device]\nid = "${resource.device_id}"\n`);
    }),

    http.get(`${BASE}/repositories/:id/resources/:resourceId/secrets`, async ({ params }) => {
      const resource = repoResources(String(params.id)).find((r) => r.id === params.resourceId);
      if (!resource) {
        return HttpResponse.json({ message: 'not found', code: 'NOT_FOUND' }, { status: 404 });
      }
      if (resource.kind !== 'device') {
        return HttpResponse.json({ secrets: [], total: 0, missing: 0 });
      }
      return HttpResponse.json(buildSecrets());
    }),

    http.get(`${BASE}/repositories/:id/jobs`, async ({ params }) => {
      return HttpResponse.json(buildJobList(repoJobs(String(params.id))));
    }),

    http.get(`${BASE}/repositories/:id/jobs/:jobId`, async ({ params }) => {
      const job = repoJobs(String(params.id)).find((j) => j.id === params.jobId);
      return job
        ? HttpResponse.json(job)
        : HttpResponse.json({ message: 'job not found', code: 'NOT_FOUND' }, { status: 404 });
    }),

    http.get(`${BASE}/repositories/:id/health`, async () => {
      return HttpResponse.json(buildHealth());
    }),

    http.get(`${BASE}/repositories/:id/discovery`, async ({ params }) => {
      const repoId = String(params.id);
      const devices = repoResources(repoId).filter((r) => r.kind === 'device');
      const orgFiles = repoResources(repoId).filter((r) => r.kind !== 'device');

      return HttpResponse.json(
        buildDiscovery({
          items: devices.map((r) => ({
            path: r.path,
            kind: 'device',
            device_id: r.device_id,
            action: 'update',
            managed: true,
          })),
          org_files: orgFiles.map((r) => ({
            path: r.path,
            org: 'acme',
            kind: 'alerts',
            action: 'update',
          })),
          total: devices.length + orgFiles.length,
        }),
      );
    }),

    http.get(`${BASE}/repositories/:id/events`, async () => {
      return HttpResponse.json(
        buildEventList([
          { id: 'ev-1', actor_kind: 'user', actor_id: 'user-1', actor_label: 'dev@vyzorix.local', type: 'sync.triggered', resource_type: 'repository', result: 'success', created_at: Date.now() },
          { id: 'ev-2', actor_kind: 'scheduler', actor_id: 'scheduler', actor_label: 'scheduler', type: 'sync.applied', resource_type: 'repository', result: 'success', created_at: Date.now() },
        ]),
      );
    }),

    http.get(`${BASE}/events`, async () => {
      return HttpResponse.json(
        buildEventList([
          { id: 'ev-1', actor_kind: 'user', actor_id: 'user-1', actor_label: 'dev@vyzorix.local', type: 'repository.created', resource_type: 'repository', result: 'success', detail: 'repository created', created_at: Date.now() },
          { id: 'ev-2', actor_kind: 'provider', actor_id: 'github', actor_label: 'github', type: 'sync.triggered', resource_type: 'repository', result: 'success', created_at: Date.now() },
          { id: 'ev-3', actor_kind: 'service_account', actor_id: 'tok-1', actor_label: 'ci-deployer', type: 'sync.applied', resource_type: 'repository', result: 'error', created_at: Date.now() },
        ]),
      );
    }),

    http.get(`${BASE}/instance`, async () => {
      return HttpResponse.json(
        buildInstanceSummary({
          repositories: store.repositories.length,
          connections: store.connections.length,
          resources: Object.values(store.resources).flat().length,
        }),
      );
    }),
  ];
}
