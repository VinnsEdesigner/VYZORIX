import { describe, it, expect, vi, afterEach } from 'vitest';
import {
  probeLocalStoreCapability,
  disableLocalStore,
  isLocalStoreDisabled,
  resetLocalStoreCapabilityCache,
} from '@/lib/local-store/capability';

/** A Worker stub that replies as a working OPFS round trip would. */
function stubWorker(size: number): void {
  vi.stubGlobal(
    'Worker',
    class {
      onmessage: ((event: { data: unknown }) => void) | null = null;
      onerror: (() => void) | null = null;

      postMessage(): void {
        queueMicrotask(() => this.onmessage?.({ data: { ok: true, size } }));
      }

      terminate(): void {}
    }
  );
}

/** A Worker stub that cannot start, standing in for a browser without OPFS. */
function stubBrokenWorker(): void {
  vi.stubGlobal(
    'Worker',
    class {
      constructor() {
        throw new Error('Worker unavailable');
      }
    }
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  resetLocalStoreCapabilityCache();
});

describe('probeLocalStoreCapability', () => {
  it('reports unsupported when there is no window (SSR)', async () => {
    vi.stubGlobal('window', undefined);

    await expect(probeLocalStoreCapability()).resolves.toBe('unsupported');
  });

  it('reports ready when wasm compiles and the OPFS round trip succeeds', async () => {
    stubWorker(0);

    await expect(probeLocalStoreCapability()).resolves.toBe('ready');
  });

  it('reports unsupported when wasm compilation is blocked', async () => {
    stubWorker(0);
    vi.spyOn(WebAssembly, 'compile').mockRejectedValue(new Error('blocked by CSP'));

    await expect(probeLocalStoreCapability()).resolves.toBe('unsupported');
  });

  it('reports unsupported when the probe file is not empty', async () => {
    stubWorker(42);

    await expect(probeLocalStoreCapability()).resolves.toBe('unsupported');
  });

  it('reports unsupported when the worker cannot start', async () => {
    stubBrokenWorker();

    await expect(probeLocalStoreCapability()).resolves.toBe('unsupported');
  });

  it('caches a success, so a second probe does not redo the work', async () => {
    stubWorker(0);
    const compile = vi.spyOn(WebAssembly, 'compile');

    await expect(probeLocalStoreCapability()).resolves.toBe('ready');
    const callsAfterFirst = compile.mock.calls.length;

    await expect(probeLocalStoreCapability()).resolves.toBe('ready');
    expect(compile.mock.calls.length).toBe(callsAfterFirst);
  });

  it('caches a failure, so it is not retried on every load', async () => {
    stubBrokenWorker();

    await expect(probeLocalStoreCapability()).resolves.toBe('unsupported');
    await expect(probeLocalStoreCapability()).resolves.toBe('disabled');
  });
});

describe('local store kill-switch', () => {
  it('suppresses the probe once disabled', async () => {
    stubWorker(0);

    disableLocalStore();

    expect(isLocalStoreDisabled()).toBe(true);
    await expect(probeLocalStoreCapability()).resolves.toBe('disabled');
  });

  it('clears the suppression once the expiry has passed', () => {
    stubWorker(0);

    localStorage.setItem('vyzorix.localStore.disableUntil', String(Date.now() - 1000));

    expect(isLocalStoreDisabled()).toBe(false);
    expect(localStorage.getItem('vyzorix.localStore.disableUntil')).toBeNull();
  });

  it('treats a disabled store as disabled on the server', () => {
    vi.stubGlobal('window', undefined);

    expect(isLocalStoreDisabled()).toBe(true);
  });
});
