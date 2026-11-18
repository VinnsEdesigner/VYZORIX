import { useMutation } from '@tanstack/react-query';
import { postAuthLogin, getAuthMe } from '@/generated-rq/auth/auth-session';
import type { LoginRequest, LoginResult } from '@vyzorix/api-client';
import { useAuthStore, type MfaChallenge } from '@/stores/auth-store';

export function useLogin() {
  const setFromMeResponse = useAuthStore((s) => s.setFromMeResponse);
  const setMfaChallenge = useAuthStore((s) => s.setMfaChallenge);

  return useMutation<LoginResult, Error, { data: LoginRequest }>({
    mutationFn: async ({ data }) => {
      const result = await postAuthLogin(data);

      if (result.requires_mfa) {
        const challenge: MfaChallenge = {
          userId: result.user_id ?? '',
          email: result.email ?? '',
          name: result.name ?? '',
          mfaEnabled: result.mfa_enabled ?? false,
        };
        setMfaChallenge(challenge);
        return result;
      }

      const me = await getAuthMe();
      if (me) setFromMeResponse(me);
      return result;
    },
  });
}
