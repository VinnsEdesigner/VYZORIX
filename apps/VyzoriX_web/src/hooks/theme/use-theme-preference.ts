import { useEffect } from 'react';
import { useGetMePreferences, usePatchMePreferences } from '@/generated-rq/settings/user-settings';
import { useAuthStore } from '@/stores';
import { useThemeStore } from '@/stores/theme-store';
import { isThemeId, resolveThemeId } from '@/lib/theme';

const THEME_KEY = 'theme';

export function useThemePreference() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const { data } = useGetMePreferences({ query: { enabled: isAuthenticated } });

  const serverTheme = data?.theme;

  useEffect(() => {
    if (!isAuthenticated) return;
    if (!isThemeId(serverTheme)) return;
    useThemeStore.getState().adoptServerTheme(resolveThemeId(serverTheme));
  }, [isAuthenticated, serverTheme]);
}

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
