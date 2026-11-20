import { css, cx } from '@emotion/css';
import { useStyles2, type VyzorixTheme } from '@vyzorix/ui';

export interface SectionCardProps {
  title: string;
  subTitle?: string;
  actions?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

export function SectionCard({ title, subTitle, actions, children, className }: SectionCardProps) {
  const styles = useStyles2(getStyles);
  return (
    <section className={cx(styles.card, className)}>
      <header className={styles.head}>
        <div>
          <h2 className={styles.title}>{title}</h2>
          {subTitle ? <span className={styles.sub}>{subTitle}</span> : null}
        </div>
        {actions ? <div className={styles.actions}>{actions}</div> : null}
      </header>
      <div className={styles.body}>{children}</div>
    </section>
  );
}

export interface MetaRowProps {
  label: string;
  value: React.ReactNode;

  trailing?: React.ReactNode;
}

export function MetaRow({ label, value, trailing }: MetaRowProps) {
  const styles = useStyles2(getStyles);
  return (
    <div className={styles.metaRow}>
      <span className={styles.metaLabel}>{label}</span>
      <span className={styles.metaValue}>{value}</span>
      {trailing ? <span className={styles.metaTrailing}>{trailing}</span> : null}
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  card: css({
    background: theme.colors.background.primary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.lg,
    overflow: 'hidden',
  }),
  head: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    padding: `${theme.spacing(1.5)} ${theme.spacing(2.5)}`,
    borderBottom: `1px solid ${theme.colors.border.weak}`,
  }),
  title: css({
    margin: 0,
    fontSize: theme.typography.h6.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  sub: css({
    display: 'block',
    marginTop: 2,
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.secondary,
  }),
  actions: css({ display: 'flex', gap: theme.spacing(1), alignItems: 'center' }),
  body: css({ padding: theme.spacing(2.5), display: 'flex', flexDirection: 'column', gap: theme.spacing(1.5) }),
  metaRow: css({
    display: 'flex',
    alignItems: 'baseline',
    gap: theme.spacing(2),
    fontSize: theme.typography.sm.fontSize,
  }),
  metaLabel: css({ color: theme.colors.text.secondary, flexShrink: 0, minWidth: 160 }),
  metaValue: css({ color: theme.colors.text.primary, wordBreak: 'break-word', flex: 1 }),
  metaTrailing: css({ flexShrink: 0 }),
});
