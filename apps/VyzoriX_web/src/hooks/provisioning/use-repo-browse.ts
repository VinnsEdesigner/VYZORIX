import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  useGetProvisioningRepositoriesIdTree,
  useGetProvisioningRepositoriesIdFile,
  useGetProvisioningRepositoriesIdHistory,
} from '@/generated-rq/provisioning/provisioning';
import {
  repoTreeFromWire,
  repoFileFromWire,
  repoHistoryFromWire,
  type RepoFile,
  type RepoHistory,
} from '@vyzorix/api-client';
import { useLocalStore, useCurrentOrganizationId } from '@/hooks/_shared';
import {
  dropProvisioningCache,
  readCachedFile,
  readCachedHistory,
  readCachedTree,
  writeCachedFile,
  writeCachedHistory,
  writeCachedTree,
} from '@/lib/provisioning/provisioning-cache';

function useCacheSeed<T>(queryKey: readonly unknown[], load: () => Promise<T | undefined>, enabled: boolean) {
  const queryClient = useQueryClient();
  const key = JSON.stringify(queryKey);

  useEffect(() => {
    if (!enabled) return;

    let cancelled = false;
    void load().then((cached) => {
      if (cancelled || cached === undefined) return;
      if (queryClient.getQueryData(queryKey) === undefined) {
        queryClient.setQueryData(queryKey, cached);
      }
    });

    return () => {
      cancelled = true;
    };

  }, [enabled, queryClient, key]);
}

export function useRepositoryTree(repositoryId: string | undefined, dir: string) {
  const store = useLocalStore();
  const organizationId = useCurrentOrganizationId() ?? '';

  const query = useGetProvisioningRepositoriesIdTree(repositoryId ?? '', { dir }, {
    query: { enabled: Boolean(repositoryId) },
  });

  const tree = repoTreeFromWire(query.data);

  useCacheSeed(
    ['getProvisioningRepositoriesIdTree', repositoryId, dir],
    () => readCachedTree(store, repositoryId ?? '', dir),
    Boolean(repositoryId),
  );

  useEffect(() => {
    if (!repositoryId || !query.isSuccess) return;
    void writeCachedTree(store, repositoryId, organizationId, dir, repoTreeFromWire(query.data));
  }, [repositoryId, organizationId, dir, query.isSuccess, query.data, store]);

  return { ...query, tree };
}

export function useRepositoryFile(repositoryId: string | undefined, path: string | undefined) {
  const store = useLocalStore();
  const organizationId = useCurrentOrganizationId() ?? '';

  const query = useGetProvisioningRepositoriesIdFile(repositoryId ?? '', { path: path ?? '' }, {
    query: { enabled: Boolean(repositoryId && path) },
  });

  const file: RepoFile | undefined = query.data ? repoFileFromWire(query.data) : undefined;

  useCacheSeed(
    ['getProvisioningRepositoriesIdFile', repositoryId, path],
    () => readCachedFile(store, repositoryId ?? '', path ?? ''),
    Boolean(repositoryId && path),
  );

  useEffect(() => {
    if (!repositoryId || !path || !query.isSuccess || !query.data) return;
    void writeCachedFile(store, repositoryId, organizationId, path, repoFileFromWire(query.data));
  }, [repositoryId, organizationId, path, query.isSuccess, query.data, store]);

  return { ...query, file };
}

export function useRepositoryHistory(repositoryId: string | undefined, limit = 20) {
  const store = useLocalStore();
  const organizationId = useCurrentOrganizationId() ?? '';

  const query = useGetProvisioningRepositoriesIdHistory(repositoryId ?? '', { limit }, {
    query: { enabled: Boolean(repositoryId) },
  });

  const history: RepoHistory = repoHistoryFromWire(query.data);

  useCacheSeed(
    ['getProvisioningRepositoriesIdHistory', repositoryId, limit],
    () => readCachedHistory(store, repositoryId ?? '') as Promise<RepoHistory | undefined>,
    Boolean(repositoryId),
  );

  useEffect(() => {
    if (!repositoryId || !query.isSuccess || !query.data) return;
    void writeCachedHistory(store, repositoryId, organizationId, repoHistoryFromWire(query.data));
  }, [repositoryId, organizationId, query.isSuccess, query.data, store]);

  return { ...query, history };
}

export function useDropProvisioningCache() {
  const store = useLocalStore();

  return (repositoryId: string) => dropProvisioningCache(store, repositoryId);
}
