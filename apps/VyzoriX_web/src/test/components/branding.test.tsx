import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BrandingProvider, BrandLogo, BrandWordmark, DEFAULT_BRANDING, resolveBranding, ThemeContext, getThemeById } from '@vyzorix/ui';

function renderThemed(ui: React.ReactElement, themeId = 'dark') {
  return render(<ThemeContext.Provider value={getThemeById(themeId)}>{ui}</ThemeContext.Provider>);
}

describe('resolveBranding', () => {
  it('returns the defaults when given nothing', () => {
    expect(resolveBranding()).toEqual(DEFAULT_BRANDING);
  });

  it('overlays supplied values on the defaults', () => {
    const resolved = resolveBranding({ appTitle: 'Acme Ops' });
    expect(resolved.appTitle).toBe('Acme Ops');

    expect(resolved.menuLogo).toBe(DEFAULT_BRANDING.menuLogo);
  });

  it('does not mutate the defaults', () => {
    resolveBranding({ appTitle: 'Changed' });
    expect(DEFAULT_BRANDING.appTitle).toBe('Vyzorix');
  });
});

describe('BrandLogo', () => {
  it('picks the mark for a dark surface on a dark theme', () => {
    renderThemed(<BrandLogo />, 'dark');
    const img = screen.getByRole('img', { name: 'Vyzorix' });
    expect(img).toHaveAttribute('src', '/brand/vyzorix_icon_on_dark.svg');
  });

  it('picks the mark for a light surface on a light theme', () => {
    renderThemed(<BrandLogo />, 'light');
    const img = screen.getByRole('img', { name: 'Vyzorix' });
    expect(img).toHaveAttribute('src', '/brand/vyzorix_icon_on_light.svg');
  });

  it('uses the app title as the accessible name', () => {
    renderThemed(
      <BrandingProvider settings={{ appTitle: 'Acme Ops' }}>
        <BrandLogo />
      </BrandingProvider>,
    );
    expect(screen.getByRole('img', { name: 'Acme Ops' })).toBeInTheDocument();
  });

  it('lets a configured logo override the theme-matched mark', () => {
    renderThemed(
      <BrandingProvider settings={{ loginLogo: '/custom/login.svg' }}>
        <BrandLogo source="login" />
      </BrandingProvider>,
    );
    expect(screen.getByRole('img')).toHaveAttribute('src', '/custom/login.svg');
  });

  it('allows an explicit alt override', () => {
    renderThemed(<BrandLogo alt="Home" />);
    expect(screen.getByRole('img', { name: 'Home' })).toBeInTheDocument();
  });
});

describe('BrandWordmark', () => {
  it('picks the dark artwork on a dark theme', () => {
    renderThemed(<BrandWordmark />, 'dark');
    expect(screen.getByRole('img', { name: 'Vyzorix' })).toHaveAttribute('src', '/brand/vyzorix_wordmark_dark.svg');
  });

  it('picks the light artwork on a light theme', () => {
    renderThemed(<BrandWordmark />, 'light');
    expect(screen.getByRole('img', { name: 'Vyzorix' })).toHaveAttribute('src', '/brand/vyzorix_wordmark_light.svg');
  });
});
