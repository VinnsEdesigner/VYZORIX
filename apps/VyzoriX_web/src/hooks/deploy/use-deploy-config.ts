import { useCurrentOrganizationId } from '@/hooks/_shared/use-current-context';
import { useGetDeployImei } from '@/generated-rq/deploy/deploy';
import { deviceConfigFromWire, type DeviceConfigSnapshot } from '@vyzorix/api-client';

export function useDeviceDeployConfig(imei: string | undefined) {
  const organizationId = useCurrentOrganizationId();
  return useGetDeployImei<DeviceConfigSnapshot>(imei ?? '', {
    query: {
      enabled: imei !== undefined && imei !== '' && organizationId !== null,
      select: deviceConfigFromWire,
    },
  });
}
