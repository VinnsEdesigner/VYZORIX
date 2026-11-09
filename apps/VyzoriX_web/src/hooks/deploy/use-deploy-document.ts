import { useCurrentOrganizationId } from '@/hooks/_shared/use-current-context';
import { useGetDeployImeiDocument } from '@/generated-rq/deploy/deploy';

/**
 * The signed config document as raw `vyzorix.v` text — what the device would
 * verify. Distinct from `useDeviceDeployConfig`, which returns the structured
 * snapshot the server wraps around it.
 */
export function useDeviceDeployDocument(imei: string | undefined) {
  const organizationId = useCurrentOrganizationId();
  return useGetDeployImeiDocument(imei ?? '', {
    query: {
      enabled: imei !== undefined && imei !== '' && organizationId !== null,
    },
  });
}
