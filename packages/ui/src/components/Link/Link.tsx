import { css, cx } from '@emotion/css';
import { forwardRef } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Icon, type IconName } from '../Icon';

export interface LinkProps extends React.AnchorHTMLAttributes<HTMLAnchorElement> {
  variant?: 'default' | 'muted';
  /** Append an external-link glyph and set the safe rel attributes. */
  external?: boolean;
  icon?: IconName;
  inline?: boolean;
}

function getStyles(theme: VyzorixTheme, variant: 'default' | 'muted', inline: boolean) {
  return {
    link: css({
      display: inline ? 'inline-flex' : 'inline-flex',
      alignItems: 'center',
      gap: theme.spacing(0.5),
      color: variant === 'muted' ? theme.colors.text.secondary : theme.colors.text.link,
      textDecoration: 'none',
      cursor: 'pointer',
      '&:hover': { textDecoration: 'underline' },
      '&:focus-visible': { outline: `2px solid ${theme.colors.primary.main}`, outlineOffset: 2, borderRadius: 2 },
    }),
  };
}

/**
 * A link. `external` sets `target`/`rel` and adds an affordance, so external
 * navigation is both obvious and safe.
 */
export const Link = forwardRef<HTMLAnchorElement, LinkProps>(
  ({ variant = 'default', external, icon, inline, className, children, ...rest }, ref) => {
    const styles = useStyles2(getStyles, variant, Boolean(inline));
    const glyph: IconName | undefined = icon ?? (external ? 'external-link' : undefined);

    return (
      <a
        ref={ref}
        className={cx(styles.link, className)}
        {...(external ? { target: '_blank', rel: 'noopener noreferrer' } : {})}
        {...rest}
      >
        {children}
        {glyph ? <Icon name={glyph} size="sm" /> : null}
      </a>
    );
  },
);

Link.displayName = 'Link';
