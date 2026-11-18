import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';

function UpdatesRoute() {
  return (
    <Page title="Updates" subTitle="Versions and push history">
      <EmptyState message="Updates is being built." icon="download" />
    </Page>
  );
}

export const Route = createFileRoute('/updates/')({
  component: UpdatesRoute,
});
