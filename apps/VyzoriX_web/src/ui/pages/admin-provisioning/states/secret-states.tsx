import { css } from '@emotion/css';
import { Icon, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import type { DeclaredSecret, SecretReadiness } from '@/hooks/provisioning';

export function SecretReady({ app, secretKey }: { app: string; secretKey: string }) {
  return <SecretRow app={app} secretKey={secretKey} set />;
}

export function SecretMissing({ app, secretKey }: { app: string; secretKey: string }) {
  return <SecretRow app={app} secretKey={secretKey} set={false} />;
}

function SecretRow({ app, secretKey, set }: { app: string; secretKey: string; set: boolean }) {
  const styles = useStyles2(getStyles, set);

  return (
    <div className={styles.row}>
      <Icon name={set ? 'check-circle' : 'alert-triangle'} size="sm" />
      <span className={styles.app}>{app}</span>
      <code className={styles.key}>{secretKey}</code>
      <span className={styles.verdict}>{set ? 'Set' : 'Not set'}</span>
    </div>
  );
}

export function SecretReadinessList({ readiness }: { readiness: SecretReadiness }) {
  const styles = useStyles2(getStyles, true);

  if (readiness.secrets.length === 0) {
    return <p className={styles.empty}>This manifest declares no secrets.</p>;
  }

  return (
    <div className={styles.list}>
      {readiness.secrets.map((secret: DeclaredSecret) =>
        secret.set ? (
          <SecretReady key={`${secret.app}/${secret.key}`} app={secret.app} secretKey={secret.key} />
        ) : (
          <SecretMissing key={`${secret.app}/${secret.key}`} app={secret.app} secretKey={secret.key} />
        ),
      )}
    </div>
  );
}

const getStyles = (theme: VyzorixTheme, set: boolean) => ({
  list: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.75) }),
  row: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    color: set ? theme.colors.success.text : theme.colors.warning.text,
    fontSize: theme.typography.sm.fontSize,
  }),
  app: css({ color: theme.colors.text.secondary }),
  key: css({
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.primary,
  }),
  verdict: css({ marginLeft: 'auto', color: theme.colors.text.secondary }),
  empty: css({ margin: 0, color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
});
