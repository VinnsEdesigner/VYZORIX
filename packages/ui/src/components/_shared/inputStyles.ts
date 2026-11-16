import { css } from '@emotion/css';
import { type VyzorixTheme } from '../../theme/types';

/**
 * The shared control surface.
 *
 * Every text-like control (Input, TextArea, Select) uses this so their height,
 * border and focus ring cannot drift apart.
 */
export function controlStyles(theme: VyzorixTheme, invalid?: boolean) {
  const borderColor = invalid ? theme.colors.error.border : theme.colors.border.medium;
  return css({
    width: '100%',
    padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`,
    fontFamily: theme.typography.fontFamily,
    fontSize: theme.typography.base.fontSize,
    color: theme.colors.text.primary,
    background: theme.colors.background.primary,
    border: `1px solid ${borderColor}`,
    borderRadius: theme.shape.radius.default,
    outline: 'none',
    '&::placeholder': { color: theme.colors.text.disabled },
    '&:hover': { borderColor: theme.colors.border.strong },
    '&:focus': {
      borderColor: invalid ? theme.colors.error.border : theme.colors.primary.border,
      boxShadow: `0 0 0 2px ${invalid ? theme.colors.error.subtleBackground : theme.colors.primary.subtleBackground}`,
    },
    '&:disabled': {
      background: theme.colors.action.disabledBackground,
      color: theme.colors.action.disabledText,
      cursor: 'not-allowed',
    },
  });
}

/** Focus ring for elements that are not form controls (buttons, links). */
export function focusRing(theme: VyzorixTheme) {
  return css({
    '&:focus-visible': {
      outline: `2px solid ${theme.colors.primary.main}`,
      outlineOffset: 2,
    },
  });
}
