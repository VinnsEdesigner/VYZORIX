import { useCurrentOrganizationId } from '@/hooks/_shared/use-current-context';
import { useGetDeployImeiDocument } from '@/generated-rq/deploy/deploy';

export function useDeviceDeployDocument(imei: string | undefined) {
  const organizationId = useCurrentOrganizationId();
  return useGetDeployImeiDocument(imei ?? '', {
    query: {
      enabled: imei !== undefined && imei !== '' && organizationId !== null,
    },
  });
}
