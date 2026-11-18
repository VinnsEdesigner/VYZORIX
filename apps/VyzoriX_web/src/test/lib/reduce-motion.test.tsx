import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render } from '@testing-library/react';
import { ThemeContext, getThemeById } from '@vyzorix/ui';
import { useDisplayPreferences } from '@/hooks/preferences/use-display-preference';
import { usePreferencesStore, PREFERENCE_DEFAULTS } from '@/stores/preferences-store';

function Probe() {
  useDisplayPreferences();
  return <div>probe</div>;
}

function renderProbe() {
  return render(
    <ThemeContext.Provider value={getThemeById('dark')}>
      <Probe />
    </ThemeContext.Provider>,
  );
}

beforeEach(() => {
  localStorage.clear();
  usePreferencesStore.setState({ ...PREFERENCE_DEFAULTS });
  delete document.documentElement.dataset.reduceMotion;
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('reduce motion', () => {
  it('sets the flag when the preference is on', async () => {
    usePreferencesStore.setState({ reduceMotion: true });
    renderProbe();
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(document.documentElement.dataset.reduceMotion).toBe('true');
  });

  it('leaves it off when the preference is off and the OS does not ask', async () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false }));
    renderProbe();
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(document.documentElement.dataset.reduceMotion).toBe('false');
  });

  it('honours the OS reduced-motion setting even when the preference is off', async () => {
    vi.stubGlobal('matchMedia', (query: string) => ({ matches: query.includes('prefers-reduced-motion') }));
    renderProbe();
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(document.documentElement.dataset.reduceMotion).toBe('true');
  });
});
