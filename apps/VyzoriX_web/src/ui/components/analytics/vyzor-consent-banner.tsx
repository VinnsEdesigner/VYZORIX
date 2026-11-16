import { css } from '@emotion/css';
import { useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { useVyzorConsent } from '@/hooks/analytics';

const getStyles = (theme: VyzorixTheme) => ({
  banner: css({
    position: 'fixed',
    bottom: 0,
    left: 0,
    right: 0,
    zIndex: theme.zIndex.modal,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    borderTop: `1px solid ${theme.colors.border.medium}`,
    background: theme.colors.background.primary,
    padding: theme.spacing(2),
    boxShadow: theme.shadows.z2,
  }),
  text: css({
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.primary,
    margin: 0,
  }),
  actions: css({ display: 'flex', flexShrink: 0, gap: theme.spacing(1) }),
  decline: css({
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.medium}`,
    padding: `${theme.spacing(0.75)} ${theme.spacing(1.5)}`,
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.secondary,
    background: 'transparent',
    cursor: 'pointer',
    '&:hover': { background: theme.colors.action.hover },
  }),
  accept: css({
    borderRadius: theme.shape.radius.default,
    border: 'none',
    padding: `${theme.spacing(0.75)} ${theme.spacing(1.5)}`,
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.primary.contrastText,
    background: theme.colors.primary.main,
    cursor: 'pointer',
    '&:hover': { background: theme.colors.primary.mainEmphasis },
  }),
});

export function VyzorConsentBanner() {
  const styles = useStyles2(getStyles);
  const { consent, bannerDismissed, grant, deny } = useVyzorConsent();

  if (consent !== 'pending' || bannerDismissed) return null;

  return (
    <div role="dialog" aria-label="Analytics consent" className={styles.banner}>
      <p className={styles.text}>
        We use anonymous analytics to improve VyzoriX. No personal data is collected.
      </p>
      <div className={styles.actions}>
        <button type="button" onClick={deny} className={styles.decline}>
          Decline
        </button>
        <button type="button" onClick={grant} className={styles.accept}>
          Accept
        </button>
      </div>
    </div>
  );
}
