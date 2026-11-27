import {
  useGetServiceAccounts,
  usePostServiceAccounts,
  usePatchServiceAccountsId,
  useDeleteServiceAccountsId,
  useGetServiceAccountsIdTokens,
  usePostServiceAccountsIdTokens,
  useDeleteServiceAccountsIdTokensToken,
  usePostServiceAccountsIdTokensTokenRotate,
  usePatchServiceAccountsIdTokensToken,
  useGetServiceAccountsRoles,
} from '@/generated-rq/service-accounts/service-accounts-management';

export function useServiceAccounts() {
  return useGetServiceAccounts();
}

export function useServiceAccountTokens(id: string | undefined) {
  return useGetServiceAccountsIdTokens(id ?? '', {
    query: { enabled: Boolean(id) },
  });
}

export function useCreateServiceAccount() {
  return usePostServiceAccounts();
}

export function useUpdateServiceAccount() {
  return usePatchServiceAccountsId();
}

export function useDeleteServiceAccount() {
  return useDeleteServiceAccountsId();
}

export function useCreateServiceAccountToken() {
  return usePostServiceAccountsIdTokens();
}

export function useRevokeServiceAccountToken() {
  return useDeleteServiceAccountsIdTokensToken();
}

export function useDeleteServiceAccountToken() {
  return useDeleteServiceAccountsIdTokensToken();
}

export function useRotateServiceAccountToken() {
  return usePostServiceAccountsIdTokensTokenRotate();
}

export function useUpdateServiceAccountTokenRoles() {
  return usePatchServiceAccountsIdTokensToken();
}

export function useServiceAccountRoleCatalog() {
  return useGetServiceAccountsRoles();
}

export type {
  ServiceAccount,
  ServiceAccountToken,
  FixedRoleGrant,
} from '@vyzorix/api-client';
