import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';

function SettingsRoute() {
  return (
    <Page title="Settings" subTitle="Organisation settings">
      <EmptyState message="Settings is being built." icon="cog" />
    </Page>
  );
}

export const Route = createFileRoute('/settings/')({
  component: SettingsRoute,
});
