import { css } from '@emotion/css';
import { Button, Combobox, useStyles2, type ComboboxOption, type VyzorixTheme } from '@vyzorix/ui';
import { useNotifications, useSlackChannels, useUpdateNotifications } from '@/hooks/settings';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { useVyzorTranslation } from '@/hooks/i18n';
import type { NotificationSettings, SlackChannelsResult } from '@vyzorix/api-client';

const SLACK_CONNECT_URL = '/api/v1/auth/me/notifications/slack/connect';

export function SlackChannelPage() {
  const styles = useStyles2(getStyles);
  const query = useNotifications();
  const update = useUpdateNotifications();
  const { t } = useVyzorTranslation();

  const settings = query.data as NotificationSettings | undefined;
  const slack = settings?.slack;
  const connected = Boolean(slack?.connected);

  const channelsQuery = useSlackChannels({ query: { enabled: connected } });
  const channels = ((channelsQuery.data as SlackChannelsResult | undefined)?.channels ?? []).filter(
    (channel) => channel.id && channel.name,
  );
  const options: ComboboxOption[] = channels.map((channel) => ({ value: channel.id!, label: `#${channel.name}` }));

  const onConnect = () => {
    window.location.href = SLACK_CONNECT_URL;
  };

  const onChooseChannel = (channelId: string | null) => {
    if (!channelId) return;
    const channel = channels.find((item) => item.id === channelId);
    update.mutate(
      { data: { slack: { channelId, channelName: channel?.name ?? '' } } as never },
      {
        onSuccess: () => notifySuccess(t('slackChannel.saved')),
        onError: () => notifyError(t('slackChannel.couldNotSave'), t('profile.tryAgain')),
      },
    );
  };

  if (query.isLoading) {
    return <div className={styles.loading}>{t('slackChannel.loading')}</div>;
  }

  return (
    <div className={styles.page}>
      <section className={styles.group}>
        <h3 className={styles.groupTitle}>{t('notif.slack')}</h3>
        <div className={styles.groupBody}>
          {connected ? (
            <>
              <div className={styles.row}>
                <div className={styles.rowText}>
                  <span className={styles.rowLabel}>{t('slackChannel.workspace')}</span>
                  <span className={styles.rowDescription}>{slack?.teamName || t('slackChannel.connected')}</span>
                </div>
                <span className={styles.status} data-connected="true">
                  {t('slackChannel.connected')}
                </span>
              </div>

              <div className={styles.field}>
                <span className={styles.rowLabel}>{t('slackChannel.postTo')}</span>
                <Combobox
                  options={options}
                  value={slack?.channelId ?? null}
                  onChange={onChooseChannel}
                  searchable
                  disabled={channelsQuery.isLoading}
                  placeholder={channelsQuery.isLoading ? t('slackChannel.loadingChannels') : t('slackChannel.chooseChannel')}
                  aria-label={t('slackChannel.postTo')}
                />
              </div>
            </>
          ) : (
            <div className={styles.row}>
              <div className={styles.rowText}>
                <span className={styles.rowLabel}>{t('slackChannel.connect')}</span>
                <span className={styles.rowDescription}>
                  {t('slackChannel.connectDesc')}
                </span>
              </div>
              <Button variant="accent" onClick={onConnect}>
                {t('slackChannel.connect')}
              </Button>
            </div>
          )}
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
  field: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5), maxWidth: 360 }),
  status: css({
    color: theme.colors.success.text,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
  }),
});
