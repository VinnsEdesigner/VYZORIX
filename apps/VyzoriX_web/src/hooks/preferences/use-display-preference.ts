import { useEffect } from 'react';
import { usePreferencesStore, type Density, type TextScale } from '@/stores/preferences-store';
import { formatBytes, formatNumber, formatTimestamp, type TimestampOptions } from '@/lib/preferences';

const TEXT_SCALE_FACTOR: Record<TextScale, number> = { small: 0.9, default: 1, large: 1.15 };
const ROOT_FONT_PX = 14;

export const DENSITY_SCALE: Record<Density, number> = {
  compact: 0.75,
  comfortable: 1,
  spacious: 1.4,
};

export const DENSITY_NAV_WIDTH: Record<Density, number> = {
  compact: 240,
  comfortable: 280,
  spacious: 320,
};

export function useDisplayPreferences(): void {
  const density = usePreferencesStore((s) => s.density);
  const textScale = usePreferencesStore((s) => s.textScale);
  const reduceMotion = usePreferencesStore((s) => s.reduceMotion);

  useEffect(() => {
    if (typeof document === 'undefined') return;
    const root = document.documentElement;
    root.dataset.density = density;
    root.style.setProperty('--vyz-density-scale', String(DENSITY_SCALE[density]));
    root.style.setProperty('--vyz-nav-width', `${DENSITY_NAV_WIDTH[density]}px`);
  }, [density]);

  useEffect(() => {
    if (typeof document === 'undefined') return;
    const root = document.documentElement;
    root.style.fontSize = `${ROOT_FONT_PX * TEXT_SCALE_FACTOR[textScale]}px`;
  }, [textScale]);

  useEffect(() => {
    if (typeof document === 'undefined') return;
    const root = document.documentElement;

    const osPrefersReduce =
      typeof window !== 'undefined' &&
      typeof window.matchMedia === 'function' &&
      window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    root.dataset.reduceMotion = reduceMotion || osPrefersReduce ? 'true' : 'false';
  }, [reduceMotion]);

  const highContrast = usePreferencesStore((s) => s.highContrast);
  const underlineLinks = usePreferencesStore((s) => s.underlineLinks);
  const alwaysShowFocus = usePreferencesStore((s) => s.alwaysShowFocus);

  useEffect(() => {
    if (typeof document === 'undefined') return;
    const root = document.documentElement;
    root.dataset.highContrast = highContrast ? 'true' : 'false';
    root.dataset.underlineLinks = underlineLinks ? 'true' : 'false';
    root.dataset.alwaysShowFocus = alwaysShowFocus ? 'true' : 'false';
  }, [highContrast, underlineLinks, alwaysShowFocus]);
}

export function usePreferencesFormat() {
  const timeFormat = usePreferencesStore((s) => s.timeFormat);
  const relativeTimestamps = usePreferencesStore((s) => s.relativeTimestamps);
  const byteUnits = usePreferencesStore((s) => s.byteUnits);
  const thousandsSeparator = usePreferencesStore((s) => s.thousandsSeparator);

  return {
    timeFormat,
    relativeTimestamps,
    byteUnits,
    thousandsSeparator,
    formatBytes: (bytes: number) => formatBytes(bytes, byteUnits),
    formatNumber: (value: number) => formatNumber(value, thousandsSeparator),
    formatTimestamp: (ms: number | null | undefined, options: TimestampOptions = {}) =>
      formatTimestamp(ms, { timeFormat, relative: relativeTimestamps, ...options }),
  };
}
