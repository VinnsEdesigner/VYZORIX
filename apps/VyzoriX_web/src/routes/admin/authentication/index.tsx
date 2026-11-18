import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';

function AuthenticationRoute() {
  return (
    <Page title="Authentication" subTitle="Sign-in methods and policy">
      <EmptyState message="Authentication is being built." icon="log-in" />
    </Page>
  );
}

export const Route = createFileRoute('/admin/authentication/')({
  component: AuthenticationRoute,
});
