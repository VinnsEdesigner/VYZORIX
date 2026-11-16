/**
 * Theme preference cache in the SQLite local store.
 *
 * The localStorage mirror (cache.ts) is what the anti-FOUC script reads
 * synchronously; this is the durable copy, stored with the user's other cached
 * data under the same per-user scope. Both functions take the repository so
 * they stay pure and testable, and are no-ops for a null store.
 */

import type { LocalStoreRepository } from '@/lib/local-store';
import { resolveThemeId, type ThemeId } from '@vyzorix/ui';

const TABLE = 'user_prefs';

export async function readCachedTheme(store: LocalStoreRepository, userId: string): Promise<ThemeId | null> {
  if (!userId) return null;
  try {
    const rows = await store.select<{ theme?: unknown }>(TABLE, {
      columns: ['theme'],
      where: { user_id: userId },
      limit: 1,
    });
    const theme = rows[0]?.theme;
    return typeof theme === 'string' ? resolveThemeId(theme) : null;
  } catch {
    return null;
  }
}

export async function writeCachedTheme(
  store: LocalStoreRepository,
  userId: string,
  themeId: string,
): Promise<void> {
  if (!userId) return;
  try {
    await store.upsert(TABLE, [
      { user_id: userId, theme: resolveThemeId(themeId), updated_at: Date.now() },
    ]);
  } catch {
    // Cache write failure is non-fatal; the localStorage mirror still holds it.
  }
}
