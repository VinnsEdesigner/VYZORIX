import { css, cx } from '@emotion/css';
import { forwardRef, type ButtonHTMLAttributes } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Icon, type IconName, type IconSize } from '../Icon';

export type ButtonVariant = 'primary' | 'secondary' | 'accent' | 'destructive' | 'success';
export type ButtonFill = 'solid' | 'outline' | 'text';
export type ButtonSize = 'sm' | 'md' | 'lg';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  fill?: ButtonFill;
  size?: ButtonSize;
  icon?: IconName;
  iconPlacement?: 'left' | 'right';
  fullWidth?: boolean;
}

const PADDING: Record<ButtonSize, string> = {
  sm: '4px 8px',
  md: '8px 16px',
  lg: '12px 24px',
};
const ICON_SIZE: Record<ButtonSize, IconSize> = { sm: 'sm', md: 'md', lg: 'lg' };
const FONT: Record<ButtonSize, 'sm' | 'base' | 'lg'> = { sm: 'sm', md: 'base', lg: 'lg' };

/**
 * Button variants map onto the theme's semantic colour roles. `destructive` and
 * `success` are UI names for the `error` and `success` roles; the theme itself
 * has no colour called "destructive".
 */
const VARIANT_COLOR: Record<ButtonVariant, keyof Pick<VyzorixTheme['colors'], 'primary' | 'secondary' | 'accent' | 'error' | 'success'>> = {
  primary: 'primary',
  secondary: 'secondary',
  accent: 'accent',
  destructive: 'error',
  success: 'success',
};

function getStyles(theme: VyzorixTheme, variant: ButtonVariant, fill: ButtonFill, size: ButtonSize, fullWidth?: boolean) {
  const color = theme.colors[VARIANT_COLOR[variant]];

  const solid = css({
    background: color.main,
    color: color.contrastText,
    border: '1px solid transparent',
    '&:hover:not(:disabled)': { background: color.mainEmphasis },
  });

  const outline = css({
    background: 'transparent',
    color: color.text,
    border: `1px solid ${color.border}`,
    '&:hover:not(:disabled)': { background: color.subtleBackground },
  });

  const text = css({
    background: 'transparent',
    color: color.text,
    border: '1px solid transparent',
    '&:hover:not(:disabled)': { background: color.subtleBackground },
  });

  return {
    base: css({
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      gap: theme.spacing(1),
      padding: PADDING[size],
      fontFamily: theme.typography.fontFamily,
      fontSize: theme.typography[FONT[size]].fontSize,
      fontWeight: theme.typography.fontWeightMedium,
      lineHeight: 1,
      borderRadius: theme.shape.radius.default,
      cursor: 'pointer',
      whiteSpace: 'nowrap',
      width: fullWidth ? '100%' : undefined,
      transition: theme.transitions.create(['background', 'border-color', 'color'], {
        duration: theme.transitions.duration.shortest,
      }),
      '&:disabled': {
        opacity: theme.colors.action.disabledOpacity,
        cursor: 'not-allowed',
      },
      '&:focus-visible': {
        outline: `2px solid ${theme.colors.primary.main}`,
        outlineOffset: 2,
      },
    }),
    fill: fill === 'solid' ? solid : fill === 'outline' ? outline : text,
  };
}

/**
 * Themed button.
 *
 * `variant` picks the colour role, `fill` picks solid/outline/text. An
 * icon-only button (no children) requires an `aria-label`.
 */
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      variant = 'primary',
      fill = 'solid',
      size = 'md',
      icon,
      iconPlacement = 'left',
      fullWidth,
      className,
      children,
      type = 'button',
      ...rest
    },
    ref,
  ) => {
    const styles = useStyles2(getStyles, variant, fill, size, fullWidth);

    if (process.env.NODE_ENV !== 'production' && !children && !rest['aria-label']) {
      console.warn('Button: an icon-only button needs an aria-label');
    }

    const glyph = icon ? <Icon name={icon} size={ICON_SIZE[size]} /> : null;

    return (
      <button ref={ref} type={type} className={cx(styles.base, styles.fill, className)} {...rest}>
        {iconPlacement === 'left' && glyph}
        {children}
        {iconPlacement === 'right' && glyph}
      </button>
    );
  },
);

Button.displayName = 'Button';
