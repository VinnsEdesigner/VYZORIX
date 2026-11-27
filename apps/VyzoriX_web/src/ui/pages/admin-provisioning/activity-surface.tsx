import { css } from '@emotion/css';
import { Button, Checkbox, Icon, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import {
  actorKindLabel,
  useDismissProvisioningEvents,
  useProvisioningEvents,
  type ProvisioningEvent,
} from '@/hooks/provisioning';
import { formatTimestamp } from '@/lib/preferences/format';
import { asQueryLike, emptyWhenList, queryView } from '@/lib/provisioning/query-view';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { describeError } from '@/lib/error';
import { useLongPress, useSelection } from '@/ui/components/shared/use-long-press';
import { ActivityEmpty, ActivityError, ActivityLoading } from './states';

export function ActivitySurface({ canWrite }: { canWrite: boolean }) {
  const styles = useStyles2(getStyles);
  const events = useProvisioningEvents(50);
  const dismiss = useDismissProvisioningEvents();
  const selection = useSelection();

  const view = queryView(asQueryLike(events), () => emptyWhenList(events.events));

  if (view.state === 'loading') return <ActivityLoading />;
  if (view.state === 'error') {
    return <ActivityError error={view.error} onRetry={() => void events.refetch()} />;
  }
  if (view.state === 'empty') return <ActivityEmpty />;

  const rows = events.events;
  const ids = rows.map((event) => event.id);

  const dismissSelected = async () => {
    const chosen = [...selection.selected];
    if (chosen.length === 0) return;

    try {
      const result = await dismiss.mutateAsync({ data: { ids: chosen } });
      selection.clear();

      if (result.dismissed === 0) {
        notifyError('Nothing was dismissed', 'Those events were already hidden.');
      } else {
        notifySuccess(
          `Dismissed ${result.dismissed} ${result.dismissed === 1 ? 'event' : 'events'}`,
          'Hidden from this list; the record is kept.',
        );
      }
    } catch (error) {
      notifyError('Could not dismiss the selection', describeError(error));
    }
  };

  return (
    <div className={styles.wrap}>
      <div className={styles.bar}>
        {selection.count > 0 ? (
          <>
            <span className={styles.count}>
              {selection.count} {selection.count === 1 ? 'event' : 'events'} selected
            </span>
            <Button variant="secondary" size="sm" onClick={selection.clear}>
              Clear
            </Button>
            {canWrite ? (
              <Button
                variant="destructive"
                size="sm"
                icon="eye-off"
                onClick={() => void dismissSelected()}
                loading={dismiss.isPending}
              >
                Dismiss
              </Button>
            ) : null}
          </>
        ) : (
          <>
            <span className={styles.hint}>Press and hold a row to select, or use the checkboxes.</span>
            {canWrite ? (
              <Button variant="secondary" size="sm" onClick={() => selection.selectAll(ids)}>
                Select all
              </Button>
            ) : null}
          </>
        )}
        <div className={styles.spacer} />
        <span className={styles.hint}>
          Dismissing hides an event here. The record is kept, because the trail says who changed what.
        </span>
      </div>

      <ul className={styles.list}>
        {rows.map((event) => (
          <ActivityRow
            key={event.id}
            event={event}
            selected={selection.isSelected(event.id)}
            selectable={canWrite}
            onToggle={() => selection.toggle(event.id)}
          />
        ))}
      </ul>
    </div>
  );
}

function ActivityRow({
  event,
  selected,
  selectable,
  onToggle,
}: {
  event: ProvisioningEvent;
  selected: boolean;
  selectable: boolean;
  onToggle: () => void;
}) {
  const styles = useStyles2(getStyles, selected);
  const longPress = useLongPress(onToggle, selectable);

  return (
    <li
      className={styles.row}
      data-selected={selected || undefined}
      {...(selectable ? longPress : {})}
    >
      {selectable ? (
        <Checkbox checked={selected} onChange={onToggle} aria-label={`Select event ${event.type}`} />
      ) : null}
      <span className={styles.when}>
        {event.createdAt ? formatTimestamp(event.createdAt.getTime(), { relative: true }) : '—'}
      </span>
      <span className={styles.who}>
        <span className={styles.actor}>{event.actorLabel || event.actorId}</span>
        <span className={styles.actorKind}>{actorKindLabel(event.actorKind)}</span>
      </span>
      <span className={styles.type}>{event.type}</span>
      <span className={event.result === 'error' ? styles.failed : styles.succeeded}>
        <Icon name={event.result === 'error' ? 'x-circle' : 'check-circle'} size="xs" />
        {event.result}
      </span>
      <span className={styles.detail} title={event.detail}>
        {event.detail}
      </span>
    </li>
  );
}

const getStyles = (theme: VyzorixTheme, selected?: boolean) => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1.5) }),
  bar: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1), flexWrap: 'wrap' }),
  spacer: css({ flex: 1 }),
  count: css({
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  hint: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  list: css({ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 2 }),
  row: css({
    display: 'grid',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    gridTemplateColumns: 'auto 90px minmax(120px, 1fr) minmax(120px, 1fr) 90px minmax(0, 2fr)',
    padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`,
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${selected ? theme.colors.primary.main : 'transparent'}`,
    background: selected ? theme.colors.primary.subtleBackground : 'transparent',
    fontSize: theme.typography.sm.fontSize,

    userSelect: 'none',
    '&:hover': {
      background: selected ? theme.colors.primary.subtleBackground : theme.colors.background.secondary,
    },
    [theme.breakpoints.down('md')]: { gridTemplateColumns: 'auto 1fr', rowGap: 2 },
  }),
  when: css({ color: theme.colors.text.secondary, whiteSpace: 'nowrap' }),
  who: css({ display: 'flex', flexDirection: 'column', gap: 1, minWidth: 0 }),
  actor: css({ color: theme.colors.text.primary, overflow: 'hidden', textOverflow: 'ellipsis' }),
  actorKind: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  type: css({ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', color: theme.colors.text.primary }),
  succeeded: css({ display: 'inline-flex', alignItems: 'center', gap: 4, color: theme.colors.success.text }),
  failed: css({ display: 'inline-flex', alignItems: 'center', gap: 4, color: theme.colors.error.text }),
  detail: css({
    color: theme.colors.text.secondary,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  }),
});
