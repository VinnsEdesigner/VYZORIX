import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { BootScreen, BrandingProvider, DEFAULT_BRANDING, ThemeContext, getThemeById } from '@vyzorix/ui';

function renderBoot(props: React.ComponentProps<typeof BootScreen> = {}) {
  return render(
    <ThemeContext.Provider value={getThemeById('dark')}>
      <BrandingProvider>
        <BootScreen {...props} />
      </BrandingProvider>
    </ThemeContext.Provider>,
  );
}

describe('BootScreen', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // matchMedia defaults to "no preference"; tests override where needed.
    vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('starts on the assemble animation', () => {
    renderBoot();
    const img = document.querySelector('img');
    expect(img).toHaveAttribute('src', DEFAULT_BRANDING.bootAnimation);
  });

  it('hands off to the looping animation after the handoff delay', () => {
    renderBoot({ handoffMs: 1000 });
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(document.querySelector('img')).toHaveAttribute('src', DEFAULT_BRANDING.bootAnimationLoop);
  });

  it('shows the still mark when the user prefers reduced motion', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true, addEventListener() {}, removeEventListener() {} }));
    renderBoot();
    expect(document.querySelector('img')).toHaveAttribute('src', DEFAULT_BRANDING.loadingLogo);
  });

  it('stays on the still mark under reduced motion even after the handoff', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true, addEventListener() {}, removeEventListener() {} }));
    renderBoot({ handoffMs: 1000 });
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(document.querySelector('img')).toHaveAttribute('src', DEFAULT_BRANDING.loadingLogo);
  });

  it('is announced as a loading status', () => {
    renderBoot({ label: 'Starting up' });
    expect(screen.getByRole('status')).toHaveAttribute('aria-label', 'Starting up');
    expect(screen.getByText('Starting up')).toBeInTheDocument();
  });

  it('uses the default loading label when none is given', () => {
    renderBoot();
    expect(screen.getByRole('status')).toHaveAttribute('aria-label', 'Loading');
  });

  it('marks the decorative mark hidden from assistive tech', () => {
    renderBoot();
    expect(document.querySelector('img')).toHaveAttribute('aria-hidden');
  });
});
