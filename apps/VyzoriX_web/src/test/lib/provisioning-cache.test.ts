import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  PROVISIONING_CACHE_MAX_ROWS,
  PROVISIONING_CACHE_TTL_MS,
  dropProvisioningCache,
  readCachedFile,
  readCachedTree,
  writeCachedFile,
  writeCachedTree,
} from '@/lib/provisioning/provisioning-cache';
import type { LocalStoreRepository, StoredRow, SelectOptions } from '@/lib/local-store';

function createFakeStore() {
  const tables = new Map<string, Map<string, StoredRow>>();

  const table = (name: string) => {
    if (!tables.has(name)) tables.set(name, new Map());

    return tables.get(name)!;
  };

  const store: LocalStoreRepository = {
    capability: async () => 'ready',
    open: async () => true,
    isWriter: () => true,
    select: async <T = StoredRow>(name: string, options?: SelectOptions) => {
      let rows = [...table(name).values()];

      for (const [column, value] of Object.entries(options?.where ?? {})) {
        rows = rows.filter((row) => row[column] === value);
      }
      if (options?.orderBy) {
        const { column, direction } = options.orderBy;
        rows.sort((a, b) => {
          const left = Number(a[column] ?? 0);
          const right = Number(b[column] ?? 0);

          return direction === 'asc' ? left - right : right - left;
        });
      }
      if (options?.limit !== undefined) rows = rows.slice(0, options.limit);

      return rows as T[];
    },
    count: async (name) => table(name).size,
    upsert: async (name, rows) => {
      for (const row of rows) table(name).set(String(row.key), { ...row });
    },
    deleteByKeys: async (name, keys) => {
      for (const key of keys) table(name).delete(String(key));
    },
    evictExpired: async () => 0,
    cursor: async () => null,
    setCursor: async () => {},
    clear: async () => {},
    close: async () => {},
  };

  return { store, table };
}

describe('provisioning cache', () => {
  beforeEach(() => {
    vi.useRealTimers();
  });

  it('round-trips a tree listing', async () => {
    const { store } = createFakeStore();
    const tree = { dir: 'devices', entries: [{ path: 'devices/a.toml', name: 'a.toml', kind: 'file' as const, size: 10 }] };

    await writeCachedTree(store, 'repo-1', 'org-1', 'devices', tree);

    expect(await readCachedTree(store, 'repo-1', 'devices')).toEqual(tree);
  });

  it('keys a listing by repository AND directory, so two dirs never collide', async () => {
    const { store } = createFakeStore();

    await writeCachedTree(store, 'repo-1', 'org-1', 'devices', { dir: 'devices', entries: [] });
    await writeCachedTree(store, 'repo-1', 'org-1', 'orgs', { dir: 'orgs', entries: [] });

    expect((await readCachedTree(store, 'repo-1', 'devices'))?.dir).toBe('devices');
    expect((await readCachedTree(store, 'repo-1', 'orgs'))?.dir).toBe('orgs');
  });

  it('keys by repository, so two sources never share a file body', async () => {
    const { store } = createFakeStore();
    const file = {
      path: 'devices/a/vyzorix.toml',
      content: 'from repo 1',
      commit: 'abc',
      message: '',
      author: '',
      when: null,
      truncated: false,
    };

    await writeCachedFile(store, 'repo-1', 'org-1', file.path, file);

    expect(await readCachedFile(store, 'repo-1', file.path)).not.toBeUndefined();
    expect(await readCachedFile(store, 'repo-2', file.path)).toBeUndefined();
  });

  it('treats an entry older than the TTL as absent', async () => {
    const { store, table } = createFakeStore();
    const tree = { dir: '', entries: [] };

    await writeCachedTree(store, 'repo-1', 'org-1', '', tree);

    const row = table('provisioning_tree').get('repo-1:');
    expect(row).not.toBeUndefined();
    row!.fetched_at = Date.now() - PROVISIONING_CACHE_TTL_MS - 1;

    expect(await readCachedTree(store, 'repo-1', '')).toBeUndefined();
  });

  it('drops every entry for one repository and leaves the others alone', async () => {
    const { store } = createFakeStore();
    const tree = { dir: '', entries: [] };

    await writeCachedTree(store, 'repo-1', 'org-1', '', tree);
    await writeCachedTree(store, 'repo-2', 'org-1', '', tree);
    await writeCachedFile(store, 'repo-1', 'org-1', 'a.toml', {
      path: 'a.toml',
      content: 'x',
      commit: 'c',
      message: '',
      author: '',
      when: null,
      truncated: false,
    });

    await dropProvisioningCache(store, 'repo-1');

    expect(await readCachedTree(store, 'repo-1', '')).toBeUndefined();
    expect(await readCachedFile(store, 'repo-1', 'a.toml')).toBeUndefined();
    expect(await readCachedTree(store, 'repo-2', '')).not.toBeUndefined();
  });

  it('bounds a table so a long browse cannot grow forever', async () => {
    const { store, table } = createFakeStore();

    for (let i = 0; i < PROVISIONING_CACHE_MAX_ROWS + 5; i += 1) {
      await writeCachedTree(store, `repo-${i}`, 'org-1', '', { dir: '', entries: [] });
    }

    expect(table('provisioning_tree').size).toBeLessThanOrEqual(PROVISIONING_CACHE_MAX_ROWS);
  });

  it('is a miss, not a crash, when the store is absent', async () => {
    expect(await readCachedTree(null, 'repo-1', '')).toBeUndefined();
    await expect(writeCachedTree(null, 'repo-1', 'org-1', '', { dir: '', entries: [] })).resolves.toBeUndefined();
    await expect(dropProvisioningCache(null, 'repo-1')).resolves.toBeUndefined();
  });

  it('survives a store that throws, so a broken cache never breaks the page', async () => {
    const broken = {
      ...createFakeStore().store,
      select: async () => {
        throw new Error('store unavailable');
      },
    } as LocalStoreRepository;

    expect(await readCachedTree(broken, 'repo-1', '')).toBeUndefined();
  });
});
