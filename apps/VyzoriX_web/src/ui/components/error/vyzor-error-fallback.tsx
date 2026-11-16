import { css } from '@emotion/css';
import { useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { useVyzorErrorRecovery } from '@/hooks/error';
import type { VyzorErrorClassification } from '@/lib/error';

interface VyzorErrorFallbackProps {
  classification: VyzorErrorClassification;
  message?: string;
  onRetry?: () => void;
  onReload?: () => void;
  onGoHome?: () => void;
  onDismiss?: () => void;
}

const getStyles = (theme: VyzorixTheme) => ({
  root: css({
    display: 'flex',
    minHeight: 400,
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    gap: theme.spacing(2),
    padding: theme.spacing(4),
    textAlign: 'center',
  }),
  title: css({ fontSize: theme.typography.xxl.fontSize, fontWeight: theme.typography.fontWeightBold, letterSpacing: '-0.01em', margin: 0 }),
  message: css({ color: theme.colors.text.secondary, maxWidth: 448, margin: 0 }),
  status: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize, margin: 0 }),
  actions: css({ display: 'flex', flexWrap: 'wrap', alignItems: 'center', justifyContent: 'center', gap: theme.spacing(1) }),
  primaryBtn: css({
    background: theme.colors.primary.main,
    color: theme.colors.primary.contrastText,
    border: 'none',
    borderRadius: theme.shape.radius.default,
    padding: `${theme.spacing(1)} ${theme.spacing(2)}`,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    cursor: 'pointer',
    '&:hover': { background: theme.colors.primary.mainEmphasis },
  }),
  secondaryBtn: css({
    background: theme.colors.secondary.main,
    color: theme.colors.secondary.contrastText,
    border: 'none',
    borderRadius: theme.shape.radius.default,
    padding: `${theme.spacing(1)} ${theme.spacing(2)}`,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    cursor: 'pointer',
    '&:hover': { background: theme.colors.secondary.shade },
  }),
  linkBtn: css({
    background: 'none',
    border: 'none',
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    textDecoration: 'underline',
    cursor: 'pointer',
    '&:hover': { color: theme.colors.text.primary },
  }),
  dismissBtn: css({
    background: 'none',
    border: 'none',
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    cursor: 'pointer',
    '&:hover': { color: theme.colors.text.primary },
  }),
});

const CATEGORY_TITLES: Record<string, string> = {
  network: 'Connection Error',
  auth: 'Authentication Required',
  server: 'Server Error',
  render: 'Something Went Wrong',
  route: 'Page Not Found',
  unknown: 'Unexpected Error',
};

export function VyzorErrorFallback({
  classification,
  message,
  onRetry,
  onReload,
  onGoHome,
  onDismiss,
}: VyzorErrorFallbackProps) {
  const styles = useStyles2(getStyles);
  const recovery = useVyzorErrorRecovery();
  const title = CATEGORY_TITLES[classification.category] ?? 'Error';
  const handleRetry = onRetry ?? recovery.retry;
  const handleReload = onReload ?? recovery.reload;
  const handleGoHome = onGoHome ?? recovery.goHome;
  const handleDismiss = onDismiss ?? recovery.dismiss;

  return (
    <div
      role="alert"
      aria-live="assertive"
      className={styles.root}
    >
      <h1 className={styles.title}>{title}</h1>
      {message && <p className={styles.message}>{message}</p>}
      {classification.statusCode && (
        <p className={styles.status}>
          Status: {classification.statusCode}
        </p>
      )}
      <div className={styles.actions}>
        {classification.retryable && (
          <button
            type="button"
            onClick={handleRetry}
            className={styles.primaryBtn}
          >
            Retry
          </button>
        )}
        <button
          type="button"
          onClick={handleReload}
          className={styles.secondaryBtn}
        >
          Reload
        </button>
        <button
          type="button"
          onClick={handleGoHome}
          className={styles.linkBtn}
        >
          Go Home
        </button>
        {onDismiss && (
          <button
            type="button"
            onClick={handleDismiss}
            className={styles.dismissBtn}
          >
            Dismiss
          </button>
        )}
      </div>
    </div>
  );
}
