import { useGetDeployWhoami } from '@/generated-rq/deploy/deploy';
import { deployIdentityFromWire, type DeployIdentity } from '@vyzorix/api-client';

export function useDeployIdentity(device?: string) {
  return useGetDeployWhoami<DeployIdentity>(
    { device },
    { query: { select: deployIdentityFromWire } },
  );
}
