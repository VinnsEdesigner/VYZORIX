import { css } from '@emotion/css';
import {
  Button,
  Combobox,
  EmptyState,
  LoadingPlaceholder,
  useStyles2,
  type ComboboxOption,
  type VyzorixTheme,
} from '@vyzorix/ui';
import { usePreferencesStore, type IdleTimeoutMinutes } from '@/stores/preferences-store';
import { usePreferencesSync } from '@/hooks/preferences/use-preferences-sync';
import { useRevokeAllDevices, useRevokeSession, useSessions } from '@/hooks/session';
import { describeUserAgent } from '@/lib/session/describe-user-agent';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { useVyzorTranslation } from '@/hooks/i18n';
import { formatDateTime } from './profile-page';

const IDLE_TIMEOUT_KEYS: { value: string; labelKey: string }[] = [
  { value: '15', labelKey: 'security.after15m' },
  { value: '30', labelKey: 'security.after30m' },
  { value: '60', labelKey: 'security.after1h' },
  { value: '0', labelKey: 'security.never' },
];

const STEP_UP_KEYS: { value: string; labelKey: string }[] = [
  { value: 'api-keys', labelKey: 'security.apiKeys' },
  { value: 'sessions', labelKey: 'security.revokingSessions' },
  { value: 'device-wipe', labelKey: 'security.deviceWipe' },
  { value: 'secret-rotate', labelKey: 'security.secretRotation' },
];

function SettingRow({
  label,
  description,
  children,
}: {
  label: string;
  description?: string;
  children: React.ReactNode;
}) {
  const styles = useStyles2(getStyles);
  return (
    <div className={styles.row}>
      <div className={styles.rowText}>
        <span className={styles.rowLabel}>{label}</span>
        {description ? <span className={styles.rowDescription}>{description}</span> : null}
      </div>
      <div className={styles.rowControl}>{children}</div>
    </div>
  );
}

function Group({ title, children }: { title: string; children: React.ReactNode }) {
  const styles = useStyles2(getStyles);
  return (
    <section className={styles.group}>
      <h3 className={styles.groupTitle}>{title}</h3>
      <div className={styles.groupBody}>{children}</div>
    </section>
  );
}

export function SecurityTab() {
  const styles = useStyles2(getStyles);
  const preferences = usePreferencesStore();
  const { setPreference } = usePreferencesSync();
  const { t } = useVyzorTranslation();
  const idleOptions: ComboboxOption[] = IDLE_TIMEOUT_KEYS.map((o) => ({ value: o.value, label: t(o.labelKey) }));
  const stepUpOptions: ComboboxOption[] = STEP_UP_KEYS.map((o) => ({ value: o.value, label: t(o.labelKey) }));

  return (
    <div className={styles.tab}>
      <SessionList />

      <Group title={t('security.sessions')}>
        <SettingRow
          label={t('security.signOutAfter')}
          description={t('security.signOutAfterDesc')}
        >
          <Combobox
            options={idleOptions}
            value={String(preferences.idleTimeoutMinutes)}
            onChange={(next) =>
              next !== null && setPreference('idleTimeoutMinutes', Number(next) as IdleTimeoutMinutes)
            }
            aria-label={t('security.signOutAfter')}
          />
        </SettingRow>
      </Group>

      <Group title={t('security.stepUp')}>
        <SettingRow
          label={t('security.requireDevice')}
          description={t('security.requireDeviceDesc')}
        >
          <Combobox
            options={stepUpOptions}
            value={preferences.stepUpActions[0] ?? ''}
            onChange={() => undefined}
            disabled
            placeholder={t('security.stepUp')}
            aria-label={t('security.requireDevice')}
          />
        </SettingRow>
        <p className={styles.hint}>{t('security.stepUpHint')}</p>
      </Group>
    </div>
  );
}

function SessionList() {
  const styles = useStyles2(getStyles);
  const table = useStyles2(getSessionTableStyles);
  const sessions = useSessions();
  const revokeOne = useRevokeSession();
  const revokeAll = useRevokeAllDevices();

  const rows = sessions.data?.sessions ?? [];

  return (
    <Group title="Active sessions">
      {sessions.isLoading ? <LoadingPlaceholder text="Loading sessions…" /> : null}

      {!sessions.isLoading && rows.length === 0 ? (
        <EmptyState icon="shield" message="No active sessions." />
      ) : null}

      {rows.length > 0 ? (
        <>
          <div className={table.wrapper}>
            <table className={table.table}>
              <thead>
                <tr>
                  <th className={table.headCell}>Signed in</th>
                  <th className={table.headCell}>Expires</th>
                  <th className={table.headCell}>IP address</th>
                  <th className={table.headCell}>Device</th>
                  <th className={table.headCell} />
                </tr>
              </thead>
              <tbody>
                {rows.map((session) => (
                  <tr key={session.id}>
                    <td className={table.cell}>
                      {formatDateTime(session.created_at)}
                      {session.is_current ? <span className={table.subLine}>this device</span> : null}
                    </td>
                    <td className={table.cell}>{formatDateTime(session.expires_at)}</td>
                    <td className={table.cell}>{session.ip_address || '—'}</td>
                    <td className={table.cell}>
                      <span className={table.ellipsis}>{describeUserAgent(session.user_agent)}</span>
                    </td>
                    <td className={table.cell}>
                      {session.is_current ? null : (
                        <Button
                          variant="secondary"
                          size="sm"
                          loading={revokeOne.isPending}
                          onClick={() =>
                            revokeOne.mutate(
                              { id: session.id ?? '' },
                              {
                                onSuccess: () => {
                                  notifySuccess('Session signed out');
                                  void sessions.refetch();
                                },
                                onError: () => notifyError('Could not sign out session'),
                              },
                            )
                          }
                        >
                          Sign out
                        </Button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className={styles.sessionFooter}>
            <Button
              variant="secondary"
              loading={revokeAll.isPending}
              onClick={() =>
                revokeAll.mutate(undefined, {
                  onSuccess: () => {
                    notifySuccess('Signed out everywhere');
                    void sessions.refetch();
                  },
                  onError: () => notifyError('Could not sign out other devices'),
                })
              }
            >
              Sign out other devices
            </Button>
          </div>
        </>
      ) : null}
    </Group>
  );
}

const getSessionTableStyles = (theme: VyzorixTheme) => ({
  wrapper: css({ width: '100%', overflowX: 'auto' }),
  table: css({
    width: '100%',
    borderCollapse: 'separate',
    'tbody tr:nth-of-type(odd)': {
      background: theme.colors.emphasize(theme.colors.background.primary, 0.02),
    },
  }),
  headCell: css({
    padding: `${theme.spacing(0.5)} ${theme.spacing(1)}`,
    textAlign: 'left',
    color: theme.colors.text.secondary,
    fontWeight: theme.typography.fontWeightMedium,
    fontSize: theme.typography.sm.fontSize,
    whiteSpace: 'nowrap',
  }),
  cell: css({
    padding: `${theme.spacing(0.5)} ${theme.spacing(1)}`,
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.primary,
    verticalAlign: 'middle',
    whiteSpace: 'nowrap',
  }),
  ellipsis: css({ display: 'block', maxWidth: 240, overflow: 'hidden', textOverflow: 'ellipsis' }),
  subLine: css({ display: 'block', color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
});

const getStyles = (theme: VyzorixTheme) => ({
  tab: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3) }),
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
  groupBody: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2.5) }),
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
  rowControl: css({ minWidth: 180 }),
  hint: css({ margin: 0, color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  sessionFooter: css({ marginTop: theme.spacing(1) }),
});
