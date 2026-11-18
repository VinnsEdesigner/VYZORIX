import { createFileRoute } from '@tanstack/react-router';
import { Page } from '@/ui/components/layout';
import { AvatarChooserPage } from '@/ui/pages/profile/avatar-chooser-page';

function AvatarRoute() {
  return (
    <Page title="Avatar" subTitle="Pick a look for your profile">
      <AvatarChooserPage />
    </Page>
  );
}

export const Route = createFileRoute('/profile/avatar')({
  component: AvatarRoute,
});
