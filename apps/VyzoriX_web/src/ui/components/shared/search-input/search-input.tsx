import { css } from '@emotion/css';
import { Icon, useStyles2, type VyzorixTheme } from '@vyzorix/ui';

export interface SearchInputProps {
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
  'aria-label'?: string;
}

export function SearchInput({ value, onChange, placeholder, 'aria-label': ariaLabel }: SearchInputProps) {
  const styles = useStyles2(getStyles);
  return (
    <div className={styles.wrapper}>
      <Icon name="search" size="sm" className={styles.icon} />
      <input
        type="search"
        className={styles.input}
        value={value}
        placeholder={placeholder}
        aria-label={ariaLabel ?? placeholder}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  wrapper: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: `0 ${theme.spacing(1.5)}`,
    height: 32,
    border: `1px solid ${theme.colors.border.medium}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    color: theme.colors.text.secondary,
    '&:focus-within': { borderColor: theme.colors.primary.border },
  }),
  icon: css({ flexShrink: 0 }),
  input: css({
    flex: 1,
    minWidth: 0,
    border: 0,
    outline: 0,
    background: 'transparent',
    color: theme.colors.text.primary,
    fontFamily: theme.typography.fontFamily,
    fontSize: theme.typography.sm.fontSize,
    '&::placeholder': { color: theme.colors.text.disabled },
  }),
});
