import { css } from '@emotion/css';
import { useStyles2, getFocusStyles, type VyzorixTheme } from '@vyzorix/ui';

export interface SegmentedFilterOption<T extends string> {
  label: string;
  value: T;
}

export interface SegmentedFilterProps<T extends string> {
  options: Array<SegmentedFilterOption<T>>;
  value: T;
  onChange: (value: T) => void;
  'aria-label': string;
}

export function SegmentedFilter<T extends string>({
  options,
  value,
  onChange,
  'aria-label': ariaLabel,
}: SegmentedFilterProps<T>) {
  const styles = useStyles2(getStyles);
  return (
    <div className={styles.group} role="group" aria-label={ariaLabel}>
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            aria-pressed={active}
            className={active ? styles.buttonActive : styles.button}
            onClick={() => onChange(option.value)}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  group: css({
    display: 'inline-flex',
    padding: 2,
    gap: 2,
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
  }),
  button: css({
    border: 0,
    background: 'transparent',
    color: theme.colors.text.secondary,
    fontFamily: theme.typography.fontFamily,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    padding: `2px ${theme.spacing(1.5)}`,
    borderRadius: theme.shape.radius.sm,
    cursor: 'pointer',
    whiteSpace: 'nowrap',
    '&:hover': { color: theme.colors.text.primary },
    '&:focus-visible': getFocusStyles(theme),
  }),
  buttonActive: css({
    border: 0,
    background: theme.colors.action.selected,
    color: theme.colors.text.primary,
    fontFamily: theme.typography.fontFamily,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    padding: `2px ${theme.spacing(1.5)}`,
    borderRadius: theme.shape.radius.sm,
    cursor: 'pointer',
    whiteSpace: 'nowrap',
    '&:focus-visible': getFocusStyles(theme),
  }),
});
