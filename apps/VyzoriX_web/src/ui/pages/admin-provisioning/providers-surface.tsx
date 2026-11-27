import { css } from '@emotion/css';
import { Link } from '@tanstack/react-router';
import { Button, Icon, Tag, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { PROVIDERS, providerInstallable, providerRoute } from '@/lib/provisioning/providers';
import { useProvisioningConnections } from '@/hooks/provisioning';

export function ProvidersSurface({ canWrite }: { canWrite: boolean }) {
  const styles = useStyles2(getStyles);
  const connections = useProvisioningConnections();

  const installedHosts = new Set(connections.connections.map((connection) => connection.type));

  return (
    <div className={styles.grid}>
      {PROVIDERS.map((provider) => {
        const installed = installedHosts.has(provider.id);
        const installable = providerInstallable(provider.id);

        return (
          <article key={provider.id} className={styles.card}>
            <header className={styles.head}>
              <span className={styles.mark} aria-hidden="true">
                <Icon name={provider.icon} size="lg" />
              </span>
              <div className={styles.titleBlock}>
                <h2 className={styles.name}>{provider.name}</h2>
                <p className={styles.host}>{provider.host}</p>
              </div>
              <Tag name={installed ? 'Installed' : 'Not installed'} />
            </header>

            <p className={styles.summary}>{provider.summary}</p>

            <footer className={styles.foot}>
              <Link
                to={providerRoute(provider.id)}
                className={styles.detailsLink}
              >
                Configure
              </Link>
              {canWrite ? (
                <Button
                  variant="secondary"
                  size="sm"
                  icon={installable ? 'plus' : 'lock'}
                  disabled={!installable}
                  title={installable ? undefined : 'The server has no GitLab OAuth handler yet'}
                  onClick={() => {
                    window.location.assign(providerRoute(provider.id));
                  }}
                >
                  {installable ? (installed ? 'Re-authorise' : 'Install') : 'Not available yet'}
                </Button>
              ) : null}
            </footer>
          </article>
        );
      })}
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  grid: css({
    display: 'grid',
    gap: theme.spacing(2),
    gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
  }),
  card: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1.5),
    padding: theme.spacing(2.5),
    background: theme.colors.background.primary,
    border: `1px solid ${theme.colors.border.medium}`,
    borderRadius: theme.shape.radius.lg,
  }),
  head: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1.5) }),
  mark: css({
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 40,
    height: 40,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    color: theme.colors.text.primary,
    flexShrink: 0,
  }),
  titleBlock: css({ display: 'flex', flexDirection: 'column', gap: 2, flex: 1, minWidth: 0 }),
  name: css({
    margin: 0,
    fontSize: theme.typography.md.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  host: css({ margin: 0, fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  summary: css({
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    lineHeight: 1.6,
    color: theme.colors.text.secondary,
    flex: 1,
  }),
  foot: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(1),
    paddingTop: theme.spacing(1.5),
    borderTop: `1px solid ${theme.colors.border.weak}`,
  }),
  detailsLink: css({
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.link,
    textDecoration: 'none',
    '&:hover': { textDecoration: 'underline' },
  }),
});
