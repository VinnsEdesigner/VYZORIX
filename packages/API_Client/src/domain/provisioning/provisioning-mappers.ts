import type {
  ConnectionJSON,
  ConnectionListJSON,
  DiscoveryItemJSON,
  DiscoveryJSON,
  EditRequest,
  EditResultJSON,
  EventJSON,
  EventListJSON,
  InstanceSummary,
  JobJSON,
  JobListJSON,
  HealthJSON,
  JobResultJSON,
  OrgResourceSummary,
  PreviewJSON,
  RepositoryJSON,
  RepositoryListJSON,
  ResourceJSON,
  ResourceListJSON,
  ResourceStats,
  SecretReadinessJSON,
  SecretRequirementJSON,
} from '../../generated/vyzorixUpdateServerAPI.schemas';
import {
  ACTOR_KINDS,
  CONNECTION_STATUSES,
  DEFAULT_SYNC_INTERVAL_SECONDS,
  DISCOVERY_ACTIONS,
  JOB_OUTCOMES,
  PROVIDER_TYPES,
  RESOURCE_KINDS,
  RESOURCE_STATES,
  SYNC_STATES,
  type ActorKind,
  type ConnectionStatus,
  type DiscoveryAction,
  type JobOutcome,
  type ProviderType,
  type ResourceKind,
  type ResourceState,
  type RepositorySyncState,
} from './provisioning-vocabulary';

function toDate(ms: number | null | undefined): Date | null {
  return ms === null || ms === undefined ? null : new Date(ms);
}

function asEnum<T extends string>(values: readonly T[], raw: string | undefined, fallback: T): T {
  return values.find((v) => v === raw) ?? fallback;
}

export interface ProvisioningConnection {
  id: string;
  name: string;
  type: ProviderType;
  host: string;
  status: ConnectionStatus;
  lastError: string;
  checkedAt: Date | null;
  createdAt: Date | null;
  updatedAt: Date | null;

  writeEnabled: boolean;
}

export function connectionFromWire(wire: ConnectionJSON): ProvisioningConnection {
  return {
    id: wire.id ?? '',
    name: wire.name ?? '',
    type: asEnum(PROVIDER_TYPES, wire.type, 'generic'),
    host: wire.host ?? '',
    status: asEnum(CONNECTION_STATUSES, wire.status, 'pending'),
    lastError: wire.last_error ?? '',
    checkedAt: toDate(wire.checked_at),
    createdAt: toDate(wire.created_at),
    updatedAt: toDate(wire.updated_at),
    writeEnabled: wire.write_enabled ?? false,
  };
}

export function connectionsFromWire(wire: ConnectionListJSON | undefined): ProvisioningConnection[] {
  return (wire?.connections ?? []).map(connectionFromWire);
}

export interface ProvisioningRepository {
  id: string;
  connectionId: string;
  name: string;
  title: string;
  description: string;
  type: ProviderType;
  url: string;
  branch: string;
  path: string;
  include: string[];
  exclude: string[];
  syncTarget: string;
  syncState: RepositorySyncState;
  syncIntervalSeconds: number;
  enabled: boolean;
  lastError: string;
  lastSyncAt: Date | null;
  createdAt: Date | null;
  updatedAt: Date | null;
  stats: ResourceStats;
}

export function repositoryFromWire(wire: RepositoryJSON): ProvisioningRepository {
  return {
    id: wire.id ?? '',
    connectionId: wire.connection_id ?? '',
    name: wire.name ?? '',
    title: wire.title ?? '',
    description: wire.description ?? '',
    type: asEnum(PROVIDER_TYPES, wire.type, 'generic'),
    url: wire.url ?? '',
    branch: wire.branch ?? '',
    path: wire.path ?? '/',
    include: wire.include ?? [],
    exclude: wire.exclude ?? [],
    syncTarget: wire.sync_target ?? 'device',
    syncState: asEnum(SYNC_STATES, wire.sync_state, 'pending'),
    syncIntervalSeconds: wire.sync_interval_seconds ?? DEFAULT_SYNC_INTERVAL_SECONDS,
    enabled: wire.enabled ?? true,
    lastError: wire.last_error ?? '',
    lastSyncAt: toDate(wire.last_sync_at),
    createdAt: toDate(wire.created_at),
    updatedAt: toDate(wire.updated_at),
    stats: wire.stats ?? {},
  };
}

