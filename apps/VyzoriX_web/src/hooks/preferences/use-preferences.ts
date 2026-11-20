import { useQueryClient } from '@tanstack/react-query';
import {
  getGetMePreferencesQueryKey,
  useGetMePreferences,
  usePatchMePreferences,
} from '@/generated-rq/settings/user-settings';
import {
  getGetOrganizationsIdSettingsQueryKey,
  useGetOrganizationsIdSettings,
  usePatchOrganizationsIdSettings,
} from '@/generated-rq/organizations/organization-management';
import { useCurrentOrganizationId } from '@/hooks/_shared/use-current-context';

export type WeekStart = 'sunday' | 'monday' | 'saturday';

export interface Preferences {
  theme: string;
  timezone: string;
  weekStart: WeekStart | '';
  language: string;
  dateFormat: string;
}

export const EMPTY_PREFERENCES: Preferences = {
  theme: '',
  timezone: '',
  weekStart: '',
  language: '',
  dateFormat: '',
};

export const WEEK_START_OPTIONS: { value: WeekStart; label: string }[] = [
  { value: 'monday', label: 'Monday' },
  { value: 'sunday', label: 'Sunday' },
  { value: 'saturday', label: 'Saturday' },
];

export const DATE_FORMAT_OPTIONS: { value: string; label: string }[] = [
  { value: 'YYYY-MM-DD', label: '2026-11-17 (ISO)' },
  { value: 'DD/MM/YYYY', label: '17/11/2026' },
  { value: 'MM/DD/YYYY', label: '11/17/2026' },
  { value: 'DD MMM YYYY', label: '17 Nov 2026' },
];

const PREF_KEY: Record<keyof Preferences, string[]> = {
  theme: ['theme'],
  timezone: ['timezone'],
  weekStart: ['week_start', 'weekStart'],
  language: ['language'],
  dateFormat: ['date_format', 'dateFormat'],
};

export function readPreferences(source: Record<string, unknown> | undefined): Preferences {
  if (!source) return EMPTY_PREFERENCES;
  const pick = (key: keyof Preferences): string => {
    for (const candidate of PREF_KEY[key]) {
      const raw = source[candidate];
      if (typeof raw === 'string') return raw;
    }
    return '';
  };
  return {
    theme: pick('theme'),
    timezone: pick('timezone'),
    weekStart: pick('weekStart') as WeekStart | '',
    language: pick('language'),
    dateFormat: pick('dateFormat'),
  };
}

function toWire(prefs: Preferences): Record<string, unknown> {
  return {
    theme: prefs.theme || null,
    timezone: prefs.timezone || null,
    week_start: prefs.weekStart || null,
    language: prefs.language || null,
    date_format: prefs.dateFormat || null,
  };
}

export function useUserPreferences() {
  const query = useGetMePreferences();
  const raw = query.data?.preferences as Record<string, unknown> | undefined;
  const resolved = query.data?.resolved as Record<string, string> | undefined;

  return {
    preferences: readPreferences(raw),
    effective: resolved ?? {},
    effectiveTheme: query.data?.theme ?? '',
    isLoading: query.isLoading,
    isError: query.isError,
  };
}

export function useUpdateUserPreferences() {
  const mutation = usePatchMePreferences();
  const queryClient = useQueryClient();

  return {
    save: async (prefs: Preferences) => {
      await mutation.mutateAsync({ data: { preferences: toWire(prefs) } as never });

      void queryClient.invalidateQueries({ queryKey: getGetMePreferencesQueryKey() });
    },
    isSaving: mutation.isPending,
    isError: mutation.isError,
  };
}

export function useOrgPreferences() {
  const organizationId = useCurrentOrganizationId();
  const query = useGetOrganizationsIdSettings(organizationId ?? '', {
    query: { enabled: Boolean(organizationId) },
  });

  return {
    preferences: readPreferences(query.data as Record<string, unknown> | undefined),
    isLoading: !organizationId || query.isLoading,
    isError: query.isError,
  };
}

export function useUpdateOrgPreferences() {
  const organizationId = useCurrentOrganizationId();
  const mutation = usePatchOrganizationsIdSettings();
  const queryClient = useQueryClient();

  return {
    save: async (prefs: Preferences) => {
      if (!organizationId) throw new Error('no organization selected');

      await mutation.mutateAsync({
        id: organizationId,
        data: {
          theme: prefs.theme,
          timezone: prefs.timezone,
          dateFormat: prefs.dateFormat,
          weekStart: prefs.weekStart,
        },
      });

      void queryClient.invalidateQueries({ queryKey: getGetOrganizationsIdSettingsQueryKey(organizationId) });

      void queryClient.invalidateQueries({ queryKey: getGetMePreferencesQueryKey() });
    },
    isSaving: mutation.isPending,
    isError: mutation.isError,
  };
}

export function getTimezoneOptions(): string[] {
  try {
    return Intl.supportedValuesOf('timeZone');
  } catch {
    return [];
  }
}
