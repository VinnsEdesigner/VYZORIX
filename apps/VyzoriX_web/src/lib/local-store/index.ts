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
