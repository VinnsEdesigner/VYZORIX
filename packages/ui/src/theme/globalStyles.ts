import { css, injectGlobal } from '@emotion/css';
import { type VyzorixTheme } from './types';

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
    @keyframes vyzorix-spin { to { transform: rotate(360deg); } }
    @keyframes vyzorix-field-shake {
      0%, 100% { transform: translateX(0); }
      20%, 60% { transform: translateX(-4px); }
      40%, 80% { transform: translateX(4px); }
    }
    :root[data-reduce-motion='true'] *, :root[data-reduce-motion='true'] *::before, :root[data-reduce-motion='true'] *::after {
      transition: none !important;
      animation: none !important;
    }
    :root[data-underline-links='true'] a { text-decoration: underline !important; }
    :root[data-always-show-focus='true'] :focus { outline: 2px solid ${theme.colors.primary.main}; outline-offset: 2px; }
    :root[data-high-contrast='true'] { --vyz-contrast-boost: 1; }
    :root[data-high-contrast='true'] a { text-decoration: underline; }
    :root[data-high-contrast='true'] * { border-color: ${theme.colors.text.secondary} !important; }
  `;
}

export function appShellStyles(theme: VyzorixTheme): string {
  return css({
    minHeight: '100%',
    background: theme.colors.background.canvas,
    color: theme.colors.text.primary,
    fontFamily: theme.typography.fontFamily,
  });
}
