import type {
  ConnectionJSON,
  ConnectionListJSON,
  DiscoveryJSON,
  EventListJSON,
  HealthJSON,
  InstanceSummary,
  JobJSON,
  JobListJSON,
  RepositoryJSON,
  RepositoryListJSON,
  ResourceListJSON,
  ResourceJSON,
  ResourceStats,
  SecretReadinessJSON,
} from '@vyzorix/api-client';

const EPOCH = 1_767_225_600_000;

export function buildStats(overrides: Partial<ResourceStats> = {}): ResourceStats {
  return { applied: 0, drifted: 0, missing: 0, error: 0, ignored: 0, total: 0, ...overrides };
}

export function buildConnection(overrides: Partial<ConnectionJSON> = {}): ConnectionJSON {
  return {
    id: 'conn-1',
    name: 'GitHub — platform',
    type: 'github',
    host: 'github.com',
    status: 'ready',
    created_at: EPOCH,
    updated_at: EPOCH,
    checked_at: EPOCH,
    write_enabled: false,
    ...overrides,
  };
}

export function buildConnectionList(connections: ConnectionJSON[]): ConnectionListJSON {
  return { connections, total: connections.length };
}

export function buildRepository(overrides: Partial<RepositoryJSON> = {}): RepositoryJSON {
  return {
    id: 'repo-1',
    connection_id: 'conn-1',
    name: 'prod-fleet',
    title: 'Production fleet',
    description: '',
    type: 'github',
    url: 'https://github.com/acme/fleet-config.git',
    branch: 'main',
    path: '/',
    include: [],
    exclude: [],
    sync_target: 'device',
    sync_state: 'success',
    sync_interval_seconds: 300,
    enabled: true,
    created_at: EPOCH,
    updated_at: EPOCH,
    last_sync_at: EPOCH,
    stats: buildStats({ applied: 3, total: 3 }),
    ...overrides,
  };
}

export function buildRepositoryList(repositories: RepositoryJSON[]): RepositoryListJSON {
  return { repositories, total: repositories.length };
}

export function buildResource(overrides: Partial<ResourceJSON> = {}): ResourceJSON {
  return {
    id: 'res-1',
    kind: 'device',
    device_id: 'living-room',
    path: 'devices/living-room/vyzorix.toml',
    content_hash: 'a'.repeat(64),
    state: 'applied',
    config_version: 4,
    last_applied_at: EPOCH,
    device_online: true,
    device_last_seen: EPOCH,
    device_reported_at: EPOCH,
    device_reported_version: 4,
    ...overrides,
  };
}

export function buildResourceList(resources: ResourceJSON[]): ResourceListJSON {
  return { resources, total: resources.length };
}

export function buildJobResult(path: string, outcome: string, message?: string) {
  return { path, outcome, ...(message ? { message } : {}) };
}

export function buildJob(overrides: Partial<JobJSON> = {}): JobJSON {
  return {
    id: 'job-1',
    repo_id: 'repo-1',
    state: 'success',
    applied: 2,
    skipped: 0,
    failed: 0,
    dry_run: false,
    started_at: EPOCH,
    finished_at: EPOCH + 1_500,
    results: [],
    ...overrides,
  };
}

export function buildJobList(jobs: JobJSON[]): JobListJSON {
  return { jobs, total: jobs.length };
}

export function buildDiscovery(overrides: Partial<DiscoveryJSON> = {}): DiscoveryJSON {
  return { items: [], unmanaged: [], org_files: [], total: 0, ...overrides };
}

export function buildHealth(overrides: Partial<HealthJSON> = {}): HealthJSON {
  return {
    average_duration_ms: 1_200,
    last_duration_ms: 900,
    failure_streak: 0,
    successes: 4,
    failures: 0,
    timeline: [
      { finished_at: '2026-11-20T10:00:00Z', state: 'success', duration_ms: 900, failed: 0 },
      { finished_at: '2026-11-20T11:00:00Z', state: 'success', duration_ms: 1_500, failed: 0 },
    ],
    ...overrides,
  };
}

export function buildSecrets(overrides: Partial<SecretReadinessJSON> = {}): SecretReadinessJSON {
  return {
    secrets: [
      { app: 'dashboard', key: 'DB_URL', set: true },
      { app: 'dashboard', key: 'API_KEY', set: false },
    ],
    total: 2,
    missing: 1,
    ...overrides,
  };
}

export function buildEventList(events: EventListJSON['events'] = []): EventListJSON {
  return { events, total: events.length };
}

export function buildInstanceSummary(overrides: Partial<InstanceSummary> = {}): InstanceSummary {
  return {
    boot_seed_configured: false,
    repositories: 1,
    resources: 3,
    connections: 1,
    ...overrides,
  };
}
