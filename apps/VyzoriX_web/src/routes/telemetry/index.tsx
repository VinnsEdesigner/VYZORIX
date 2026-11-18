import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';

function TelemetryRoute() {
  return (
    <Page title="Telemetry" subTitle="Metrics collected from the fleet">
      <EmptyState message="Telemetry is being built." icon="activity" />
    </Page>
  );
}

export const Route = createFileRoute('/telemetry/')({
  component: TelemetryRoute,
});
