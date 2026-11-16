import { css, cx } from '@emotion/css';
import { forwardRef, memo } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { iconRegistry, isIconName, type IconName } from './iconNames';

/** Icon sizes, on a shared scale with the rest of the design system. */
export type IconSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl' | 'xxl' | 'xxxl';

const SIZE_PX: Record<IconSize, number> = {
  xs: 12,
  sm: 14,
  md: 16,
  lg: 18,
  xl: 24,
  xxl: 36,
  xxxl: 48,
};

export interface IconProps extends Omit<React.SVGProps<SVGSVGElement>, 'ref' | 'name'> {
  /** A semantic icon name from the registry. */
  name: IconName;
  size?: IconSize;
  /** Accessible label. When omitted the icon is hidden from screen readers. */
  title?: string;
}

const getStyles = (theme: VyzorixTheme) => ({
  icon: css({
    display: 'inline-block',
    flexShrink: 0,
    verticalAlign: 'middle',
    label: 'Icon',
    // line-height: 0 keeps the icon optically centred against text in Safari.
    lineHeight: 0,
    color: 'inherit',
  }),
  spin: css({
    [theme.transitions.handleMotion('no-preference', 'reduce')]: {
      animation: 'vyzorix-spin 2s infinite linear',
    },
  }),
});

/**
 * A themed icon.
 *
 * Renders a lucide icon (ISC-licensed) by semantic name. Colour follows
 * `currentColor`, so it inherits from the surrounding text — set a colour on the
 * parent rather than passing one here.
 *
 * https://lucide.dev
 */
export const Icon = memo(
  forwardRef<SVGSVGElement, IconProps>(({ name, size = 'md', title, className, ...rest }, ref) => {
    const styles = useStyles2(getStyles);

    if (!isIconName(name)) {
      console.warn(`Icon received an unknown name: ${String(name)}`);
      return null;
    }

    const Glyph = iconRegistry[name];
    const px = SIZE_PX[size];

    // An icon is decorative unless it is given a label; hide it from assistive
    // tech when it is, so a screen reader does not announce a stray graphic.
    const labelled = Boolean(title) || rest['aria-label'] != null || rest['aria-labelledby'] != null;

    return (
      <Glyph
        ref={ref}
        data-testid={`icon-${name}`}
        width={px}
        height={px}
        aria-hidden={labelled ? undefined : true}
        className={cx(styles.icon, name === 'spinner' && styles.spin, className)}
        {...rest}
      >
        {title ? <title>{title}</title> : null}
      </Glyph>
    );
  }),
);

Icon.displayName = 'Icon';
