/**
 * Build the theme's colour model.
 *
 * A theme definition supplies a *partial* colour set; it is merged onto a dark
 * or light base, and `getRichColor` derives the rest (emphasis, contrast text,
 * borders, subtle variants) from each colour's `main`. A theme can therefore be
 * as small as `{ mode, primary: { main } }`.
 */

import { alpha, darken, emphasize, getContrastRatio, lighten } from './colorManipulator';
import { palette } from './palette';
import { type DeepRequired, type RichColorName, type ThemeRichColor, type ThemeRichColorInput } from './types';

export type ThemeColorsMode = 'light' | 'dark';

export interface ThemeColorsInput {
  mode?: ThemeColorsMode;
  primary?: ThemeRichColorInput;
  secondary?: ThemeRichColorInput;
  tertiary?: ThemeRichColorInput;
  accent?: ThemeRichColorInput;
  info?: ThemeRichColorInput;
  error?: ThemeRichColorInput;
  success?: ThemeRichColorInput;
  warning?: ThemeRichColorInput;

  text?: {
    primary?: string;
    secondary?: string;
    disabled?: string;
    link?: string;
    maxContrast?: string;
  };
  background?: {
    canvas?: string;
    page?: string;
    primary?: string;
    secondary?: string;
    elevated?: string;
  };
  border?: {
    weak?: string;
    medium?: string;
    strong?: string;
  };
  gradients?: {
    brandVertical?: string;
    brandHorizontal?: string;
  };
  action?: {
    selected?: string;
    selectedBorder?: string;
    hover?: string;
    hoverOpacity?: number;
    focus?: string;
    disabledBackground?: string;
    disabledText?: string;
    disabledOpacity?: number;
  };
  scrollbar?: string;
  hoverFactor?: number;
  contrastThreshold?: number;
  tonalOffset?: number;
}

type ThemeColorsBase<TColor> = DeepRequired<
  Omit<ThemeColorsInput, 'primary' | 'secondary' | 'tertiary' | 'accent' | 'info' | 'error' | 'success' | 'warning'>
> & {
  primary: TColor;
  secondary: TColor;
  tertiary: TColor;
  accent: TColor;
  info: TColor;
  error: TColor;
  success: TColor;
  warning: TColor;
};

export interface ThemeColors extends ThemeColorsBase<ThemeRichColor> {
  /** Returns a text colour that contrasts with the given background. */
  getContrastText(background: string, threshold?: number): string;
  /** Brighten or darken a colour by the given factor (0-1). */
  emphasize(color: string, amount?: number): string;
}

class DarkColors implements ThemeColorsBase<Partial<ThemeRichColor>> {
  mode: ThemeColorsMode = 'dark';

  whiteBase = '204, 204, 220';

  border = {
    weak: `rgba(${this.whiteBase}, 0.12)`,
    medium: `rgba(${this.whiteBase}, 0.2)`,
    strong: `rgba(${this.whiteBase}, 0.30)`,
  };

  text = {
    primary: `rgb(${this.whiteBase})`,
    secondary: `rgba(${this.whiteBase}, 0.65)`,
    disabled: `rgba(${this.whiteBase}, 0.61)`,
    link: palette.blueDarkText,
    maxContrast: palette.white,
  };

  primary = { main: palette.blueDarkMain, text: palette.blueDarkText, border: palette.blueDarkText };
  secondary = {
    main: palette.gray20,
    shade: palette.gray25,
    transparent: `rgba(${this.whiteBase}, 0.08)`,
    text: this.text.primary,
    contrastText: `rgb(${this.whiteBase})`,
    border: this.border.strong,
    subtleBorder: this.border.weak,
  };
  tertiary = { main: palette.purpleDarkMain, text: palette.purpleDarkText };
  accent = this.primary;
  info = this.primary;
  error = { main: palette.redDarkMain, text: palette.redDarkText };
  success = { main: palette.greenDarkMain, text: palette.greenDarkText };
  warning = { main: palette.orangeDarkMain, text: palette.orangeDarkText };

  background = {
    canvas: palette.gray05,
    page: palette.gray10,
    primary: palette.gray10,
    secondary: palette.gray15,
    elevated: palette.gray15,
  };

  action = {
    hover: `rgba(${this.whiteBase}, 0.16)`,
    selected: `rgba(${this.whiteBase}, 0.12)`,
    selectedBorder: palette.orangeDarkMain,
    focus: `rgba(${this.whiteBase}, 0.16)`,
    hoverOpacity: 0.08,
    disabledText: this.text.disabled,
    disabledBackground: `rgba(${this.whiteBase}, 0.04)`,
    disabledOpacity: 0.38,
  };

  gradients = {
    brandHorizontal: 'linear-gradient(270deg, #F55F3E 0%, #FF8833 100%)',
    brandVertical: 'linear-gradient(0.01deg, #F55F3E 0.01%, #FF8833 99.99%)',
  };

  scrollbar = `rgba(${this.whiteBase}, 0.3)`;
  contrastThreshold = 3;
  hoverFactor = 0.03;
  tonalOffset = 0.15;
}

class LightColors implements ThemeColorsBase<Partial<ThemeRichColor>> {
  mode: ThemeColorsMode = 'light';

  blackBase = '36, 41, 46';

