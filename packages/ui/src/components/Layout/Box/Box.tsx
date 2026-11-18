import { css, cx } from '@emotion/css';
import { type ElementType, forwardRef, type PropsWithChildren } from 'react';
import * as React from 'react';
import { type VyzorixTheme } from '../../../theme/types';
import { type ThemeSpacingTokens } from '../../../theme/createSpacing';
import { type Radii } from '../../../theme/createShape';
import { type ThemeShadows } from '../../../theme/createShadows';
import { useStyles2 } from '../../../theme/ThemeContext';
import { type AlignItems, type Direction, type FlexProps, type JustifyContent } from '../types';
import { type ResponsiveProp, getResponsiveStyle } from '../utils/responsiveness';
import { getSizeStyles, type SizeProps } from '../utils/styles';

type Display = 'flex' | 'block' | 'inline' | 'inline-block' | 'none';
export type BackgroundColor = keyof VyzorixTheme['colors']['background'] | 'error' | 'success' | 'warning' | 'info';
export type BorderStyle = 'solid' | 'dashed';
export type BorderColor = keyof VyzorixTheme['colors']['border'] | 'error' | 'success' | 'warning' | 'info';
export type BorderRadius = keyof Radii;
export type BoxShadow = keyof ThemeShadows;
type Position = 'static' | 'relative' | 'absolute' | 'fixed' | 'sticky';

export interface BoxProps extends FlexProps, SizeProps, Omit<React.HTMLAttributes<HTMLElement>, 'className' | 'style'> {
  margin?: ResponsiveProp<ThemeSpacingTokens>;
  marginX?: ResponsiveProp<ThemeSpacingTokens>;
  marginY?: ResponsiveProp<ThemeSpacingTokens>;
  marginTop?: ResponsiveProp<ThemeSpacingTokens>;
  marginBottom?: ResponsiveProp<ThemeSpacingTokens>;
  marginLeft?: ResponsiveProp<ThemeSpacingTokens>;
  marginRight?: ResponsiveProp<ThemeSpacingTokens>;
  padding?: ResponsiveProp<ThemeSpacingTokens>;
  paddingX?: ResponsiveProp<ThemeSpacingTokens>;
  paddingY?: ResponsiveProp<ThemeSpacingTokens>;
  paddingTop?: ResponsiveProp<ThemeSpacingTokens>;
  paddingBottom?: ResponsiveProp<ThemeSpacingTokens>;
  paddingLeft?: ResponsiveProp<ThemeSpacingTokens>;
  paddingRight?: ResponsiveProp<ThemeSpacingTokens>;
  borderStyle?: ResponsiveProp<BorderStyle>;
  borderColor?: ResponsiveProp<BorderColor>;
  borderRadius?: ResponsiveProp<BorderRadius>;
  alignItems?: ResponsiveProp<AlignItems>;
  direction?: ResponsiveProp<Direction>;
  justifyContent?: ResponsiveProp<JustifyContent>;
  gap?: ResponsiveProp<ThemeSpacingTokens>;
  backgroundColor?: ResponsiveProp<BackgroundColor>;
  display?: ResponsiveProp<Display>;
  boxShadow?: ResponsiveProp<BoxShadow>;
  element?: ElementType;
  position?: ResponsiveProp<Position>;
}

export const Box = forwardRef<HTMLElement, PropsWithChildren<BoxProps>>((props, ref) => {
  const {
    children,
    margin,
    marginX,
    marginY,
    marginTop,
    marginBottom,
    marginLeft,
    marginRight,
    padding,
    paddingX,
    paddingY,
    paddingTop,
    paddingBottom,
    paddingLeft,
    paddingRight,
    display,
    backgroundColor,
    grow,
    shrink,
    basis,
    flex,
    borderColor,
    borderStyle,
    borderRadius,
    direction,
    justifyContent,
    alignItems,
    boxShadow,
    element,
    gap,
    width,
    minWidth,
    maxWidth,
    height,
    minHeight,
    maxHeight,
    position,
    ...rest
  } = props;

  const styles = useStyles2(
    getStyles,
    margin,
    marginX,
    marginY,
    marginTop,
    marginBottom,
    marginLeft,
    marginRight,
    padding,
    paddingX,
    paddingY,
    paddingTop,
    paddingBottom,
    paddingLeft,
    paddingRight,
    display,
    backgroundColor,
    grow,
    shrink,
    basis,
    flex,
    borderColor,
    borderStyle,
    borderRadius,
    direction,
    justifyContent,
    alignItems,
    boxShadow,
    gap,
    position,
  );
  const sizeStyles = useStyles2(getSizeStyles, width, minWidth, maxWidth, height, minHeight, maxHeight);
  const Element = element ?? 'div';

  return (
    <Element ref={ref} className={cx(styles.root, sizeStyles)} {...rest}>
      {children}
    </Element>
  );
});

Box.displayName = 'Box';

const customBorderColor = (color: BorderColor, theme: VyzorixTheme) => {
  switch (color) {
    case 'error':
    case 'success':
    case 'info':
    case 'warning':
      return theme.colors[color].subtleBorder;
    default:
      return color ? theme.colors.border[color] : undefined;
  }
};

