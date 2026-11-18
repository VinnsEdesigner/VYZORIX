import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';

function PermissionsRoute() {
  return (
    <Page title="Permissions" subTitle="Members, roles and audit">
      <EmptyState message="Permissions is being built." icon="shield" />
    </Page>
  );
}

export const Route = createFileRoute('/permissions/')({
  component: PermissionsRoute,
});
