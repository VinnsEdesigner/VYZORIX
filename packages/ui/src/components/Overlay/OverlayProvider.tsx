import { createContext, useContext, useMemo, useSyncExternalStore, type ReactNode } from 'react';
import { useTheme2 } from '../../theme/ThemeContext';
import { type Breakpoint } from '../../theme/breakpoints';

export type OverlayMode = 'sheet' | 'popover' | 'auto';

export type OverlayPresentation = 'sheet' | 'popover';

export interface OverlayContextValue {
  mode: OverlayMode;
  presentation: OverlayPresentation;
}

const OverlayContext = createContext<OverlayContextValue | null>(null);

function subscribe(query: string, onChange: () => void): () => void {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return () => undefined;
  }
  const list = window.matchMedia(query);
  list.addEventListener('change', onChange);

  return () => list.removeEventListener('change', onChange);
}

function getMatches(query: string): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false;

  return window.matchMedia(query).matches;
}

export interface OverlayProviderProps {
  children: ReactNode;
  mode?: OverlayMode;
  sheetBelow?: Breakpoint;
}

export function OverlayProvider({
  children,
  mode = 'sheet',
  sheetBelow = 'md',
}: OverlayProviderProps) {
  const theme = useTheme2();
  const query = `(max-width: ${theme.breakpoints.values[sheetBelow]}px)`;
  const narrow = useSyncExternalStore(
    (onChange) => subscribe(query, onChange),
    () => getMatches(query),
    () => false,
  );

  const presentation: OverlayPresentation = mode === 'auto' ? (narrow ? 'sheet' : 'popover') : mode;

  const value = useMemo<OverlayContextValue>(() => ({ mode, presentation }), [mode, presentation]);

  return <OverlayContext.Provider value={value}>{children}</OverlayContext.Provider>;
}

const FALLBACK: OverlayContextValue = { mode: 'sheet', presentation: 'sheet' };

export function useOverlay(): OverlayContextValue {
  return useContext(OverlayContext) ?? FALLBACK;
}
