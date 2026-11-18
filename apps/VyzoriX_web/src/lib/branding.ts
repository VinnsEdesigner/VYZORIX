import { type BrandingSettings } from '@vyzorix/ui';

function env(key: string): string | undefined {
  const value = import.meta.env[key];
  return typeof value === 'string' && value.length > 0 ? value : undefined;
}

function defined(entries: Record<string, string | undefined>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [key, value] of Object.entries(entries)) {
    if (value !== undefined) out[key] = value;
  }
  return out;
}

export const brandingSettings: BrandingSettings = defined({
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
});
