import { createRouter } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { routeTree } from './routeTree.gen';
import { Page } from './ui/components/layout';

function NotFound() {
  return (
    <Page title="Not found" subTitle="That page does not exist">
      <EmptyState message="The page you were looking for is not here." icon="alert-triangle" />
    </Page>
  );
}

export function getRouter() {
  return createRouter({
    routeTree,
    defaultPreload: 'intent',
    defaultNotFoundComponent: NotFound,
  });
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof getRouter>;
  }
}
