import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { AdminServiceAccountDetailPage } from '@/ui/pages/admin-service-accounts/admin-service-account-detail-page';

function AdminServiceAccountDetailRoute() {
  const { id } = Route.useParams();
  const navigate = useNavigate();

  return (
    <AdminServiceAccountDetailPage
      accountId={id}
      onBack={() => void navigate({ to: '/admin/service-accounts' })}
    />
  );
}

export const Route = createFileRoute('/admin/service-accounts/$id')({
  component: AdminServiceAccountDetailRoute,
});
