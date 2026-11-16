import { css } from '@emotion/css';
import { type VyzorixTheme } from '@vyzorix/ui';

/**
 * Shared styles for the simple list pages (alerts, notifications, service
 * accounts). Each page passes this to `useStyles2`; keeping one factory means
 * the placeholder pages stay visually consistent.
 */
export const listPageStyles = (theme: VyzorixTheme) => ({
  page: css({ margin: '0 auto', maxWidth: 896, padding: theme.spacing(3) }),
  h1: css({ marginBottom: theme.spacing(3), fontSize: theme.typography.xxl.fontSize, fontWeight: theme.typography.fontWeightBold }),
  h2: css({ marginBottom: theme.spacing(2), fontWeight: theme.typography.fontWeightMedium }),
  h3: css({ fontWeight: theme.typography.fontWeightMedium, margin: 0 }),
  panel: css({ marginBottom: theme.spacing(4), border: `1px solid ${theme.colors.border.medium}`, borderRadius: theme.shape.radius.default, padding: theme.spacing(2) }),
  grid: css({ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: theme.spacing(1.5) }),
  input: css({ borderRadius: theme.shape.radius.sm, border: `1px solid ${theme.colors.border.medium}`, padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`, background: theme.colors.background.primary, color: theme.colors.text.primary }),
  textarea: css({ marginTop: theme.spacing(1.5), width: '100%', borderRadius: theme.shape.radius.sm, border: `1px solid ${theme.colors.border.medium}`, padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`, background: theme.colors.background.primary, color: theme.colors.text.primary }),
  labelRow: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1), padding: `${theme.spacing(1)} ${theme.spacing(1.5)}` }),
  submit: css({ marginTop: theme.spacing(2), borderRadius: theme.shape.radius.sm, background: theme.colors.primary.main, color: theme.colors.primary.contrastText, border: 'none', padding: `${theme.spacing(1)} ${theme.spacing(2)}`, cursor: 'pointer', '&:disabled': { opacity: 0.5 } }),
  card: css({ borderRadius: theme.shape.radius.default, border: `1px solid ${theme.colors.border.medium}`, padding: theme.spacing(2) }),
  row: css({ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }),
  rowActions: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1) }),
  meta: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary, margin: 0 }),
  metaSm: css({ marginTop: theme.spacing(1), fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  badge: css({ borderRadius: theme.shape.radius.pill, padding: `2px ${theme.spacing(1)}`, fontSize: theme.typography.sm.fontSize, fontWeight: theme.typography.fontWeightMedium }),
  smallBtn: css({ borderRadius: theme.shape.radius.sm, border: `1px solid ${theme.colors.border.medium}`, padding: `2px ${theme.spacing(1)}`, fontSize: theme.typography.sm.fontSize, background: 'transparent', color: theme.colors.text.primary, cursor: 'pointer' }),
  smallBtnDanger: css({ borderRadius: theme.shape.radius.sm, border: `1px solid ${theme.colors.border.medium}`, padding: `2px ${theme.spacing(1)}`, fontSize: theme.typography.sm.fontSize, background: 'transparent', color: theme.colors.error.text, cursor: 'pointer' }),
  stack: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1.5) }),
  error: css({ color: theme.colors.error.text }),
  rowFlex: css({ display: 'flex', gap: theme.spacing(1) }),
  flex1: css({ flex: 1, borderRadius: theme.shape.radius.sm, border: `1px solid ${theme.colors.border.medium}`, padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`, background: theme.colors.background.primary, color: theme.colors.text.primary }),
});
