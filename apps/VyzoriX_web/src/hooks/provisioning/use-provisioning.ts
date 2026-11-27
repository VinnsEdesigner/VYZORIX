import { useQueryClient } from '@tanstack/react-query';
import {
  useGetProvisioningConnections,
  useGetProvisioningConnectionsId,
  usePostProvisioningConnections,
  usePatchProvisioningConnectionsId,
  useDeleteProvisioningConnectionsId,
  usePostProvisioningConnectionsIdCheck,
  useGetProvisioningRepositories,
  useGetProvisioningRepositoriesId,
  usePostProvisioningRepositories,
  usePatchProvisioningRepositoriesId,
  useDeleteProvisioningRepositoriesId,
  useGetProvisioningRepositoriesIdDiscovery,
  useGetProvisioningRepositoriesIdHealth,
  useGetProvisioningRepositoriesIdResources,
  useGetProvisioningRepositoriesIdResourcesResourceIdSecrets,
  useGetProvisioningRepositoriesIdResourcesResourceIdDocument,
  useGetProvisioningRepositoriesIdJobs,
  useGetProvisioningRepositoriesIdJobsJobId,
  useGetProvisioningRepositoriesIdEvents,
  useGetProvisioningEvents,
  useDeleteProvisioningEvents,
  useGetProvisioningInstance,
  useGetProvisioningScheduler,
  usePostProvisioningRepositoriesIdSync,
  usePostProvisioningRepositoriesIdResourcesResourceIdSync,
  usePostProvisioningRepositoriesIdResourcesResourceIdPreview,
  usePostProvisioningRepositoriesIdResourcesResourceIdPush,
} from '@/generated-rq/provisioning/provisioning';
import {
  connectionsFromWire,
  discoveredFromWire,
  eventsFromWire,
  healthFromWire,
  instanceSummaryFromWire,
  jobFromWire,
  jobsFromWire,
  repositoriesFromWire,
  resourcesFromWire,
  secretReadinessFromWire,
} from '@vyzorix/api-client';

const SYNC_POLL_MS = 3_000;

export function useInvalidateProvisioning() {
  const queryClient = useQueryClient();

  return () =>
    queryClient.invalidateQueries({
      predicate: (query) =>
        query.queryKey.some((part) => typeof part === 'string' && part.startsWith('/provisioning/')),
    });
}

interface InvalidatableMutation {
  mutateAsync: (...args: never[]) => Promise<unknown>;
}

function useInvalidating<TMutation extends InvalidatableMutation>(
  mutation: TMutation,
  invalidate: () => Promise<unknown>,
): TMutation {
  return {
    ...mutation,
    mutateAsync: async (...args: Parameters<TMutation['mutateAsync']>) => {
      const result = await mutation.mutateAsync(...args);
      await invalidate();
      return result;
    },
  };
}

export function useProvisioningConnections() {
  const query = useGetProvisioningConnections();

  return { ...query, connections: connectionsFromWire(query.data) };
}

export function useProvisioningConnection(id: string | undefined) {
  const query = useGetProvisioningConnectionsId(id ?? '', { query: { enabled: Boolean(id) } });

  return {
    ...query,
    connection: query.data ? connectionsFromWire({ connections: [query.data] })[0] : undefined,
  };
}

export function useCreateProvisioningConnection() {
  const invalidate = useInvalidateProvisioning();

  return useInvalidating(usePostProvisioningConnections(), invalidate);
}

export function useUpdateProvisioningConnection() {
  const invalidate = useInvalidateProvisioning();

  return useInvalidating(usePatchProvisioningConnectionsId(), invalidate);
}

export function useDeleteProvisioningConnection() {
  const invalidate = useInvalidateProvisioning();

  return useInvalidating(useDeleteProvisioningConnectionsId(), invalidate);
}

export function useCheckProvisioningConnection() {
  const invalidate = useInvalidateProvisioning();

  return useInvalidating(usePostProvisioningConnectionsIdCheck(), invalidate);
}

export function useProvisioningRepositories() {
  const query = useGetProvisioningRepositories();

  return { ...query, repositories: repositoriesFromWire(query.data) };
}

export function useProvisioningRepository(id: string | undefined) {
  const query = useGetProvisioningRepositoriesId(id ?? '', { query: { enabled: Boolean(id) } });

  return {
    ...query,
    repository: query.data ? repositoriesFromWire({ repositories: [query.data] })[0] : undefined,
  };
}

export function useCreateProvisioningRepository() {
  const invalidate = useInvalidateProvisioning();

  return useInvalidating(usePostProvisioningRepositories(), invalidate);
}

export function useUpdateProvisioningRepository() {
  const invalidate = useInvalidateProvisioning();

  return useInvalidating(usePatchProvisioningRepositoriesId(), invalidate);
}

export function useDeleteProvisioningRepository() {
  const invalidate = useInvalidateProvisioning();

  return useInvalidating(useDeleteProvisioningRepositoriesId(), invalidate);
}

export function useProvisioningDiscovery(repositoryId: string | undefined) {
  const query = useGetProvisioningRepositoriesIdDiscovery(repositoryId ?? '', {
    query: { enabled: Boolean(repositoryId) },
  });

  return { ...query, discovered: discoveredFromWire(query.data) };
}

export function useProvisioningHealth(repositoryId: string | undefined, limit = 20) {
  const query = useGetProvisioningRepositoriesIdHealth(repositoryId ?? '', { limit }, {
    query: { enabled: Boolean(repositoryId) },
  });

  return { ...query, health: healthFromWire(query.data) };
}

