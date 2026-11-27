import { describe, it, expect } from 'vitest';
import {
  ACTOR_KINDS,
  CONNECTION_STATUSES,
  DISCOVERY_ACTIONS,
  EVENT_TYPES,
  JOB_OUTCOMES,
  PROVIDER_TYPES,
  RESOURCE_KINDS,
  RESOURCE_STATES,
  SYNC_STATES,
  actorKindLabel,
  connectionsFromWire,
  discoveredFromWire,
  eventsFromWire,
  healthFromWire,
  instanceSummaryFromWire,
  isDeviceKind,
  jobFromWire,
  jobsFromWire,
  providerLabel,
  repositoriesFromWire,
  repositoryFromWire,
  resourceFromWire,
  resourceKindLabel,
  resourceStateLabel,
  resourcesFromWire,
  secretReadinessFromWire,
  syncStateLabel,
} from '@vyzorix/api-client';

describe('provisioning vocabulary', () => {
  it('lists the provider types the server accepts', () => {
    expect([...PROVIDER_TYPES]).toEqual(['github', 'gitlab', 'generic']);
  });

  it('lists every connection status', () => {
    expect([...CONNECTION_STATUSES]).toEqual(['pending', 'ready', 'error']);
  });

  it('lists every repository sync state', () => {
    expect([...SYNC_STATES]).toEqual(['pending', 'success', 'warning', 'working', 'error', 'disabled']);
  });

  it('lists every resource state, including the unconfirmed and out-of-scope ones', () => {
    expect([...RESOURCE_STATES]).toEqual(['applied', 'pending', 'drifted', 'missing', 'error', 'ignored']);
  });

  it('lists every resource kind', () => {
    expect([...RESOURCE_KINDS]).toEqual(['device', 'alerts', 'contact_points', 'annotations', 'groups']);
  });

  it('lists every job outcome', () => {
    expect([...JOB_OUTCOMES]).toEqual([
      'applied',
      'updated',
      'unchanged',
      'missing',
      'ignored',
      'skipped',
      'failed',
    ]);
  });

  it('lists every discovery action', () => {
    expect([...DISCOVERY_ACTIONS]).toEqual([
      'create',
      'update',
      'unchanged',
      'missing',
      'conflict',
      'error',
      'ignored',
    ]);
  });

  it('lists every actor kind, including the provider webhook', () => {
    expect([...ACTOR_KINDS]).toEqual(['user', 'service_account', 'scheduler', 'system', 'provider']);
  });

  it('lists the audit event types', () => {
    expect(EVENT_TYPES).toContain('sync.triggered');
    expect(EVENT_TYPES).toContain('document.pushed');
    expect(EVENT_TYPES).toHaveLength(10);
  });
});

describe('provisioning labels', () => {
  it('labels every kind', () => {
    expect(resourceKindLabel('device')).toBe('Devices');
    expect(resourceKindLabel('contact_points')).toBe('Contact points');
    expect(resourceKindLabel(undefined)).toBe('Devices');
  });

  it('treats a missing kind as a device', () => {
    expect(isDeviceKind(undefined)).toBe(true);
    expect(isDeviceKind('device')).toBe(true);
    expect(isDeviceKind('alerts')).toBe(false);
  });

  it('labels every provider', () => {
    expect(providerLabel('github')).toBe('GitHub');
    expect(providerLabel('gitlab')).toBe('GitLab');
    expect(providerLabel('generic')).toBe('Git');
    expect(providerLabel(undefined)).toBe('Git');
  });

  it('distinguishes never-synced from failed', () => {
    expect(syncStateLabel('pending')).toBe('Not synced');
    expect(syncStateLabel('error')).toBe('Failed');
    expect(syncStateLabel('warning')).toBe('Partial');
  });

  it('labels out-of-scope separately from missing', () => {
    expect(resourceStateLabel('missing')).toBe('Missing');
    expect(resourceStateLabel('ignored')).toBe('Out of scope');
  });

  it('labels the provider actor distinctly from the scheduler', () => {
    expect(actorKindLabel('provider')).toBe('Git provider');
    expect(actorKindLabel('scheduler')).toBe('Scheduler');
  });
});

