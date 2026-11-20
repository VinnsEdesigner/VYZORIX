import { describe, it, expect } from 'vitest';
import { http, HttpResponse } from 'msw';
import { waitFor } from '@testing-library/react';
import { renderHookWithQueryClient } from '../helpers/render-hook';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { authContext } from '@vyzorix/api-client';
import { useCapabilities } from '@/hooks/_shared/use-capabilities';

const { server } = setupIntegrationTest();

function capabilitiesHandler(org: string[], instance: string[] = []) {
  return http.get(/me\/capabilities/, () =>
    HttpResponse.json({ personal: [], instance, org }),
  );
}

describe('useCapabilities', () => {
  it('admits members.write when the server reports it, and refuses it when it does not', async () => {
    server.use(capabilitiesHandler(['members.read:org:*', 'members.write:org:*']));

    const { result } = renderHookWithQueryClient(() => useCapabilities());
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.can('members.write')).toBe(true);
    expect(result.current.can('members.delete')).toBe(false);
  });

  it('never reads authority from a role name: no grants means no permission', async () => {
    server.use(capabilitiesHandler([]));
    authContext.setFromMeResponse({
      id: 'u1',
      email: 'admin@acme.io',
      name: 'Admin',
      organizations: [{ id: 'org-a', name: 'Acme', role: 'super_admin' }],
      selected_organization: { id: 'org-a', name: 'Acme', role: 'super_admin' },
    });

    const { result } = renderHookWithQueryClient(() => useCapabilities());
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.myRole).toBe('super_admin');
    expect(result.current.can('members.write')).toBe(false);
    expect(result.current.canManageMember('viewer')).toBe(false);
  });

  it('requires a strictly higher role to manage a member', async () => {
    server.use(capabilitiesHandler(['members.write:org:*']));
    authContext.setFromMeResponse({
      id: 'u1',
      email: 'admin@acme.io',
      name: 'Admin',
      organizations: [{ id: 'org-a', name: 'Acme', role: 'admin' }],
      selected_organization: { id: 'org-a', name: 'Acme', role: 'admin' },
    });

    const { result } = renderHookWithQueryClient(() => useCapabilities());
    await waitFor(() => expect(result.current.can('members.write')).toBe(true));

    expect(result.current.canManageMember('viewer')).toBe(true);
    expect(result.current.canManageMember('operator')).toBe(true);
    expect(result.current.canManageMember('admin')).toBe(false);
    expect(result.current.canManageMember('super_admin')).toBe(false);
  });
});
