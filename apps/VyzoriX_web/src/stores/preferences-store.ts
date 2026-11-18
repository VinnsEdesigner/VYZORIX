import { createVyzorStore } from '@/lib/state';

export type Density = 'comfortable' | 'compact' | 'spacious';
export type TextScale = 'small' | 'default' | 'large';
export type TimeFormat = '12h' | '24h';
export type ByteUnits = 'binary' | 'decimal';
export type LandingPage = 'last' | 'home' | 'devices' | 'logs';
export type DefaultTimeRange = '15m' | '1h' | '6h' | '24h' | '7d';
export type TablePageSize = 25 | 50 | 100;
export type IdleTimeoutMinutes = 0 | 15 | 30 | 60;
export type StepUpAction = 'api-keys' | 'sessions' | 'device-wipe' | 'secret-rotate';

export interface PreferencesState {
  density: Density;
  textScale: TextScale;
  reduceMotion: boolean;
  timeFormat: TimeFormat;
  relativeTimestamps: boolean;
  byteUnits: ByteUnits;
  thousandsSeparator: boolean;
  landingPage: LandingPage;
  defaultTimeRange: DefaultTimeRange;
  tablePageSize: TablePageSize;
  liveUpdates: boolean;
  confirmDestructive: boolean;
  idleTimeoutMinutes: IdleTimeoutMinutes;
  stepUpActions: StepUpAction[];
  highContrast: boolean;
  underlineLinks: boolean;
  alwaysShowFocus: boolean;
  setPreference: <K extends keyof PreferenceValues>(key: K, value: PreferenceValues[K]) => void;
  resetPreferences: () => void;
}

export type PreferenceValues = Omit<PreferencesState, 'setPreference' | 'resetPreferences'>;

export const PREFERENCE_DEFAULTS: PreferenceValues = {
  density: 'comfortable',
  textScale: 'default',
  reduceMotion: false,
  timeFormat: '24h',
  relativeTimestamps: true,
  byteUnits: 'binary',
  thousandsSeparator: true,
  landingPage: 'last',
  defaultTimeRange: '1h',
  tablePageSize: 50,
  liveUpdates: true,
  confirmDestructive: true,
  idleTimeoutMinutes: 30,
  stepUpActions: ['api-keys'],
  highContrast: false,
  underlineLinks: false,
  alwaysShowFocus: false,
};

export const usePreferencesStore = createVyzorStore<PreferencesState>(
  'PreferencesStore',
  (set) => ({
    ...PREFERENCE_DEFAULTS,
    setPreference: (key, value) => set({ [key]: value } as Partial<PreferencesState>),
    resetPreferences: () => set({ ...PREFERENCE_DEFAULTS }),
  }),
  {
    persist: {
      name: 'vyzorix-preferences',
      partialize: (state) => {
        const persisted: Record<string, unknown> = {};
        for (const key of Object.keys(PREFERENCE_DEFAULTS)) {
          persisted[key] = (state as unknown as Record<string, unknown>)[key];
        }
        return persisted as Partial<PreferencesState>;
      },
    },
  },
);
