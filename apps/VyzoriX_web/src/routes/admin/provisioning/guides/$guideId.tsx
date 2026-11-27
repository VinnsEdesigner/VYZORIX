import { createFileRoute, notFound } from '@tanstack/react-router';
import { EmptyState } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';
import { useCapabilities } from '@/hooks/_shared/use-capabilities';
import { guideById, isGuideId } from '@/lib/provisioning/guides';
import { GuidePage } from '@/ui/pages/admin-provisioning/guide-page';

function GuideRoute() {
  const { guideId } = Route.useParams();
  const capabilities = useCapabilities();
  const guide = guideById(guideId);

  if (!guide) throw notFound();

  if (!capabilities.isLoading && !capabilities.can('provisioning.read')) {
    return (
      <Page title="Guides" subTitle="Provisioning">
        <EmptyState
          icon="lock"
          message="Provisioning is for administrators. Reading a guide is harmless, but the section it describes is not readable without admin authority."
        />
      </Page>
    );
  }

  return (
    <Page title={guide.title} subTitle="Provisioning guide">
      <GuidePage guideId={guide.id} />
    </Page>
  );
}

export const Route = createFileRoute('/admin/provisioning/guides/$guideId')({
  params: {
    parse: (params) => {
      if (!isGuideId(params.guideId)) throw notFound();
      return { guideId: params.guideId };
    },
  },
  component: GuideRoute,
});