describe('connection mapping', () => {
  it('maps a connection and its timestamps', () => {
    const connection = connectionsFromWire({
      connections: [
        {
          id: 'c1',
          name: 'deploy-bot',
          type: 'github',
          host: 'github.com',
          status: 'ready',
          created_at: 1_767_225_600_000,
          updated_at: 1_767_225_600_000,
          checked_at: 1_767_225_600_000,
          write_enabled: true,
        },
      ],
      total: 1,
    })[0]!;

    expect(connection.type).toBe('github');
    expect(connection.status).toBe('ready');
    expect(connection.createdAt).toEqual(new Date(1_767_225_600_000));
    expect(connection.writeEnabled).toBe(true);
  });

  it('never carries a token, because the wire type has none', () => {
    const connection = connectionsFromWire({
      connections: [{ id: 'c1', name: 'x', type: 'generic', status: 'pending' }],
    })[0]!;

    expect(Object.keys(connection)).not.toContain('token');
  });

  it('defaults an unknown provider and status rather than inventing one', () => {
    const connection = connectionsFromWire({
      connections: [{ id: 'c1', type: 'bitbucket', status: 'weird' }],
    })[0]!;

    expect(connection.type).toBe('generic');
    expect(connection.status).toBe('pending');
  });

  it('returns an empty list for a missing body', () => {
    expect(connectionsFromWire(undefined)).toEqual([]);
  });
});

describe('repository mapping', () => {
  it('maps the scope patterns and stats', () => {
    const repository = repositoryFromWire({
      id: 'r1',
      name: 'prod',
      url: 'https://github.com/acme/prod.git',
      branch: 'main',
      path: '/apps',
      include: ['devices/*'],
      exclude: ['devices/old/*'],
      sync_state: 'warning',
      sync_interval_seconds: 120,
      stats: { applied: 3, drifted: 1, missing: 0, error: 0, ignored: 2, total: 6 },
    });

    expect(repository.include).toEqual(['devices/*']);
    expect(repository.exclude).toEqual(['devices/old/*']);
    expect(repository.syncState).toBe('warning');
    expect(repository.stats.ignored).toBe(2);
  });

  it('defaults the interval so a card never shows zero seconds', () => {
    expect(repositoryFromWire({ id: 'r1' }).syncIntervalSeconds).toBe(300);
  });

  it('returns an empty list for a missing body', () => {
    expect(repositoriesFromWire(undefined)).toEqual([]);
  });
});

describe('resource mapping', () => {
  it('keeps the kind so a device row and an org row are distinguishable', () => {
    const resources = resourcesFromWire({
      resources: [
        { id: 'res1', kind: 'device', device_id: 'dev-1', state: 'applied', config_version: 4 },
        { id: 'res2', kind: 'alerts', path: 'orgs/acme/alerts.toml', state: 'drifted' },
      ],
    });

    expect(resources[0]?.kind).toBe('device');
    expect(resources[0]?.deviceId).toBe('dev-1');
    expect(resources[1]?.kind).toBe('alerts');
    expect(resources[1]?.deviceId).toBe('');
  });

  it('treats an unknown state as an error, never as applied', () => {
    expect(resourceFromWire({ id: 'res1', state: 'exploded' }).state).toBe('error');
  });

  it('maps device presence, keeping unknown distinct from offline', () => {
    const online = resourceFromWire({
      id: 'res1',
      kind: 'device',
      device_id: 'dev-1',
      device_online: true,
      device_last_seen: 1_700_000_000_000,
      device_reported_at: 1_700_000_000_000,
      device_reported_version: 7,
    });
    expect(online.deviceOnline).toBe(true);
    expect(online.deviceLastSeen).toBeInstanceOf(Date);
    expect(online.deviceReportedVersion).toBe(7);

    // A device the server could not read, and an org resource, both arrive with no presence.
    // Both must map to null — "unknown" — not to a fabricated offline.
    const unknown = resourceFromWire({ id: 'res2', kind: 'alerts', path: 'orgs/acme/alerts.toml' });
    expect(unknown.deviceOnline).toBeNull();
    expect(unknown.deviceLastSeen).toBeNull();
    expect(unknown.deviceReportedVersion).toBeNull();
  });
});

