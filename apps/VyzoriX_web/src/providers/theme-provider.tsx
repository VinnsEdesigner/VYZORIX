import { useEffect, type ReactNode } from 'react';
import { ThemeContext, injectGlobalStyles, type VyzorixTheme } from '@vyzorix/ui';
import { useTheme } from '@/hooks/theme/use-theme';
import { useThemePreference } from '@/hooks/theme/use-theme-preference';
import { useLocalStore, useCurrentUserId } from '@/hooks/_shared';
import { readCachedTheme, writeCachedTheme } from '@/lib/theme';
import { useThemeStore } from '@/stores/theme-store';

/**
 * Provides the theme object and keeps it in step with the user's preference.
 *
 * The active theme object is supplied through ThemeContext so `useTheme2` and
 * `useStyles2` can read it anywhere below. Also:
 *  - `useTheme` re-resolves on mount and follows the OS when asked,
 *  - `useThemePreference` reconciles with the server value on load,
 *  - the resolved theme id is mirrored into the durable SQLite cache.
 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  useTheme();
  useThemePreference();

  const theme = useThemeStore((s) => s.theme);
  const themeId = useThemeStore((s) => s.themeId);
  const store = useLocalStore();
  const userId = useCurrentUserId();

  // Inject the global base styles whenever the theme changes.
  useEffect(() => {
    injectGlobalStyles(theme);
  }, [theme]);

  // Hydrate from the durable cache once available, then keep it in sync.
  useEffect(() => {
    if (!store || !userId) return;
    let cancelled = false;
    void readCachedTheme(store, userId).then((cached) => {
      if (cancelled || !cached) return;
      useThemeStore.getState().adoptServerTheme(cached);
    });
    return () => {
      cancelled = true;
    };
  }, [store, userId]);

  useEffect(() => {
    if (!store || !userId) return;
    void writeCachedTheme(store, userId, themeId);
  }, [store, userId, themeId]);

  return <ThemeContext.Provider value={theme as VyzorixTheme}>{children}</ThemeContext.Provider>;
}
