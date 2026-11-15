import {
  useGetAdminApiKeys,
  useGetAdminApiKeysUserUserId,
  useGetAdminApiKeysStats,
  useGetAdminApiKeysStatsUserUserId,
  useDeleteAdminApiKeysKeyId,
} from '@/generated-rq/admin/admin-operations';

export function useAdminApiKeys(params?: { page?: number; limit?: number; userId?: string; search?: string }) {
  return useGetAdminApiKeys({
    page: params?.page,
    limit: params?.limit,
    user_id: params?.userId,
    search: params?.search,
  });
}

export function useAdminUserKeys(userId: string | undefined) {
  return useGetAdminApiKeysUserUserId(
    userId ?? '',
    undefined,
    { query: { enabled: userId !== undefined } },
  );
}

export function useGlobalStats() {
  return useGetAdminApiKeysStats();
}

export function useAdminUserKeyStats(userId: string | undefined) {
  return useGetAdminApiKeysStatsUserUserId(
    userId ?? '',
    { query: { enabled: userId !== undefined } },
  );
}

export function useForceRevokeKey() {
  return useDeleteAdminApiKeysKeyId();
}
