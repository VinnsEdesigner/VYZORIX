import { createFileRoute } from '@tanstack/react-router';
import { Card } from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';
import { PreferencesForm } from '@/ui/components/preferences';
import { useOrgPreferences, useUpdateOrgPreferences } from '@/hooks/preferences';

function DefaultPreferencesRoute() {
  const { preferences, isLoading, isError } = useOrgPreferences();
  const { save, isSaving, isError: isSaveError } = useUpdateOrgPreferences();

  return (
    <Page
      title="Organisation defaults"
      subTitle="The defaults every member inherits. A member's own preference wins; anything they leave unset falls back to these, and anything left unset here falls back to the system default. Changes apply to the whole organisation."
    >
      <Card heading="Defaults">
        <PreferencesForm
          scope="org"
          preferences={preferences}
          isLoading={isLoading}
          isSaving={isSaving}
          isError={isError}
          isSaveError={isSaveError}
          onSave={save}
          savedMessage="Every member now inherits these defaults until they set their own."
        />
      </Card>
    </Page>
  );
}

export const Route = createFileRoute('/admin/default-preferences/')({
  component: DefaultPreferencesRoute,
});
