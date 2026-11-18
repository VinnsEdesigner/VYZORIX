import { useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { getAuthMe, usePostAuthOrganizationsSelect } from '@/generated-rq/auth/auth-session';
import type { MeResult, OrganizationInfo, SelectOrganizationResult } from '@vyzorix/api-client';
import { useAuthStore } from '@/stores/auth-store';

export function useMe() {
  const setFromMeResponse = useAuthStore((s) => s.setFromMeResponse);
  return {
    data: undefined as MeResult | undefined,

    refetch: useCallback(async () => {
      try {
        const me = await getAuthMe();
        setFromMeResponse(me);
        return me;
      } catch {
        return null;
      }
    }, [setFromMeResponse]),
  };
}

export function useMyOrganizations() {
  return useQueryClient().getQueryData<{ organizations: OrganizationInfo[] }>(['me', 'organizations']) ?? { organizations: [] };
}

export function useSelectOrganization() {
  const setOrganization = useAuthStore((s) => s.setOrganization);
  return usePostAuthOrganizationsSelect({
    mutation: {
      onSuccess: (org) => {

        setOrganization((org as SelectOrganizationResult).organization_id ?? null);
      },
    },
  });
}
