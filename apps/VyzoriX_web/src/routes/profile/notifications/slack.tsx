import { createFileRoute } from '@tanstack/react-router';
import { Page } from '@/ui/components/layout';
import { SlackChannelPage } from '@/ui/pages/profile/slack-channel-page';

function SlackChannelRoute() {
  return (
    <Page title="Slack notifications" subTitle="Link a workspace and pick a channel">
      <SlackChannelPage />
    </Page>
  );
}

export const Route = createFileRoute('/profile/notifications/slack')({
  component: SlackChannelRoute,
});
