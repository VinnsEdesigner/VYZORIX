import { css, cx } from '@emotion/css';
import { useEffect } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { IconButton } from '../IconButton';
import { Portal } from '../Portal';
import { Text } from '../Text';

export interface DrawerProps {
  title: string;
  isOpen: boolean;
  onClose: () => void;
  children: React.ReactNode;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}

const WIDTH = { sm: 320, md: 480, lg: 640 };

function getStyles(theme: VyzorixTheme, size: 'sm' | 'md' | 'lg') {
  return {
    backdrop: css({
      position: 'fixed',
      inset: 0,
      zIndex: theme.zIndex.modalBackdrop,
      background: 'rgba(0, 0, 0, 0.6)',
    }),
    drawer: css({
      position: 'fixed',
      top: 0,
      right: 0,
      bottom: 0,
      zIndex: theme.zIndex.modal,
      width: '100%',
      maxWidth: WIDTH[size],
      display: 'flex',
      flexDirection: 'column',
      background: theme.colors.background.primary,
      borderLeft: `1px solid ${theme.colors.border.medium}`,
      boxShadow: theme.shadows.z3,
    }),
    header: css({
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: theme.spacing(2),
      padding: theme.spacing(2),
      borderBottom: `1px solid ${theme.colors.border.weak}`,
    }),
    body: css({ padding: theme.spacing(2), overflowY: 'auto', flex: 1 }),
  };
}

/** A side panel that slides in from the right. Closes on Escape. */
export function Drawer({ title, isOpen, onClose, children, size = 'md', className }: DrawerProps) {
  const styles = useStyles2(getStyles, size);

  useEffect(() => {
    if (!isOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  return (
    <Portal>
      <div className={styles.backdrop} onClick={onClose} />
      <div role="dialog" aria-modal="true" aria-label={title} className={cx(styles.drawer, className)}>
        <div className={styles.header}>
          <Text variant="lg" weight="medium" element="div">
            {title}
          </Text>
          <IconButton name="x" aria-label="Close" fill="text" size="sm" onClick={onClose} />
        </div>
        <div className={styles.body}>{children}</div>
      </div>
    </Portal>
  );
}
