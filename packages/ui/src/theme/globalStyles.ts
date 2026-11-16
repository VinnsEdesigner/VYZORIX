import { css, injectGlobal } from '@emotion/css';
import { type VyzorixTheme } from './types';

/**
 * Global styles injected once per theme change.
 *
 * Sets the document background/foreground and typography from the active theme,
 * plus a minimal reset. Kept small: component styling lives in each component's
 * `useStyles2` block, not here.
 */
export function injectGlobalStyles(theme: VyzorixTheme): void {
  injectGlobal`
    *, *::before, *::after { box-sizing: border-box; }
    html, body, #root { height: 100%; }
    body {
      margin: 0;
      background: ${theme.colors.background.canvas};
      color: ${theme.colors.text.primary};
      font-family: ${theme.typography.fontFamily};
      font-size: ${theme.typography.fontSize}px;
      line-height: 1.5;
      -webkit-font-smoothing: antialiased;
    }
    a { color: ${theme.colors.text.link}; }
    ::selection { background: ${theme.colors.primary.main}; color: ${theme.colors.primary.contrastText}; }
  `;
}

/** The base style applied to the app shell. */
export function appShellStyles(theme: VyzorixTheme): string {
  return css({
    minHeight: '100%',
    background: theme.colors.background.canvas,
    color: theme.colors.text.primary,
    fontFamily: theme.typography.fontFamily,
  });
}
