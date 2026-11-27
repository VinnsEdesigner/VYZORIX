import { css } from '@emotion/css';
import { Link } from '@tanstack/react-router';
import { Icon, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { emptyStateAvatar } from '@/lib/avatar';
import { CodeBlock } from '@/ui/components/shared/code-block';
import { GUIDES, guideById, guideRoute, type GuideId } from '@/lib/provisioning/guides';

export function GuidePage({ guideId }: { guideId: GuideId }) {
  const styles = useStyles2(getStyles);
  const guide = guideById(guideId);

  if (!guide) return null;

  const others = GUIDES.filter((candidate) => candidate.id !== guide.id);

  return (
    <div className={styles.wrap}>
      <Link to="/admin/provisioning" search={{ tab: 'getting-started' }} className={styles.back}>
        <Icon name="arrow-left" size="sm" />
        All guides
      </Link>

      <header className={styles.header}>
        <img
          src={emptyStateAvatar(guide.illustration, 480)}
          alt=""
          aria-hidden="true"
          className={styles.art}
          width={200}
          height={200}
          draggable={false}
        />
        <div className={styles.headerCopy}>
          <h2 className={styles.title}>{guide.title}</h2>
          <p className={styles.summary}>{guide.summary}</p>
          <span className={styles.meta}>
            <Icon name="clock" size="xs" />
            {guide.minutes} min read
          </span>
        </div>
      </header>

      <article className={styles.body}>
        {guide.sections.map((section) => (
          <section key={section.heading} className={styles.section}>
            <h3 className={styles.sectionHeading}>{section.heading}</h3>
            {section.body ? <p className={styles.prose}>{section.body}</p> : null}
            {section.code ? <CodeBlock code={section.code} language="toml" className={styles.code} /> : null}
            {section.steps ? (
              <ol className={styles.steps}>
                {section.steps.map((step) => (
                  <li key={step}>{step}</li>
                ))}
              </ol>
            ) : null}
            {section.points ? (
              <ul className={styles.points}>
                {section.points.map((point) => (
                  <li key={point}>{point}</li>
                ))}
              </ul>
            ) : null}
          </section>
        ))}
      </article>

      <footer className={styles.more}>
        <h3 className={styles.moreHeading}>Other guides</h3>
        <div className={styles.moreGrid}>
          {others.map((other) => (
            <Link key={other.id} to={guideRoute(other.id)} className={styles.moreCard}>
              <img
                src={emptyStateAvatar(other.illustration, 240)}
                alt=""
                aria-hidden="true"
                className={styles.moreArt}
                width={56}
                height={56}
                draggable={false}
              />
              <span className={styles.moreTitle}>{other.title}</span>
              <Icon name="chevron-right" size="xs" />
            </Link>
          ))}
        </div>
      </footer>
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3), maxWidth: 820 }),
  back: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.75),
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.secondary,
    textDecoration: 'none',
    '&:hover': { color: theme.colors.text.primary },
  }),
  header: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(3),
    paddingBottom: theme.spacing(2),
    borderBottom: `1px solid ${theme.colors.border.weak}`,
    [theme.breakpoints.down('md')]: { flexDirection: 'column', alignItems: 'flex-start', gap: theme.spacing(1.5) },
  }),
  art: css({
    width: 200,
    height: 200,
    flexShrink: 0,
    userSelect: 'none',
    [theme.breakpoints.down('md')]: { width: 140, height: 140 },
  }),
  headerCopy: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1), minWidth: 0 }),
  title: css({
    margin: 0,
    fontSize: theme.typography.xxl.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    letterSpacing: '-0.02em',
    lineHeight: 1.15,
    color: theme.colors.text.primary,
  }),
  summary: css({
    margin: 0,
    fontSize: theme.typography.base.fontSize,
    lineHeight: 1.6,
    color: theme.colors.text.secondary,
  }),
  meta: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.secondary,
  }),
  body: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3) }),
  section: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  sectionHeading: css({
    margin: 0,
    fontSize: theme.typography.md.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  prose: css({
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    lineHeight: 1.75,
    color: theme.colors.text.secondary,
  }),
  code: css({
    margin: 0,
    padding: theme.spacing(2),
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
    fontSize: theme.typography.sm.fontSize,
    lineHeight: 1.7,
    color: theme.colors.text.primary,
    overflowX: 'auto',
    whiteSpace: 'pre',
  }),
  steps: css({
    margin: 0,
    paddingLeft: theme.spacing(2.5),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.75),
    fontSize: theme.typography.sm.fontSize,
    lineHeight: 1.7,
    color: theme.colors.text.secondary,
  }),
  points: css({
    margin: 0,
    paddingLeft: theme.spacing(2.5),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.75),
    fontSize: theme.typography.sm.fontSize,
    lineHeight: 1.7,
    color: theme.colors.text.secondary,
  }),
  more: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1.5),
    paddingTop: theme.spacing(2),
    borderTop: `1px solid ${theme.colors.border.weak}`,
  }),
  moreHeading: css({
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
  }),
  moreGrid: css({
    display: 'grid',
    gap: theme.spacing(1),
    gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
  }),
  moreCard: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    padding: theme.spacing(1),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.weak}`,
    textDecoration: 'none',
    color: theme.colors.text.secondary,
    '&:hover': { background: theme.colors.background.secondary },
  }),
  moreArt: css({ width: 56, height: 56, flexShrink: 0, userSelect: 'none' }),
  moreTitle: css({
    flex: 1,
    minWidth: 0,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
});
