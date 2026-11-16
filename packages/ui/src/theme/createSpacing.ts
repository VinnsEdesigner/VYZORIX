/**
 * Theme spacing: an 8px grid, callable and tokenised.
 *
 * `theme.spacing(2)` returns "16px"; `theme.spacing.x2` is the same value.
 */

type ThemeSpacingArgument = number | string;

export interface ThemeSpacing {
  (): string;
  (value: ThemeSpacingArgument): string;
  (topBottom: ThemeSpacingArgument, rightLeft: ThemeSpacingArgument): string;
  (top: ThemeSpacingArgument, rightLeft: ThemeSpacingArgument, bottom: ThemeSpacingArgument): string;
  (top: ThemeSpacingArgument, right: ThemeSpacingArgument, bottom: ThemeSpacingArgument, left: ThemeSpacingArgument): string;
  gridSize: number;
  x0: string;
  x0_25: string;
  x0_5: string;
  x1: string;
  x1_5: string;
  x2: string;
  x2_5: string;
  x3: string;
  x4: string;
  x5: string;
  x6: string;
  x8: string;
  x10: string;
}

export interface ThemeSpacingOptions {
  gridSize?: number;
}

export function createSpacing(options: ThemeSpacingOptions = {}): ThemeSpacing {
  const { gridSize = 8 } = options;

  const transform = (value: ThemeSpacingArgument): number | string =>
    typeof value === 'string' ? value : value * gridSize;

  const spacing = ((...args: Array<number | string>): string => {
    if (args.length === 0) args[0] = 1;
    return args
      .map((argument) => {
        const output = transform(argument);
        return typeof output === 'number' ? `${output}px` : output;
      })
      .join(' ');
  }) as ThemeSpacing;

  spacing.gridSize = gridSize;
  spacing.x0 = '0px';
  spacing.x0_25 = '2px';
  spacing.x0_5 = '4px';
  spacing.x1 = '8px';
  spacing.x1_5 = '12px';
  spacing.x2 = '16px';
  spacing.x2_5 = '20px';
  spacing.x3 = '24px';
  spacing.x4 = '32px';
  spacing.x5 = '40px';
  spacing.x6 = '48px';
  spacing.x8 = '64px';
  spacing.x10 = '80px';

  return spacing;
}
