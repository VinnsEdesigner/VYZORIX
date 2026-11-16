import { useEffect, useState } from 'react';
import { useAuthStore } from '@/stores';
import { disposeLocalStore, getLocalStore, type LocalStoreRepository } from '@/lib/local-store';

export function useCurrentUserId(): string | null {
  return useAuthStore((s) => s.user?.id ?? null);
}

/**
 * Resolve the local store for the signed-in user, and dispose it on sign-out.
 *
 * The store is scoped to the user id, so one browser profile cannot serve one
 * user's cached devices or logs to another. Sign-out disposes it, which both
 * closes the worker and clears the cached rows — otherwise the next person to
 * use this browser would read the previous user's data.
 *
 * Returns null while signed out or before the store resolves. Callers treat a
 * null store as a cache miss and read from the network.
 */
export function useLocalStore(): LocalStoreRepository | null {
  const userId = useCurrentUserId();
  const [store, setStore] = useState<LocalStoreRepository | null>(null);

  useEffect(() => {
    if (!userId) {
      // Signed out: drop whatever the previous session left behind.
      void disposeLocalStore();
      setStore(null);
      return;
    }

    let cancelled = false;

    void getLocalStore(userId).then((resolved) => {
      if (!cancelled) setStore(resolved);
    });

    return () => {
      cancelled = true;
    };
  }, [userId]);

  return store;
}

