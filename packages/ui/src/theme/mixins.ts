import { type VyzorixTheme } from './types';

export function getFocusStyles(theme: VyzorixTheme, inset?: boolean) {
  const boxShadow = inset
    ? `inset 0 0 0px 2px ${theme.colors.primary.main}`
    : `0 0 0 2px ${theme.colors.background.canvas}, 0 0 0px 3px ${theme.colors.primary.main}`;

  return {
    outline: '2px dotted transparent',
    outlineOffset: inset ? '-2px' : '2px',
    boxShadow,
    transitionTimingFunction: 'cubic-bezier(0.19, 1, 0.22, 1)',
    transitionDuration: '0.2s',
    transitionProperty: 'box-shadow',
  };
}
