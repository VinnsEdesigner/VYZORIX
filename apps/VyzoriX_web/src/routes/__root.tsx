import { HeadContent, Outlet, Scripts, createRootRoute } from '@tanstack/react-router';
import { useEffect, useState } from 'react';
import { BrandingProvider, OverlayProvider, resolveBranding } from '@vyzorix/ui';
import { QueryProvider } from '@/providers/query-provider';
import { ThemeProvider } from '@/providers/theme-provider';
import { VyzorI18nProvider } from '@/providers/vyzor-i18n-provider';
import { VyzorAnalyticsProvider } from '@/providers/vyzor-analytics-provider';
import { ToastProvider } from '@/providers/toast-provider';
import { useWebSocketConnection } from '@/hooks/realtime/use-realtime';
import { useInitConnectivity } from '@/hooks/connectivity/use-connectivity';
import { useSessionBootstrap } from '@/hooks/auth';
import { VyzorErrorBoundary } from '@/ui/components/error/vyzor-error-boundary';
import { VyzorRouteError } from '@/ui/components/error/vyzor-route-error';
import { VyzorConsentBanner } from '@/ui/components/analytics/vyzor-consent-banner';
import { AppShell } from '@/ui/components/layout';
import { LoginPage } from '@/ui/pages/login/login-page';
import { useAuthStore } from '@/stores';
import { brandingSettings } from '@/lib/branding';

const branding = resolveBranding(brandingSettings);

const DARK_THEME_IDS = ['dark', 'burgundy', 'matrix', 'tron', 'synthwave'];
const ANTI_FLASH = `(function(){try{
var id=localStorage.getItem('vyzorix.theme');
var follow=localStorage.getItem('vyzorix.theme.followSystem')==='true';
if(follow){id=(window.matchMedia&&window.matchMedia('(prefers-color-scheme: dark)').matches)?'dark':'light';}
var mode=(${JSON.stringify(DARK_THEME_IDS)}).indexOf(id)>=0?'dark':'light';
document.documentElement.style.colorScheme=mode;
document.documentElement.style.background=mode==='dark'?'#161616':'#ffffff';
}catch(e){}})();`;

const BOOT_MIN_MS = 1700;
const BOOT_SHOWN_KEY = 'vyzorix.booted';
const BOOT_GIF = '/brand/vyzorix_boot_assemble.gif';

const BOOT_MARKUP = `<style>
#vyzorix-boot{position:fixed;inset:0;display:flex;align-items:center;justify-content:center;background:#161616;z-index:9999}
#vyzorix-boot img{width:min(440px,72vw);height:auto;display:block}
</style>
<div id="vyzorix-boot" role="status" aria-label="Loading">
<img src="${BOOT_GIF}" alt="" aria-hidden="true"/>
</div>`;

function prefersReducedMotion(): boolean {
  if (typeof window === 'undefined') return false;
  try {
    const raw = window.localStorage.getItem('vyzorix-preferences');
    if (raw) {
      const parsed = JSON.parse(raw) as { state?: { reduceMotion?: boolean } };
      if (parsed?.state?.reduceMotion === true) return true;
    }
  } catch {
    void 0;
  }
  return typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

function BootGate({ children }: { children: React.ReactNode }) {
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let alreadyShown = false;
    try {
      alreadyShown = window.sessionStorage.getItem(BOOT_SHOWN_KEY) === 'true';
    } catch {
      void 0;
    }

    if (alreadyShown || prefersReducedMotion()) {
      setReady(true);
      return;
    }

    try {
      window.sessionStorage.setItem(BOOT_SHOWN_KEY, 'true');
    } catch {
      void 0;
    }

    const id = window.setTimeout(() => setReady(true), BOOT_MIN_MS);
    return () => window.clearTimeout(id);
  }, []);

  return (
    <>
      {children}
      {!ready ? <div dangerouslySetInnerHTML={{ __html: BOOT_MARKUP }} /> : null}
    </>
  );
}

function Shell() {
  const sessionChecked = useAuthStore((s) => s.sessionChecked);
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  if (!sessionChecked) return null;

  if (!isAuthenticated) return <LoginPage />;

  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}

function RootComponent() {
  useSessionBootstrap();
  useWebSocketConnection();
  useInitConnectivity();

  return (
    <QueryProvider>
      <BrandingProvider settings={brandingSettings}>
        <ThemeProvider>
          <OverlayProvider mode="sheet">
            <VyzorI18nProvider>
              <VyzorAnalyticsProvider>
                <ToastProvider>
                  <VyzorErrorBoundary>
                    <BootGate>
                      <Shell />
                      <VyzorConsentBanner />
                    </BootGate>
                  </VyzorErrorBoundary>
                </ToastProvider>
              </VyzorAnalyticsProvider>
            </VyzorI18nProvider>
          </OverlayProvider>
        </ThemeProvider>
      </BrandingProvider>
    </QueryProvider>
  );
}

function RootDocument({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <meta name="color-scheme" content="dark light" />
        {}
        <link rel="icon" type="image/svg+xml" href="/brand/vyzorix_icon_dark.svg" />
        <link rel="icon" type="image/png" sizes="32x32" href={branding.favIcon} />
        <link rel="icon" type="image/png" sizes="16x16" href="/brand/favicon-16.png" />
        <link rel="apple-touch-icon" sizes="180x180" href={branding.appleTouchIcon} />
        <link rel="mask-icon" href={branding.maskIcon} color={branding.maskIconColor} />
        <meta name="theme-color" content={branding.maskIconColor} />
        <title>{branding.appTitle}</title>
        <script dangerouslySetInnerHTML={{ __html: ANTI_FLASH }} />
        <HeadContent />
      </head>
      <body>
        {children}
        <Scripts />
      </body>
    </html>
  );
}

export const Route = createRootRoute({
  component: RootComponent,
  shellComponent: RootDocument,
  errorComponent: ({ error }) => <VyzorRouteError error={error} />,
});
