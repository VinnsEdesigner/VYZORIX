import { http, HttpResponse, delay } from 'msw';

const API_BASE = '/v1/auth';

const MOCK_ORG = {
  id: 'org-test-1',
  name: 'Test Organization',
  role: 'admin',
};

const MOCK_MEMBERSHIP = {
  id: 'membership-test-1',
  organization_id: 'org-test-1',
  organization_name: 'Test Organization',
  role: 'admin',
  joined_at: 1767225600000,
};

const MOCK_SESSION = {
  id: 'session-test-1',
  ip_address: '192.168.1.1',
  user_agent: 'Mozilla/5.0 Chrome',
  created_at: 1767225600000,
  expires_at: 1767312000000,
  is_current: true,
  selected_organization_id: 'org-test-1',
};

export function createAuthHandlers() {
  return [

    http.get(`${API_BASE}/csrf-token`, async () => {
      return HttpResponse.json({ csrf_token: 'mock-csrf-token' });
    }),

    http.post(`${API_BASE}/login`, async ({ request }) => {
      await delay(50);
      const body = (await request.json()) as { email?: string; password?: string };
      if (!body.email || !body.password) {
        return HttpResponse.json({ error: 'invalid credentials' }, { status: 401 });
      }
      return HttpResponse.json({
        user_id: 'user-test-1',
        email: body.email,
        name: 'Test User',
        role: 'admin',
        mfa_enabled: false,
        needs_organization: false,
        organizations: [MOCK_ORG],
        selected_organization: MOCK_ORG,
        last_organization_id: 'org-test-1',
      });
    }),

    http.post(`${API_BASE}/login/tokens`, async ({ request }) => {
      await delay(50);
      const body = (await request.json()) as { email?: string; password?: string };
      if (!body.email || !body.password) {
        return HttpResponse.json({ error: 'invalid credentials' }, { status: 401 });
      }
      return HttpResponse.json({
        user_id: 'user-test-1',
        email: body.email,
        name: 'Test User',
        role: 'admin',
        mfa_enabled: false,
        needs_organization: false,
        organizations: [MOCK_ORG],
        memberships: [MOCK_MEMBERSHIP],
        selected_organization: MOCK_ORG,
        last_organization_id: 'org-test-1',
        access_token: 'mock-access-token',
        refresh_token: 'mock-refresh-token',
        expires_at: Math.floor(Date.now() / 1000) + 3600,
        session_id: 'session-test-1',
        signing_key: 'mock-signing-key',
      });
    }),

    http.post(`${API_BASE}/logout`, async () => {
      await delay(30);
      return HttpResponse.json({ success: true });
    }),

    http.post(`${API_BASE}/refresh`, async ({ request }) => {
      await delay(30);
      const body = (await request.json()) as { refresh_token?: string };
      if (!body.refresh_token) {
        return HttpResponse.json({ error: 'refresh token required' }, { status: 400 });
      }
      return HttpResponse.json({
        access_token: 'mock-access-token-refreshed',
        refresh_token: 'mock-refresh-token-refreshed',
        expires_at: Math.floor(Date.now() / 1000) + 3600,
        session_id: 'session-test-1',
      });
    }),

    http.post(`${API_BASE}/register`, async ({ request }) => {
      await delay(50);
      const body = (await request.json()) as { email?: string; password?: string; name?: string };
      if (!body.email || !body.password || !body.name) {
        return HttpResponse.json({ error: 'missing fields' }, { status: 400 });
      }
      return HttpResponse.json({
        user_id: 'user-test-2',
        email: body.email,
        name: body.name,
      });
    }),

    http.get(`${API_BASE}/me`, async () => {
      await delay(30);
      return HttpResponse.json({
        id: 'user-test-1',
        email: 'test@vyzorix.com',
        name: 'Test User',
        mfa_enabled: false,
        email_verified: true,
        needs_organization: false,
        organizations: [MOCK_ORG],
        memberships: [MOCK_MEMBERSHIP],
        selected_organization: MOCK_ORG,
        last_organization_id: 'org-test-1',
        avatar_url: '',
        created_at: 1767225600000,
        last_login_at: 1767312000000,
        thresholds: {
          riskWarn: 70,
          riskCrit: 90,
          thermalWarn: 60,
          thermalCrit: 85,
          bufferWarn: 70,
          bufferCrit: 95,
        },
        client: {
          requestTimeoutMs: 8000,
          retryAttempts: 3,
        },
      });
    }),

    http.patch(`${API_BASE}/me`, async ({ request }) => {
      const body = (await request.json()) as { name?: string };
      return HttpResponse.json({
        id: 'user-test-1',
        email: 'test@vyzorix.com',
        name: body.name ?? 'Test User',
        role: 'admin',
        mfa_enabled: false,
        email_verified: true,
      });
    }),

    http.get(`${API_BASE}/me/profile-summary`, async () => {
      await delay(20);
      return HttpResponse.json({ active_sessions: 1, actions: 42 });
    }),

    http.post(`${API_BASE}/me/avatar`, async () => {
      await delay(20);
      return HttpResponse.json({
        id: 'user-test-1',
        name: 'Test User',
        email: 'test@vyzorix.com',
        avatar_url: '/avatars/user-test-1-abc123.png',
      });
    }),

    http.get('/v1/me/invitations', async () => {
      return HttpResponse.json({ invitations: [] });
    }),

    http.get(`${API_BASE}/organizations`, async () => {
      return HttpResponse.json({ organizations: [MOCK_ORG] });
    }),

    http.post(`${API_BASE}/organizations/select`, async ({ request }) => {
      const body = (await request.json()) as { organization_id?: string };
      return HttpResponse.json({ ...MOCK_ORG, id: body.organization_id ?? MOCK_ORG.id });
    }),

    http.get(`${API_BASE}/mfa/status`, async () => {
      return HttpResponse.json({ mfa_enabled: false, backup_codes: undefined });
    }),

    http.post(`${API_BASE}/mfa/enroll`, async () => {
      return HttpResponse.json({ secret: 'JBSWY3DPEHPK3PXP', uri: 'otpauth://totp/VyzoriX:test@vyzorix.com?secret=JBSWY3DPEHPK3PXP&issuer=VyzoriX' });
    }),

    http.post(`${API_BASE}/mfa/verify-setup`, async ({ request }) => {
      const body = (await request.json()) as { code?: string };
      return HttpResponse.json({ verified: body.code === '123456' });
    }),

    http.post(`${API_BASE}/mfa/enable`, async ({ request }) => {
      const body = (await request.json()) as { code?: string };
      if (body.code !== '123456') {
        return HttpResponse.json({ error: 'invalid code' }, { status: 400 });
      }
      return HttpResponse.json({
        success: true,
        backup_codes: ['bc-001', 'bc-002', 'bc-003', 'bc-004', 'bc-005'],
      });
    }),

    http.post(`${API_BASE}/mfa/disable`, async ({ request }) => {
      const body = (await request.json()) as { code?: string };
      return HttpResponse.json({ success: body.code === '123456' });
    }),

    http.post(`${API_BASE}/mfa/verify`, async ({ request }) => {
      const body = (await request.json()) as { user_id?: string; code?: string };
      if (body.code !== '123456') {
        return HttpResponse.json({ error: 'invalid code' }, { status: 400 });
      }
      return HttpResponse.json({
        success: true,
        session_id: 'session-test-1',
        access_token: 'mock-access-token-mfa',
        refresh_token: 'mock-refresh-token-mfa',
        expires_at: Math.floor(Date.now() / 1000) + 3600,
        signing_key: 'mock-signing-key-mfa',
        user: {
          id: body.user_id ?? 'user-test-1',
          email: 'test@vyzorix.com',
          name: 'Test User',
          role: 'admin',
          mfa_enabled: true,
        },
      });
    }),

    http.post(`${API_BASE}/mfa/verify-backup`, async ({ request }) => {
      const body = (await request.json()) as { code?: string };
      return HttpResponse.json({ valid: body.code === 'bc-001' });
    }),

    http.post(`${API_BASE}/mfa/regenerate-backup-codes`, async () => {
      return HttpResponse.json({
        backup_codes: ['new-bc-001', 'new-bc-002', 'new-bc-003', 'new-bc-004', 'new-bc-005'],
      });
    }),

    http.post(`${API_BASE}/forgot-password`, async ({ request }) => {
      const body = (await request.json()) as { email?: string };
      if (!body.email) {
        return HttpResponse.json({ error: 'email required' }, { status: 400 });
      }
      return HttpResponse.json({ success: true, message: 'If that email exists, a password reset link has been sent.' });
    }),

    http.post(`${API_BASE}/reset-password`, async ({ request }) => {
      const body = (await request.json()) as { token?: string; newPassword?: string };
      if (!body.token || !body.newPassword) {
        return HttpResponse.json({ error: 'token and newPassword required' }, { status: 400 });
      }
      return HttpResponse.json({ success: true, message: 'Password has been reset successfully.' });
    }),

    http.post(`${API_BASE}/resend-password-reset`, async ({ request }) => {
      const body = (await request.json()) as { email?: string };
      if (!body.email) {
        return HttpResponse.json({ error: 'email required' }, { status: 400 });
      }
      return HttpResponse.json({ success: true, message: 'reset email sent' });
    }),

    http.get(`${API_BASE}/verify-email`, async ({ request }) => {
      const url = new URL(request.url);
      const token = url.searchParams.get('token');
      return HttpResponse.json({ verified: token === 'valid-token', email: 'test@vyzorix.com' });
    }),

    http.post(`${API_BASE}/verify-email`, async ({ request }) => {

      const body = (await request.json()) as { token?: string };
      return HttpResponse.json({ verified: body.token === 'valid-token', email: 'test@vyzorix.com' });
    }),

    http.post(`${API_BASE}/resend-verification`, async ({ request }) => {
      await request.json();
      return HttpResponse.json({ message: 'verification email sent' });
    }),

    http.post(`${API_BASE}/cancel-verification`, async ({ request }) => {
      await request.json();
      return HttpResponse.json({ success: true });
    }),

    http.get(`${API_BASE}/poll-verification`, async ({ request }) => {
      const url = new URL(request.url);
      const token = url.searchParams.get('token');
      return HttpResponse.json({ verified: token === 'valid-token', email: 'test@vyzorix.com' });
    }),

    http.get(`${API_BASE}/sessions`, async () => {
      await delay(30);
      return HttpResponse.json({
        sessions: [MOCK_SESSION],
        total: 1,
      });
    }),

    http.get(`${API_BASE}/sessions/concurrent`, async () => {
      await delay(20);
      return HttpResponse.json({
        has_concurrent: false,
        count: 1,
        sessions: [{
          session_id: 'session-test-1',
          ip_address: '192.168.1.1',
          user_agent: 'Mozilla/5.0 Chrome',
          created_at: 1767225600000,
        }],
      });
    }),

    http.delete(`${API_BASE}/sessions/:sessionId`, async ({ params }) => {
      return HttpResponse.json({ success: true, message: `session ${params.sessionId} revoked` });
    }),

    http.delete(`${API_BASE}/sessions`, async () => {
      return HttpResponse.json({ success: true, revoked_count: 2, message: 'all other sessions revoked' });
    }),

    http.post(`${API_BASE}/sessions/revoke-all`, async () => {
      return HttpResponse.json({ success: true, revokedCount: 3, message: 'all device sessions revoked' });
    }),
  ];
}
