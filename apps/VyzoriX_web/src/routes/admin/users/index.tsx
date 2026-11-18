import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';

function UsersRoute() {
  return (
    <Page title="Users" subTitle="Everyone in this organisation">
      <EmptyState message="Users is being built." icon="users" />
    </Page>
  );
}

export const Route = createFileRoute('/admin/users/')({
  component: UsersRoute,
});