const customBackgroundColor = (color: BackgroundColor, theme: VyzorixTheme) => {
  switch (color) {
    case 'error':
    case 'success':
    case 'info':
    case 'warning':
      return theme.colors[color].subtleBackground;
    default:
      return color ? theme.colors.background[color] : undefined;
  }
};

const getStyles = (
  theme: VyzorixTheme,
  margin: BoxProps['margin'],
  marginX: BoxProps['marginX'],
  marginY: BoxProps['marginY'],
  marginTop: BoxProps['marginTop'],
  marginBottom: BoxProps['marginBottom'],
  marginLeft: BoxProps['marginLeft'],
  marginRight: BoxProps['marginRight'],
  padding: BoxProps['padding'],
  paddingX: BoxProps['paddingX'],
  paddingY: BoxProps['paddingY'],
  paddingTop: BoxProps['paddingTop'],
  paddingBottom: BoxProps['paddingBottom'],
  paddingLeft: BoxProps['paddingLeft'],
  paddingRight: BoxProps['paddingRight'],
  display: BoxProps['display'],
  backgroundColor: BoxProps['backgroundColor'],
  grow: BoxProps['grow'],
  shrink: BoxProps['shrink'],
  basis: BoxProps['basis'],
  flex: BoxProps['flex'],
  borderColor: BoxProps['borderColor'],
  borderStyle: BoxProps['borderStyle'],
  borderRadius: BoxProps['borderRadius'],
  direction: BoxProps['direction'],
  justifyContent: BoxProps['justifyContent'],
  alignItems: BoxProps['alignItems'],
  boxShadow: BoxProps['boxShadow'],
  gap: BoxProps['gap'],
  position: BoxProps['position'],
) => {
  return {
    root: css([
      getResponsiveStyle(theme, margin, (val) => ({ margin: theme.spacing(val) })),
      getResponsiveStyle(theme, marginX, (val) => ({ marginLeft: theme.spacing(val), marginRight: theme.spacing(val) })),
      getResponsiveStyle(theme, marginY, (val) => ({ marginTop: theme.spacing(val), marginBottom: theme.spacing(val) })),
      getResponsiveStyle(theme, marginTop, (val) => ({ marginTop: theme.spacing(val) })),
      getResponsiveStyle(theme, marginBottom, (val) => ({ marginBottom: theme.spacing(val) })),
      getResponsiveStyle(theme, marginLeft, (val) => ({ marginLeft: theme.spacing(val) })),
      getResponsiveStyle(theme, marginRight, (val) => ({ marginRight: theme.spacing(val) })),
      getResponsiveStyle(theme, padding, (val) => ({ padding: theme.spacing(val) })),
      getResponsiveStyle(theme, paddingX, (val) => ({ paddingLeft: theme.spacing(val), paddingRight: theme.spacing(val) })),
      getResponsiveStyle(theme, paddingY, (val) => ({ paddingTop: theme.spacing(val), paddingBottom: theme.spacing(val) })),
      getResponsiveStyle(theme, paddingTop, (val) => ({ paddingTop: theme.spacing(val) })),
      getResponsiveStyle(theme, paddingBottom, (val) => ({ paddingBottom: theme.spacing(val) })),
      getResponsiveStyle(theme, paddingLeft, (val) => ({ paddingLeft: theme.spacing(val) })),
      getResponsiveStyle(theme, paddingRight, (val) => ({ paddingRight: theme.spacing(val) })),
      getResponsiveStyle(theme, display, (val) => ({ display: val })),
      getResponsiveStyle(theme, backgroundColor, (val) => ({ backgroundColor: customBackgroundColor(val, theme) })),
      getResponsiveStyle(theme, direction, (val) => ({ flexDirection: val })),
      getResponsiveStyle(theme, grow, (val) => ({ flexGrow: val })),
      getResponsiveStyle(theme, shrink, (val) => ({ flexShrink: val })),
      getResponsiveStyle(theme, basis, (val) => ({ flexBasis: val })),
      getResponsiveStyle(theme, flex, (val) => ({ flex: val })),
      getResponsiveStyle(theme, borderStyle, (val) => ({ borderStyle: val })),
      getResponsiveStyle(theme, borderColor, (val) => ({ borderColor: customBorderColor(val, theme) })),
      (borderStyle || borderColor) && { borderWidth: '1px' },
      getResponsiveStyle(theme, justifyContent, (val) => ({ justifyContent: val })),
      getResponsiveStyle(theme, alignItems, (val) => ({ alignItems: val })),
      getResponsiveStyle(theme, borderRadius, (val) => ({ borderRadius: theme.shape.radius[val] })),
      getResponsiveStyle(theme, boxShadow, (val) => ({ boxShadow: theme.shadows[val] })),
      getResponsiveStyle(theme, gap, (val) => ({ gap: theme.spacing(val) })),
      getResponsiveStyle(theme, position, (val) => ({ position: val })),
    ]),
  };
};
