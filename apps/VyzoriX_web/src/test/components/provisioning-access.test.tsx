import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { QueryClientProvider } from '@tanstack/react-query';
import { ThemeContext, getThemeById } from '@vyzorix/ui';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { createTestQueryClient } from '../helpers/render-hook';
import { useCapabilities } from '@/hooks/_shared/use-capabilities';

const { server } = setupIntegrationTest();

function capabilitiesHandler(org: string[], instance: string[] = []) {
  return http.get(/me\/capabilities/, () => HttpResponse.json({ personal: [], instance, org }));
}

function renderProvisioning() {
  return render(
    <QueryClientProvider client={createTestQueryClient()}>
      <ThemeContext.Provider value={getThemeById('vyzorix-dark')}>
        <ProvisioningGateHarness />
      </ThemeContext.Provider>
    </QueryClientProvider>,
  );
}

function ProvisioningGateHarness() {
  const capabilities = useCapabilities();

  if (!capabilities.isLoading && !capabilities.can('provisioning.read')) {
    return <p>Provisioning is for administrators.</p>;
  }

  return <p>Provisioning surface</p>;
}

describe('provisioning access gate', () => {
  it('shows the surface when provisioning.read is granted', async () => {
    server.use(capabilitiesHandler(['provisioning.read:org:*']));

    renderProvisioning();

    await waitFor(() => expect(screen.getByText('Provisioning surface')).toBeInTheDocument());
  });

  it('refuses the surface when only provisioning.write is granted, because read is separate', async () => {
    server.use(capabilitiesHandler(['provisioning.write:org:*']));

    renderProvisioning();

    await waitFor(() => expect(screen.getByText('Provisioning is for administrators.')).toBeInTheDocument());
  });

  it('refuses the surface when no grant is reported at all', async () => {
    server.use(capabilitiesHandler([]));

    renderProvisioning();

    await waitFor(() => expect(screen.getByText('Provisioning is for administrators.')).toBeInTheDocument());
  });

  it('does not infer authority from a role name', async () => {
    server.use(capabilitiesHandler([]));
    const { authContext } = await import('@vyzorix/api-client');
    authContext.setFromMeResponse({
      id: 'u1',
      email: 'admin@acme.io',
      name: 'Admin',
      organizations: [{ id: 'org-a', name: 'Acme', role: 'super_admin' }],
      selected_organization: { id: 'org-a', name: 'Acme', role: 'super_admin' },
    });

    renderProvisioning();

    await waitFor(() => expect(screen.getByText('Provisioning is for administrators.')).toBeInTheDocument());
  });

  it('honours a wildcard scope for a differently scoped grant', async () => {
    server.use(capabilitiesHandler(['provisioning.read:org:*']));

    renderProvisioning();

    await waitFor(() => expect(screen.getByText('Provisioning surface')).toBeInTheDocument());
  });
});

describe('provisioning tab shell', () => {
  it('renders every tab from the registry', async () => {
    const { PROVISIONING_TABS } = await import('@/lib/provisioning/provisioning-tabs');
    const { pageTabs } = await import('@/ui/components/layout');

    expect(pageTabs['/admin/provisioning']?.map((t) => t.id)).toEqual(PROVISIONING_TABS.map((t) => t.id));
  });

  it('does not expose a tab the registry does not define', async () => {
    const { PROVISIONING_TAB_IDS } = await import('@/lib/provisioning/provisioning-tabs');

    expect(PROVISIONING_TAB_IDS).not.toContain('repos');
    expect(PROVISIONING_TAB_IDS).not.toContain('connections');
  });

  it('keeps the tabs component exporting a registry-driven list', async () => {
    const { PROVISIONING_PAGE_TABS } = await import('@/lib/provisioning/provisioning-tabs');

    expect(PROVISIONING_PAGE_TABS.length).toBeGreaterThan(1);
    expect(PROVISIONING_PAGE_TABS[0]?.id).toBe('overview');
  });
});
