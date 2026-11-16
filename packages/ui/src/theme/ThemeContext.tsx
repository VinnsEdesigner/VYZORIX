import { createContext, useContext } from 'react';
import memoize from 'micro-memoize';
import { type VyzorixTheme } from './types';

/**
 * Theme context and the styling hooks.
 *
 * `useStyles2(getStyles)` is the styling entry point: a component passes a
 * `(theme) => css({...})` function and gets memoized styles back, with the theme
 * supplied from context.
 */

export const ThemeContext = createContext<VyzorixTheme | null>(null);

/** A mock context for tests, set via `mockThemeContext`. */
let ThemeContextMock: React.Context<VyzorixTheme | null> | null = null;

/** Read the theme object. Throws when no provider is mounted. */
export function useTheme2(): VyzorixTheme {
  const theme = useContext(ThemeContextMock ?? ThemeContext);
  if (!theme) {
    throw new Error('useTheme2 must be used within a <ThemeProvider>.');
  }
  return theme;
}

const memoizedStyleCreators = new WeakMap();

/**
 * Memoized styles. Pass a style creator declared at module scope (stable
 * reference) and any extra args; the result is cached per creator.
 */
export function useStyles2<T extends unknown[], CSSReturnValue>(
  getStyles: (theme: VyzorixTheme, ...args: T) => CSSReturnValue,
  ...additionalArguments: T
): CSSReturnValue {
  const theme = useTheme2();
  let memoized = memoizedStyleCreators.get(getStyles);
  if (!memoized) {
    memoized = memoize(getStyles, { maxSize: 10 });
    memoizedStyleCreators.set(getStyles, memoized);
  }
  return memoized(theme, ...additionalArguments);
}

/** Enable theme context mocking in tests. Returns a restore function. */
export function mockThemeContext(theme: VyzorixTheme) {
  ThemeContextMock = createContext<VyzorixTheme | null>(theme);
  return () => {
    ThemeContextMock = null;
  };
}
