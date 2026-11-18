import { describe, it, expect, vi, afterEach } from 'vitest';
import { probeLocalStoreCapability, isLocalStoreDisabled } from '@/lib/local-store/capability';
import { createNullLocalStore, createWorkerLocalStore } from '@/lib/local-store/index';

describe('SSR safety', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('probes as unsupported with no window, and does not throw', async () => {
    vi.stubGlobal('window', undefined);
    await expect(probeLocalStoreCapability()).resolves.toBe('unsupported');
    expect(isLocalStoreDisabled()).toBe(true);
  });

  it('the null store answers every call without touching a browser API', async () => {
    const store = createNullLocalStore();
    await expect(store.capability()).resolves.toBe('unsupported');
    expect(store.isWriter()).toBe(false);
    await expect(store.open()).resolves.toBe(false);
    await expect(store.select('devices')).resolves.toEqual([]);
    await expect(store.count('devices')).resolves.toBe(0);
    await expect(store.upsert('devices', [{ id: 'x' }])).resolves.toBeUndefined();
    await expect(store.deleteByKeys('devices', ['x'])).resolves.toBeUndefined();
    await expect(store.evictExpired()).resolves.toBe(0);
    await expect(store.cursor()).resolves.toBeNull();
    await expect(store.setCursor('abc')).resolves.toBeUndefined();
    await expect(store.clear()).resolves.toBeUndefined();
    await expect(store.close()).resolves.toBeUndefined();
  });

  it('creating the worker store does no work until first use', () => {
    vi.stubGlobal('window', undefined);

    expect(() => createWorkerLocalStore('user-1')).not.toThrow();
  });
});
