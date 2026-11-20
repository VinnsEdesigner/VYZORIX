import { css } from '@emotion/css';
import { type VyzorixTheme } from '@vyzorix/ui';

export const filterTableStyles = (theme: VyzorixTheme) => {
  const rowHeight = '30px';
  const cellPadding = `${theme.spacing(0.5)} ${theme.spacing(1)}`;

  return {
    wrapper: css({
      width: '100%',
      overflowX: 'auto',
    }),
    table: css({
      width: '100%',
      borderCollapse: 'separate',
      'tbody tr:nth-of-type(odd)': {
        background: theme.colors.emphasize(theme.colors.background.primary, 0.02),
      },
      'tbody tr:hover': {
        background: theme.colors.emphasize(theme.colors.background.primary, 0.05),
      },
    }),
    headCell: css({
      width: 'auto',
      padding: cellPadding,
      textAlign: 'left',
      lineHeight: rowHeight,
      height: rowHeight,
      whiteSpace: 'nowrap',
      color: theme.colors.text.secondary,
      fontWeight: theme.typography.fontWeightMedium,
      fontSize: theme.typography.sm.fontSize,
    }),
    cell: css({
      padding: cellPadding,
      lineHeight: rowHeight,
      height: rowHeight,
      whiteSpace: 'nowrap',
      verticalAlign: 'middle',
      fontSize: theme.typography.sm.fontSize,
      color: theme.colors.text.primary,
    }),

    linkCell: css({
      padding: 0,
      lineHeight: rowHeight,
      height: rowHeight,
      whiteSpace: 'nowrap',
    }),
    link: css({
      display: 'block',
      width: '100%',
      padding: `0 ${theme.spacing(1)}`,
      height: rowHeight,
      lineHeight: rowHeight,
      border: 0,
      background: 'transparent',
      color: 'inherit',
      font: 'inherit',
      textAlign: 'left',
      cursor: 'pointer',
      '&:hover': { textDecoration: 'underline' },
    }),
    ellipsis: css({
      display: 'block',
      width: '100%',
      whiteSpace: 'nowrap',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    }),
    alignRight: css({ textAlign: 'right' }),
    muted: css({ color: theme.colors.text.secondary }),
    actions: css({
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'flex-end',
      gap: theme.spacing(1),
    }),

    avatar: css({
      width: 24,
      height: 24,
      borderRadius: theme.shape.radius.circle,
      display: 'block',
      objectFit: 'cover',
    }),
    subLine: css({
      display: 'block',
      fontSize: theme.typography.sm.fontSize,
      color: theme.colors.text.secondary,
      lineHeight: 1.2,
    }),
    actionBar: css({
      marginBottom: theme.spacing(2),
      display: 'flex',
      alignItems: 'flex-start',
      gap: theme.spacing(2),
    }),
    actionBarSpacer: css({ flexGrow: 1, width: theme.spacing(2) }),
    actionBarGrow: css({ flexGrow: 1 }),
  };
};
