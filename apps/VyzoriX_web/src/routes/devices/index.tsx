import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';

function DevicesRoute() {
  return (
    <Page title="Devices" subTitle="Your fleet, one card per device">
      <EmptyState message="Devices is being built." icon="smartphone" />
    </Page>
  );
}

export const Route = createFileRoute('/devices/')({
  component: DevicesRoute,
});
