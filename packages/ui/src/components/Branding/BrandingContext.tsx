import { createContext, useContext } from 'react';
import { resolveBranding, type BrandingSettings } from './types';

/**
 * Branding is supplied through context so a deployment can override it at boot
 * and any component can read it. The default value is the resolved built-in
 * settings, so `useBranding()` works with no provider — a component or a test
 * does not have to wrap itself just to render a logo.
 */
const BrandingContext = createContext(resolveBranding());

export interface BrandingProviderProps {
  settings?: BrandingSettings;
  children: React.ReactNode;
}

export function BrandingProvider({ settings, children }: BrandingProviderProps) {
  return <BrandingContext.Provider value={resolveBranding(settings)}>{children}</BrandingContext.Provider>;
}

/** Read the branding in force. Always returns a fully resolved object. */
export function useBranding() {
  return useContext(BrandingContext);
}

export { BrandingContext };
