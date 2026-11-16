import { describe, it, expect, vi, afterEach } from 'vitest';
import {
  getLocalStore,
  disposeLocalStore,
  resetLocalStoreInstance,
  createNullLocalStore,
} from '@/lib/local-store';
import { BOUNDED_WINDOW_DAYS, TABLES } from '@/lib/local-store/schema';

afterEach(async () => {
  await disposeLocalStore();
  vi.unstubAllGlobals();
  vi.resetModules();
});

describe('getLocalStore', () => {
  it('returns the null store when the probe cannot run (no window)', async () => {
    vi.stubGlobal('window', undefined);

    const store = await getLocalStore('user-1');

    expect(store.isWriter()).toBe(false);
    await expect(store.select('devices')).resolves.toEqual([]);
    await expect(store.count('devices')).resolves.toBe(0);
  });

  it('memoises the store, so the probe runs once', async () => {
    vi.stubGlobal('window', undefined);

    const first = await getLocalStore('user-1');
    const second = await getLocalStore('user-1');

    expect(second).toBe(first);
  });

  it('resolves concurrent callers to the same instance', async () => {
    vi.stubGlobal('window', undefined);

    const [a, b] = await Promise.all([getLocalStore('user-1'), getLocalStore('user-1')]);

    expect(a).toBe(b);
  });

  it('falls back rather than throwing when the probe cannot succeed', async () => {
    vi.stubGlobal('window', {});
    vi.stubGlobal('Worker', undefined);

    const store = await getLocalStore('user-1');

    expect(store.isWriter()).toBe(false);
  });

  it('disposeLocalStore forgets the instance', async () => {
    vi.stubGlobal('window', undefined);
    const first = await getLocalStore('user-1');

    await disposeLocalStore();
    const second = await getLocalStore('user-1');

    expect(second).not.toBe(first);
  });
});

describe('the null store contract', () => {
  it('answers every method without a browser API', async () => {
    const store = createNullLocalStore();

    expect(store.isWriter()).toBe(false);
    await expect(store.capability()).resolves.toBe('unsupported');
    await expect(store.open()).resolves.toBe(false);
    await expect(store.select('devices')).resolves.toEqual([]);
    await expect(store.count('devices')).resolves.toBe(0);
    await expect(store.upsert('devices', [{ id: 'x' }])).resolves.toBeUndefined();
    await expect(store.deleteByKeys('devices', ['x'])).resolves.toBeUndefined();
    await expect(store.evictExpired()).resolves.toBe(0);
    await expect(store.cursor()).resolves.toBeNull();
    await expect(store.setCursor('sn-1')).resolves.toBeUndefined();
    await expect(store.clear()).resolves.toBeUndefined();
    await expect(store.close()).resolves.toBeUndefined();
  });
});

describe('the bounded window', () => {
  it('is a positive number of days', () => {
    expect(BOUNDED_WINDOW_DAYS).toBeGreaterThan(0);
  });

  it('is only declared on tables that have an age column', () => {
    for (const table of TABLES.filter((t) => t.bounded)) {
      expect(table.ageColumn, `${table.name} is bounded but has no ageColumn`).toBeTruthy();
      expect(table.columns).toContain(table.ageColumn);
    }
  });

  it('has at least one bounded table, or eviction is untested dead code', () => {
    expect(TABLES.some((t) => t.bounded)).toBe(true);
  });
});

describe('resetLocalStoreInstance', () => {
  it('clears the memoised instance without touching the capability cache', async () => {
    vi.stubGlobal('window', undefined);
    const first = await getLocalStore('user-1');

    resetLocalStoreInstance();
    const second = await getLocalStore('user-1');

    expect(second).not.toBe(first);
  });
});
