import { useEffect, useRef } from 'react';
import { useGetMePreferences, usePatchMePreferences } from '@/generated-rq/settings/user-settings';
import { useAuthStore } from '@/stores';
import { usePreferencesStore, type PreferenceValues } from '@/stores/preferences-store';

export type SyncableKey = Exclude<keyof PreferenceValues, 'stepUpActions' | 'highContrast' | 'underlineLinks' | 'alwaysShowFocus'>;

export const SERVER_KEY: Record<SyncableKey, string> = {
  density: 'density',
  textScale: 'text_scale',
  reduceMotion: 'reduce_motion',
  timeFormat: 'time_format',
  relativeTimestamps: 'relative_timestamps',
  byteUnits: 'byte_units',
  thousandsSeparator: 'thousands_separator',
  landingPage: 'landing_page',
  defaultTimeRange: 'default_time_range',
  tablePageSize: 'table_page_size',
  liveUpdates: 'live_updates',
  confirmDestructive: 'confirm_destructive',
  idleTimeoutMinutes: 'idle_timeout_minutes',
};

const LOCAL_KEY = Object.fromEntries(
  Object.entries(SERVER_KEY).map(([local, server]) => [server, local]),
) as Record<string, keyof PreferenceValues>;

export function usePreferencesSync(): { setPreference: <K extends keyof PreferenceValues>(key: K, value: PreferenceValues[K]) => void } {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const { data } = useGetMePreferences({ query: { enabled: isAuthenticated } });
  const mutation = usePatchMePreferences();
  const adopted = useRef(false);

  useEffect(() => {
    if (!isAuthenticated) {
      adopted.current = false;
      return;
    }
    const raw = data?.preferences as Record<string, unknown> | undefined;
    if (!raw || adopted.current) return;

    const patch: Record<string, unknown> = {};
    for (const [serverKey, localKey] of Object.entries(LOCAL_KEY)) {
      const value = raw[serverKey];
      if (value === undefined || value === null) continue;
      patch[localKey] = value;
    }
    if (Object.keys(patch).length > 0) {
      usePreferencesStore.setState(patch as Partial<PreferenceValues>);
    }
    adopted.current = true;
  }, [isAuthenticated, data]);

  const setPreference = <K extends keyof PreferenceValues>(key: K, value: PreferenceValues[K]) => {
    usePreferencesStore.getState().setPreference(key, value);
    if (!isAuthenticated) return;
    const serverKey = SERVER_KEY[key as SyncableKey];
    if (!serverKey) return;
    mutation.mutate({ data: { preferences: { [serverKey]: value } } as never });
  };

  return { setPreference };
}
