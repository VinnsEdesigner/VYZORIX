import { describe, it, expect, beforeEach } from 'vitest';
import { usePreferencesStore, PREFERENCE_DEFAULTS } from '@/stores/preferences-store';

function reset() {
  localStorage.clear();
  usePreferencesStore.setState({ ...PREFERENCE_DEFAULTS });
}

describe('usePreferencesStore', () => {
  beforeEach(reset);

  it('starts from the documented defaults', () => {
    const state = usePreferencesStore.getState();
    expect(state.density).toBe('comfortable');
    expect(state.timeFormat).toBe('24h');
    expect(state.relativeTimestamps).toBe(true);
    expect(state.tablePageSize).toBe(50);
    expect(state.confirmDestructive).toBe(true);
  });

  it('sets a single preference', () => {
    usePreferencesStore.getState().setPreference('density', 'compact');
    expect(usePreferencesStore.getState().density).toBe('compact');
  });

  it('persists changes to the localStorage mirror', () => {
    usePreferencesStore.getState().setPreference('textScale', 'large');
    expect(localStorage.getItem('vyzorix-preferences')).toContain('large');
  });

  it('resets to defaults', () => {
    usePreferencesStore.getState().setPreference('liveUpdates', false);
    usePreferencesStore.getState().resetPreferences();
    expect(usePreferencesStore.getState().liveUpdates).toBe(true);
  });
});
