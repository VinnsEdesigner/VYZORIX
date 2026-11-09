import { useGetDeployWhoami } from '@/generated-rq/deploy/deploy';
import { deployIdentityFromWire, type DeployIdentity } from '@vyzorix/api-client';

/**
 * The operator's identity on the deploy surface, optionally scoped to a device.
 * Backed by `GET /deploy/whoami`.
 */
export function useDeployIdentity(device?: string) {
  return useGetDeployWhoami<DeployIdentity>(
    { device },
    { query: { select: deployIdentityFromWire } },
  );
}
