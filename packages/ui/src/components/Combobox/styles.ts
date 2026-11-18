import { css } from '@emotion/css';
import { type VyzorixTheme } from '../../theme/types';
import { getFocusStyles } from '../../theme/mixins';

export const MENU_PADDING = 4;
export const MENU_ITEM_GAP = 2;
const MENU_ITEM_PADDING = 8;
const MENU_ITEM_LINE_HEIGHT = 1.5;

export const MENU_MAX_HEIGHT = 320;

export const getComboboxStyles = (theme: VyzorixTheme) => ({
  menu: css({
    label: 'combobox-menu',
    position: 'fixed',
    zIndex: theme.zIndex.dropdown,
    background: theme.colors.background.elevated,
    border: `1px solid ${theme.colors.border.medium}`,
    borderRadius: theme.shape.radius.lg,
    boxShadow: theme.shadows.z2,
    overflow: 'hidden',
    pointerEvents: 'auto',
  }),

  menuScroll: css({
    label: 'combobox-menu-scroll',
    overflowY: 'auto',
    overscrollBehavior: 'contain',
    padding: MENU_PADDING,
  }),
  menuClosed: css({ display: 'none' }),
  list: css({ listStyle: 'none', margin: 0, padding: 0 }),
  groupHeader: css({
    label: 'combobox-group',
    padding: MENU_ITEM_PADDING,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
    borderTop: `1px solid ${theme.colors.border.weak}`,
    '&:first-child': { borderTop: 'none' },
  }),
  option: css({
    label: 'combobox-option',
    position: 'relative',
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    width: '100%',
    padding: MENU_ITEM_PADDING,
    marginBottom: MENU_ITEM_GAP,
    borderRadius: theme.shape.radius.default,
    fontSize: theme.typography.base.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: MENU_ITEM_LINE_HEIGHT,
    cursor: 'pointer',
    color: theme.colors.text.primary,
    '&:last-child': { marginBottom: 0 },
  }),
  optionBody: css({ display: 'flex', flexDirection: 'column', flexGrow: 1, minWidth: 0 }),
  optionLabel: css({ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }),
  optionDescription: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightRegular,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  }),

  optionFocused: css({
    background: theme.colors.action.hover,
  }),

  optionFocusRing: css({ ...getFocusStyles(theme), outlineOffset: '-2px' }),
  optionSelected: css({
    background: theme.colors.action.selected,
    '&::before': {
      content: '" "',
      position: 'absolute',
      left: 0,
      top: 0,
      height: '100%',
      width: theme.spacing(0.5),
      borderRadius: theme.shape.radius.default,
      background: theme.colors.primary.main,
    },
  }),
  optionInfo: css({
    color: theme.colors.text.disabled,
    cursor: 'not-allowed',
    '&:hover': { background: 'transparent' },
  }),
  message: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(2),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
  }),
  clear: css({
    cursor: 'pointer',
    pointerEvents: 'auto',
    '&:hover': { color: theme.colors.text.primary },
  }),

  noCaret: css({
    caretColor: 'transparent',
    cursor: 'pointer',
    '&::selection': { background: 'transparent' },
  }),
  chevron: css({
    pointerEvents: 'none',
    color: theme.colors.text.secondary,
  }),
});
