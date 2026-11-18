import { css } from '@emotion/css';
import { getFocusStyles, useStyles2, type VyzorixTheme } from '@vyzorix/ui';

export interface SegmentedOption<T extends string | number> {
  label: string;
  value: T;
}

export interface SegmentedSettingProps<T extends string | number> {
  label: string;
  description?: string;
  options: SegmentedOption<T>[];
  value: T;
  onChange: (value: T) => void;
  disabled?: boolean;
}

export function SegmentedSetting<T extends string | number>({
  label,
  description,
  options,
  value,
  onChange,
  disabled,
}: SegmentedSettingProps<T>) {
  const styles = useStyles2(getStyles);
  return (
    <div className={styles.row}>
      <div className={styles.text}>
        <span className={styles.label}>{label}</span>
        {description ? <span className={styles.description}>{description}</span> : null}
      </div>
      <div className={styles.group} role="group" aria-label={label}>
        {options.map((option) => (
          <button
            key={String(option.value)}
            type="button"
            className={`${styles.option} ${option.value === value ? styles.optionActive : ''}`}
            aria-pressed={option.value === value}
            disabled={disabled}
            onClick={() => onChange(option.value)}
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  row: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
  }),
  text: css({ display: 'flex', flexDirection: 'column', gap: 2, minWidth: 200 }),
  label: css({ color: theme.colors.text.primary, fontSize: theme.typography.base.fontSize }),
  description: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  group: css({
    display: 'inline-flex',
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
    padding: 2,
    gap: 2,
  }),
  option: css({
    border: 0,
    borderRadius: theme.shape.radius.sm,
    padding: `${theme.spacing(0.5)} ${theme.spacing(1.5)}`,
    background: 'transparent',
    color: theme.colors.text.secondary,
    fontFamily: 'inherit',
    fontSize: theme.typography.sm.fontSize,
    cursor: 'pointer',
    '&:hover:not(:disabled)': { color: theme.colors.text.primary },
    '&:disabled': { opacity: theme.colors.action.disabledOpacity, cursor: 'not-allowed' },
    '&:focus-visible': getFocusStyles(theme),
  }),
  optionActive: css({
    background: theme.colors.accent.main,
    color: theme.colors.accent.contrastText,
    '&:hover:not(:disabled)': { color: theme.colors.accent.contrastText },
  }),
});
