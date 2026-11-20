import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { AdminUserDetailPage } from '@/ui/pages/admin-users/admin-user-detail-page';

function AdminUserDetailRoute() {
  const { userId } = Route.useParams();
  const navigate = useNavigate();

  return (
    <AdminUserDetailPage
      userId={userId}
      onBack={() => void navigate({ to: '/admin/users' })}
    />
  );
}

export const Route = createFileRoute('/admin/users/$userId')({
  component: AdminUserDetailRoute,
});
