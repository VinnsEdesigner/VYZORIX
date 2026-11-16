export type LocalStoreCapability = 'ready' | 'unsupported' | 'disabled';

const RETRY_AFTER_MS = 7 * 24 * 60 * 60 * 1000;

const STORAGE_KEYS = {
  ready: 'vyzorix.localStore.opfsReady',
  disabled: 'vyzorix.localStore.disableUntil',
} as const;

function isBrowser(): boolean {
  return typeof window !== 'undefined' && typeof Worker !== 'undefined';
}

function safeGet(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch (_ignored) {
    return null;
  }
}

function safeSet(key: string, value: string): void {
  try {
    window.localStorage.setItem(key, value);
  } catch (_ignored) {
    void _ignored;
  }
}

function safeRemove(key: string): void {
  try {
    window.localStorage.removeItem(key);
  } catch (_ignored) {
    void _ignored;
  }
}

export function disableLocalStore(): void {
  if (!isBrowser()) return;
  safeRemove(STORAGE_KEYS.ready);
  safeSet(STORAGE_KEYS.disabled, String(Date.now() + RETRY_AFTER_MS));
}

export function isLocalStoreDisabled(): boolean {
  if (!isBrowser()) return true;

  const until = safeGet(STORAGE_KEYS.disabled);
  if (!until) return false;

  const expiry = Number(until);
  if (!Number.isFinite(expiry) || expiry <= Date.now()) {
    safeRemove(STORAGE_KEYS.disabled);
    return false;
  }

  return true;
}

async function wasmCompiles(): Promise<boolean> {
  try {

    await WebAssembly.compile(Uint8Array.of(0, 97, 115, 109, 1, 0, 0, 0));
    return true;
  } catch (_ignored) {
    return false;
  }
}

const OPFS_PROBE_SOURCE = `
self.onmessage = function () {
  (async function () {
    const root = await navigator.storage.getDirectory();
    const handle = await root.getFileHandle('_vyzorix_probe_.bin', { create: true });
    const access = await handle.createSyncAccessHandle();
    const size = access.getSize();
    access.close();
    await root.removeEntry('_vyzorix_probe_.bin');
    self.postMessage({ ok: true, size: size });
  })().catch(function (err) {
    self.postMessage({ ok: false, error: String((err && err.message) || err) });
  });
};
`;

function opfsRoundTrips(timeoutMs = 5000): Promise<boolean> {
  return new Promise((resolve) => {
    let settled = false;
    let blobUrl = '';
    let worker: Worker | null = null;

    const finish = (ok: boolean): void => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      if (worker) worker.terminate();
      if (blobUrl) URL.revokeObjectURL(blobUrl);
      resolve(ok);
    };

    const timer = setTimeout(() => finish(false), timeoutMs);

    try {
      blobUrl = URL.createObjectURL(new Blob([OPFS_PROBE_SOURCE], { type: 'application/javascript' }));
      worker = new Worker(blobUrl);

      worker.onmessage = (event: MessageEvent) => {
        const data = event.data as { ok?: boolean; size?: number } | undefined;

        finish(Boolean(data && data.ok && data.size === 0));
      };
      worker.onerror = () => finish(false);
      worker.postMessage({});
    } catch (_ignored) {
      finish(false);
    }
  });
}

export async function probeLocalStoreCapability(): Promise<LocalStoreCapability> {
  if (!isBrowser()) return 'unsupported';
  if (isLocalStoreDisabled()) return 'disabled';
  if (safeGet(STORAGE_KEYS.ready) === '1') return 'ready';

  const usable = (await wasmCompiles()) && (await opfsRoundTrips());

  if (!usable) {
    disableLocalStore();
    return 'unsupported';
  }

  safeSet(STORAGE_KEYS.ready, '1');
  return 'ready';
}

export function resetLocalStoreCapabilityCache(): void {
  if (!isBrowser()) return;
  safeRemove(STORAGE_KEYS.ready);
  safeRemove(STORAGE_KEYS.disabled);
}
