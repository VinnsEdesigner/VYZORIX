export const PROVIDER_TYPES = ['github', 'gitlab', 'generic'] as const;
export type ProviderType = (typeof PROVIDER_TYPES)[number];

export const CONNECTION_STATUSES = ['pending', 'ready', 'error'] as const;
export type ConnectionStatus = (typeof CONNECTION_STATUSES)[number];

export const SYNC_STATES = ['pending', 'success', 'warning', 'working', 'error', 'disabled'] as const;
export type RepositorySyncState = (typeof SYNC_STATES)[number];

export const RESOURCE_STATES = ['applied', 'pending', 'drifted', 'missing', 'error', 'ignored'] as const;
export type ResourceState = (typeof RESOURCE_STATES)[number];

export const RESOURCE_KINDS = ['device', 'alerts', 'contact_points', 'annotations', 'groups'] as const;
export type ResourceKind = (typeof RESOURCE_KINDS)[number];

export const JOB_OUTCOMES = ['applied', 'updated', 'unchanged', 'missing', 'ignored', 'skipped', 'failed'] as const;
export type JobOutcome = (typeof JOB_OUTCOMES)[number];

export const DISCOVERY_ACTIONS = ['create', 'update', 'unchanged', 'missing', 'conflict', 'error', 'ignored'] as const;
export type DiscoveryAction = (typeof DISCOVERY_ACTIONS)[number];

export const ACTOR_KINDS = ['user', 'service_account', 'scheduler', 'system', 'provider'] as const;
export type ActorKind = (typeof ACTOR_KINDS)[number];

export const EVENT_TYPES = [
  'connection.created',
  'connection.updated',
  'connection.deleted',
  'connection.checked',
  'repository.created',
  'repository.updated',
  'repository.deleted',
  'sync.triggered',
  'sync.applied',
  'document.pushed',
] as const;
export type ProvisioningEventType = (typeof EVENT_TYPES)[number];

export const MIN_SYNC_INTERVAL_SECONDS = 60;
export const DEFAULT_SYNC_INTERVAL_SECONDS = 300;

export function isDeviceKind(kind: string | undefined): boolean {
  return (kind ?? 'device') === 'device';
}

export function resourceKindLabel(kind: string | undefined): string {
  switch (kind) {
    case 'alerts':
      return 'Alerts';
    case 'contact_points':
      return 'Contact points';
    case 'annotations':
      return 'Annotations';
    case 'groups':
      return 'Groups';
    default:
      return 'Devices';
  }
}

export function providerLabel(type: string | undefined): string {
  switch (type) {
    case 'github':
      return 'GitHub';
    case 'gitlab':
      return 'GitLab';
    case 'generic':
      return 'Git';
    default:
      return type ? type : 'Git';
  }
}

export function syncStateLabel(state: string | undefined): string {
  switch (state) {
    case 'success':
      return 'Synced';
    case 'warning':
      return 'Partial';
    case 'working':
      return 'Syncing';
    case 'error':
      return 'Failed';
    case 'disabled':
      return 'Disabled';
    default:
      return 'Not synced';
  }
}

export function resourceStateLabel(state: string | undefined): string {
  switch (state) {
    case 'applied':
      return 'Applied';
    case 'pending':
      return 'Awaiting device';
    case 'drifted':
      return 'Drifted';
    case 'missing':
      return 'Missing';
    case 'error':
      return 'Failed';
    case 'ignored':
      return 'Out of scope';
    default:
      return state ? state : 'Unknown';
  }
}

/**
 * devicePresenceLabel words a device's reachability for a person.
 *
 * `null` is "unknown", not "offline": a device the server could not read, or an org resource
 * that has no device, must not be described as down. That distinction is the whole point of
 * carrying presence beside the state.
 */
export function devicePresenceLabel(online: boolean | null | undefined): string {
  if (online === null || online === undefined) return 'Reachability unknown';

  return online ? 'Online' : 'Offline';
}

export function discoveryActionLabel(action: string | undefined): string {
  switch (action) {
    case 'create':
      return 'Will be applied';
    case 'update':
      return 'Will be updated';
    case 'unchanged':
      return 'Unchanged';
    case 'missing':
      return 'Missing';
    case 'conflict':
      return 'Conflict';
    case 'ignored':
      return 'Out of scope';
    case 'error':
      return 'Will fail';
    default:
      return action ? action : 'Unknown';
  }
}

export function actorKindLabel(kind: string | undefined): string {
  switch (kind) {
    case 'user':
      return 'Person';
    case 'service_account':
      return 'Service account';
    case 'scheduler':
      return 'Scheduler';
    case 'provider':
      return 'Git provider';
    default:
      return 'Server';
  }
}
