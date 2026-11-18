export interface BrandingSettings {
  appTitle?: string;

  loginTitle?: string;

  loginSubtitle?: string;

  loginLogo?: string;

  loginBackground?: string;

  menuLogo?: string;

  loadingLogo?: string;

  bootAnimation?: string;

  bootAnimationLoop?: string;

  favIcon?: string;

  appleTouchIcon?: string;

  maskIcon?: string;

  maskIconColor?: string;
}

export const DEFAULT_BRANDING: Required<BrandingSettings> = {
  appTitle: 'Vyzorix',
  loginTitle: 'Welcome to Vyzorix',
  loginSubtitle: 'Sign in to manage your devices',
  loginLogo: '',
  loginBackground: '',
  menuLogo: '',
  loadingLogo: '',
  bootAnimation: '/brand/vyzorix_boot_assemble.gif',
  bootAnimationLoop: '/brand/vyzorix_boot_orbit.gif',
  favIcon: '/brand/favicon-32.png',
  appleTouchIcon: '/brand/apple-touch-icon.png',
  maskIcon: '/brand/vyzorix_mask_icon.svg',
  maskIconColor: '#be123c',
};

export function resolveBranding(settings?: BrandingSettings): Required<BrandingSettings> {
  return { ...DEFAULT_BRANDING, ...(settings ?? {}) };
}
