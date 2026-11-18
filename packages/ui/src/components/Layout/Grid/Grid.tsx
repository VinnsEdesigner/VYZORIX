import { css } from '@emotion/css';
import { forwardRef, type HTMLAttributes } from 'react';
import * as React from 'react';
import { type VyzorixTheme } from '../../../theme/types';
import { type ThemeSpacingTokens } from '../../../theme/createSpacing';
import { useStyles2 } from '../../../theme/ThemeContext';
import { type AlignItems } from '../types';
import { type ResponsiveProp, getResponsiveStyle } from '../utils/responsiveness';

interface GridPropsBase extends Omit<HTMLAttributes<HTMLDivElement>, 'className' | 'style'> {
  children: NonNullable<React.ReactNode>;
  gap?: ResponsiveProp<ThemeSpacingTokens>;
  rowGap?: ResponsiveProp<ThemeSpacingTokens>;
  columnGap?: ResponsiveProp<ThemeSpacingTokens>;
  alignItems?: ResponsiveProp<AlignItems>;
}

interface PropsWithColumns extends GridPropsBase {
  columns?: ResponsiveProp<1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12>;
  minColumnWidth?: never;
}

interface PropsWithMinColumnWidth extends GridPropsBase {
  columns?: never;
  minColumnWidth?: ResponsiveProp<1 | 2 | 3 | 5 | 8 | 13 | 16 | 21 | 34 | 44 | 55 | 72 | 89 | 144>;
}

type GridProps = PropsWithColumns | PropsWithMinColumnWidth;

export const Grid = forwardRef<HTMLDivElement, GridProps>((props, ref) => {
  const { alignItems, children, gap, rowGap, columnGap, columns, minColumnWidth, ...rest } = props;
  const styles = useStyles2(getGridStyles, gap, rowGap, columnGap, columns, minColumnWidth, alignItems);

  return (
    <div ref={ref} {...rest} className={styles.grid}>
      {children}
    </div>
  );
});

Grid.displayName = 'Grid';

const getGridStyles = (
  theme: VyzorixTheme,
  gap: GridProps['gap'],
  rowGap: GridProps['rowGap'],
  columnGap: GridProps['columnGap'],
  columns: GridProps['columns'],
  minColumnWidth: GridProps['minColumnWidth'],
  alignItems: GridProps['alignItems'],
) => {
  return {
    grid: css([
      { display: 'grid' },
      getResponsiveStyle(theme, gap, (val) => ({ gap: theme.spacing(val) })),
      getResponsiveStyle(theme, rowGap, (val) => ({ rowGap: theme.spacing(val) })),
      getResponsiveStyle(theme, columnGap, (val) => ({ columnGap: theme.spacing(val) })),
      minColumnWidth &&
        getResponsiveStyle(theme, minColumnWidth, (val) => ({
          gridTemplateColumns: `repeat(auto-fill, minmax(${theme.spacing(val)}, 1fr))`,
        })),
      columns &&
        getResponsiveStyle(theme, columns, (val) => ({
          gridTemplateColumns: `repeat(${val}, 1fr)`,
        })),
      getResponsiveStyle(theme, alignItems, (val) => ({ alignItems: val })),
    ]),
  };
};
