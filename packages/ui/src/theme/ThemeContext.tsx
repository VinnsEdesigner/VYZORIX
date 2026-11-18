import { createContext, useContext } from 'react';
import memoize from 'micro-memoize';
import { type VyzorixTheme } from './types';

export const ThemeContext = createContext<VyzorixTheme | null>(null);

let ThemeContextMock: React.Context<VyzorixTheme | null> | null = null;

export function useTheme2(): VyzorixTheme {
  const theme = useContext(ThemeContextMock ?? ThemeContext);
  if (!theme) {
    throw new Error('useTheme2 must be used within a <ThemeProvider>.');
  }
  return theme;
}

const memoizedStyleCreators = new WeakMap();

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

export function mockThemeContext(theme: VyzorixTheme) {
  ThemeContextMock = createContext<VyzorixTheme | null>(theme);
  return () => {
    ThemeContextMock = null;
  };
}
