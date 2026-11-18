import { createFileRoute } from '@tanstack/react-router';
import { Page } from '@/ui/components/layout';
import { EmailChannelPage } from '@/ui/pages/profile/email-channel-page';

function EmailChannelRoute() {
  return (
    <Page title="Email notifications" subTitle="Where your notification email is sent">
      <EmailChannelPage />
    </Page>
  );
}

export const Route = createFileRoute('/profile/notifications/email')({
  component: EmailChannelRoute,
});
