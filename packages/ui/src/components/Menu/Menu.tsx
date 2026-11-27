import { css, cx } from '@emotion/css';
import { useEffect, useRef, useState } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Icon, type IconName } from '../Icon';
import { OverlaySheet } from '../Overlay';
import { useOverlay } from '../Overlay/OverlayProvider';

export interface MenuItem {
  label: string;
  onClick: () => void;
  icon?: IconName;
  disabled?: boolean;

  destructive?: boolean;
}

export interface MenuProps {
  items: MenuItem[];

  children: React.ReactElement<{ onClick?: () => void; 'aria-haspopup'?: string; 'aria-expanded'?: boolean }>;
  className?: string;
}

function getStyles(theme: VyzorixTheme) {
  return {
    wrapper: css({ position: 'relative', display: 'inline-flex' }),
    menu: css({
      position: 'absolute',
      top: '100%',
      right: 0,
      marginTop: theme.spacing(0.5),
      minWidth: 180,
      padding: theme.spacing(0.5),
      background: theme.colors.background.elevated,
      border: `1px solid ${theme.colors.border.medium}`,
      borderRadius: theme.shape.radius.default,
      boxShadow: theme.shadows.z2,
      zIndex: theme.zIndex.dropdown,
    }),
    item: css({
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(1),
      width: '100%',
      padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`,
      background: 'transparent',
      border: 'none',
      borderRadius: theme.shape.radius.sm,
      color: theme.colors.text.primary,
      fontFamily: 'inherit',
      fontSize: theme.typography.base.fontSize,
      textAlign: 'left',
      cursor: 'pointer',
      '&:hover:not(:disabled)': { background: theme.colors.action.hover },
      '&:disabled': { color: theme.colors.action.disabledText, cursor: 'not-allowed' },
    }),
    destructive: css({ color: theme.colors.error.text }),

    sheetMenu: css({ display: 'flex', flexDirection: 'column', gap: 2 }),
    sheetItem: css({ minHeight: 44, padding: `${theme.spacing(1)} ${theme.spacing(1.5)}` }),
  };
}

export function Menu({ items, children, className }: MenuProps) {
  const styles = useStyles2(getStyles);
  const { presentation } = useOverlay();
  const isSheet = presentation === 'sheet';
  const [open, setOpen] = useState(false);
  const wrapperRef = useRef<HTMLDivElement>(null);
  const sheetContentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      const target = e.target as Node;
      if (wrapperRef.current?.contains(target)) return;

      window.setTimeout(() => {
        if (sheetContentRef.current?.contains(target)) return;
        setOpen(false);
      }, 0);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const trigger = {
    ...children,
    props: {
      ...children.props,
      onClick: () => setOpen((v) => !v),
      'aria-haspopup': 'menu',
      'aria-expanded': open,
    },
  };

  return (
    <div ref={wrapperRef} className={cx(styles.wrapper, className)}>
      {trigger}
      {open
        ? (() => {
            const list = (
              <div role="menu" className={isSheet ? styles.sheetMenu : styles.menu}>
                {items.map((item) => (
                  <button
                    key={item.label}
                    type="button"
                    role="menuitem"
                    disabled={item.disabled}
                    className={cx(styles.item, isSheet && styles.sheetItem, item.destructive && styles.destructive)}
                    onClick={() => {
                      setOpen(false);
                      item.onClick();
                    }}
                  >
                    {item.icon ? <Icon name={item.icon} size={isSheet ? 'lg' : 'sm'} /> : null}
                    {item.label}
                  </button>
                ))}
              </div>
            );

            return isSheet ? (
              <OverlaySheet
                open
                onClose={() => setOpen(false)}
                label="Actions"
                rootRef={sheetContentRef}
              >
                {list}
              </OverlaySheet>
            ) : (
              list
            );
          })()
        : null}
    </div>
  );
}
