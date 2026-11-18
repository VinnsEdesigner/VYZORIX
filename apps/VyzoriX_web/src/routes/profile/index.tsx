import { createFileRoute } from '@tanstack/react-router';
import { Page } from '@/ui/components/layout';
import { ProfilePage } from '@/ui/pages/profile/profile-page';
import { PreferencesTab } from '@/ui/pages/profile/preferences-tab';
import { NotificationsTab } from '@/ui/pages/profile/notifications-tab';
import { SecurityTab } from '@/ui/pages/profile/security-tab';

interface ProfileSearch {
  tab?: string;
}

const TABS = ['profile', 'preferences', 'notifications', 'security'] as const;
type ProfileTab = (typeof TABS)[number];

const TAB_META: Record<ProfileTab, { title: string; subTitle: string }> = {
  profile: { title: 'Profile', subTitle: 'Your account and activity' },
  preferences: { title: 'Preferences', subTitle: 'Your personal settings' },
  notifications: { title: 'Notifications', subTitle: 'What you are told about, and where' },
  security: { title: 'Security', subTitle: 'Sessions, verification and access' },
};

function isProfileTab(value: string | undefined): value is ProfileTab {
  return value !== undefined && (TABS as readonly string[]).includes(value);
}

function ProfileRoute() {
  const { tab } = Route.useSearch();
  const active: ProfileTab = isProfileTab(tab) ? tab : 'profile';
  const meta = TAB_META[active];

  return (
    <Page title={meta.title} subTitle={meta.subTitle}>
      {active === 'preferences' ? <PreferencesTab /> : null}
      {active === 'notifications' ? <NotificationsTab /> : null}
      {active === 'security' ? <SecurityTab /> : null}
      {active === 'profile' ? <ProfilePage /> : null}
    </Page>
  );
}

export const Route = createFileRoute('/profile/')({
  validateSearch: (search: Record<string, unknown>): ProfileSearch => ({
    tab: typeof search.tab === 'string' ? search.tab : undefined,
  }),
  component: ProfileRoute,
});
