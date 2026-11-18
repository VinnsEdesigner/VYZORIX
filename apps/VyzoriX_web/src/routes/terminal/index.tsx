import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';

function TerminalRoute() {
  return (
    <Page title="Terminal" subTitle="A shell on a device">
      <EmptyState message="Terminal is being built." icon="terminal" />
    </Page>
  );
}

export const Route = createFileRoute('/terminal/')({
  component: TerminalRoute,
});
