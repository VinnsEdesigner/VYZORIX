import { useMemo, useState } from 'react';
import { css, cx } from '@emotion/css';
import { Button, EmptyState, Icon, LoadingPlaceholder, Switch, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { useQueryClient } from '@tanstack/react-query';
import { getGetAuthMeNotificationsHistoryQueryKey, useNotificationHistory, useMarkNotificationsRead } from '@/hooks/settings';
import { useVyzorTranslation } from '@/hooks/i18n';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import type { NotificationHistoryEntry, NotificationHistoryResult } from '@vyzorix/api-client';

const PAGE_SIZE = 25;
const LIVE_INTERVAL_MS = 15_000;

const CHANNEL_ICON: Record<string, 'mail' | 'bell' | 'link' | 'zap'> = {
  email: 'mail',
  slack: 'link',
  webhook: 'zap',
};

function eventLabelKey(eventType: string): string {
  const map: Record<string, string> = {
    threshold_breach: 'notif.thresholdBreach',
    device_offline: 'notif.deviceOffline',
    device_online: 'notif.deviceOnline',
    update_available: 'notif.updateAvailable',
    registration_request: 'notif.registrationRequest',
    error: 'notif.errorEvent',
  };
  return map[eventType] ?? '';
}

export function NotificationHistoryPage() {
  const styles = useStyles2(getStyles);
  const { t } = useVyzorTranslation();
  const queryClient = useQueryClient();
  const markRead = useMarkNotificationsRead();

  const [channel, setChannel] = useState('');
  const [status, setStatus] = useState('');
  const [offset, setOffset] = useState(0);
  const [live, setLive] = useState(false);

  const params = useMemo(
    () => ({
      limit: PAGE_SIZE,
      offset,
      ...(channel ? { channel } : {}),
      ...(status ? { status } : {}),
    }) as { limit: number; offset: number; channel?: string; status?: string },
    [channel, status, offset],
  );

  const query = useNotificationHistory(params, {
    query: { refetchInterval: live ? LIVE_INTERVAL_MS : false },
  });
  const data = query.data as NotificationHistoryResult | undefined;
  const entries = data?.entries ?? [];
  const total = data?.total ?? 0;
  const unread = data?.unread ?? 0;
  const page = Math.floor(offset / PAGE_SIZE) + 1;
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const onMarkAllRead = () => {
    markRead.mutate(undefined as never, {
      onSuccess: () => {
        notifySuccess(t('history.markedRead'));
        void queryClient.invalidateQueries({ queryKey: getGetAuthMeNotificationsHistoryQueryKey() });
      },
      onError: () => notifyError(t('history.markReadFailed'), t('profile.tryAgain')),
    });
  };

  const onExport = () => {
    if (entries.length === 0) return;
    const header = 'sent_at,event_type,channel,device_id,status,error\n';
    const body = entries
      .map((entry) =>
        [
          new Date(entry.sentAt ?? 0).toISOString(),
          entry.eventType ?? '',
          entry.channel ?? '',
          entry.deviceId ?? '',
          entry.success ? 'sent' : 'failed',
          (entry.errorMsg ?? '').replace(/"/g, '""'),
        ]
          .map((cell) => `"${cell}"`)
          .join(','),
      )
      .join('\n');

    const blob = new Blob([header + body], { type: 'text/csv;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `notification-history-${new Date().toISOString().slice(0, 10)}.csv`;
    link.click();
    URL.revokeObjectURL(url);
  };

  const filters = [
    { id: '', label: t('history.filterAll') },
    { id: 'email', label: t('notif.email') },
    { id: 'slack', label: t('notif.slack') },
    { id: 'webhook', label: t('history.webhook') },
  ];
  const statuses = [
    { id: '', label: t('history.statusAll') },
    { id: 'sent', label: t('history.sent') },
    { id: 'failed', label: t('history.failed') },
  ];

  const formatWhen = (ms: number) =>
    new Date(ms).toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <div className={styles.toolbarLeft}>
          {unread > 0 ? (
            <span className={styles.unreadBadge} aria-label={t('account.unreadCount', { count: unread })}>
              {unread}
            </span>
          ) : null}
          <span className={styles.toolbarText}>
            {unread > 0 ? t('history.unreadCount', { count: unread }) : t('history.allRead')}
          </span>
        </div>
        <div className={styles.toolbarActions}>
          <label className={styles.liveToggle}>
            <Switch checked={live} onChange={(event) => setLive(event.target.checked)} aria-label={t('history.live')} />
            <span>{t('history.live')}</span>
          </label>
          <Button variant="secondary" fill="outline" size="sm" onClick={onMarkAllRead} disabled={unread === 0 || markRead.isPending}>
            {t('history.markAllRead')}
          </Button>
          <Button variant="secondary" fill="outline" size="sm" icon="download" onClick={onExport} disabled={entries.length === 0}>
            {t('history.export')}
          </Button>
        </div>
      </div>

      <div className={styles.filters}>
        <div className={styles.filterGroup} role="group" aria-label={t('history.channel')}>
          {filters.map((filter) => (
            <button
              key={filter.id || 'all'}
              type="button"
              className={cx(styles.chip, filter.id === channel && styles.chipActive)}
              aria-pressed={filter.id === channel}
              onClick={() => {
                setChannel(filter.id);
                setOffset(0);
              }}
            >
              {filter.label}
            </button>
          ))}
        </div>
        <div className={styles.filterGroup} role="group" aria-label={t('history.status')}>
          {statuses.map((filter) => (
            <button
              key={filter.id || 'all'}
              type="button"
              className={cx(styles.chip, filter.id === status && styles.chipActive)}
              aria-pressed={filter.id === status}
              onClick={() => {
                setStatus(filter.id);
                setOffset(0);
              }}
            >
              {filter.label}
            </button>
          ))}
        </div>
      </div>

      {query.isLoading ? (
        <LoadingPlaceholder text={t('history.loading')} />
      ) : entries.length === 0 ? (
        <EmptyState message={t('history.empty')} icon="bell" />
      ) : (
        <ul className={styles.list}>
          {entries.map((entry) => (
            <HistoryRow key={entry.id} entry={entry} when={formatWhen(entry.sentAt ?? 0)} />
          ))}
        </ul>
      )}

      {total > PAGE_SIZE ? (
        <div className={styles.pager}>
          <button
            type="button"
            className={styles.pageButton}
            disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
          >
            {t('history.previous')}
          </button>
          <span className={styles.pageInfo}>{t('history.pageOf', { page, pages: pageCount })}</span>
          <button
            type="button"
            className={styles.pageButton}
            disabled={offset + PAGE_SIZE >= total}
            onClick={() => setOffset(offset + PAGE_SIZE)}
          >
            {t('history.next')}
          </button>
        </div>
      ) : null}
    </div>
  );
}

function HistoryRow({ entry, when }: { entry: NotificationHistoryEntry; when: string }) {
  const styles = useStyles2(getStyles);
  const { t } = useVyzorTranslation();
  const labelKey = eventLabelKey(entry.eventType ?? '');

  return (
    <li className={styles.item}>
      <span className={cx(styles.icon, entry.success ? styles.iconOk : styles.iconFailed)}>
        <Icon name={CHANNEL_ICON[entry.channel ?? ''] ?? 'bell'} size="md" />
      </span>
      <div className={styles.itemBody}>
        <div className={styles.itemHead}>
          <span className={styles.itemTitle}>
            {labelKey ? t(labelKey) : entry.eventType}
          </span>
          <span className={styles.itemChannel}>{entry.channel}</span>
        </div>
        <div className={styles.itemMeta}>
          <span>{when}</span>
          {entry.deviceId ? <span>· {entry.deviceId}</span> : null}
        </div>
        {!entry.success && entry.errorMsg ? <div className={styles.itemError}>{entry.errorMsg}</div> : null}
      </div>
      <span className={cx(styles.pill, entry.success ? styles.pillOk : styles.pillFailed)}>
        {entry.success ? t('history.sent') : t('history.failed')}
      </span>
    </li>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  page: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2.5) }),
  toolbar: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
  }),
  toolbarLeft: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1) }),
  toolbarText: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  unreadBadge: css({
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    minWidth: 22,
    height: 22,
    padding: `0 ${theme.spacing(0.5)}`,
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.error.main,
    color: theme.colors.error.contrastText,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    lineHeight: 1,
  }),
  toolbarActions: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1.5), flexWrap: 'wrap' }),
  liveToggle: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
    cursor: 'pointer',
  }),
  filters: css({
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(2),
    justifyContent: 'space-between',
  }),
  filterGroup: css({ display: 'inline-flex', flexWrap: 'wrap', gap: theme.spacing(0.75) }),
  chip: css({
    border: 0,
    borderRadius: theme.shape.radius.pill,
    padding: `${theme.spacing(0.5)} ${theme.spacing(1.5)}`,
    background: theme.colors.background.secondary,
    color: theme.colors.text.secondary,
    fontFamily: 'inherit',
    fontSize: theme.typography.sm.fontSize,
    cursor: 'pointer',
    '&:hover': { color: theme.colors.text.primary },
  }),
  chipActive: css({
    background: theme.colors.accent.subtleBackground,
    color: theme.colors.accent.text,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  list: css({ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  item: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
    padding: theme.spacing(1.75, 2),
    background: theme.colors.background.primary,
    borderRadius: theme.shape.radius.default,
  }),
  icon: css({
    display: 'grid',
    placeItems: 'center',
    width: 36,
    height: 36,
    flexShrink: 0,
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.background.secondary,
    color: theme.colors.text.secondary,
  }),
  iconOk: css({ color: theme.colors.success.text }),
  iconFailed: css({ color: theme.colors.error.text }),
  itemBody: css({ display: 'flex', flexDirection: 'column', gap: 2, flex: 1, minWidth: 0 }),
  itemHead: css({ display: 'flex', alignItems: 'baseline', gap: theme.spacing(1) }),
  itemTitle: css({ color: theme.colors.text.primary, fontWeight: theme.typography.fontWeightMedium }),
  itemChannel: css({
    color: theme.colors.text.disabled,
    fontSize: theme.typography.sm.fontSize,
    textTransform: 'capitalize',
  }),
  itemMeta: css({ display: 'flex', gap: theme.spacing(0.5), color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  itemError: css({ color: theme.colors.error.text, fontSize: theme.typography.sm.fontSize, wordBreak: 'break-word' }),
  pill: css({
    flexShrink: 0,
    padding: `${theme.spacing(0.25)} ${theme.spacing(1)}`,
    borderRadius: theme.shape.radius.pill,
    fontSize: theme.typography.sm.fontSize,
    background: theme.colors.background.secondary,
  }),
  pillOk: css({ color: theme.colors.success.text }),
  pillFailed: css({ color: theme.colors.error.text }),
  pager: css({ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: theme.spacing(2) }),
  pageButton: css({
    border: `1px solid ${theme.colors.border.medium}`,
    borderRadius: theme.shape.radius.default,
    padding: `${theme.spacing(0.5)} ${theme.spacing(1.5)}`,
    background: 'transparent',
    color: theme.colors.text.primary,
    fontFamily: 'inherit',
    fontSize: theme.typography.sm.fontSize,
    cursor: 'pointer',
    '&:disabled': { opacity: theme.colors.action.disabledOpacity, cursor: 'not-allowed' },
  }),
  pageInfo: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
});
