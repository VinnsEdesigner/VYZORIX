import { HeadContent, Outlet, Scripts, createRootRoute } from '@tanstack/react-router';
import { BrandingProvider, resolveBranding } from '@vyzorix/ui';
import { QueryProvider } from '@/providers/query-provider';
import { ThemeProvider } from '@/providers/theme-provider';
import { VyzorI18nProvider } from '@/providers/vyzor-i18n-provider';
import { VyzorAnalyticsProvider } from '@/providers/vyzor-analytics-provider';
import { useWebSocketConnection } from '@/hooks/realtime/use-realtime';
import { useInitConnectivity } from '@/hooks/connectivity/use-connectivity';
import { VyzorErrorBoundary } from '@/ui/components/error/vyzor-error-boundary';
import { VyzorRouteError } from '@/ui/components/error/vyzor-route-error';
import { VyzorConsentBanner } from '@/ui/components/analytics/vyzor-consent-banner';
import { brandingSettings } from '@/lib/branding';

const branding = resolveBranding(brandingSettings);

/**
 * Pre-hydration theme hint.
 *
 * The theme is a JS object injected by ThemeProvider, so the very first frame
 * (before React mounts) has no styling yet. This sets `color-scheme` on the
 * document from the cached theme id so the browser paints the correct default
 * background and form-control colours instead of a white flash. It reads only
 * the localStorage mirror and validates against the known ids.
 */
const DARK_THEME_IDS = ['dark', 'burgundy', 'matrix', 'tron', 'synthwave'];
const ANTI_FLASH = `(function(){try{
var id=localStorage.getItem('vyzorix.theme');
var follow=localStorage.getItem('vyzorix.theme.followSystem')==='true';
if(follow){id=(window.matchMedia&&window.matchMedia('(prefers-color-scheme: dark)').matches)?'dark':'light';}
var mode=(${JSON.stringify(DARK_THEME_IDS)}).indexOf(id)>=0?'dark':'light';
document.documentElement.style.colorScheme=mode;
document.documentElement.style.background=mode==='dark'?'#0d0d0d':'#ffffff';
}catch(e){}})();`;

function RootComponent() {
  useWebSocketConnection();
  useInitConnectivity();

  return (
    <QueryProvider>
      <BrandingProvider settings={brandingSettings}>
        <ThemeProvider>
          <VyzorI18nProvider>
            <VyzorAnalyticsProvider>
              <VyzorErrorBoundary>
                <Outlet />
              </VyzorErrorBoundary>
              <VyzorConsentBanner />
            </VyzorAnalyticsProvider>
          </VyzorI18nProvider>
        </ThemeProvider>
      </BrandingProvider>
    </QueryProvider>
  );
}

/**
 * The document shell. Without it the server renders a fragment with no <html>,
 * <head> or <body>, and a browser receiving that gets an incomplete page.
 */
function RootDocument({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <meta name="color-scheme" content="dark light" />
        {/* Brand icons. A deployment overrides these through branding settings;
            the defaults point at the assets in `public/brand`. */}
        <link rel="icon" type="image/svg+xml" href="/brand/vyzorix_icon.svg" />
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
