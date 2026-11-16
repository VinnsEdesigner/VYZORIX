import { HeadContent, Outlet, Scripts, createRootRoute } from '@tanstack/react-router';
import { QueryProvider } from '@/providers/query-provider';
import { VyzorI18nProvider } from '@/providers/vyzor-i18n-provider';
import { VyzorAnalyticsProvider } from '@/providers/vyzor-analytics-provider';
import { useWebSocketConnection } from '@/hooks/realtime/use-realtime';
import { useInitConnectivity } from '@/hooks/connectivity/use-connectivity';
import { VyzorErrorBoundary } from '@/ui/components/error/vyzor-error-boundary';
import { VyzorRouteError } from '@/ui/components/error/vyzor-route-error';
import { VyzorConsentBanner } from '@/ui/components/analytics/vyzor-consent-banner';

function RootComponent() {
  useWebSocketConnection();
  useInitConnectivity();

  return (
    <QueryProvider>
      <VyzorI18nProvider>
        <VyzorAnalyticsProvider>
          <VyzorErrorBoundary>
            <Outlet />
          </VyzorErrorBoundary>
          <VyzorConsentBanner />
        </VyzorAnalyticsProvider>
      </VyzorI18nProvider>
    </QueryProvider>
  );
}

/**
 * The document shell. Without it the server renders a fragment with no <html>,
 * <head> or <body>, and a browser receiving that gets an incomplete page — the
 * hydration script runs but the document has no title, no meta and no root
 * element for the client to mount into.
 */
function RootDocument({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <meta name="color-scheme" content="dark light" />
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
