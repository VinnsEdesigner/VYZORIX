import { probeLocalStoreCapability, resetLocalStoreCapabilityCache } from './capability';
import { localStoreMetrics } from './metrics';
import { createNullLocalStore, createWorkerLocalStore, type LocalStoreRepository } from './repository';

export type {
  LocalStoreRepository,
  SelectOptions,
  StoredRow,
} from './repository';
export { createNullLocalStore, createWorkerLocalStore } from './repository';
export type { LocalStoreCapability } from './capability';
export { resetLocalStoreCapabilityCache } from './capability';

let instance: LocalStoreRepository | null = null;
let instanceScope: string | null = null;
let chain: Promise<void> = Promise.resolve();

/**
 * Wrap a store so that closing it also clears the factory's memo.
 *
 * `disposeLocalStore` is the sanctioned way to close, but `close()` is on the
 * public interface, and a caller that uses it directly would otherwise leave the
 * memo pointing at a closed store — every later `getLocalStore` would return it,
 * reporting no writer and serving empty reads.
 */
function own(store: LocalStoreRepository): LocalStoreRepository {
  const wrapper: LocalStoreRepository = {
    ...store,
    close: async () => {
      await store.close();
      if (instance === wrapper) {
        instance = null;
        instanceScope = null;
      }
    },
  };
  return wrapper;
}

/**
 * Resolve the store for this browser and this user.
 *
 * Resolution is serialised on `chain`: two concurrent callers, or a call that
 * arrives while a previous store is closing, queue behind one another rather
 * than racing to create two workers. Creating two workers would mean two
 * connections to one OPFS file, which is what the cross-tab lock exists to
 * prevent.
 */
export function getLocalStore(scope: string): Promise<LocalStoreRepository> {
  if (!scope) return Promise.resolve(createNullLocalStore());

  const result = chain.then(() => resolveNow(scope));
  chain = result.then(
    () => undefined,
    () => undefined,
  );
  return result;
}


async function resolveNow(scope: string): Promise<LocalStoreRepository> {
  if (instance && instanceScope === scope) return instance;

  if (instance) {
    // A different user: close the old store before opening the new one, so its
    // worker releases the OPFS file and the writer lock.
    await instance.close();
    instance = null;
    instanceScope = null;
  }

  const capability = await probeLocalStoreCapability().catch(() => 'unsupported' as const);
  localStoreMetrics()?.recordCapability(capability);

  if (capability !== 'ready') {
    instance = createNullLocalStore(scope);
    instanceScope = scope;
    return instance;
  }

  const store = createWorkerLocalStore(scope);
  const isWriter = await store.open();

  if (!isWriter) {
    await store.close();
    instance = createNullLocalStore(scope);
    instanceScope = scope;
    return instance;
  }

  const owned = own(store);
  instance = owned;
  instanceScope = scope;
  await owned.evictExpired();

  return owned;
}

export async function disposeLocalStore(): Promise<void> {
  const result = chain.then(async () => {
    if (instance) await instance.close();
    instance = null;
    instanceScope = null;
    resetLocalStoreCapabilityCache();
  });
  chain = result.then(
    () => undefined,
    () => undefined,
  );
  await result;
}

export function resetLocalStoreInstance(): void {
  instance = null;
  instanceScope = null;
}
