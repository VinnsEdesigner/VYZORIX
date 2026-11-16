import { useEffect } from 'react';
import { useGetMePreferences, usePatchMePreferences } from '@/generated-rq/settings/user-settings';
import { useAuthStore } from '@/stores';
import { useThemeStore } from '@/stores/theme-store';
import { isThemeId, resolveThemeId } from '@/lib/theme';

const THEME_KEY = 'theme';

/**
 * Sync the theme with the user's server preference.
 *
 * The server resolves the cascade (user preference > organisation default >
 * system default) and returns the effective theme as `theme` on the
 * preferences result. The client adopts that value, so a member with no
 * personal preference inherits the organisation's default without needing to
 * read the organisation's settings — which a non-admin member cannot.
 *
 * The store remains the source of truth for rendering, so the network
 * round-trip never blocks the visual change.
 */
export function useThemePreference() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const { data } = useGetMePreferences({ query: { enabled: isAuthenticated } });

  // `theme` is the server-resolved effective theme; the raw preference value is
  // not used for rendering, because it cannot distinguish "unset" from "chose
  // the default" and would defeat the org tier.
  const serverTheme = data?.theme;

  useEffect(() => {
    if (!isAuthenticated) return;
    if (!isThemeId(serverTheme)) return;
    useThemeStore.getState().adoptServerTheme(resolveThemeId(serverTheme));
  }, [isAuthenticated, serverTheme]);
}

/**
 * Persist a theme change to the user's preference.
 *
 * Applies locally first (instant, no flash), then PATCHes. A failed write
 * leaves the local choice in place; the next load reconciles from the server.
 */
export function useUpdateThemePreference() {
  const mutation = usePatchMePreferences();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  const setThemePreference = (id: string) => {
    useThemeStore.getState().setTheme(id);
    if (!isAuthenticated) return;
    mutation.mutate({ data: { preferences: { [THEME_KEY]: resolveThemeId(id) } } });
  };

  return { setThemePreference, isSaving: mutation.isPending, error: mutation.error };
}
