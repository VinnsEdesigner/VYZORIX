import { css, keyframes } from '@emotion/css';
import { useEffect, useRef, type ReactNode } from 'react';
import { useTheme2, useStyles2 } from '../../theme/ThemeContext';
import { getFocusStyles } from '../../theme/mixins';
import { type VyzorixTheme } from '../../theme/types';
import { Portal } from '../Portal';

export const SHEET_HEIGHT = 'min(70dvh, 560px)';
export const SHEET_MIN_HEIGHT = '320px';

const sheetUp = keyframes`
  from { transform: translateY(101%); }
  to   { transform: translateY(0); }
`;

const scrimIn = keyframes`
  from { opacity: 0; }
  to   { opacity: 1; }
`;

export interface OverlaySheetProps {
  open: boolean;
  onClose: () => void;
  label: string;
  children: ReactNode;

  header?: ReactNode;
  testId?: string;

  rootRef?: React.Ref<HTMLDivElement>;
}

export function OverlaySheet({
  open,
  onClose,
  label,
  children,
  header,
  testId,
  rootRef,
}: OverlaySheetProps) {
  const styles = useStyles2(getStyles);
  const theme = useTheme2();
  const contentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.stopPropagation();
        onClose();
      }
    };
    document.addEventListener('keydown', onKey);
    const previous = document.body.style.overflow;
    document.body.style.overflow = 'hidden';

    return () => {
      document.removeEventListener('keydown', onKey);
      document.body.style.overflow = previous;
    };
  }, [open, onClose]);

  useEffect(() => {
    if (!open) return;
    const first = contentRef.current?.querySelector<HTMLElement>(
      '[role="option"], button:not([disabled]), a[href], input',
    );
    first?.focus();
  }, [open]);

  if (!open) return null;

  return (
    <Portal>
      <div className={styles.scrim} onClick={onClose} aria-hidden="true" />
      <div
        ref={rootRef}
        role="dialog"
        aria-modal="true"
        aria-label={label}
        data-testid={testId}
        className={styles.sheet}
        style={{ maxWidth: theme.breakpoints.values.sm, marginInline: 'auto' }}
      >
        <div className={styles.grab} aria-hidden="true" />
        {header ? <div className={styles.header}>{header}</div> : null}
        <div ref={contentRef} className={styles.content}>
          {children}
        </div>
      </div>
    </Portal>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  scrim: css({
    position: 'fixed',
    inset: 0,
    zIndex: theme.zIndex.modalBackdrop,
    background: 'rgba(0, 0, 0, 0.5)',
    [theme.transitions.handleMotion('no-preference')]: {
      animation: `${scrimIn} ${theme.transitions.duration.enteringScreen}ms ${theme.transitions.easing.easeOut}`,
    },
  }),
  sheet: css({
    position: 'fixed',
    left: 0,
    right: 0,
    bottom: 0,
    zIndex: theme.zIndex.modal,
    display: 'flex',
    flexDirection: 'column',

    height: SHEET_HEIGHT,

    maxHeight: '85dvh',
    minHeight: SHEET_MIN_HEIGHT,
    paddingBottom: 'env(safe-area-inset-bottom, 0px)',
    overflow: 'hidden',
    background: theme.colors.background.primary,
    borderTop: `1px solid ${theme.colors.border.medium}`,
    borderRadius: '16px 16px 0 0',
    boxShadow: theme.shadows.z3,
    [theme.transitions.handleMotion('no-preference')]: {
      animation: `${sheetUp} ${theme.transitions.duration.short}ms ${theme.transitions.easing.easeOut}`,
    },
  }),
  grab: css({
    width: 36,
    height: 4,
    flexShrink: 0,
    margin: `${theme.spacing(1)} auto ${theme.spacing(0.5)}`,
    borderRadius: 2,
    background: theme.colors.border.medium,
  }),
  header: css({
    flexShrink: 0,
    padding: theme.spacing(0.5, 1.5, 1),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
    borderBottom: `1px solid ${theme.colors.border.weak}`,
  }),
  content: css({
    flexGrow: 1,
    minHeight: 0,
    overflowY: 'auto',
    overscrollBehavior: 'contain',
    padding: theme.spacing(0.5, 1, 1),
    '& [role="option"], & [role="menuitem"]': {
      borderRadius: theme.shape.radius.default,
      '&:focus-visible': getFocusStyles(theme, true),
    },
  }),
});
