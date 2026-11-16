import { css, cx } from '@emotion/css';
import { forwardRef } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';

export type TextVariant =
  | 'xxl' | 'xl' | 'lg' | 'md' | 'base' | 'sm' | 'code'
  | 'body' | 'bodySmall' | 'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6';

export type TextColor = 'primary' | 'secondary' | 'disabled' | 'link' | 'error' | 'success' | 'warning';
export type TextWeight = 'light' | 'regular' | 'medium' | 'bold';
export type TextAlignment = 'left' | 'center' | 'right' | 'justify';
export type TextElement = 'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6' | 'p' | 'span' | 'div' | 'label';

export interface TextProps extends React.HTMLAttributes<HTMLElement> {
  /** Rendered tag. Defaults to `p`, or a matching `hN` for an `hN` variant. */
  element?: TextElement;
  variant?: TextVariant;
  color?: TextColor;
  weight?: TextWeight;
  italic?: boolean;
  textAlignment?: TextAlignment;
  /** Clip to a single line with an ellipsis. */
  truncate?: boolean;
  /** Tabular (monospaced) numerals, for aligned numeric columns. */
  tabular?: boolean;
}

const WEIGHT: Record<TextWeight, number> = { light: 300, regular: 400, medium: 500, bold: 700 };

const TEXT_COLOR = new Set(['primary', 'secondary', 'disabled', 'link']);

function getStyles(theme: VyzorixTheme, variant: TextVariant, color: TextColor, weight: TextWeight) {
  // `primary`/`secondary`/`disabled`/`link` live under colors.text; the semantic
  // names (error, success, warning) are rich colours, so take their `text` token.
  const resolved = TEXT_COLOR.has(color)
    ? theme.colors.text[color as 'primary' | 'secondary' | 'disabled' | 'link']
    : theme.colors[color as 'error' | 'success' | 'warning'].text;

  return {
    root: css({
      margin: 0,
      fontSize: theme.typography[variant].fontSize,
      fontWeight: WEIGHT[weight],
      lineHeight: theme.typography[variant].lineHeight,
      fontFamily: theme.typography[variant].fontFamily,
      color: resolved,
    }),
    italic: css({ fontStyle: 'italic' }),
    truncate: css({ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }),
    tabular: css({ fontVariantNumeric: 'tabular-nums' }),
  };
}

function defaultElement(variant: TextVariant): TextElement {
  switch (variant) {
    case 'h1': case 'xxl': return 'h1';
    case 'h2': case 'xl': return 'h2';
    case 'h3': case 'lg': return 'h3';
    case 'h4': case 'md': return 'h4';
    case 'h5': return 'h5';
    case 'h6': return 'h6';
    default: return 'p';
  }
}

const ALIGN: Record<TextAlignment, 'left' | 'center' | 'right' | 'justify'> = {
  left: 'left', center: 'center', right: 'right', justify: 'justify',
};

/**
 * The typography primitive.
 *
 * Every piece of text should come through here so it inherits the active
 * theme's type scale and colour. Size comes from `variant` (the theme's own
 * scale), never a hard-coded pixel value.
 */
export const Text = forwardRef<HTMLElement, TextProps>(
  (
    {
      element, variant = 'base', color = 'primary', weight = 'regular',
      italic, textAlignment, truncate, tabular, className, style, children, ...rest
    },
    ref,
  ) => {
    const styles = useStyles2(getStyles, variant, color, weight);
    const Tag = (element ?? defaultElement(variant)) as 'p';

    return (
      <Tag
        ref={ref as React.Ref<HTMLParagraphElement>}
        className={cx(styles.root, italic && styles.italic, truncate && styles.truncate, tabular && styles.tabular, className)}
        style={{ textAlign: textAlignment ? ALIGN[textAlignment] : undefined, ...style }}
        {...rest}
      >
        {children}
      </Tag>
    );
  },
);

Text.displayName = 'Text';
