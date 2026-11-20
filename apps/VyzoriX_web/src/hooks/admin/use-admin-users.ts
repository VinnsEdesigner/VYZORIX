import {
  useGetAuthAdminUsers,
  useGetAuthAdminUsersStats,
  useGetAuthAdminUsersId,
  useDeleteAuthAdminUsersIdSessions,
  useDeleteAuthAdminUsersIdSessionsSessionId,
} from '@/generated-rq/admin/admin-operations';
import {
  useDeleteInstanceUsersId,
  usePostInstanceUsersIdDisable,
  usePostInstanceUsersIdEnable,
} from '@/generated-rq/instance/instance';

export interface AdminUserListParams {
  page?: number;
  limit?: number;
  q?: string;
  sort?: string;
  order?: 'asc' | 'desc';
  status?: string;
}

export function useAdminUsers(params: AdminUserListParams = {}) {
  return useGetAuthAdminUsers({
    page: params.page,
    limit: params.limit,
    q: params.q,
    sort: params.sort,
    order: params.order,
    status: params.status,
  });
}

export function useAdminUserStats() {
  return useGetAuthAdminUsersStats();
}

export function useAdminUser(id: string | undefined) {
  return useGetAuthAdminUsersId(id ?? '', {
    query: { enabled: Boolean(id) },
  });
}

export function useRevokeAdminUserSession() {
  return useDeleteAuthAdminUsersIdSessionsSessionId();
}

export function useRevokeAllAdminUserSessions() {
  return useDeleteAuthAdminUsersIdSessions();
}

export function useDisableAccount() {
  return usePostInstanceUsersIdDisable();
}

export function useEnableAccount() {
  return usePostInstanceUsersIdEnable();
}

export function useDeleteAccount() {
  return useDeleteInstanceUsersId();
}
