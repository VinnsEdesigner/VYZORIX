import {
  useGetOrganizationsIdMembers,
  useDeleteOrganizationsIdMembersMemberId,
  usePostOrganizationsIdMembersMemberIdSuspend,
  usePostOrganizationsIdMembersMemberIdReinstate,
  usePatchOrganizationsIdMembersMemberIdRole,
} from '@/generated-rq/members/organization-members';
import {
  usePostInvitations,
  useGetOrganizationsIdInvitations,
  useDeleteInvitationsId,
} from '@/generated-rq/invitations/organization-invitations';

export function useOrganizationMembers(orgId: string | undefined, status?: string) {
  return useGetOrganizationsIdMembers(orgId ?? '', status ? { status } : undefined, {
    query: { enabled: Boolean(orgId) },
  });
}

export function useSuspendMember() {
  return usePostOrganizationsIdMembersMemberIdSuspend();
}

export function useReinstateMember() {
  return usePostOrganizationsIdMembersMemberIdReinstate();
}

export function useRemoveMember() {
  return useDeleteOrganizationsIdMembersMemberId();
}

export function useUpdateMemberRole() {
  return usePatchOrganizationsIdMembersMemberIdRole();
}

export function useInviteMember() {
  return usePostInvitations();
}

export function useOrganizationInvitations(orgId: string | undefined, status?: string) {
  return useGetOrganizationsIdInvitations(orgId ?? '', status ? { status } : undefined, {
    query: { enabled: Boolean(orgId) },
  });
}

export function useRevokeInvitation() {
  return useDeleteInvitationsId();
}
