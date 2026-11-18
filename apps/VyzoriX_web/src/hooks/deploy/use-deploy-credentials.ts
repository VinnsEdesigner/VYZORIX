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

export function useDeployCredentials(imei: string | undefined) {
  const organizationId = useCurrentOrganizationId();
  return useGetDeployImeiCredentials<DeployCredential[]>(imei ?? '', {
    query: {
      enabled: imei !== undefined && imei !== '' && organizationId !== null,
      select: deployCredentialsFromWire,
    },
  });
}

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