describe('job mapping', () => {
  it('maps the counters and per-resource results', () => {
    const job = jobFromWire({
      id: 'j1',
      repo_id: 'r1',
      state: 'warning',
      applied: 2,
      failed: 1,
      skipped: 0,
      dry_run: true,
      results: [
        { path: 'devices/a/vyzorix.toml', outcome: 'applied' },
        { path: 'devices/b/vyzorix.toml', outcome: 'failed', message: 'parse error' },
      ],
    });

    expect(job.dryRun).toBe(true);
    expect(job.failed).toBe(1);
    expect(job.results[1]?.outcome).toBe('failed');
    expect(job.results[1]?.message).toBe('parse error');
  });

  it('maps a list body', () => {
    expect(jobsFromWire({ jobs: [{ id: 'j1' }], total: 1 })[0]?.id).toBe('j1');
    expect(jobsFromWire(undefined)).toEqual([]);
  });
});

describe('discovery mapping', () => {
  it('merges device items and org files, carrying the kind', () => {
    const discovered = discoveredFromWire({
      items: [{ path: 'devices/a/vyzorix.toml', kind: 'device', device_id: 'a', action: 'create' }],
      org_files: [{ path: 'orgs/acme/alerts.toml', org: 'acme', kind: 'alerts', action: 'update' }],
    });

    expect(discovered).toHaveLength(2);
    expect(discovered[1]?.kind).toBe('alerts');
    expect(discovered[1]?.org).toBe('acme');
  });

  it('marks a conflict as a row the wizard must not offer', () => {
    const [conflict] = discoveredFromWire({
      items: [
        {
          path: 'devices/a/vyzorix.toml',
          kind: 'device',
          device_id: 'a',
          action: 'conflict',
          message: 'owned by repo-demo',
        },
      ],
    });

    expect(conflict?.action).toBe('conflict');
    expect(conflict?.message).toBe('owned by repo-demo');
  });
});

describe('secret readiness', () => {
  it('counts the missing keys and never carries a value', () => {
    const readiness = secretReadinessFromWire({
      secrets: [
        { app: 'dashboard', key: 'DB_URL', set: true },
        { app: 'dashboard', key: 'API_KEY', set: false },
      ],
      total: 2,
      missing: 1,
    });

    expect(readiness.missing).toBe(1);
    expect(readiness.secrets[1]).toEqual({ app: 'dashboard', key: 'API_KEY', set: false });
  });

  it('derives the count when the server omits it', () => {
    const readiness = secretReadinessFromWire({
      secrets: [
        { app: 'a', key: 'K1', set: false },
        { app: 'a', key: 'K2', set: false },
      ],
    });

    expect(readiness.missing).toBe(2);
  });
});

describe('health mapping', () => {
  it('maps the timeline oldest-first with ISO timestamps', () => {
    const health = healthFromWire({
      average_duration_ms: 1200,
      last_duration_ms: 900,
      failure_streak: 2,
      successes: 8,
      failures: 2,
      timeline: [
        { finished_at: '2026-11-20T10:00:00Z', state: 'success', duration_ms: 900, failed: 0 },
        { finished_at: '2026-11-20T11:00:00Z', state: 'error', duration_ms: 1500, failed: 1 },
      ],
    });

    expect(health.failureStreak).toBe(2);
    expect(health.timeline[0]?.finishedAt).toEqual(new Date('2026-11-20T10:00:00Z'));
    expect(health.timeline[1]?.state).toBe('error');
  });

  it('reports no history as zero rather than as instant', () => {
    expect(healthFromWire(undefined).averageDurationMs).toBe(0);
    expect(healthFromWire(undefined).timeline).toEqual([]);
  });
});

describe('events and instance summary', () => {
  it('maps the actor so a scheduler is distinguishable from a person', () => {
    const events = eventsFromWire({
      events: [
        { id: 'e1', actor_kind: 'scheduler', type: 'sync.triggered', created_at: 1_767_225_600_000 },
        { id: 'e2', actor_kind: 'user', actor_label: 'dev@vyzorix.local', type: 'sync.triggered' },
      ],
    });

    expect(events[0]?.actorKind).toBe('scheduler');
    expect(events[1]?.actorLabel).toBe('dev@vyzorix.local');
    expect(events[0]?.createdAt).toEqual(new Date(1_767_225_600_000));
  });

  it('maps the instance summary without the boot seed path', () => {
    const summary = instanceSummaryFromWire({
      boot_seed_configured: true,
      repositories: 2,
      resources: 9,
      connections: 1,
    });

    expect(summary).toEqual({
      bootSeedConfigured: true,
      repositories: 2,
      resources: 9,
      connections: 1,
    });
  });
});
