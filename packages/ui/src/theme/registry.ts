import { createTheme, type NewThemeOptions } from './createTheme';
import { type VyzorixTheme } from './types';

import burgundy from './themeDefinitions/burgundy.json';
import dark from './themeDefinitions/dark.json';
import light from './themeDefinitions/light.json';
import matrix from './themeDefinitions/matrix.json';
import rose500 from './themeDefinitions/rose-500.json';
import synthwave from './themeDefinitions/synthwave.json';
import tron from './themeDefinitions/tron.json';
import zen from './themeDefinitions/zen.json';

export interface ThemeRegistryItem {
  id: string;
  name: string;

  isExtra?: boolean;
  build: () => VyzorixTheme;
}

const definitions: Record<string, NewThemeOptions> = {
  dark: dark as NewThemeOptions,
  light: light as NewThemeOptions,
  'rose-500': rose500 as NewThemeOptions,
  burgundy: burgundy as NewThemeOptions,
  matrix: matrix as NewThemeOptions,
  tron: tron as NewThemeOptions,
  synthwave: synthwave as NewThemeOptions,
  zen: zen as NewThemeOptions,
};

export const THEME_IDS = [
  'dark',
  'light',
  'rose-500',
  'burgundy',
  'matrix',
  'tron',
  'synthwave',
  'zen',
] as const;

export type ThemeId = (typeof THEME_IDS)[number];

export const DEFAULT_THEME_ID: ThemeId = 'dark';

export const SYSTEM_THEME_ID = 'system';

function systemPreference(): VyzorixTheme {
  if (typeof window === 'undefined' || !window.matchMedia) {
    return getThemeById('dark');
  }
  return getThemeById(window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
}

const registry = new Map<string, ThemeRegistryItem>();
registry.set(SYSTEM_THEME_ID, { id: SYSTEM_THEME_ID, name: 'System preference', build: systemPreference });

for (const id of THEME_IDS) {
  const def = definitions[id]!;
  registry.set(id, {
    id,
    name: def.name,
    isExtra: id !== 'dark' && id !== 'light',
    build: () => createTheme(def),
  });
}

export function getThemeById(id: string): VyzorixTheme {
  return (registry.get(id) ?? registry.get(DEFAULT_THEME_ID))!.build();
}

export function getBuiltInThemes(allowedExtras: string[] = [...THEME_IDS]): ThemeRegistryItem[] {
  const items = [...registry.values()].filter((item) => {
    if (item.isExtra) return allowedExtras.includes(item.id);
    return true;
  });

  const collator = new Intl.Collator();
  return items.sort((a, b) => {
    if (a.isExtra !== b.isExtra) return a.isExtra ? 1 : -1;
    return collator.compare(a.name, b.name);
  });
}

export function themeIds(): string[] {
  return [...THEME_IDS];
}

export function isThemeId(id: unknown): id is ThemeId {
  return typeof id === 'string' && (THEME_IDS as readonly string[]).includes(id);
}

export function resolveThemeId(id: string | null | undefined): ThemeId {
  return isThemeId(id) ? id : DEFAULT_THEME_ID;
}
