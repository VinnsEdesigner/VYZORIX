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

  themeId: ThemeId;

  resolvedThemeId: ThemeId;

  theme: VyzorixTheme;

  followSystem: boolean;

  themeSettled: boolean;

  setTheme: (id: string) => void;

  setFollowSystem: (follow: boolean) => void;

  applyToDocument: () => void;

  adoptServerTheme: (id: string | null | undefined) => void;

  adoptCachedTheme: (id: string | null | undefined) => void;
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
    themeSettled: false,

    setTheme: (id) => {
      const themeId = resolveThemeId(id);
      writeCachedThemeId(themeId);
      set({ themeId, resolvedThemeId: themeId, theme: getThemeById(themeId), themeSettled: true });
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
      set({ themeId, resolvedThemeId: themeId, theme: getThemeById(themeId), themeSettled: true });
    },

    adoptCachedTheme: (id) => {
      if (get().themeSettled) return;
      if (id === null || id === undefined || id === '') return;
      const themeId = resolveThemeId(id);
      set({ themeId, resolvedThemeId: themeId, theme: getThemeById(themeId) });
    },
  };
});

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
