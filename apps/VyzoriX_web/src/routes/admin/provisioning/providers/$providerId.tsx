import { createFileRoute, notFound } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';
import { useCapabilities } from '@/hooks/_shared/use-capabilities';
import { isProviderId, providerById } from '@/lib/provisioning/providers';
import { ProviderDetail } from '@/ui/pages/admin-provisioning/provider-detail';

function ProviderRoute() {
  const { providerId } = Route.useParams();
  const capabilities = useCapabilities();
  const provider = providerById(providerId);

  if (!provider) throw notFound();

  if (!capabilities.isLoading && !capabilities.can('provisioning.read')) {
    return (
      <Page title={provider.name} subTitle={provider.host}>
        <EmptyState
          icon="lock"
          message="Provisioning is for administrators. A provider grant can read your repositories, so configuring one needs admin authority."
        />
      </Page>
    );
  }

  return (
    <Page title={`${provider.name} provider`} subTitle="Authorise VYZORIX to read from this host">
      <ProviderDetail providerId={provider.id} canWrite={capabilities.can('provisioning.write')} />
    </Page>
  );
}

export const Route = createFileRoute('/admin/provisioning/providers/$providerId')({
  params: {
    parse: (params) => {
      if (!isProviderId(params.providerId)) throw notFound();
      return { providerId: params.providerId };
    },
  },
  component: ProviderRoute,
});
