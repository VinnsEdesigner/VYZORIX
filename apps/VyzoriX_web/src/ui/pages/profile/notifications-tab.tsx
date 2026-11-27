import { useEffect, useState } from 'react';
import { css, cx } from '@emotion/css';
import { Button, Icon, Switch, getFocusStyles, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { useNavigate } from '@tanstack/react-router';
import {
  useNotifications,
  useUpdateNotifications,
  useUnreadNotifications,
  useSendTestPush,
  useSubscribeToPush,
  useUnsubscribeFromPush,
} from '@/hooks/settings';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { useVyzorTranslation } from '@/hooks/i18n';
import { pushSupported, hasPushSubscription, notificationPermission } from '@/lib/push/web-push';
import type { NotificationSettings } from '@/hooks/settings';

interface ChannelRow {
  id: string;
  label: string;
  description: string;
  route: string;
}

export function NotificationsTab() {
  const styles = useStyles2(getStyles);
  const navigate = useNavigate();
  const query = useNotifications();
  const update = useUpdateNotifications();
  const testPush = useSendTestPush();
  const push = useSubscribeToPush();
  const unpush = useUnsubscribeFromPush();
  const { t } = useVyzorTranslation();
  const unread = useUnreadNotifications();
  const [pushReady, setPushReady] = useState(false);
  const [pushBusy, setPushBusy] = useState(false);
  const [pushDenied, setPushDenied] = useState(false);

  useEffect(() => {
    if (!pushSupported()) return;
    setPushDenied(notificationPermission() === 'denied');
    void hasPushSubscription().then(setPushReady);
  }, []);

  const settings = query.data as NotificationSettings | undefined;
  const channels = settings?.channels ?? [];

  const masterOn = settings?.enabled ?? true;

  const channelRows: ChannelRow[] = [
    {
      id: 'email',
      label: t('notif.email'),
      description: settings?.emailAddress
        ? t('notif.emailToCustom', { address: settings.emailAddress })
        : t('notif.emailToAccount'),
      route: '/profile/notifications/email',
    },
    {
      id: 'slack',
      label: t('notif.slack'),
      description: settings?.slack?.connected
        ? t('notif.slackConnected', { team: settings.slack.teamName || '' })
        : t('notif.slackNotConnected'),
      route: '/profile/notifications/slack',
    },
  ];

  const onTogglePush = async (on: boolean) => {
    setPushBusy(true);
    try {
      if (on) {
        await push.subscribe();
        setPushReady(true);
        toggleChannel('push', true);
      } else {
        await unpush.unsubscribe();
        setPushReady(false);
        toggleChannel('push', false);
      }
    } catch (error) {
      const reason = error instanceof Error ? error.message : 'error';
      if (reason === 'permission_denied') setPushDenied(true);
      notifyError(t('notif.pushFailed'), reason === 'permission_denied' ? t('notif.pushPermission') : t('profile.tryAgain'));
    } finally {
      setPushBusy(false);
    }
  };

  const onSendTestPush = () => {
    testPush.mutate(undefined as never, {
      onSuccess: (result) => {
        const delivered = (result as { delivered?: number } | undefined)?.delivered ?? 0;
        if (delivered > 0) {
          notifySuccess(t('notif.pushTestSent'));
        } else {
          notifyError(t('notif.pushTestNone'), t('notif.pushTestNoneHint'));
        }
      },
      onError: () => notifyError(t('notif.pushTestFailed'), t('profile.tryAgain')),
    });
  };

  const save = (patch: Record<string, unknown>) => {
    update.mutate(
      { data: patch as never },
      {
        onSuccess: () => notifySuccess(t('notif.saved')),
        onError: () => notifyError(t('notif.couldNotSave'), t('profile.tryAgain')),
      },
    );
  };

  const toggleChannel = (id: string, on: boolean) => {
    const next = on ? [...new Set([...channels, id])] : channels.filter((channel) => channel !== id);
    save({ channels: next });
  };

  if (query.isLoading) {
    return <div className={styles.loading}>{t('notif.loading')}</div>;
  }

  return (
    <div className={styles.tab}>
      <section className={styles.group}>
        <h3 className={styles.groupTitle}>{t('notif.title')}</h3>
        <div className={styles.groupBody}>
          <div className={styles.row}>
            <div className={styles.rowText}>
              <span className={styles.rowLabel}>{t('notif.enable')}</span>
              <span className={styles.rowDescription}>{t('notif.enableDesc')}</span>
            </div>
            <Switch
              checked={settings?.enabled ?? true}
              onChange={(event) => save({ enabled: event.target.checked })}
              aria-label={t('notif.enable')}
            />
          </div>
        </div>
      </section>

      <section className={styles.group}>
        <h3 className={styles.groupTitle}>
          {t('notif.channels')}
          {unread > 0 ? (
            <span className={styles.countBadge} aria-label={t('account.unreadCount', { count: unread })}>
              {unread}
            </span>
          ) : null}
        </h3>
        <p className={styles.groupHint}>{t('notif.channelsDesc')}</p>
        <div className={cx(styles.groupBody, !masterOn && styles.groupDisabled)}>
          {channelRows.map((channel) => {
            const on = channels.includes(channel.id);
            return (
              <div key={channel.id} className={styles.channelRow}>
                <button
                  type="button"
                  className={styles.channelMain}
                  onClick={() => void navigate({ to: channel.route as never })}
                >
                  <span className={styles.linkRowText}>
                    <span className={styles.rowLabel}>{channel.label}</span>
                    <span className={styles.rowDescription}>{channel.description}</span>
                  </span>
                  <Icon name="chevron-right" size="md" className={styles.chevron} />
                </button>
                <Switch
                  checked={on}
                  disabled={!masterOn}
                  onChange={(event) => toggleChannel(channel.id, event.target.checked)}
                  aria-label={t('notif.enableChannel', { channel: channel.label })}
                />
              </div>
            );
          })}

          {pushSupported() ? (
            <div className={styles.channelRow}>
              <div className={styles.channelMain}>
                <span className={styles.linkRowText}>
                  <span className={styles.rowLabel}>{t('notif.push')}</span>
                  <span className={styles.rowDescription}>
                    {pushDenied ? t('notif.pushBlocked') : t('notif.pushDesc')}
                  </span>
                </span>
              </div>
              <Switch
                checked={pushReady}
                disabled={pushBusy || !masterOn || pushDenied}
                onChange={(event) => void onTogglePush(event.target.checked)}
                aria-label={t('notif.push')}
              />
              {pushReady ? (
                <Button variant="secondary" fill="outline" size="sm" onClick={onSendTestPush} loading={testPush.isPending}>
                  {t('notif.pushTest')}
                </Button>
              ) : null}
            </div>
          ) : null}
        </div>
      </section>

      <button
        type="button"
        className={cx(styles.group, styles.historyRow, unread > 0 && styles.historyRowUnread)}
        onClick={() => void navigate({ to: '/notifications' as never })}
      >
        <span className={styles.linkRowText}>
          <span className={styles.rowLabel}>{t('notif.history')}</span>
          <span className={styles.rowDescription}>{t('notif.historyDesc')}</span>
        </span>
        {unread > 0 ? (
          <span className={styles.countBadge} aria-label={t('account.unreadCount', { count: unread })}>
            {unread}
          </span>
        ) : null}
        <Icon name="chevron-right" size="md" className={styles.chevron} />
      </button>
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  tab: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3) }),
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
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    margin: 0,
    fontSize: theme.typography.h6.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  countBadge: css({
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    minWidth: 20,
    height: 20,
    padding: `0 ${theme.spacing(0.5)}`,
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.error.main,
    color: theme.colors.error.contrastText,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: 1,
  }),
  groupBody: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  groupDisabled: css({
    opacity: 0.5,
    '& button': { pointerEvents: 'none' },
  }),
  row: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
  }),
  groupHint: css({
    margin: `0 0 ${theme.spacing(0.5)}`,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
  }),
  channelRow: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
  }),
  channelMain: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
    flex: 1,
    minWidth: 0,
    padding: theme.spacing(1.5),
    border: 0,
    borderRadius: theme.shape.radius.default,
    background: 'transparent',
    color: 'inherit',
    textAlign: 'left',
    cursor: 'pointer',
    '&:hover': { background: theme.colors.action.hover },
    '&:focus-visible': getFocusStyles(theme),
  }),
  historyRowUnread: css({
    background: theme.colors.error.subtleBackground,
  }),
  historyRow: css({
    display: 'flex',
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    border: 0,
    color: 'inherit',
    textAlign: 'left',
    cursor: 'pointer',
    '&:hover': { background: theme.colors.background.secondary },
    '&:focus-visible': getFocusStyles(theme),
  }),
  linkRowText: css({ display: 'flex', flexDirection: 'column', gap: 2, flex: 1, minWidth: 0 }),
  rowText: css({ display: 'flex', flexDirection: 'column', gap: 2, minWidth: 200 }),
  rowLabel: css({ color: theme.colors.text.primary, fontSize: theme.typography.base.fontSize }),
  rowDescription: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  chevron: css({ flexShrink: 0, color: theme.colors.text.disabled }),
});
