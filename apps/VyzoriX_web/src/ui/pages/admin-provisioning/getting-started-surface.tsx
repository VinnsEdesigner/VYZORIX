import { css } from '@emotion/css';
import { Link } from '@tanstack/react-router';
import { Button, Icon, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { emptyStateAvatar } from '@/lib/avatar';
import { GUIDES, guideRoute } from '@/lib/provisioning/guides';

export function GettingStartedSurface({ canWrite }: { canWrite: boolean }) {
  const styles = useStyles2(getStyles);

  return (
    <div className={styles.wrap}>
      <section className={styles.hero}>
        <img
          src={emptyStateAvatar('sources', 600)}
          alt=""
          aria-hidden="true"
          className={styles.heroArt}
          width={300}
          height={300}
          draggable={false}
        />
        <div className={styles.heroCopy}>
          <p className={styles.eyebrow}>Guides</p>
          <h2 className={styles.heroTitle}>Your devices, described in git</h2>
          <p className={styles.heroBody}>
            Write each device&apos;s configuration as a file in a repository you control. VYZORIX
            reads it, applies each file to the device it names, and tells you when a device no longer
            matches. Pick a guide below — each is a few minutes and covers the mistakes that actually
            happen.
          </p>
          <div className={styles.heroActions}>
            <Link to="/admin/provisioning" search={{ tab: 'providers' }} className={styles.plainLink}>
              <Button variant="primary" icon="plus">
                {canWrite ? 'Install a provider' : 'See the providers'}
              </Button>
            </Link>
            <Link to="/admin/provisioning" search={{ tab: 'sources' }} className={styles.plainLink}>
              <Button variant="secondary" fill="text" iconPlacement="right" icon="arrow-right">
                Browse sources
              </Button>
            </Link>
          </div>
        </div>
      </section>

      <section className={styles.guides}>
        {GUIDES.map((guide) => (
          <Link key={guide.id} to={guideRoute(guide.id)} className={styles.card}>
            <img
              src={emptyStateAvatar(guide.illustration, 320)}
              alt=""
              aria-hidden="true"
              className={styles.cardArt}
              width={96}
              height={96}
              draggable={false}
            />
            <div className={styles.cardBody}>
              <h3 className={styles.cardTitle}>{guide.title}</h3>
              <p className={styles.cardSummary}>{guide.summary}</p>
              <span className={styles.cardMeta}>
                <Icon name="clock" size="xs" />
                {guide.minutes} min read
              </span>
            </div>
            <Icon name="chevron-right" size="sm" />
          </Link>
        ))}
      </section>

      {!canWrite ? (
        <p className={styles.note}>
          You can read everything here, but installing a provider and running syncs needs admin
          authority — a provisioned source can rewrite any device&apos;s configuration.
        </p>
      ) : null}
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(4), maxWidth: 1000 }),

  hero: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(4),
    padding: theme.spacing(4),
    borderRadius: theme.shape.radius.lg,
    border: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.primary,
    [theme.breakpoints.down('md')]: { flexDirection: 'column', alignItems: 'flex-start', gap: theme.spacing(2) },
  }),
  heroArt: css({
    width: 300,
    height: 300,
    flexShrink: 0,
    userSelect: 'none',
    [theme.breakpoints.down('md')]: { width: 220, height: 220 },
  }),
  heroCopy: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1.5), minWidth: 0 }),
  eyebrow: css({
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    letterSpacing: '0.08em',
    textTransform: 'uppercase',
    color: theme.colors.primary.text,
  }),
  heroTitle: css({
    margin: 0,
    fontSize: theme.typography.xxl.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    letterSpacing: '-0.02em',
    lineHeight: 1.15,
    color: theme.colors.text.primary,
  }),
  heroBody: css({
    margin: 0,
    fontSize: theme.typography.base.fontSize,
    lineHeight: 1.65,
    color: theme.colors.text.secondary,
  }),
  heroActions: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1), flexWrap: 'wrap' }),
  plainLink: css({ textDecoration: 'none' }),

  guides: css({
    display: 'grid',
    gap: theme.spacing(2),
    gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))',
  }),
  card: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
    padding: theme.spacing(2),
    borderRadius: theme.shape.radius.lg,
    border: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.primary,
    textDecoration: 'none',
    color: theme.colors.text.secondary,
    '&:hover': { borderColor: theme.colors.border.medium, background: theme.colors.background.secondary },
  }),
  cardArt: css({ width: 96, height: 96, flexShrink: 0, userSelect: 'none' }),
  cardBody: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5), flex: 1, minWidth: 0 }),
  cardTitle: css({
    margin: 0,
    fontSize: theme.typography.md.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  cardSummary: css({
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    lineHeight: 1.55,
    color: theme.colors.text.secondary,
  }),
  cardMeta: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.secondary,
  }),
  note: css({
    margin: 0,
    padding: theme.spacing(1.5),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.secondary,
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.secondary,
  }),
});
