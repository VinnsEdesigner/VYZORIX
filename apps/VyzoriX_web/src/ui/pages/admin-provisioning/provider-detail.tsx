import { css } from '@emotion/css';
import { Link } from '@tanstack/react-router';
import { Alert, Button, Icon, Tag, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { describeError } from '@/lib/error';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { providerById, providerInstallable, type ProviderId } from '@/lib/provisioning/providers';
import { useAuthorizeProvider } from '@/hooks/provisioning';

interface ProviderDetailProps {
  providerId: ProviderId;
  canWrite: boolean;
}

export function ProviderDetail({ providerId, canWrite }: ProviderDetailProps) {
  const styles = useStyles2(getStyles);
  const provider = providerById(providerId);
  const { authorizeGithub, isAuthorizing, isError, error } = useAuthorizeProvider();

  if (!provider) {
    return <p className={styles.prose}>Unknown provider.</p>;
  }

  const installable = providerInstallable(provider.id);

  const authorize = async () => {
    try {
      await authorizeGithub(`${window.location.origin}/admin/provisioning/providers/${provider.id}`);
      notifySuccess('Redirecting to ' + provider.name, 'Approve the request there to finish.');
    } catch (caught) {

      notifyError(`Could not start ${provider.name} authorisation`, describeError(caught));
    }
  };

  return (
    <div className={styles.wrap}>
      <Link to="/admin/provisioning" search={{ tab: 'providers' }} className={styles.back}>
        <Icon name="arrow-left" size="sm" />
        All providers
      </Link>

      <header className={styles.head}>
        <span className={styles.mark} aria-hidden="true">
          <Icon name={provider.icon} size="xl" />
        </span>
        <div className={styles.titleBlock}>
          <h2 className={styles.name}>{provider.name}</h2>
          <p className={styles.host}>{provider.host}</p>
        </div>
        <Tag name={installable ? 'Available' : 'Not available yet'} />
      </header>

      <p className={styles.prose}>{provider.summary}</p>

      {!installable ? (
        <Alert severity="info" title={`${provider.name} authorisation is not wired up yet`}>
          The server has no {provider.name} OAuth handler, so installing it would only fail.
          The scopes and setup below are what it will ask for once that lands.
        </Alert>
      ) : null}

      <section className={styles.section}>
        <h3 className={styles.sectionTitle}>What authorising grants</h3>
        <ul className={styles.scopeList}>
          {provider.scopes.map((scope) => (
            <li key={scope.scope} className={styles.scopeRow}>
              <code className={styles.scope}>{scope.scope}</code>
              <span className={styles.scopeReason}>{scope.reason}</span>
            </li>
          ))}
        </ul>
      </section>

      {canWrite ? (
        <footer className={styles.foot}>
          <Button
            variant="primary"
            icon="log-in"
            loading={isAuthorizing}
            disabled={!installable}
            onClick={() => void authorize()}
          >
            {isAuthorizing ? 'Opening ' + provider.name + '…' : `Authorise with ${provider.name}`}
          </Button>
          {isError ? <span className={styles.errorText}>{describeError(error)}</span> : null}
        </footer>
      ) : (
        <p className={styles.prose}>You can read this page, but authorising needs admin authority.</p>
      )}
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3), maxWidth: 760 }),
  back: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.75),
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.secondary,
    textDecoration: 'none',
    '&:hover': { color: theme.colors.text.primary },
  }),
  head: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(2) }),
  mark: css({
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 56,
    height: 56,
    borderRadius: theme.shape.radius.lg,
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    color: theme.colors.text.primary,
    flexShrink: 0,
  }),
  titleBlock: css({ display: 'flex', flexDirection: 'column', gap: 2, flex: 1, minWidth: 0 }),
  name: css({
    margin: 0,
    fontSize: theme.typography.xl.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    letterSpacing: '-0.01em',
    color: theme.colors.text.primary,
  }),
  host: css({ margin: 0, fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  prose: css({
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    lineHeight: 1.65,
    color: theme.colors.text.secondary,
  }),
  section: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1.5) }),
  sectionTitle: css({
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
  }),
  scopeList: css({ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  scopeRow: css({
    display: 'flex',
    alignItems: 'baseline',
    gap: theme.spacing(1.5),
    paddingBottom: theme.spacing(1),
    borderBottom: `1px solid ${theme.colors.border.weak}`,
  }),
  scope: css({
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.primary,
    whiteSpace: 'nowrap',
  }),
  scopeReason: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  foot: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
    paddingTop: theme.spacing(1),
    flexWrap: 'wrap',
  }),
  errorText: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.error.text }),
});