export function repositoriesFromWire(wire: RepositoryListJSON | undefined): ProvisioningRepository[] {
  return (wire?.repositories ?? []).map(repositoryFromWire);
}

export interface ProvisionedResource {
  id: string;
  kind: ResourceKind;
  deviceId: string;
  path: string;
  contentHash: string;
  state: ResourceState;
  lastError: string;
  configVersion: number;
  lastAppliedAt: Date | null;

  /**
   * Presence is what makes a state honest: "applied" is a claim about a device, so the
   * console reads it next to whether the device is reachable. All three are null for an org
   * resource (no device) and for a device the server could not read — which the UI renders as
   * unknown, never as offline.
   */
  deviceOnline: boolean | null;
  deviceLastSeen: Date | null;
  deviceReportedAt: Date | null;
  deviceReportedVersion: number | null;
}

export function resourceFromWire(wire: ResourceJSON): ProvisionedResource {
  return {
    id: wire.id ?? '',
    kind: asEnum(RESOURCE_KINDS, wire.kind, 'device'),
    deviceId: wire.device_id ?? '',
    path: wire.path ?? '',
    contentHash: wire.content_hash ?? '',
    state: asEnum(RESOURCE_STATES, wire.state, 'error'),
    lastError: wire.last_error ?? '',
    configVersion: wire.config_version ?? 0,
    lastAppliedAt: toDate(wire.last_applied_at),
    deviceOnline: wire.device_online ?? null,
    deviceLastSeen: toDate(wire.device_last_seen),
    deviceReportedAt: toDate(wire.device_reported_at),
    deviceReportedVersion: wire.device_reported_version ?? null,
  };
}

export function resourcesFromWire(wire: ResourceListJSON | undefined): ProvisionedResource[] {
  return (wire?.resources ?? []).map(resourceFromWire);
}

export interface SyncJobResult {
  resourceId: string;
  path: string;
  outcome: JobOutcome;
  message: string;
}

export function jobResultFromWire(wire: JobResultJSON): SyncJobResult {
  return {
    resourceId: wire.resource_id ?? '',
    path: wire.path ?? '',
    outcome: asEnum(JOB_OUTCOMES, wire.outcome, 'failed'),
    message: wire.message ?? '',
  };
}

export interface SyncJob {
  id: string;
  repoId: string;
  state: RepositorySyncState;
  error: string;
  dryRun: boolean;
  applied: number;
  skipped: number;
  failed: number;
  startedAt: Date | null;
  finishedAt: Date | null;
  results: SyncJobResult[];
}

export function jobFromWire(wire: JobJSON): SyncJob {
  return {
    id: wire.id ?? '',
    repoId: wire.repo_id ?? '',
    state: asEnum(SYNC_STATES, wire.state, 'pending'),
    error: wire.error ?? '',
    dryRun: wire.dry_run ?? false,
    applied: wire.applied ?? 0,
    skipped: wire.skipped ?? 0,
    failed: wire.failed ?? 0,
    startedAt: toDate(wire.started_at),
    finishedAt: toDate(wire.finished_at),
    results: (wire.results ?? []).map(jobResultFromWire),
  };
}

export function jobsFromWire(wire: JobListJSON | undefined): SyncJob[] {
  return (wire?.jobs ?? []).map(jobFromWire);
}

export interface DiscoveredResource {
  path: string;
  kind: ResourceKind;
  deviceId: string;
  org: string;
  action: DiscoveryAction;
  message: string;
  managed: boolean;
}

export function discoveredFromWire(wire: DiscoveryJSON | undefined): DiscoveredResource[] {
  const devices: DiscoveredResource[] = (wire?.items ?? []).map((item: DiscoveryItemJSON) => ({
    path: item.path ?? '',
    kind: asEnum(RESOURCE_KINDS, item.kind, 'device'),
    deviceId: item.device_id ?? '',
    org: '',
    action: asEnum(DISCOVERY_ACTIONS, item.action, 'unchanged'),
    message: item.message ?? '',
    managed: item.managed ?? false,
  }));

  const orgFiles: DiscoveredResource[] = (wire?.org_files ?? []).map((item: OrgResourceSummary) => ({
    path: item.path ?? '',
    kind: asEnum(RESOURCE_KINDS, item.kind, 'alerts'),
    deviceId: '',
    org: item.org ?? '',
    action: asEnum(DISCOVERY_ACTIONS, item.action, 'unchanged'),
    message: item.message ?? '',
    managed: item.action !== 'create',
  }));

  return [...devices, ...orgFiles];
}

