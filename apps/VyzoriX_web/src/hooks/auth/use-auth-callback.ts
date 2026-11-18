import { useEffect, useState } from 'react';
import {
  oauth,
  type OAuthCallbackResult,
} from '@vyzorix/api-client';
import { useAuthStore } from '@/stores/auth-store';
import { useMe } from './use-me';

export interface UseAuthCallbackOptions {

  url?: string;

  redirectOnSuccess?: string;

  fetchMe?: boolean;
}

export interface AuthCallbackState {
  result: OAuthCallbackResult | null;
  isLoading: boolean;
  error: string | null;
}

export function useAuthCallback(options: UseAuthCallbackOptions = {}): AuthCallbackState {
  const { url, fetchMe = true } = options;
  const [state, setState] = useState<AuthCallbackState>({
    result: null,
    isLoading: true,
    error: null,
  });

  const setFromMeResponse = useAuthStore((s) => s.setFromMeResponse);
  const meQuery = useMe();

  const meRefetch = meQuery.refetch;

  useEffect(() => {
    let cancelled = false;
    const targetUrl = url ?? (typeof window !== 'undefined' ? window.location.href : '');

    async function run() {
      if (!targetUrl) {
        setState({ result: null, isLoading: false, error: 'No callback URL' });
        return;
      }
      const parsed = oauth.parseCallback(targetUrl);
      if (cancelled) return;

      if (!parsed.success) {
        setState({
          result: parsed,
          isLoading: false,
          error: parsed.errorMessage ?? 'OAuth authentication failed',
        });
        return;
      }

      if (fetchMe) {
        try {
          const me = await meRefetch();
          if (cancelled) return;
          if (me) {
            setFromMeResponse(me);
          }
        } catch {
          void 0;
        }
      }

      setState({ result: parsed, isLoading: false, error: null });
    }

    run();
    return () => {
      cancelled = true;
    };
  }, [url, fetchMe, meRefetch, setFromMeResponse]);

  return state;
}

export function useInitiateGoogleLogin() {
  return (redirectAfterCallback?: string) =>
    oauth.initiateGoogleLogin({ redirectAfterCallback });
}

export function useInitiateGitHubLogin() {
  return (redirectAfterCallback?: string) =>
    oauth.initiateGitHubLogin({ redirectAfterCallback });
}