  primary = { main: palette.blueLightMain, border: palette.blueLightText, text: palette.blueLightText };
  text = {
    primary: `rgba(${this.blackBase}, 1)`,
    secondary: `rgba(${this.blackBase}, 0.75)`,
    disabled: `rgba(${this.blackBase}, 0.65)`,
    link: this.primary.text,
    maxContrast: palette.black,
  };
  border = {
    weak: `rgba(${this.blackBase}, 0.12)`,
    medium: `rgba(${this.blackBase}, 0.3)`,
    strong: `rgba(${this.blackBase}, 0.4)`,
  };
  secondary = {
    main: palette.gray90,
    shade: palette.gray85,
    transparent: `rgba(${this.blackBase}, 0.08)`,
    contrastText: `rgba(${this.blackBase},  1)`,
    text: this.text.primary,
    border: this.border.strong,
    subtleBorder: this.border.weak,
  };
  tertiary = { main: palette.purpleLightMain, text: palette.purpleLightText };
  accent = this.primary;
  info = { main: palette.blueLightMain, text: palette.blueLightText };
  error = { main: palette.redLightMain, text: palette.redLightText, border: palette.redLightText };
  success = { main: palette.greenLightMain, text: palette.greenLightText };
  warning = { main: palette.orangeLightMain, text: palette.orangeLightText };

  background = {
    canvas: palette.gray100,
    page: palette.white,
    primary: palette.white,
    secondary: palette.gray95,
    elevated: palette.white,
  };

  action = {
    hover: `rgba(${this.blackBase}, 0.12)`,
    selected: `rgba(${this.blackBase}, 0.08)`,
    selectedBorder: palette.orangeLightMain,
    hoverOpacity: 0.08,
    focus: `rgba(${this.blackBase}, 0.12)`,
    disabledBackground: `rgba(${this.blackBase}, 0.04)`,
    disabledText: this.text.disabled,
    disabledOpacity: 0.38,
  };

  gradients = {
    brandHorizontal: 'linear-gradient(90deg, #FF8833 0%, #F53E4C 100%)',
    brandVertical: 'linear-gradient(0.01deg, #F53E4C -31.2%, #FF8833 113.07%)',
  };

  scrollbar = `rgba(${this.blackBase}, 0.3)`;
  contrastThreshold = 3;
  hoverFactor = 0.03;
  tonalOffset = 0.2;
}

interface GetRichColorProps {
  color: ThemeRichColorInput;
  name: RichColorName;
}

export function createColors(colors: ThemeColorsInput): ThemeColors {
  const dark = new DarkColors();
  const light = new LightColors();
  const base = (colors.mode ?? 'dark') === 'dark' ? dark : light;
  const {
    primary = base.primary,
    secondary = base.secondary,
    tertiary = base.tertiary,
    accent = base.accent,
    info = base.info,
    warning = base.warning,
    success = base.success,
    error = base.error,
    tonalOffset = base.tonalOffset,
    hoverFactor = base.hoverFactor,
    contrastThreshold = base.contrastThreshold,
    ...other
  } = colors;

  function getContrastText(background: string, threshold: number = contrastThreshold): string {
    return getContrastRatio(dark.text.maxContrast, background, base.background.primary) >= threshold
      ? dark.text.maxContrast
      : light.text.maxContrast;
  }

  const getRichColor = ({ color, name }: GetRichColorProps): ThemeRichColor => {
    const rich: ThemeRichColorInput = { ...color, name };
    rich.main ??= base[name].main;
    rich.mainEmphasis ??= emphasize(rich.main, tonalOffset);
    rich.contrastText ??= getContrastText(rich.main);
    rich.background ??= rich.main;
    rich.backgroundEmphasis ??= emphasize(rich.background, tonalOffset);
    rich.text ??= rich.main;
    rich.textEmphasis ??=
      base.mode === 'light' ? darken(rich.text, tonalOffset) : lighten(rich.text, tonalOffset);
    rich.border ??= rich.text;
    rich.borderEmphasis ??= emphasize(rich.border, tonalOffset);
    rich.subtleBackground ??= alpha(rich.background, 0.15);
    rich.subtleBorder ??= alpha(rich.border, 0.25);
    // Deprecated aliases kept so older call sites keep working.
    rich.shade ??= base.mode === 'light' ? darken(rich.main, tonalOffset) : lighten(rich.main, tonalOffset);
    rich.transparent ??= alpha(rich.main, 0.15);
    rich.borderTransparent ??= alpha(rich.border, 0.25);
    return rich as ThemeRichColor;
  };

  return {
    ...base,
    primary: getRichColor({ color: primary, name: 'primary' }),
    secondary: getRichColor({ color: secondary, name: 'secondary' }),
    tertiary: getRichColor({ color: tertiary, name: 'tertiary' }),
    accent: getRichColor({ color: accent, name: 'accent' }),
    info: getRichColor({ color: info, name: 'info' }),
    error: getRichColor({ color: error, name: 'error' }),
    success: getRichColor({ color: success, name: 'success' }),
    warning: getRichColor({ color: warning, name: 'warning' }),
    getContrastText,
    emphasize: (color: string, factor?: number) => emphasize(color, factor ?? hoverFactor),
    ...other,
  } as ThemeColors;
}