export interface ProvisioningEvent {
  id: string;
  actorKind: ActorKind;
  actorId: string;
  actorLabel: string;
  type: string;
  resourceType: string;
  resourceId: string;
  result: string;
  detail: string;
  createdAt: Date | null;
}

export function eventFromWire(wire: EventJSON): ProvisioningEvent {
  return {
    id: wire.id ?? '',
    actorKind: asEnum(ACTOR_KINDS, wire.actor_kind, 'system'),
    actorId: wire.actor_id ?? '',
    actorLabel: wire.actor_label ?? '',
    type: wire.type ?? '',
    resourceType: wire.resource_type ?? '',
    resourceId: wire.resource_id ?? '',
    result: wire.result ?? '',
    detail: wire.detail ?? '',
    createdAt: toDate(wire.created_at),
  };
}

export function eventsFromWire(wire: EventListJSON | undefined): ProvisioningEvent[] {
  return (wire?.events ?? []).map(eventFromWire);
}

export interface DeclaredSecret {
  app: string;
  key: string;
  set: boolean;
}

export interface SecretReadiness {
  secrets: DeclaredSecret[];
  total: number;
  missing: number;
}

export function secretReadinessFromWire(wire: SecretReadinessJSON | undefined): SecretReadiness {
  const secrets: DeclaredSecret[] = (wire?.secrets ?? []).map((s: SecretRequirementJSON) => ({
    app: s.app ?? '',
    key: s.key ?? '',
    set: s.set ?? false,
  }));

  return {
    secrets,
    total: wire?.total ?? secrets.length,
    missing: wire?.missing ?? secrets.filter((s) => !s.set).length,
  };
}

export interface HealthTimelinePoint {
  finishedAt: Date | null;
  state: RepositorySyncState;
  durationMs: number;
  failed: number;
}

export interface RepositorySyncHealth {
  averageDurationMs: number;
  lastDurationMs: number;
  failureStreak: number;
  successes: number;
  failures: number;
  timeline: HealthTimelinePoint[];
}

export function healthFromWire(wire: HealthJSON | undefined): RepositorySyncHealth {
  return {
    averageDurationMs: wire?.average_duration_ms ?? 0,
    lastDurationMs: wire?.last_duration_ms ?? 0,
    failureStreak: wire?.failure_streak ?? 0,
    successes: wire?.successes ?? 0,
    failures: wire?.failures ?? 0,
    timeline: (wire?.timeline ?? []).map((p) => ({
      finishedAt: p.finished_at ? new Date(p.finished_at) : null,
      state: asEnum(SYNC_STATES, p.state, 'pending'),
      durationMs: p.duration_ms ?? 0,
      failed: p.failed ?? 0,
    })),
  };
}

export interface ProvisioningInstanceSummary {
  bootSeedConfigured: boolean;
  repositories: number;
  resources: number;
  connections: number;
}

export function instanceSummaryFromWire(wire: InstanceSummary | undefined): ProvisioningInstanceSummary {
  return {
    bootSeedConfigured: wire?.boot_seed_configured ?? false,
    repositories: wire?.repositories ?? 0,
    resources: wire?.resources ?? 0,
    connections: wire?.connections ?? 0,
  };
}

export interface ManifestPreview {
  path: string;
  diff: string;
  baseCommit: string;
  changed: boolean;
}

export function editPreviewFromWire(wire: PreviewJSON | undefined): ManifestPreview {
  return {
    path: wire?.path ?? '',
    diff: wire?.diff ?? '',
    baseCommit: wire?.base_commit ?? '',
    changed: wire?.changed ?? false,
  };
}

export interface ManifestEditResult {
  branch: string;
  commit: string;
  pullRequestUrl: string;
  pullRequestError: string;
  changed: boolean;
}

export function editResultFromWire(wire: EditResultJSON | undefined): ManifestEditResult {
  return {
    branch: wire?.branch ?? '',
    commit: wire?.commit ?? '',
    pullRequestUrl: wire?.pull_request_url ?? '',
    pullRequestError: wire?.pull_request_error ?? '',
    changed: wire?.changed ?? false,
  };
}

export function editRequestFor(
  content: string,
  baseCommit: string,
  mode?: 'pr' | 'direct',
  commitMessage?: string,
): EditRequest {
  const body: EditRequest = { content, base_commit: baseCommit };
  if (mode) body.mode = mode;
  if (commitMessage) body.commit_message = commitMessage;

  return body;
}
