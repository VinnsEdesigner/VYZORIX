import type { ComponentType } from 'react';
import { createFileRoute } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';
import { useCapabilities } from '@/hooks/_shared/use-capabilities';
import { resolveProvisioningTab } from '@/lib/provisioning/provisioning-tabs';
import { OverviewSurface } from '@/ui/pages/admin-provisioning/overview-surface';
import { ProvidersSurface } from '@/ui/pages/admin-provisioning/providers-surface';
import { SourcesSurface } from '@/ui/pages/admin-provisioning/sources-surface';
import { CodeSurface } from '@/ui/pages/admin-provisioning/code-surface';
import { CredentialsSurface } from '@/ui/pages/admin-provisioning/credentials-surface';
import { ResourcesSurface } from '@/ui/pages/admin-provisioning/resources-surface';
import { ActivitySurface } from '@/ui/pages/admin-provisioning/activity-surface';
import { SchedulerSurface } from '@/ui/pages/admin-provisioning/scheduler-surface';
import { GettingStartedSurface } from '@/ui/pages/admin-provisioning/getting-started-surface';

interface ProvisioningSearch {
  tab?: string;
}

interface SurfaceProps {
  canWrite: boolean;
}

const SURFACES: Record<string, ComponentType<SurfaceProps>> = {
  overview: OverviewSurface,
  providers: ProvidersSurface,
  sources: SourcesSurface,
  code: CodeSurface,
  credentials: CredentialsSurface,
  resources: ResourcesSurface,
  activity: ActivitySurface,
  scheduler: SchedulerSurface,
  'getting-started': GettingStartedSurface,
};

function ProvisioningRoute() {
  const { tab } = Route.useSearch();
  const active = resolveProvisioningTab(tab);
  const capabilities = useCapabilities();

  if (!capabilities.isLoading && !capabilities.can('provisioning.read')) {
    return (
      <Page title="Provisioning" subTitle="Keep your device configuration in a git repository">
        <EmptyState
          icon="lock"
          message="Provisioning is for administrators. A provisioned source can rewrite any device's configuration, so reading it needs admin authority."
        />
      </Page>
    );
  }

  const Surface = SURFACES[active.id] ?? OverviewSurface;

  return (
    <Page title="Provisioning" subTitle={active.subTitle || 'Keep your device configuration in a git repository'}>
      <Surface canWrite={capabilities.can('provisioning.write')} />
    </Page>
  );
}

export const Route = createFileRoute('/admin/provisioning/')({
  validateSearch: (search: Record<string, unknown>): ProvisioningSearch => ({
    tab: typeof search.tab === 'string' ? search.tab : undefined,
  }),
  component: ProvisioningRoute,
});
