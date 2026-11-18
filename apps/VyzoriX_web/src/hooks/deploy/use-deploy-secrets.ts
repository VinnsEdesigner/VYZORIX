import { useQueryClient } from '@tanstack/react-query';
import { useCurrentOrganizationId } from '@/hooks/_shared/use-current-context';
import {
  useGetDeployImeiSecretsApp,
  usePostDeployImeiSecretsApp,
  useDeleteDeployImeiSecretsAppKey,
  invalidateGetDeployImeiSecretsApp,
  invalidateGetDeployImeiDocument,
} from '@/generated-rq/deploy/deploy';
import { deployAppSecretsFromWire, type DeployAppSecrets } from '@vyzorix/api-client';

export function useDeployAppSecrets(imei: string | undefined, app: string | undefined) {
  const organizationId = useCurrentOrganizationId();
  return useGetDeployImeiSecretsApp<DeployAppSecrets>(imei ?? '', app ?? '', {
    query: {
      enabled:
        imei !== undefined &&
        imei !== '' &&
        app !== undefined &&
        app !== '' &&
        organizationId !== null,
      select: deployAppSecretsFromWire,
    },
  });
}

export function useSetDeploySecret(imei: string, app: string) {
  const queryClient = useQueryClient();
  return usePostDeployImeiSecretsApp({
    mutation: {
      onSuccess: () => {
        void invalidateGetDeployImeiSecretsApp(queryClient, imei, app);
        void invalidateGetDeployImeiDocument(queryClient, imei);
      },
    },
  });
}

export function useRemoveDeploySecret(imei: string, app: string) {
  const queryClient = useQueryClient();
  return useDeleteDeployImeiSecretsAppKey({
    mutation: {
      onSuccess: () => {
        void invalidateGetDeployImeiSecretsApp(queryClient, imei, app);
        void invalidateGetDeployImeiDocument(queryClient, imei);
      },
    },
  });
}
