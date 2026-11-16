import { css, cx } from '@emotion/css';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Icon, type IconName } from '../Icon';

export interface RadioButtonOption<T extends string> {
  label: string;
  value: T;
  icon?: IconName;
  disabled?: boolean;
}

export interface RadioButtonGroupProps<T extends string> {
  options: Array<RadioButtonOption<T>>;
  value: T;
  onChange: (value: T) => void;
  className?: string;
  'aria-label'?: string;
}

function getStyles(theme: VyzorixTheme) {
  return {
    group: css({
      display: 'inline-flex',
      border: `1px solid ${theme.colors.border.medium}`,
      borderRadius: theme.shape.radius.default,
      overflow: 'hidden',
    }),
    option: css({
      display: 'inline-flex',
      alignItems: 'center',
      gap: theme.spacing(0.5),
      padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`,
      background: 'transparent',
      border: 'none',
      borderRight: `1px solid ${theme.colors.border.medium}`,
      color: theme.colors.text.secondary,
      fontFamily: 'inherit',
      fontSize: theme.typography.sm.fontSize,
      cursor: 'pointer',
      '&:last-child': { borderRight: 'none' },
      '&:hover:not(:disabled)': { background: theme.colors.action.hover },
      '&:disabled': { opacity: theme.colors.action.disabledOpacity, cursor: 'not-allowed' },
    }),
    selected: css({
      background: theme.colors.primary.main,
      color: theme.colors.primary.contrastText,
      '&:hover:not(:disabled)': { background: theme.colors.primary.mainEmphasis },
    }),
  };
}

/** A segmented control: one choice from a small set, shown inline. */
export function RadioButtonGroup<T extends string>({
  options,
  value,
  onChange,
  className,
  'aria-label': ariaLabel,
}: RadioButtonGroupProps<T>) {
  const styles = useStyles2(getStyles);

  return (
    <div role="radiogroup" aria-label={ariaLabel} className={cx(styles.group, className)}>
      {options.map((option) => {
        const selected = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            role="radio"
            aria-checked={selected}
            disabled={option.disabled}
            className={cx(styles.option, selected && styles.selected)}
            onClick={() => onChange(option.value)}
          >
            {option.icon ? <Icon name={option.icon} size="sm" /> : null}
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
