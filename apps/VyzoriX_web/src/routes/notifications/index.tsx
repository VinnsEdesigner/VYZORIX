import { createFileRoute } from '@tanstack/react-router';
import { Page } from '@/ui/components/layout';
import { NotificationHistoryPage } from '@/ui/pages/profile/notification-history-page';

function NotificationsRoute() {
  return (
    <Page
      title="Notification history"
      subTitle="Every notification sent to you, and whether it was delivered"
    >
      <NotificationHistoryPage />
    </Page>
  );
}

export const Route = createFileRoute('/notifications/')({
  component: NotificationsRoute,
});
