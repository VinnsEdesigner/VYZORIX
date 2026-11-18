import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';

function AppsRoute() {
  return (
    <Page title="Apps" subTitle="Deployed apps and the catalog">
      <EmptyState message="Apps is being built." icon="box" />
    </Page>
  );
}

export const Route = createFileRoute('/apps/')({
  component: AppsRoute,
});