export function useProvisioningResources(repositoryId: string | undefined) {
  const query = useGetProvisioningRepositoriesIdResources(repositoryId ?? '', {
    query: { enabled: Boolean(repositoryId) },
  });

  return { ...query, resources: resourcesFromWire(query.data) };
}

export function useProvisioningResourceSecrets(
  repositoryId: string | undefined,
  resourceId: string | undefined,
) {
  const query = useGetProvisioningRepositoriesIdResourcesResourceIdSecrets(repositoryId ?? '', resourceId ?? '', {
    query: { enabled: Boolean(repositoryId && resourceId) },
  });

  return { ...query, readiness: secretReadinessFromWire(query.data) };
}

export function useProvisioningResourceDocument(
  repositoryId: string | undefined,
  resourceId: string | undefined,
) {
  return useGetProvisioningRepositoriesIdResourcesResourceIdDocument(repositoryId ?? '', resourceId ?? '', {
    query: { enabled: Boolean(repositoryId && resourceId) },
  });
}

export function useProvisioningJobs(repositoryId: string | undefined, limit = 20) {
  const query = useGetProvisioningRepositoriesIdJobs(repositoryId ?? '', { limit }, {
    query: { enabled: Boolean(repositoryId), refetchInterval: SYNC_POLL_MS },
  });

  return { ...query, jobs: jobsFromWire(query.data) };
}

export function useProvisioningJob(repositoryId: string | undefined, jobId: string | undefined) {
  const query = useGetProvisioningRepositoriesIdJobsJobId(repositoryId ?? '', jobId ?? '', {
    query: { enabled: Boolean(repositoryId && jobId) },
  });

  return { ...query, job: query.data ? jobFromWire(query.data) : undefined };
}

export function useProvisioningRepositoryEvents(repositoryId: string | undefined, limit = 20) {
  const query = useGetProvisioningRepositoriesIdEvents(repositoryId ?? '', { limit }, {
    query: { enabled: Boolean(repositoryId) },
  });

  return { ...query, events: eventsFromWire(query.data) };
}

export function useProvisioningEvents(limit = 20) {
  const query = useGetProvisioningEvents({ limit });

  return { ...query, events: eventsFromWire(query.data) };
}

export function useDismissProvisioningEvents() {
  const invalidate = useInvalidateProvisioning();

  return useInvalidating(useDeleteProvisioningEvents(), invalidate);
}

export function useProvisioningInstance() {
  const query = useGetProvisioningInstance();

  return { ...query, summary: instanceSummaryFromWire(query.data) };
}

export function useProvisioningScheduler() {
  return useGetProvisioningScheduler({
    query: { refetchInterval: 15_000 },
  });
}

export function useSyncProvisioningRepository() {
  const invalidate = useInvalidateProvisioning();
  const mutation = useInvalidating(usePostProvisioningRepositoriesIdSync(), invalidate);

  return {
    ...mutation,
    mutateAsync: async (...args: Parameters<typeof mutation.mutateAsync>) => {
      const job = await mutation.mutateAsync(...args);
      return jobFromWire(job);
    },
  };
}

export function useSyncProvisionedResource() {
  const invalidate = useInvalidateProvisioning();
  const mutation = useInvalidating(usePostProvisioningRepositoriesIdResourcesResourceIdSync(), invalidate);

  return {
    ...mutation,
    mutateAsync: async (...args: Parameters<typeof mutation.mutateAsync>) => {
      const job = await mutation.mutateAsync(...args);
      return jobFromWire(job);
    },
  };
}

export function usePreviewManifestEdit() {
  return usePostProvisioningRepositoriesIdResourcesResourceIdPreview();
}

export function usePushManifestEdit() {
  const invalidate = useInvalidateProvisioning();

  return useInvalidating(usePostProvisioningRepositoriesIdResourcesResourceIdPush(), invalidate);
}

export {
  actorKindLabel,
  devicePresenceLabel,
  providerLabel,
  resourceKindLabel,
  resourceStateLabel,
  syncStateLabel,
  PROVIDER_TYPES,
  CONNECTION_STATUSES,
  SYNC_STATES,
  RESOURCE_STATES,
  RESOURCE_KINDS,
  MIN_SYNC_INTERVAL_SECONDS,
  DEFAULT_SYNC_INTERVAL_SECONDS,
  isDeviceKind,
  inScope,
  parsePatternList,
  formatPatternList,
  validateScopePatterns,
  validateRepositoryUrl,
  validateConnectionDraft,
  validateRepositoryDraft,
  type ActorKind,
  type ConnectionStatus,
  type DeclaredSecret,
  type DiscoveredResource,
  type HealthTimelinePoint,
  type JobOutcome,
  type ProvisioningConnection,
  type ProvisioningEvent,
  type ProvisioningInstanceSummary,
  type ProvisioningRepository,
  type ProvisionedResource,
  type RepositorySyncHealth,
  type RepositorySyncState,
  type ResourceKind,
  type ResourceStats,
  type ResourceState,
  type SecretReadiness,
  type SyncJob,
  type SyncJobResult,
  editPreviewFromWire,
  editResultFromWire,
  repoTreeFromWire,
  repoFileFromWire,
  repoHistoryFromWire,
  pathCrumbs,
  parentDir,
  type ManifestPreview,
  type ManifestEditResult,
  type RepoEntry,
  type RepoTree,
  type RepoFile,
  type RepoCommit,
  type RepoHistory,
} from '@vyzorix/api-client';
