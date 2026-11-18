import { css, cx } from '@emotion/css';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Icon, type IconName } from '../Icon';

export interface TabItem<T extends string> {
  label: string;
  value: T;
  icon?: IconName;
  disabled?: boolean;
}

export interface TabsProps<T extends string> {
  tabs: Array<TabItem<T>>;
  value: T;
  onChange: (value: T) => void;
  className?: string;
  'aria-label'?: string;
}

function getStyles(theme: VyzorixTheme) {
  return {
    list: css({
      display: 'flex',
      gap: theme.spacing(0.5),
      borderBottom: `1px solid ${theme.colors.border.weak}`,
    }),
    tab: css({
      display: 'inline-flex',
      alignItems: 'center',
      gap: theme.spacing(0.5),
      padding: `${theme.spacing(1)} ${theme.spacing(2)}`,
      background: 'transparent',
      border: 'none',
      borderBottom: '2px solid transparent',
      marginBottom: -1,
      color: theme.colors.text.secondary,
      fontFamily: 'inherit',
      fontSize: theme.typography.base.fontSize,
      cursor: 'pointer',
      '&:hover:not(:disabled)': { color: theme.colors.text.primary },
      '&:disabled': { color: theme.colors.action.disabledText, cursor: 'not-allowed' },
    }),
    active: css({
      color: theme.colors.text.primary,
      borderBottomColor: theme.colors.primary.main,
    }),
  };
}

export function Tabs<T extends string>({ tabs, value, onChange, className, 'aria-label': ariaLabel }: TabsProps<T>) {
  const styles = useStyles2(getStyles);

  return (
    <div role="tablist" aria-label={ariaLabel} className={cx(styles.list, className)}>
      {tabs.map((tab) => {
        const selected = tab.value === value;
        return (
          <button
            key={tab.value}
            type="button"
            role="tab"
            aria-selected={selected}
            disabled={tab.disabled}
            className={cx(styles.tab, selected && styles.active)}
            onClick={() => onChange(tab.value)}
          >
            {tab.icon ? <Icon name={tab.icon} size="sm" /> : null}
            {tab.label}
          </button>
        );
      })}
    </div>
  );
}
