import { css, cx } from '@emotion/css';
import { forwardRef, type HTMLAttributes } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Icon, type IconName } from '../Icon';

export interface TagProps extends Omit<HTMLAttributes<HTMLElement>, 'onClick'> {
  name: string;
  icon?: IconName;
  colorIndex?: number;
  onClick?: (name: string, event: React.MouseEvent<HTMLElement>) => void;
}

/** A rotating palette for tags that are not tied to a status colour. */
const TAG_COLORS = ['primary', 'success', 'warning', 'error', 'info'] as const;

function getStyles(theme: VyzorixTheme, colorIndex: number) {
  const rich = theme.colors[TAG_COLORS[colorIndex % TAG_COLORS.length] ?? 'primary'];
  return {
    tag: css({
      display: 'inline-flex',
      alignItems: 'center',
      gap: theme.spacing(0.5),
      padding: `2px ${theme.spacing(1)}`,
      borderRadius: theme.shape.radius.default,
      background: theme.colors.background.secondary,
      color: theme.colors.text.secondary,
      border: `1px solid ${theme.colors.border.weak}`,
      fontSize: theme.typography.sm.fontSize,
      fontFamily: 'inherit',
      lineHeight: 1.4,
    }),
    clickable: css({
      cursor: 'pointer',
      '&:hover': { background: rich.subtleBackground, color: rich.text, borderColor: rich.subtleBorder },
    }),
  };
}

/** A label chip. Renders a `<button>` when given an `onClick`. */
export const Tag = forwardRef<HTMLElement, TagProps>(
  ({ name, icon, colorIndex = 0, onClick, className, ...rest }, ref) => {
    const styles = useStyles2(getStyles, colorIndex);

    const content = (
      <>
        {icon ? <Icon name={icon} size="xs" /> : null}
        {name}
      </>
    );

    if (onClick) {
      return (
        <button
          ref={ref as React.Ref<HTMLButtonElement>}
          type="button"
          className={cx(styles.tag, styles.clickable, className)}
          onClick={(event) => {
            event.preventDefault();
            event.stopPropagation();
            onClick(name, event);
          }}
          {...rest}
        >
          {content}
        </button>
      );
    }

    return (
      <span ref={ref as React.Ref<HTMLSpanElement>} className={cx(styles.tag, className)} {...rest}>
        {content}
      </span>
    );
  },
);

Tag.displayName = 'Tag';
