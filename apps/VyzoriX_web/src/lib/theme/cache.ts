import { isThemeId, resolveThemeId, type ThemeId } from '@vyzorix/ui';

export const THEME_STORAGE_KEY = 'vyzorix.theme';
const FOLLOW_SYSTEM_KEY = 'vyzorix.theme.followSystem';

function safeGet(key: string): string | null {
  if (typeof window === 'undefined') return null;
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function safeSet(key: string, value: string): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(key, value);
  } catch {
    void 0;
  }
}

export function readCachedThemeId(): ThemeId | null {
  const raw = safeGet(THEME_STORAGE_KEY);
  return isThemeId(raw) ? raw : null;
}

export function writeCachedThemeId(themeId: string): void {
  safeSet(THEME_STORAGE_KEY, resolveThemeId(themeId));
}

export function readFollowSystem(): boolean {
  return safeGet(FOLLOW_SYSTEM_KEY) === 'true';
}

export function writeFollowSystem(follow: boolean): void {
  safeSet(FOLLOW_SYSTEM_KEY, follow ? 'true' : 'false');
}
