import { useEffect, useRef } from 'react';
import { getAuthMe } from '@/generated-rq/auth/auth-session';
import { useAuthStore } from '@/stores';

export function useSessionBootstrap(): void {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const hasUser = useAuthStore((s) => s.user !== null);
  const markSessionChecked = useAuthStore((s) => s.markSessionChecked);
  const attempted = useRef(false);

  useEffect(() => {
    if (attempted.current) return;
    if (isAuthenticated && hasUser) {

      markSessionChecked();
      return;
    }

    attempted.current = true;

    let cancelled = false;
    void (async () => {
      try {
        const me = await getAuthMe();
        if (!cancelled && me) {
          useAuthStore.getState().setFromMeResponse(me);
        }
      } catch {
        void 0;
      } finally {

        if (!cancelled) markSessionChecked();
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [isAuthenticated, hasUser, markSessionChecked]);
}
