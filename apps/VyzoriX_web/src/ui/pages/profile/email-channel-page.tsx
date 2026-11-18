import { useEffect, useState } from 'react';
import { css } from '@emotion/css';
import { Button, Switch, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { useNotifications, useUpdateNotifications } from '@/hooks/settings';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { describeError } from '@/lib/error';
import { useCurrentUser } from '@/hooks/_shared';
import { useVyzorTranslation } from '@/hooks/i18n';
import type { NotificationSettings } from '@vyzorix/api-client';

type EmailEventKey = 'thresholdBreach' | 'deviceOffline' | 'deviceOnline' | 'updateAvailable' | 'registrationRequest';

const EMAIL_EVENT_KEYS: { key: EmailEventKey; labelKey: string; descKey: string }[] = [
  { key: 'thresholdBreach', labelKey: 'notif.thresholdBreach', descKey: 'notif.thresholdBreachDesc' },
  { key: 'deviceOffline', labelKey: 'notif.deviceOffline', descKey: 'notif.deviceOfflineDesc' },
  { key: 'deviceOnline', labelKey: 'notif.deviceOnline', descKey: 'notif.deviceOnlineDesc' },
  { key: 'updateAvailable', labelKey: 'notif.updateAvailable', descKey: 'notif.updateAvailableDesc' },
  { key: 'registrationRequest', labelKey: 'notif.registrationRequest', descKey: 'notif.registrationRequestDesc' },
];

export function EmailChannelPage() {
  const styles = useStyles2(getStyles);
  const user = useCurrentUser();
  const query = useNotifications();
  const update = useUpdateNotifications();
  const { t } = useVyzorTranslation();

  const settings = query.data as NotificationSettings | undefined;
  const [useCustom, setUseCustom] = useState(false);
  const [address, setAddress] = useState('');

  useEffect(() => {
    if (!settings) return;
    setUseCustom(Boolean(settings.emailAddress));
    setAddress(settings.emailAddress ?? '');
  }, [settings]);

  const save = (patch: Record<string, unknown>, message = t('emailChannel.saved')) => {
    update.mutate(
      { data: patch as never },
      {
        onSuccess: () => notifySuccess(message),
        onError: (error) => notifyError(t('emailChannel.couldNotSave'), describeError(error)),
      },
    );
  };

  const onSaveAddress = () => {
    save({ emailAddress: useCustom ? address.trim() : '' });
  };

  const toggleEvent = (key: EmailEventKey, checked: boolean) => {
    save({ email: { ...(settings?.email ?? {}), [key]: checked } });
  };

  if (query.isLoading) {
    return <div className={styles.loading}>{t('emailChannel.loading')}</div>;
  }

  return (
    <div className={styles.page}>
      <section className={styles.group}>
        <h3 className={styles.groupTitle}>{t('emailChannel.sendTo')}</h3>
        <div className={styles.groupBody}>
          <label className={styles.radioRow}>
            <input
              type="radio"
              name="email-destination"
              checked={!useCustom}
              onChange={() => setUseCustom(false)}
              aria-label={t('emailChannel.account')}
            />
            <span className={styles.radioText}>
              <span className={styles.rowLabel}>{t('emailChannel.account')}</span>
              <span className={styles.rowDescription}>{user.email ?? 'your account address'}</span>
            </span>
          </label>

          <label className={styles.radioRow}>
            <input
              type="radio"
              name="email-destination"
              checked={useCustom}
              onChange={() => setUseCustom(true)}
              aria-label={t('emailChannel.different')}
            />
            <span className={styles.radioText}>
              <span className={styles.rowLabel}>{t('emailChannel.different')}</span>
              <span className={styles.rowDescription}>{t('emailChannel.differentDesc')}</span>
            </span>
          </label>

          {useCustom ? (
            <input
              className={styles.textInput}
              type="email"
              value={address}
              onChange={(event) => setAddress(event.target.value)}
              placeholder="alerts@example.com"
              aria-label={t('emailChannel.address')}
            />
          ) : null}

          <div className={styles.actions}>
            <Button variant="accent" onClick={onSaveAddress} loading={update.isPending}>
              {t('emailChannel.saveAddress')}
            </Button>
          </div>
        </div>
      </section>

      <section className={styles.group}>
        <h3 className={styles.groupTitle}>{t('emailChannel.events')}</h3>
        <div className={styles.groupBody}>
          {EMAIL_EVENT_KEYS.map((event) => (
            <div key={event.key} className={styles.row}>
              <div className={styles.rowText}>
                <span className={styles.rowLabel}>{t(event.labelKey)}</span>
                <span className={styles.rowDescription}>{t(event.descKey)}</span>
              </div>
              <Switch
                checked={Boolean(settings?.email?.[event.key])}
                onChange={(changeEvent) => toggleEvent(event.key, changeEvent.target.checked)}
                aria-label={t(event.labelKey)}
              />
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  page: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3) }),
  loading: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  group: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
    padding: theme.spacing(2.5),
    background: theme.colors.background.primary,
    borderRadius: theme.shape.radius.lg,
  }),
  groupTitle: css({
    margin: 0,
    fontSize: theme.typography.h6.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  groupBody: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  radioRow: css({ display: 'flex', alignItems: 'flex-start', gap: theme.spacing(1.5), cursor: 'pointer' }),
  radioText: css({ display: 'flex', flexDirection: 'column', gap: 2 }),
  textInput: css({
    maxWidth: 360,
    borderRadius: theme.shape.radius.sm,
    border: `1px solid ${theme.colors.border.medium}`,
    padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`,
    background: theme.colors.background.primary,
    color: theme.colors.text.primary,
    fontFamily: 'inherit',
    fontSize: theme.typography.base.fontSize,
  }),
  actions: css({ display: 'flex', gap: theme.spacing(1) }),
  row: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
  }),
  rowText: css({ display: 'flex', flexDirection: 'column', gap: 2, minWidth: 200 }),
  rowLabel: css({ color: theme.colors.text.primary, fontSize: theme.typography.base.fontSize }),
  rowDescription: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
});
