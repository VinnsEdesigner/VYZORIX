import { css, cx } from '@emotion/css';
import { cloneElement, useId, useState, type ReactElement } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Portal } from '../Portal';

export type TooltipPlacement = 'top' | 'bottom' | 'left' | 'right';

export interface TooltipProps {
  content: React.ReactNode;
  placement?: TooltipPlacement;
  children: ReactElement<{ 'aria-describedby'?: string }>;
  className?: string;
}

function getStyles(theme: VyzorixTheme, placement: TooltipPlacement) {
  const offset = theme.spacing(1);
  const position: Record<string, string | number> = { position: 'fixed', zIndex: theme.zIndex.tooltip };
  switch (placement) {
    case 'top':
      position.left = '50%';
      position.top = `calc(50% - ${offset})`;
      position.transform = 'translate(-50%, -100%)';
      break;
    case 'bottom':
      position.left = '50%';
      position.top = `calc(50% + ${offset})`;
      position.transform = 'translate(-50%, 0)';
      break;
    case 'left':
      position.right = `calc(50% + ${offset})`;
      position.top = '50%';
      position.transform = 'translate(0, -50%)';
      break;
    case 'right':
      position.left = `calc(50% + ${offset})`;
      position.top = '50%';
      position.transform = 'translate(0, -50%)';
      break;
  }

  return {
    wrapper: css({ position: 'relative', display: 'inline-flex' }),
    tip: css({
      ...position,
      maxWidth: 280,
      padding: `${theme.spacing(0.5)} ${theme.spacing(1)}`,
      background: theme.colors.background.elevated,
      color: theme.colors.text.primary,
      border: `1px solid ${theme.colors.border.strong}`,
      borderRadius: theme.shape.radius.default,
      boxShadow: theme.shadows.z2,
      fontSize: theme.typography.sm.fontSize,
      pointerEvents: 'none',
    }),
  };
}

/**
 * A tooltip on hover/focus.
 *
 * The trigger is wired with `aria-describedby` while open, so the content is
 * announced to screen readers rather than being visual-only. Prefer a visible
 * label wherever there is room.
 */
export function Tooltip({ content, placement = 'top', children, className }: TooltipProps) {
  const styles = useStyles2(getStyles, placement);
  const [open, setOpen] = useState(false);
  const id = useId();

  const trigger = cloneElement(children, {
    'aria-describedby': open ? id : children.props['aria-describedby'],
  });

  return (
    <span
      className={cx(styles.wrapper, className)}
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
      onFocus={() => setOpen(true)}
      onBlur={() => setOpen(false)}
    >
      {trigger}
      {open ? (
        <Portal>
          <span role="tooltip" id={id} className={styles.tip} style={{ position: 'absolute' }}>
            {content}
          </span>
        </Portal>
      ) : null}
    </span>
  );
}
