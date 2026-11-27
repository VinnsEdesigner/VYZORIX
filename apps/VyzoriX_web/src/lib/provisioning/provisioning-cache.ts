import type { LocalStoreRepository } from '@/lib/local-store';
import type { RepoFile, RepoTree, ProvisionedResource } from '@/hooks/provisioning';

export const PROVISIONING_CACHE_TTL_MS = 5 * 60_000;

export const PROVISIONING_CACHE_MAX_ROWS = 200;

interface CachedRow {
  key: string;
  payload: string;
  fetched_at: number;
}

function treeKey(repositoryId: string, dir: string): string {
  return `${repositoryId}:${dir}`;
}

function fileKey(repositoryId: string, path: string): string {
  return `${repositoryId}:${path}`;
}

function resourcesKey(repositoryId: string): string {
  return `${repositoryId}:resources`;
}

function historyKey(repositoryId: string): string {
  return `${repositoryId}:history`;
}

function isFresh(row: CachedRow, now: number): boolean {
  return now - row.fetched_at <= PROVISIONING_CACHE_TTL_MS;
}

async function readOne<T>(
  store: LocalStoreRepository | null,
  table: string,
  key: string,
): Promise<T | undefined> {
  if (!store) return undefined;

  try {
    const rows = await store.select<CachedRow>(table, { where: { key }, limit: 1 });
    const row = rows[0];
    if (!row || !isFresh(row, Date.now())) return undefined;

    return JSON.parse(row.payload) as T;
  } catch {

    return undefined;
  }
}

async function writeOne(
  store: LocalStoreRepository | null,
  table: string,
  key: string,
  organizationId: string,
  value: unknown,
): Promise<void> {
  if (!store) return;

  try {
    await store.upsert(table, [
      {
        key,
        repository_id: key.split(':')[0] ?? '',
        organization_id: organizationId,
        fetched_at: Date.now(),
        payload: JSON.stringify(value),
      },
    ]);
    await evictOldest(store, table);
  } catch {
    void 0;
  }
}

async function evictOldest(store: LocalStoreRepository, table: string): Promise<void> {
  const count = await store.count(table);
  if (count <= PROVISIONING_CACHE_MAX_ROWS) return;

  const rows = await store.select<CachedRow>(table, {
    orderBy: { column: 'fetched_at', direction: 'asc' },
    limit: count - PROVISIONING_CACHE_MAX_ROWS,
  });

  const keys = rows.map((row) => row.key).filter(Boolean);
  if (keys.length > 0) await store.deleteByKeys(table, keys);
}

export function readCachedTree(store: LocalStoreRepository | null, repositoryId: string, dir: string) {
  return readOne<RepoTree>(store, 'provisioning_tree', treeKey(repositoryId, dir));
}

export function writeCachedTree(
  store: LocalStoreRepository | null,
  repositoryId: string,
  organizationId: string,
  dir: string,
  tree: RepoTree,
): Promise<void> {
  return writeOne(store, 'provisioning_tree', treeKey(repositoryId, dir), organizationId, tree);
}

export function readCachedFile(store: LocalStoreRepository | null, repositoryId: string, path: string) {
  return readOne<RepoFile>(store, 'provisioning_file', fileKey(repositoryId, path));
}

export function writeCachedFile(
  store: LocalStoreRepository | null,
  repositoryId: string,
  organizationId: string,
  path: string,
  file: RepoFile,
): Promise<void> {
  return writeOne(store, 'provisioning_file', fileKey(repositoryId, path), organizationId, file);
}

export function readCachedResources(store: LocalStoreRepository | null, repositoryId: string) {
  return readOne<ProvisionedResource[]>(store, 'provisioning_resources', resourcesKey(repositoryId));
}

export function writeCachedResources(
  store: LocalStoreRepository | null,
  repositoryId: string,
  organizationId: string,
  resources: ProvisionedResource[],
): Promise<void> {
  return writeOne(store, 'provisioning_resources', resourcesKey(repositoryId), organizationId, resources);
}

export function readCachedHistory(store: LocalStoreRepository | null, repositoryId: string) {
  return readOne<{ commits: unknown[]; depth: number }>(
    store,
    'provisioning_history',
    historyKey(repositoryId),
  );
}

export function writeCachedHistory(
  store: LocalStoreRepository | null,
  repositoryId: string,
  organizationId: string,
  history: { commits: unknown[]; depth: number },
): Promise<void> {
  return writeOne(store, 'provisioning_history', historyKey(repositoryId), organizationId, history);
}

export async function dropProvisioningCache(
  store: LocalStoreRepository | null,
  repositoryId: string,
): Promise<void> {
  if (!store) return;

  for (const table of [
    'provisioning_tree',
    'provisioning_file',
    'provisioning_resources',
    'provisioning_history',
  ]) {
    try {
      const rows = await store.select<CachedRow>(table, { where: { repository_id: repositoryId } });
      const keys = rows.map((row) => row.key).filter(Boolean);
      if (keys.length > 0) await store.deleteByKeys(table, keys);
    } catch {
      void 0;
    }
  }
}
