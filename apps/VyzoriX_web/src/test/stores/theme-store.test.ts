import { describe, it, expect, beforeEach, vi } from 'vitest';
import { useThemeStore } from '@/stores/theme-store';
import { getThemeById } from '@vyzorix/ui';

function resetStore() {
  localStorage.clear();
  useThemeStore.setState({
    themeId: 'dark',
    resolvedThemeId: 'dark',
    theme: getThemeById('dark'),
    followSystem: false,
    themeSettled: false,
  });
}

describe('useThemeStore', () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
    resetStore();
  });

  it('defaults to the dark theme', () => {
    expect(useThemeStore.getState().themeId).toBe('dark');
    expect(useThemeStore.getState().theme.isDark).toBe(true);
  });

  it('setTheme swaps the theme object', () => {
    useThemeStore.getState().setTheme('tron');
    expect(useThemeStore.getState().themeId).toBe('tron');
    expect(useThemeStore.getState().theme.isDark).toBe(true);
    expect(useThemeStore.getState().theme.name).toBe('Tron');
  });

  it('setTheme persists to the localStorage mirror', () => {
    useThemeStore.getState().setTheme('zen');
    expect(localStorage.getItem('vyzorix.theme')).toBe('zen');
  });

  it('setTheme rejects an unknown id and falls back to the default', () => {
    useThemeStore.getState().setTheme('not-a-theme');
    expect(useThemeStore.getState().themeId).toBe('dark');
  });

  it('setTheme switches to a light theme object', () => {
    useThemeStore.getState().setTheme('light');
    expect(useThemeStore.getState().theme.isLight).toBe(true);
  });

  it('followSystem resolves to light when the OS prefers light', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false }));
    useThemeStore.getState().setTheme('tron');
    useThemeStore.getState().setFollowSystem(true);
    expect(useThemeStore.getState().resolvedThemeId).toBe('light');
    expect(useThemeStore.getState().theme.isLight).toBe(true);
  });

  it('followSystem resolves to dark when the OS prefers dark', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }));
    useThemeStore.getState().setFollowSystem(true);
    expect(useThemeStore.getState().resolvedThemeId).toBe('dark');
  });

  it('setFollowSystem persists the flag', () => {
    useThemeStore.getState().setFollowSystem(true);
    expect(localStorage.getItem('vyzorix.theme.followSystem')).toBe('true');
  });

  it('adoptServerTheme applies a server value', () => {
    useThemeStore.getState().adoptServerTheme('synthwave');
    expect(useThemeStore.getState().themeId).toBe('synthwave');
    expect(useThemeStore.getState().theme.name).toBe('Synthwave');
  });

  it('adoptServerTheme ignores empty or null input', () => {
    useThemeStore.getState().setTheme('matrix');
    useThemeStore.getState().adoptServerTheme(null);
    useThemeStore.getState().adoptServerTheme('');
    expect(useThemeStore.getState().themeId).toBe('matrix');
  });

  it('adoptServerTheme marks the theme settled', () => {
    expect(useThemeStore.getState().themeSettled).toBe(false);
    useThemeStore.getState().adoptServerTheme('zen');
    expect(useThemeStore.getState().themeSettled).toBe(true);
  });

  it('setTheme marks the theme settled, so a late cache read cannot override it', () => {
    useThemeStore.getState().setTheme('tron');
    expect(useThemeStore.getState().themeSettled).toBe(true);

    useThemeStore.getState().adoptCachedTheme('dark');
    expect(useThemeStore.getState().themeId).toBe('tron');
  });

  it('adoptCachedTheme applies a value while the theme is unsettled', () => {
    useThemeStore.getState().adoptCachedTheme('synthwave');
    expect(useThemeStore.getState().themeId).toBe('synthwave');
  });

  it('adoptCachedTheme ignores null and empty input', () => {
    useThemeStore.getState().adoptCachedTheme(null);
    useThemeStore.getState().adoptCachedTheme('');
    expect(useThemeStore.getState().themeId).toBe('dark');
  });
});
