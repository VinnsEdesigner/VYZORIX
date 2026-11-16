import { css, cx } from '@emotion/css';
import { useState } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Icon } from '../Icon';
import { Text } from '../Text';

export interface CollapseProps {
  label: React.ReactNode;
  children: React.ReactNode;
  isOpen?: boolean;
  onToggle?: (isOpen: boolean) => void;
  className?: string;
}

const getStyles = (theme: VyzorixTheme) => ({
  wrapper: css({ border: `1px solid ${theme.colors.border.weak}`, borderRadius: theme.shape.radius.default, overflow: 'hidden' }),
  trigger: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    width: '100%',
    padding: theme.spacing(1.5),
    background: theme.colors.background.secondary,
    border: 'none',
    color: theme.colors.text.primary,
    fontFamily: 'inherit',
    textAlign: 'left',
    cursor: 'pointer',
    '&:hover': { background: theme.colors.action.hover },
  }),
  body: css({ padding: theme.spacing(1.5), borderTop: `1px solid ${theme.colors.border.weak}` }),
});

/** A collapsible section. Uncontrolled by default; pass `isOpen` to control it. */
export function Collapse({ label, children, isOpen, onToggle, className }: CollapseProps) {
  const styles = useStyles2(getStyles);
  const [internalOpen, setInternalOpen] = useState(false);
  const open = isOpen ?? internalOpen;

  const toggle = () => {
    const next = !open;
    if (isOpen === undefined) setInternalOpen(next);
    onToggle?.(next);
  };

  return (
    <div className={cx(styles.wrapper, className)}>
      <button type="button" className={styles.trigger} aria-expanded={open} onClick={toggle}>
        <Icon name={open ? 'chevron-down' : 'chevron-right'} size="sm" />
        <Text variant="base" weight="medium">
          {label}
        </Text>
      </button>
      {open ? <div className={styles.body}>{children}</div> : null}
    </div>
  );
}
