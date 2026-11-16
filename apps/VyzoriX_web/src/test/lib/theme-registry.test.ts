import { describe, it, expect } from 'vitest';
import {
  THEMES_UNDER_TEST,
  getThemeById,
  getBuiltInThemes,
  themeIds,
  isThemeId,
  resolveThemeId,
  DEFAULT_THEME_ID,
  getContrastRatio,
} from './theme-test-helpers';

describe('theme registry', () => {
  it('exposes exactly eight themes', () => {
    expect(themeIds()).toHaveLength(8);
  });

  it('has unique ids and includes the required four', () => {
    const ids = themeIds();
    expect(new Set(ids).size).toBe(ids.length);
    for (const id of ['dark', 'light', 'rose-500', 'burgundy']) {
      expect(isThemeId(id)).toBe(true);
    }
  });

  it('validates ids and resolves unknown to the default', () => {
    expect(isThemeId('tron')).toBe(true);
    expect(isThemeId('nope')).toBe(false);
    expect(resolveThemeId('nope')).toBe(DEFAULT_THEME_ID);
  });

  it('builds a full theme object from a definition', () => {
    const theme = getThemeById('dark');
    expect(theme.name).toBe('Vyzorix Dark');
    expect(theme.isDark).toBe(true);
    expect(theme.colors.primary.main).toBe('#e11d48');
    // getRichColor derives the rest from main.
    expect(theme.colors.primary.contrastText).toBeTruthy();
    expect(theme.colors.primary.mainEmphasis).toBeTruthy();
    expect(typeof theme.spacing(2)).toBe('string');
    expect(theme.shape.radius.default).toBe('6px');
    expect(theme.typography.fontFamily).toContain('Inter');
    expect(theme.shadows.z1).toBeTruthy();
  });

  it('falls back to the default for an unknown id', () => {
    expect(getThemeById('does-not-exist').name).toBe('Vyzorix Dark');
  });

  it('lists built-ins first when filtering extras', () => {
    const items = getBuiltInThemes(['tron']);
    const extras = items.filter((i) => i.isExtra).map((i) => i.id);
    expect(extras).toEqual(['tron']);
    expect(items[0]!.isExtra).toBeFalsy();
  });
});

describe('theme contrast (WCAG AA)', () => {
  it.each(THEMES_UNDER_TEST)('%s: text on background meets 4.5:1', (id) => {
    const theme = getThemeById(id);
    const ratio = getContrastRatio(theme.colors.text.primary, theme.colors.background.canvas);
    expect(ratio, `${id} text/canvas = ${ratio.toFixed(2)}`).toBeGreaterThanOrEqual(4.5);
  });

  it.each(THEMES_UNDER_TEST)('%s: primary contrast text meets 4.5:1', (id) => {
    const theme = getThemeById(id);
    const ratio = getContrastRatio(theme.colors.primary.contrastText, theme.colors.primary.main);
    expect(ratio, `${id} primary = ${ratio.toFixed(2)}`).toBeGreaterThanOrEqual(4.5);
  });
});
