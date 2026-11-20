import { createFileRoute } from '@tanstack/react-router';
import { AdminServiceAccountsPage } from '@/ui/pages/admin-service-accounts/admin-service-accounts-page';

export const Route = createFileRoute('/admin/service-accounts/')({
  component: AdminServiceAccountsPage,
});
