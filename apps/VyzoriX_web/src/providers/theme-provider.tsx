import { useEffect, type ReactNode } from 'react';
import { ThemeContext, injectGlobalStyles, type VyzorixTheme } from '@vyzorix/ui';
import { useTheme } from '@/hooks/theme/use-theme';
import { useThemePreference } from '@/hooks/theme/use-theme-preference';
import { useDisplayPreferences } from '@/hooks/preferences/use-display-preference';
import { useLocalStore, useCurrentUserId } from '@/hooks/_shared';
import { readCachedTheme, writeCachedTheme } from '@/lib/theme';
import { useThemeStore } from '@/stores/theme-store';

export function ThemeProvider({ children }: { children: ReactNode }) {
  useTheme();
  useThemePreference();
  useDisplayPreferences();

  const theme = useThemeStore((s) => s.theme);
  const themeId = useThemeStore((s) => s.themeId);
  const themeSettled = useThemeStore((s) => s.themeSettled);
  const store = useLocalStore();
  const userId = useCurrentUserId();

  useEffect(() => {
    injectGlobalStyles(theme);
  }, [theme]);

  useEffect(() => {
    if (!store || !userId) return;
    let cancelled = false;
    void readCachedTheme(store, userId).then((cached) => {
      if (cancelled) return;
      useThemeStore.getState().adoptCachedTheme(cached);
    });
    return () => {
      cancelled = true;
    };
  }, [store, userId]);

  useEffect(() => {
    if (!store || !userId || !themeSettled) return;
    void writeCachedTheme(store, userId, themeId);
  }, [store, userId, themeId, themeSettled]);

  return <ThemeContext.Provider value={theme as VyzorixTheme}>{children}</ThemeContext.Provider>;
}
