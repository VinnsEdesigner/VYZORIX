import { createContext, useContext } from 'react';
import { resolveBranding, type BrandingSettings } from './types';

const BrandingContext = createContext(resolveBranding());

export interface BrandingProviderProps {
  settings?: BrandingSettings;
  children: React.ReactNode;
}

export function BrandingProvider({ settings, children }: BrandingProviderProps) {
  return <BrandingContext.Provider value={resolveBranding(settings)}>{children}</BrandingContext.Provider>;
}

export function useBranding() {
  return useContext(BrandingContext);
}

export { BrandingContext };
