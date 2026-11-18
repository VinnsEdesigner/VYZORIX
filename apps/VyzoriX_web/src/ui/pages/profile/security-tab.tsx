import { css } from '@emotion/css';
import { Combobox, useStyles2, type ComboboxOption, type VyzorixTheme } from '@vyzorix/ui';
import { usePreferencesStore, type IdleTimeoutMinutes } from '@/stores/preferences-store';
import { usePreferencesSync } from '@/hooks/preferences/use-preferences-sync';
import { useVyzorTranslation } from '@/hooks/i18n';

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
});
