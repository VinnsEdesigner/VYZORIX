import { css } from '@emotion/css';
import { type VyzorixTheme } from '../../../theme/types';
import { getResponsiveStyle, type ResponsiveProp } from './responsiveness';

export interface SizeProps {
  minWidth?: ResponsiveProp<number | string>;
  maxWidth?: ResponsiveProp<number | string>;
  width?: ResponsiveProp<number | string>;
  minHeight?: ResponsiveProp<number | string>;
  maxHeight?: ResponsiveProp<number | string>;
  height?: ResponsiveProp<number | string>;
}

export const getSizeStyles = (
  theme: VyzorixTheme,
  width: SizeProps['width'],
  minWidth: SizeProps['minWidth'],
  maxWidth: SizeProps['maxWidth'],
  height: SizeProps['height'],
  minHeight: SizeProps['minHeight'],
  maxHeight: SizeProps['maxHeight'],
) => {
  return css([
    getResponsiveStyle(theme, width, (val) => ({ width: theme.spacing(val) })),
    getResponsiveStyle(theme, minWidth, (val) => ({ minWidth: theme.spacing(val) })),
    getResponsiveStyle(theme, maxWidth, (val) => ({ maxWidth: theme.spacing(val) })),
    getResponsiveStyle(theme, height, (val) => ({ height: theme.spacing(val) })),
    getResponsiveStyle(theme, minHeight, (val) => ({ minHeight: theme.spacing(val) })),
    getResponsiveStyle(theme, maxHeight, (val) => ({ maxHeight: theme.spacing(val) })),
  ]);
};
