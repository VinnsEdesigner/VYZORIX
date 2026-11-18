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
