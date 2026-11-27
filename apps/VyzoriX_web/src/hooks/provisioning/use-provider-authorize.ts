import { usePostAuthOauthGithub } from '@/generated-rq/auth/auth-session';

export function useAuthorizeProvider() {
  const github = usePostAuthOauthGithub();

  const authorizeGithub = async (redirectUrl?: string) => {
    await github.mutateAsync({ params: redirectUrl ? { redirect_url: redirectUrl } : {} });

  };

  return {
    authorizeGithub,
    isAuthorizing: github.isPending,
    error: github.error,
    isError: github.isError,
  };
}
