import { css, cx } from '@emotion/css';
import { useEffect, useState } from 'react';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { describeError } from '@/lib/error';
import {
  Alert,
  Button,
  Combobox,
  Divider,
  Field,
  getBuiltInThemes,
  useStyles2,
  type ComboboxOption,
  type VyzorixTheme,
} from '@vyzorix/ui';
import {
  DATE_FORMAT_OPTIONS,
  WEEK_START_OPTIONS,
  getTimezoneOptions,
  type Preferences,
} from '@/hooks/preferences';

export interface PreferencesFormProps {
  preferences: Preferences;
  scope: 'user' | 'org';
  isLoading?: boolean;
  isSaving?: boolean;
  isError?: boolean;
  isSaveError?: boolean;
  onSave: (prefs: Preferences) => Promise<void>;
  onConfirm?: () => Promise<boolean>;
  description?: React.ReactNode;

  savedMessage?: string;
}

const THEME_OPTIONS: ComboboxOption[] = [
  { value: '', label: 'Default', description: 'Inherit the organisation default' },
  ...getBuiltInThemes().map((theme) => ({ value: theme.id, label: theme.name })),
];

const LANGUAGE_OPTIONS: ComboboxOption[] = [
  { value: '', label: 'Default', description: 'Inherit the organisation default' },
  { value: 'en', label: 'English' },
  { value: 'fr', label: 'Français' },
];

const WEEK_START_COMBO_OPTIONS: ComboboxOption[] = [
  { value: '', label: 'Default', description: 'Inherit the organisation default' },
  ...WEEK_START_OPTIONS.map((option) => ({ value: option.value, label: option.label })),
];

const DATE_FORMAT_COMBO_OPTIONS: ComboboxOption[] = [
  { value: '', label: 'Default', description: 'Inherit the organisation default' },
  ...DATE_FORMAT_OPTIONS.map((option) => ({ value: option.value, label: option.label })),
];

const TIMEZONE_OPTIONS: ComboboxOption[] = [
  { value: '', label: 'Default', description: 'Use the browser timezone' },
  ...getTimezoneOptions().map((zone) => ({ value: zone, label: zone })),
];

function getStyles(theme: VyzorixTheme) {
  return {
    form: css({ width: '100%', maxWidth: 600, display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
    actions: css({ marginTop: theme.spacing(1) }),
    description: css({
      margin: 0,
      color: theme.colors.text.secondary,
      fontSize: theme.typography.sm.fontSize,
    }),
  };
}

export function PreferencesForm({
  preferences,
  scope,
  isLoading,
  isSaving,
  isError,
  isSaveError,
  onSave,
  onConfirm,
  description,
  savedMessage,
}: PreferencesFormProps) {
  const styles = useStyles2(getStyles);
  const [draft, setDraft] = useState<Preferences>(preferences);

  useEffect(() => {
    setDraft(preferences);
  }, [preferences]);

  const update = <K extends keyof Preferences>(key: K, value: Preferences[K]) =>
    setDraft((current) => ({ ...current, [key]: value }));

  const disabled = Boolean(isLoading) || Boolean(isSaving);

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (onConfirm && !(await onConfirm())) return;
    try {
      await onSave(draft);
      notifySuccess('Preferences saved', savedMessage);
    } catch (error) {
      notifyError('Could not save preferences', describeError(error));
    }
  };

  return (
    <form className={styles.form} onSubmit={handleSubmit}>
      {description ? <p className={styles.description}>{description}</p> : null}
      {isError ? <Alert title="Error loading preferences" severity="error" /> : null}
      {isSaveError ? <Alert title="Error saving preferences" severity="error" /> : null}

      <Field label="Interface theme" description="Applies to this browser only.">
        <Combobox
          options={THEME_OPTIONS}
          value={draft.theme}
          onChange={(next) => update('theme', next ?? '')}
          disabled={disabled}
          placeholder="Default (inherit)"
        />
      </Field>

      <Field label="Timezone" description="Used for timestamps in logs, events and charts.">
        <Combobox
          options={TIMEZONE_OPTIONS}
          value={draft.timezone}
          onChange={(next) => update('timezone', next ?? '')}
          disabled={disabled}
          searchable
          placeholder="Default (browser)"
        />
      </Field>

      <Field label="Date format" description="How dates are shown across the console.">
        <Combobox
          options={DATE_FORMAT_COMBO_OPTIONS}
          value={draft.dateFormat}
          onChange={(next) => update('dateFormat', next ?? '')}
          disabled={disabled}
          placeholder="Default"
        />
      </Field>

      <Field label="Week start" description="The first day of the week in date pickers.">
        <Combobox
          options={WEEK_START_COMBO_OPTIONS}
          value={draft.weekStart}
          onChange={(next) => update('weekStart', (next ?? '') as Preferences['weekStart'])}
          disabled={disabled}
          placeholder="Default"
        />
      </Field>

      {scope === 'user' ? (
        <>
          <Divider />
          <Field label="Language" description="The interface language.">
            <Combobox
              options={LANGUAGE_OPTIONS}
              value={draft.language}
              onChange={(next) => update('language', next ?? '')}
              disabled={disabled}
              placeholder="Default"
            />
          </Field>
        </>
      ) : null}

      <div className={cx(styles.actions)}>
        <Button type="submit" variant="primary" loading={isSaving} disabled={disabled}>
          Save preferences
        </Button>
      </div>
    </form>
  );
}
