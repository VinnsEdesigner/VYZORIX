import { type BrandingSettings } from '@vyzorix/ui';

/**
 * Deployment branding.
 *
 * Read from build-time env so a deployment can swap its title, logos and icons
 * without touching code. Every value is optional: anything unset falls back to
 * the bundled assets in `public/brand`, so the app renders correctly with none
 * of these set.
 *
 * These are `VITE_`-prefixed because they must reach the client bundle. They are
 * not secrets — a logo URL is public by definition.
 */
function env(key: string): string | undefined {
  const value = import.meta.env[key];
  return typeof value === 'string' && value.length > 0 ? value : undefined;
}

export const brandingSettings: BrandingSettings = {
  appTitle: env('VITE_BRAND_APP_TITLE'),
  loginTitle: env('VITE_BRAND_LOGIN_TITLE'),
  loginSubtitle: env('VITE_BRAND_LOGIN_SUBTITLE'),
  loginLogo: env('VITE_BRAND_LOGIN_LOGO'),
  loginBackground: env('VITE_BRAND_LOGIN_BACKGROUND'),
  menuLogo: env('VITE_BRAND_MENU_LOGO'),
  loadingLogo: env('VITE_BRAND_LOADING_LOGO'),
  bootAnimation: env('VITE_BRAND_BOOT_ANIMATION'),
  bootAnimationLoop: env('VITE_BRAND_BOOT_ANIMATION_LOOP'),
  favIcon: env('VITE_BRAND_FAVICON'),
  appleTouchIcon: env('VITE_BRAND_APPLE_TOUCH_ICON'),
  maskIcon: env('VITE_BRAND_MASK_ICON'),
  maskIconColor: env('VITE_BRAND_MASK_ICON_COLOR'),
};
