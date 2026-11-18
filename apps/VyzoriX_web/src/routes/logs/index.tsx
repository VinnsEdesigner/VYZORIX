import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';

function LogsRoute() {
  return (
    <Page title="Logs" subTitle="The device log stream">
      <EmptyState message="Logs is being built." icon="file-text" />
    </Page>
  );
}

export const Route = createFileRoute('/logs/')({
  component: LogsRoute,
});
