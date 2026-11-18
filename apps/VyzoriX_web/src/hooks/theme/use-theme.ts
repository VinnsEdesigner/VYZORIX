import { useEffect } from 'react';
import { subscribeToSystemTheme, useThemeStore } from '@/stores/theme-store';
import { type VyzorixTheme } from '@/lib/theme';

export function useTheme(): {
  themeId: string;
  resolvedThemeId: string;
  theme: VyzorixTheme;
  followSystem: boolean;
  setTheme: (id: string) => void;
  setFollowSystem: (follow: boolean) => void;
} {
  const themeId = useThemeStore((s) => s.themeId);
  const resolvedThemeId = useThemeStore((s) => s.resolvedThemeId);
  const theme = useThemeStore((s) => s.theme);
  const followSystem = useThemeStore((s) => s.followSystem);
  const setTheme = useThemeStore((s) => s.setTheme);
  const setFollowSystem = useThemeStore((s) => s.setFollowSystem);

  useEffect(() => {
    useThemeStore.getState().applyToDocument();
    return subscribeToSystemTheme();
  }, []);

  return { themeId, resolvedThemeId, theme, followSystem, setTheme, setFollowSystem };
}
