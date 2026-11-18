import { css, cx } from '@emotion/css';
import { useStyles2, type VyzorixTheme } from '@vyzorix/ui';

export interface PageProps {
  title: string;
  subTitle?: React.ReactNode;
  actions?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

export function Page({ title, subTitle, actions, children, className }: PageProps) {
  const styles = useStyles2(getStyles);

  return (
    <div className={cx(styles.page, className)}>
      <header className={styles.pageHead}>
        <div>
          <h1 className={styles.title}>{title}</h1>
          {subTitle ? <p className={styles.subtitle}>{subTitle}</p> : null}
        </div>
        {actions ? <div className={styles.actions}>{actions}</div> : null}
      </header>
      {children}
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  page: css({ display: 'flex', flexDirection: 'column', flexGrow: 1 }),
  pageHead: css({
    display: 'flex',
    alignItems: 'flex-end',
    gap: theme.spacing(2),
    marginBottom: theme.spacing(3),
    [theme.breakpoints.down('md')]: { flexDirection: 'column', alignItems: 'flex-start', gap: theme.spacing(1.5) },
  }),
  title: css({
    margin: 0,
    fontSize: theme.typography.xxl.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    letterSpacing: '-0.01em',
  }),
  subtitle: css({
    margin: `${theme.spacing(0.5)} 0 0`,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
  }),
  actions: css({
    marginLeft: 'auto',
    display: 'flex',
    gap: theme.spacing(1),
    [theme.breakpoints.down('md')]: { marginLeft: 0 },
  }),
});
