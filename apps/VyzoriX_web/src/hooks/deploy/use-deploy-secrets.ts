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

/**
 * The secret *names* an app expects and their generation. Never a value — a
 * secret value is encrypted to the device and is not readable from the console
 * (contract, US-010).
 */
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

/**
 * Set an app secret. The value is encrypted to the device's key server-side and
 * is never echoed back; the config only gains the key *name*, so the document
 * query is invalidated too.
 */
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

/** Remove an app secret. */
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
