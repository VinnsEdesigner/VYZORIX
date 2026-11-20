import { useEffect, useCallback, useRef } from 'react';
import { authContext } from '@vyzorix/api-client';
import { useAuthStore } from '@/stores/auth-store';
import { usePreferencesStore } from '@/stores/preferences-store';

const ACTIVITY_EVENTS = ['mousedown', 'keydown', 'scroll', 'touchstart'] as const;

export interface UseAuthSessionOptions {

  sessionTimeoutMs?: number;

  disableActivityWatch?: boolean;
}

export interface UseAuthSessionResult {
  refreshSession: () => Promise<void>;
}

export function useAuthSession(
  options: UseAuthSessionOptions = {},
): UseAuthSessionResult {
  const { sessionTimeoutMs: overrideMs, disableActivityWatch = false } = options;
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const clear = useAuthStore((s) => s.clear);
  const refreshTokens = useAuthStore((s) => s.refreshTokens);
  const idleTimeoutMinutes = usePreferencesStore((s) => s.idleTimeoutMinutes);
  const activityTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const sessionTimeoutMs = overrideMs ?? idleTimeoutMinutes * 60 * 1000;

  const refreshSession = useCallback(async () => {
    await refreshTokens();
  }, [refreshTokens]);

  useEffect(() => {
    if (disableActivityWatch || !isAuthenticated || typeof window === 'undefined') {
      return;
    }
    if (sessionTimeoutMs === 0) return;

    const resetTimer = () => {
      if (activityTimerRef.current) {
        clearTimeout(activityTimerRef.current);
      }
      activityTimerRef.current = setTimeout(() => {
        clear();
      }, sessionTimeoutMs);
    };

    ACTIVITY_EVENTS.forEach((event) => window.addEventListener(event, resetTimer));
    resetTimer();

    return () => {
      ACTIVITY_EVENTS.forEach((event) => window.removeEventListener(event, resetTimer));
      if (activityTimerRef.current) {
        clearTimeout(activityTimerRef.current);
        activityTimerRef.current = null;
      }
    };
  }, [isAuthenticated, sessionTimeoutMs, disableActivityWatch, clear]);

  useEffect(() => {
    const unsubscribe = authContext.onTokenRefresh(() => {

    });
    return unsubscribe;
  }, []);

  return { refreshSession };
}
