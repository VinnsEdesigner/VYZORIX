import { useMutation, useQueryClient } from '@tanstack/react-query';
import { restClient } from '@vyzorix/api-client';
import type { MeResult, ProfileActionResult, ProfileSummaryResult } from '@vyzorix/api-client';
import { getAuthMe, useGetAuthMeProfileSummary } from '@/generated-rq/auth/auth-session';
import { usePatchAuthMe } from '@/generated-rq/settings/user-settings';
import { useAuthStore } from '@/stores';

export function useProfileSummary() {
  return useGetAuthMeProfileSummary<ProfileSummaryResult>({
    query: { select: (data) => data as ProfileSummaryResult },
  });
}

export function useUpdateProfileName() {
  const patch = usePatchAuthMe();
  const setFromMeResponse = useAuthStore((s) => s.setFromMeResponse);
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (name: string) => patch.mutateAsync({ data: { name } }),
    onSuccess: async () => {
      const me = await getAuthMe();
      if (me) setFromMeResponse(me as MeResult);
      void queryClient.invalidateQueries({ queryKey: ['getAuthMe'] });
    },
  });
}

export function useUploadAvatar() {
  const setFromMeResponse = useAuthStore((s) => s.setFromMeResponse);
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (file: File) => {
      const form = new FormData();
      form.append('file', file);
      return restClient.post<ProfileActionResult>('/v1/auth/me/avatar', form, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
    },
    onSuccess: async () => {
      const me = await getAuthMe();
      if (me) setFromMeResponse(me as MeResult);
      void queryClient.invalidateQueries({ queryKey: ['getAuthMe'] });
    },
  });
}
