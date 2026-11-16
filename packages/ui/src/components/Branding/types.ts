/**
 * What a deployment may customise without a rebuild.
 *
 * Every field is optional: the components fall back to the bundled assets, so
 * the app renders correctly with no settings at all. A deployment that supplies
 * settings swaps its own logo, title and favicon.
 */
export interface BrandingSettings {
  /** Product name, shown next to the mark and in the document title. */
  appTitle?: string;
  /** Shown above the sign-in form. */
  loginTitle?: string;
  /** Sub-line under the sign-in title. */
  loginSubtitle?: string;
  /** Full logo for the sign-in page. Defaults to the bundled mark. */
  loginLogo?: string;
  /** Full-bleed artwork behind the sign-in form. */
  loginBackground?: string;
  /** Mark shown in the top bar / navigation. */
  menuLogo?: string;
  /** Mark shown while the app boots. */
  loadingLogo?: string;
  /** Browser tab icon. */
  favIcon?: string;
  /** iOS home-screen icon. */
  appleTouchIcon?: string;
  /** Safari pinned-tab silhouette. */
  maskIcon?: string;
  /** Accent for the pinned-tab silhouette. */
  maskIconColor?: string;
}

/**
 * The settings actually in force, with every field resolved to a value.
 *
 * The defaults point at the assets the web app ships in `public/brand`, which
 * are served from the site root, so a path is enough — no import needed.
 */
export const DEFAULT_BRANDING: Required<BrandingSettings> = {
  appTitle: 'Vyzorix',
  loginTitle: 'Welcome to Vyzorix',
  loginSubtitle: 'Sign in to manage your devices',
  loginLogo: '/brand/vyzorix_icon.svg',
  loginBackground: '',
  menuLogo: '/brand/vyzorix_icon.svg',
  loadingLogo: '/brand/vyzorix_icon.svg',
  favIcon: '/brand/favicon-32.png',
  appleTouchIcon: '/brand/apple-touch-icon.png',
  maskIcon: '/brand/vyzorix_mask_icon.svg',
  maskIconColor: '#e11d48',
};

/** Merge supplied settings over the defaults. */
export function resolveBranding(settings?: BrandingSettings): Required<BrandingSettings> {
  return { ...DEFAULT_BRANDING, ...(settings ?? {}) };
}
