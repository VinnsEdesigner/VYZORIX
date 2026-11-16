/**
 * Synchronous theme mirror.
 *
 * The durable cache is the SQLite local store (src/lib/local-store). This is a
 * small localStorage mirror on top of it so the anti-FOUC script in __root.tsx
 * can read the last theme id before hydration, without awaiting a worker.
 */

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
    // Storage can be full or blocked; the mirror is an optimisation, so a
    // failure is non-fatal — the server value still wins on load.
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
