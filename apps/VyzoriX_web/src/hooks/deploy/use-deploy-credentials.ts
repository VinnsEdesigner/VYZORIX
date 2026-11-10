import { useQueryClient } from '@tanstack/react-query';
import { useCurrentOrganizationId } from '@/hooks/_shared/use-current-context';
import {
  useGetDeployImeiCredentials,
  usePostDeployImeiCredentials,
  useDeleteDeployImeiCredentialsId,
  invalidateGetDeployImei,
  invalidateGetDeployImeiCredentials,
} from '@/generated-rq/deploy/deploy';
import { deployCredentialsFromWire, type DeployCredential } from '@vyzorix/api-client';

/**
 * A device's deploy credentials. Names, prefixes and revocation state only —
 * a token is never returned by a GET.
 */
export function useDeployCredentials(imei: string | undefined) {
  const organizationId = useCurrentOrganizationId();
  return useGetDeployImeiCredentials<DeployCredential[]>(imei ?? '', {
    query: {
      enabled: imei !== undefined && imei !== '' && organizationId !== null,
      select: deployCredentialsFromWire,
    },
  });
}

/**
 * Mint a device-scoped credential. The response is the only place the token
 * appears; the caller is responsible for showing it exactly once.
 */
export function useMintDeployCredential(imei: string) {
  const queryClient = useQueryClient();
  return usePostDeployImeiCredentials({
    mutation: {
      onSuccess: () => {
        void invalidateGetDeployImeiCredentials(queryClient, imei);
        void invalidateGetDeployImei(queryClient, imei);
      },
    },
  });
}

/** Revoke a credential; a revoked credential is refused on its next use. */
export function useRevokeDeployCredential(imei: string) {
  const queryClient = useQueryClient();
  return useDeleteDeployImeiCredentialsId({
    mutation: {
      onSuccess: () => {
        void invalidateGetDeployImeiCredentials(queryClient, imei);
        void invalidateGetDeployImei(queryClient, imei);
      },
    },
  });
}
