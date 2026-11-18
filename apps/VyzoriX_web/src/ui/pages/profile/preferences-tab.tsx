import { css } from '@emotion/css';
import { Combobox, Switch, useStyles2, type ComboboxOption, type VyzorixTheme } from '@vyzorix/ui';
import { SegmentedSetting } from '@/ui/components/preferences/segmented-setting';
import { usePreferencesStore } from '@/stores/preferences-store';
import { usePreferencesSync } from '@/hooks/preferences/use-preferences-sync';
import { useVyzorI18nStore } from '@/stores/vyzor-i18n-store';
import { VYZOR_AVAILABLE_LANGUAGES } from '@/lib/i18n';
import { useVyzorTranslation } from '@/hooks/i18n';

const LANGUAGE_OPTIONS: ComboboxOption[] = VYZOR_AVAILABLE_LANGUAGES.map((language) => ({
  value: language.value,
  label: language.label,
}));

const LANDING_OPTIONS: ComboboxOption[] = [
  { value: 'last', label: 'Last visited page' },
  { value: 'home', label: 'Home' },
  { value: 'devices', label: 'Devices' },
  { value: 'logs', label: 'Logs' },
];

const TIME_RANGE_OPTIONS: ComboboxOption[] = [
  { value: '15m', label: 'Last 15 minutes' },
  { value: '1h', label: 'Last hour' },
  { value: '6h', label: 'Last 6 hours' },
  { value: '24h', label: 'Last 24 hours' },
  { value: '7d', label: 'Last 7 days' },
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

export function PreferencesTab() {
  const styles = useStyles2(getStyles);
  const preferences = usePreferencesStore();
  const { setPreference } = usePreferencesSync();
  const { t } = useVyzorTranslation();
  const locale = useVyzorI18nStore((s) => s.locale);
  const setLocale = useVyzorI18nStore((s) => s.setLocale);

  return (
    <div className={styles.tab}>
      <Group title={t('prefs.languageRegion')}>
        <SettingRow label={t('prefs.language')} description={t('prefs.languageDesc')}>
          <Combobox
            options={LANGUAGE_OPTIONS}
            value={locale}
            onChange={(next) => next && setLocale(next as typeof locale)}
            aria-label={t('prefs.language')}
          />
        </SettingRow>
      </Group>

      <Group title={t('prefs.appearance')}>
        <SegmentedSetting
          label={t('prefs.density')}
          description={t('prefs.densityDesc')}
          value={preferences.density}
          onChange={(value) => setPreference('density', value)}
          options={[
            { label: t('prefs.compact'), value: 'compact' },
            { label: t('prefs.comfortable'), value: 'comfortable' },
            { label: t('prefs.spacious'), value: 'spacious' },
          ]}
        />
        <SegmentedSetting
          label={t('prefs.textSize')}
          description={t('prefs.textSizeDesc')}
          value={preferences.textScale}
          onChange={(value) => setPreference('textScale', value)}
          options={[
            { label: t('prefs.small'), value: 'small' },
            { label: t('prefs.default'), value: 'default' },
            { label: t('prefs.large'), value: 'large' },
          ]}
        />
        <SettingRow label={t('prefs.reduceMotion')} description={t('prefs.reduceMotionDesc')}>
          <Switch
            checked={preferences.reduceMotion}
            onChange={(event) => setPreference('reduceMotion', event.target.checked)}
            aria-label={t('prefs.reduceMotion')}
          />
        </SettingRow>
        <SegmentedSetting
          label={t('prefs.timeFormat')}
          description={t('prefs.timeFormatDesc')}
          value={preferences.timeFormat}
          onChange={(value) => setPreference('timeFormat', value)}
          options={[
            { label: t('prefs.24h'), value: '24h' },
            { label: t('prefs.12h'), value: '12h' },
          ]}
        />
        <SettingRow label={t('prefs.relativeTimestamps')} description={t('prefs.relativeTimestampsDesc')}>
          <Switch
            checked={preferences.relativeTimestamps}
            onChange={(event) => setPreference('relativeTimestamps', event.target.checked)}
            aria-label={t('prefs.relativeTimestamps')}
          />
        </SettingRow>
        <SegmentedSetting
          label={t('prefs.byteUnits')}
          description={t('prefs.byteUnitsDesc')}
          value={preferences.byteUnits}
          onChange={(value) => setPreference('byteUnits', value)}
          options={[
            { label: 'KiB', value: 'binary' },
            { label: 'KB', value: 'decimal' },
          ]}
        />
        <SettingRow label={t('prefs.thousandsSeparator')} description={t('prefs.thousandsSeparatorDesc')}>
          <Switch
            checked={preferences.thousandsSeparator}
            onChange={(event) => setPreference('thousandsSeparator', event.target.checked)}
            aria-label={t('prefs.thousandsSeparator')}
          />
        </SettingRow>
      </Group>

      <Group title={t('prefs.behaviour')}>
        <SettingRow label={t('prefs.landingPage')} description={t('prefs.landingPageDesc')}>
          <Combobox
            options={LANDING_OPTIONS}
            value={preferences.landingPage}
            onChange={(next) => next && setPreference('landingPage', next as typeof preferences.landingPage)}
            aria-label={t('prefs.landingPage')}
          />
        </SettingRow>
        <SettingRow label={t('prefs.defaultTimeRange')} description={t('prefs.defaultTimeRangeDesc')}>
          <Combobox
            options={TIME_RANGE_OPTIONS}
            value={preferences.defaultTimeRange}
            onChange={(next) => next && setPreference('defaultTimeRange', next as typeof preferences.defaultTimeRange)}
            aria-label={t('prefs.defaultTimeRange')}
          />
        </SettingRow>
        <SegmentedSetting
          label={t('prefs.rowsPerPage')}
          description={t('prefs.rowsPerPageDesc')}
          value={preferences.tablePageSize}
          onChange={(value) => setPreference('tablePageSize', value)}
          options={[
            { label: '25', value: 25 },
            { label: '50', value: 50 },
            { label: '100', value: 100 },
          ]}
        />
        <SettingRow label={t('prefs.liveUpdates')} description={t('prefs.liveUpdatesDesc')}>
          <Switch
            checked={preferences.liveUpdates}
            onChange={(event) => setPreference('liveUpdates', event.target.checked)}
            aria-label={t('prefs.liveUpdates')}
          />
        </SettingRow>
        <SettingRow
          label={t('prefs.confirmDestructive')}
          description={t('prefs.confirmDestructiveDesc')}
        >
          <Switch
            checked={preferences.confirmDestructive}
            onChange={(event) => setPreference('confirmDestructive', event.target.checked)}
            aria-label={t('prefs.confirmDestructive')}
          />
        </SettingRow>
      </Group>

      <Group title={t('prefs.accessibility')}>
        <SettingRow label={t('prefs.highContrast')} description={t('prefs.highContrastDesc')}>
          <Switch
            checked={preferences.highContrast}
            onChange={(event) => setPreference('highContrast', event.target.checked)}
            aria-label={t('prefs.highContrast')}
          />
        </SettingRow>
        <SettingRow label={t('prefs.underlineLinks')} description={t('prefs.underlineLinksDesc')}>
          <Switch
            checked={preferences.underlineLinks}
            onChange={(event) => setPreference('underlineLinks', event.target.checked)}
            aria-label={t('prefs.underlineLinks')}
          />
        </SettingRow>
        <SettingRow label={t('prefs.alwaysShowFocus')} description={t('prefs.alwaysShowFocusDesc')}>
          <Switch
            checked={preferences.alwaysShowFocus}
            onChange={(event) => setPreference('alwaysShowFocus', event.target.checked)}
            aria-label={t('prefs.alwaysShowFocus')}
          />
        </SettingRow>
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
});
