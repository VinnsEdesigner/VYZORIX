// Re-export the theme registry surface for the tests, plus the id list used in
// the parametrised contrast cases.
export {
  getThemeById,
  getBuiltInThemes,
  themeIds,
  isThemeId,
  resolveThemeId,
  DEFAULT_THEME_ID,
  getContrastRatio,
} from '@vyzorix/ui';

export const THEMES_UNDER_TEST = [
  'dark',
  'light',
  'rose-500',
  'burgundy',
  'matrix',
  'tron',
  'synthwave',
  'zen',
] as const;
