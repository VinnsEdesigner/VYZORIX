import { createVyzorStore } from '@/lib/state';
import {
  DEFAULT_THEME_ID,
  getThemeById,
  readCachedThemeId,
  readFollowSystem,
  resolveThemeId,
  writeCachedThemeId,
  writeFollowSystem,
  type ThemeId,
  type VyzorixTheme,
} from '@/lib/theme';

/**
 * The theme store.
 *
 * Holds the active theme id and the resolved theme object (the same object the
 * provider delivers through context), plus whether the user follows the OS
 * preference. It is the only place that writes the theme to the synchronous
 * mirror; server persistence is the theme hooks' job.
 *
 * Resolution: an explicit choice wins; with follow-system on, the OS picks a
 * light or dark theme.
 */

const SYSTEM_DARK: ThemeId = 'dark';
const SYSTEM_LIGHT: ThemeId = 'light';

function systemThemeId(): ThemeId {
  if (typeof window === 'undefined' || !window.matchMedia) return SYSTEM_DARK;
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? SYSTEM_DARK : SYSTEM_LIGHT;
}

function effectiveThemeId(explicitId: ThemeId, followSystem: boolean): ThemeId {
  return followSystem ? systemThemeId() : explicitId;
}

function initialExplicitId(): ThemeId {
  return readCachedThemeId() ?? DEFAULT_THEME_ID;
}

export interface ThemeState {
  /** The user's chosen theme id. */
  themeId: ThemeId;
  /** The theme actually applied (resolved through follow-system). */
  resolvedThemeId: ThemeId;
  /** The built theme object for the resolved id. */
  theme: VyzorixTheme;
  /** Whether the OS preference decides the light/dark theme. */
  followSystem: boolean;
  /** Set the explicit theme. Applies and mirrors; does not touch the server. */
  setTheme: (id: string) => void;
  /** Toggle follow-system and re-resolve. */
  setFollowSystem: (follow: boolean) => void;
  /** Re-resolve the current theme (e.g. after hydration). */
  applyToDocument: () => void;
  /** Adopt a theme id that came from the server without re-persisting it. */
  adoptServerTheme: (id: string | null | undefined) => void;
}

export const useThemeStore = createVyzorStore<ThemeState>('ThemeStore', (set, get) => {
  const explicit = initialExplicitId();
  const follow = readFollowSystem();
  const resolved = effectiveThemeId(explicit, follow);

  return {
    themeId: explicit,
    resolvedThemeId: resolved,
    theme: getThemeById(resolved),
    followSystem: follow,

    setTheme: (id) => {
      const themeId = resolveThemeId(id);
      writeCachedThemeId(themeId);
      set({ themeId, resolvedThemeId: themeId, theme: getThemeById(themeId) });
    },

    setFollowSystem: (followSystem) => {
      writeFollowSystem(followSystem);
      const resolvedThemeId = effectiveThemeId(get().themeId, followSystem);
      set({ followSystem, resolvedThemeId, theme: getThemeById(resolvedThemeId) });
    },

    applyToDocument: () => {
      const { themeId, followSystem } = get();
      const resolvedThemeId = effectiveThemeId(themeId, followSystem);
      set({ resolvedThemeId, theme: getThemeById(resolvedThemeId) });
    },

    adoptServerTheme: (id) => {
      if (id === null || id === undefined || id === '') return;
      const themeId = resolveThemeId(id);
      writeCachedThemeId(themeId);
      set({ themeId, resolvedThemeId: themeId, theme: getThemeById(themeId) });
    },
  };
});

/**
 * Subscribe to OS preference changes so a follow-system theme updates live.
 * Returns an unsubscribe function; a no-op when matchMedia is unavailable (SSR).
 */
export function subscribeToSystemTheme(): () => void {
  if (typeof window === 'undefined' || !window.matchMedia) return () => {};
  const query = window.matchMedia('(prefers-color-scheme: dark)');
  const handler = () => {
    if (useThemeStore.getState().followSystem) {
      useThemeStore.getState().applyToDocument();
    }
  };
  query.addEventListener('change', handler);
  return () => query.removeEventListener('change', handler);
}
