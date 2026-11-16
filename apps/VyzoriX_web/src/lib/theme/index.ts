// App-side theme surface. The theme system itself lives in @vyzorix/ui; this
// module re-exports it and adds the browser-side cache mirror.
export {
  getThemeById,
  getBuiltInThemes,
  themeIds,
  isThemeId,
  resolveThemeId,
  DEFAULT_THEME_ID,
  SYSTEM_THEME_ID,
  THEME_IDS,
  type ThemeId,
  type ThemeRegistryItem,
  type VyzorixTheme,
  useTheme2,
  useStyles2,
} from '@vyzorix/ui';

export {
  THEME_STORAGE_KEY,
  readCachedThemeId,
  writeCachedThemeId,
  readFollowSystem,
  writeFollowSystem,
} from './cache';
export { readCachedTheme, writeCachedTheme } from './prefs-cache';
