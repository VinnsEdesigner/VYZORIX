import { createFileRoute } from '@tanstack/react-router';
import { css } from '@emotion/css';
import { useStyles2, type VyzorixTheme } from '@vyzorix/ui';

const getStyles = (theme: VyzorixTheme) => ({
  wrapper: css({
    display: 'flex',
    minHeight: '100vh',
    alignItems: 'center',
    justifyContent: 'center',
  }),
  center: css({ textAlign: 'center' }),
  title: css({
    fontSize: theme.typography.xxl.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    margin: 0,
  }),
  subtitle: css({
    color: theme.colors.text.secondary,
    marginTop: theme.spacing(1),
  }),
});

function IndexRoute() {
  const styles = useStyles2(getStyles);
  return (
    <div className={styles.wrapper}>
      <div className={styles.center}>
        <h1 className={styles.title}>VyzoriX</h1>
        <p className={styles.subtitle}>Runtime ready</p>
      </div>
    </div>
  );
}

export const Route = createFileRoute('/')({
  component: IndexRoute,
});
