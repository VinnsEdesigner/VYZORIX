import { useEffect, useState } from 'react';
import { useAuthStore } from '@/stores';
import { disposeLocalStore, getLocalStore, type LocalStoreRepository } from '@/lib/local-store';

export function useCurrentUserId(): string | null {
  return useAuthStore((s) => s.user?.id ?? null);
}

export interface CurrentUser {
  id: string | null;
  name: string | null;
  email: string | null;
  avatarUrl: string | null;
}

export function useCurrentUser(): CurrentUser {
  const id = useAuthStore((s) => s.user?.id ?? null);
  const name = useAuthStore((s) => s.user?.name ?? null);
  const email = useAuthStore((s) => s.user?.email ?? null);
  const avatarUrl = useAuthStore((s) => s.user?.avatar_url || null);

  return { id, name, email, avatarUrl };
}

export function useLocalStore(): LocalStoreRepository | null {
  const userId = useCurrentUserId();
  const [store, setStore] = useState<LocalStoreRepository | null>(null);

  useEffect(() => {
    if (!userId) {
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
