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
    void 0;
  }
}
