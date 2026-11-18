import { css, cx } from '@emotion/css';
import { useEffect } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { IconButton } from '../IconButton';
import { Portal } from '../Portal';
import { Text } from '../Text';

export interface ModalProps {
  title: string;
  isOpen: boolean;
  onDismiss: () => void;
  children: React.ReactNode;
  className?: string;

  width?: number;
}

function getStyles(theme: VyzorixTheme, width: number) {
  return {
    backdrop: css({
      position: 'fixed',
      inset: 0,
      zIndex: theme.zIndex.modalBackdrop,
      background: 'rgba(0, 0, 0, 0.6)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: theme.spacing(2),
    }),
    modal: css({
      zIndex: theme.zIndex.modal,
      width: '100%',
      maxWidth: width,
      maxHeight: '90vh',
      overflow: 'auto',
      background: theme.colors.background.primary,
      border: `1px solid ${theme.colors.border.medium}`,
      borderRadius: theme.shape.radius.lg,
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
    body: css({ padding: theme.spacing(2) }),
  };
}

export function Modal({ title, isOpen, onDismiss, children, className, width = 480 }: ModalProps) {
  const styles = useStyles2(getStyles, width);

  useEffect(() => {
    if (!isOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onDismiss();
    };
    document.addEventListener('keydown', onKey);
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', onKey);
      document.body.style.overflow = previousOverflow;
    };
  }, [isOpen, onDismiss]);

  if (!isOpen) return null;

  return (
    <Portal>
      <div className={styles.backdrop} onMouseDown={(e) => e.target === e.currentTarget && onDismiss()}>
        <div
          role="dialog"
          aria-modal="true"
          aria-label={title}
          className={cx(styles.modal, className)}
          onMouseDown={(e) => e.stopPropagation()}
        >
          <div className={styles.header}>
            <Text variant="lg" weight="medium" element="div">
              {title}
            </Text>
            <IconButton name="x" aria-label="Close" fill="text" size="sm" onClick={onDismiss} />
          </div>
          <div className={styles.body}>{children}</div>
        </div>
      </div>
    </Portal>
  );
}
