import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';

function ProvisioningRoute() {
  return (
    <Page title="Provisioning" subTitle="Repositories and connections">
      <EmptyState message="Provisioning is being built." icon="download" />
    </Page>
  );
}

export const Route = createFileRoute('/admin/provisioning/')({
  component: ProvisioningRoute,
});
