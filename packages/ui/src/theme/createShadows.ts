import { type ThemeColors } from './createColors';

export interface ThemeShadowsInput {

  z1?: string;

  z2?: string;

  z3?: string;
}

export interface ThemeShadows {
  z1: string;
  z2: string;
  z3: string;
}

const DEFAULT_DARK_SHADOWS: ThemeShadows = {
  z1: '0px 1px 2px rgba(1, 4, 9, 0.75)',
  z2: '0px 4px 8px rgba(1, 4, 9, 0.75)',
  z3: '0px 8px 24px rgb(1, 4, 9)',
};

const DEFAULT_LIGHT_SHADOWS: ThemeShadows = {
  z1: '0px 1px 2px rgba(24, 26, 27, 0.2)',
  z2: '0px 4px 8px rgba(24, 26, 27, 0.2)',
  z3: '0px 13px 20px 1px rgba(24, 26, 27, 0.18)',
};

export function createShadows(colors: ThemeColors, input: ThemeShadowsInput = {}): ThemeShadows {
  const defaults = colors.mode === 'dark' ? DEFAULT_DARK_SHADOWS : DEFAULT_LIGHT_SHADOWS;
  return {
    z1: input.z1 ?? defaults.z1,
    z2: input.z2 ?? defaults.z2,
    z3: input.z3 ?? defaults.z3,
  };
}
