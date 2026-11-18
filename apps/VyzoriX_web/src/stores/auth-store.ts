import { createVyzorStore } from '@/lib/state';
import {
  authContext,
  type AuthState,
  type LockoutState,
  type LoginWithTokensResult,
  type MeResult,
  type MFAVerifyResult,
} from '@vyzorix/api-client';
import { getAuthMe } from '@/generated-rq/auth/auth-session';

export interface MfaChallenge {
  userId: string;
  email: string;
  name: string;
  mfaEnabled: boolean;
}

export type AuthStatus =
  | 'unauthenticated'
  | 'mfa_required'
  | 'needs_organization'
  | 'authenticated'
  | 'locked';

export interface AuthStoreState extends AuthState, LockoutState {
  mfaChallenge: MfaChallenge | null;
  status: AuthStatus;
  sessionChecked: boolean;
  setFromLoginWithTokens: (response: LoginWithTokensResult) => void;
  setFromMeResponse: (me: MeResult) => void;
  setFromMfaVerify: (response: MFAVerifyResult) => Promise<MeResult | null>;
  setMfaChallenge: (challenge: MfaChallenge | null) => void;
  setOrganization: (orgId: string | null) => void;
  setAccessToken: (token: string | null) => void;
  refreshTokens: () => Promise<void>;
  setLockout: (state: LockoutState) => void;
  markSessionChecked: () => void;
  clear: () => void;
}

function deriveStatus(
  auth: AuthState,
  lockout: LockoutState,
  mfaChallenge: MfaChallenge | null,
): AuthStatus {
  if (lockout.isLocked) return 'locked';
  if (mfaChallenge) return 'mfa_required';
  if (!auth.isAuthenticated || !auth.user) return 'unauthenticated';
  if (auth.user.needs_organization) return 'needs_organization';
  return 'authenticated';
}

function buildSnapshot(overrides?: Partial<AuthStoreState>): Partial<AuthStoreState> {
  const auth = authContext.getState();
  const lockout = authContext.getLockoutState();
  const base: Partial<AuthStoreState> = {
    isAuthenticated: auth.isAuthenticated,
    user: auth.user,
    organizationId: auth.organizationId,
    accessToken: auth.accessToken,
    refreshToken: auth.refreshToken,
    tokenExpiresAt: auth.tokenExpiresAt,
    isLocked: lockout.isLocked,
    retryAfter: lockout.retryAfter,
    lockedUntil: lockout.lockedUntil,
  };
  const merged = { ...base, ...overrides } as AuthStoreState;
  return { ...merged, status: deriveStatus(auth, lockout, merged.mfaChallenge ?? null) };
}

export const useAuthStore = createVyzorStore<AuthStoreState>('AuthStore', (set) => {
  const initial = buildSnapshot() as AuthStoreState;

  authContext.onChange(() => {
    set(buildSnapshot());
  });

  return {
    ...initial,
    mfaChallenge: null,

    sessionChecked: false,
    markSessionChecked: () => set({ sessionChecked: true }),
    setFromLoginWithTokens: (response) => {
      authContext.setFromLoginWithTokens(response);
      set(buildSnapshot({ mfaChallenge: null }));
    },
    setFromMeResponse: (me) => {
      authContext.setFromMeResponse(me);
      set(buildSnapshot({ mfaChallenge: null }));
    },
    setFromMfaVerify: async (response) => {
      authContext.setFromMfaVerify(response);
      set(buildSnapshot({ mfaChallenge: null }));

      if (response.success && response.access_token) {
        try {
          const me = await getAuthMe();
          if (me) {
            authContext.setFromMeResponse(me);
            set(buildSnapshot({ mfaChallenge: null }));
            return me;
          }
        } catch {
          void 0;
        }
      }
      return null;
    },
    setMfaChallenge: (challenge) => {
      set(buildSnapshot({ mfaChallenge: challenge }));
    },
    setOrganization: (orgId) => {
      authContext.setOrganization(orgId);
      set((state) => buildSnapshot({ mfaChallenge: state.mfaChallenge }));
    },
    setAccessToken: (token) => {
      authContext.setAccessToken(token);
      set((state) => buildSnapshot({ mfaChallenge: state.mfaChallenge }));
    },
    refreshTokens: async () => {
      await authContext.refreshTokens();
      set((state) => buildSnapshot({ mfaChallenge: state.mfaChallenge }));
    },
    setLockout: (lockout) => {
      authContext.setLockout(lockout);
      set((state) => buildSnapshot({ mfaChallenge: state.mfaChallenge }));
    },
    clear: () => {
      authContext.clear();
      set(buildSnapshot({ mfaChallenge: null }));
    },
  };
});
