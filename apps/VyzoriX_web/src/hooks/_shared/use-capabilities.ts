import { useAuthStore } from '@/stores';
import { useGetMeCapabilities } from '@/generated-rq/me/me';

export const ORG_ROLE_LEVEL: Record<string, number> = {
  viewer: 1,
  operator: 2,
  admin: 3,
  super_admin: 4,
};

export interface Capabilities {
  grants: ReadonlySet<string>;
  myRole: string | undefined;
  can: (action: string, scope?: string) => boolean;
  canManageMember: (targetRole: string | undefined) => boolean;
  isLoading: boolean;
}

function matchesGrant(grant: string, action: string, scope: string): boolean {
  const separator = grant.indexOf(':');
  if (separator < 0) return false;

  const grantAction = grant.slice(0, separator);
  const grantScope = grant.slice(separator + 1);
  if (grantAction !== action || !grantScope) return false;
  if (grantScope === scope) return true;

  return grantScope.endsWith('*') && scope.startsWith(grantScope.slice(0, -1));
}

export function useCapabilities(): Capabilities {
  const query = useGetMeCapabilities();
  const organizationId = useAuthStore((s) => s.organizationId);
  const myRole = useAuthStore(
    (s) => s.user?.organizations?.find((org) => org.id === organizationId)?.role,
  );

  const grants = new Set<string>([
    ...(query.data?.personal ?? []),
    ...(query.data?.instance ?? []),
    ...(query.data?.org ?? []),
  ]);

  const can = (action: string, scope = 'org:*') => {
    for (const grant of grants) {
      if (matchesGrant(grant, action, scope)) return true;
    }
    return false;
  };

  const canManageMember = (targetRole: string | undefined) => {
    if (!can('members.write')) return false;
    const mine = ORG_ROLE_LEVEL[myRole ?? ''] ?? 0;
    const theirs = ORG_ROLE_LEVEL[targetRole ?? ''] ?? 0;
    return mine > theirs;
  };

  return { grants, myRole, can, canManageMember, isLoading: query.isLoading };
}
