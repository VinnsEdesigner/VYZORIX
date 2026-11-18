import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';

function HomeRoute() {
  return (
    <Page title="Good afternoon." subTitle="Welcome to the fleet console.">
      <EmptyState
        message="The home dashboards are being built. Recent dashboards, firing alerts and quick actions will appear here."
        icon="dashboard"
      />
    </Page>
  );
}

export const Route = createFileRoute('/')({
  component: HomeRoute,
});
