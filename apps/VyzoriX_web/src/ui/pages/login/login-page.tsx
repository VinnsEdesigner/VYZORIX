import { useState } from 'react';
import { css } from '@emotion/css';
import { useNavigate } from '@tanstack/react-router';
import {
  Alert,
  BrandLogo,
  Button,
  Field,
  Input,
  useBranding,
  useStyles2,
  type VyzorixTheme,
} from '@vyzorix/ui';
import { parseApiError } from '@vyzorix/api-client';
import { useLogin, useMfaVerify } from '@/hooks/auth';
import { useAuthStore } from '@/stores';

function errorMessage(error: unknown): string {
  const parsed = parseApiError(error);
  return parsed.message || 'Could not sign you in. Please try again.';
}

export function LoginPage() {
  const styles = useStyles2(getStyles);
  const branding = useBranding();
  const login = useLogin();
  const navigate = useNavigate();
  const mfaChallenge = useAuthStore((s) => s.mfaChallenge);
  const status = useAuthStore((s) => s.status);

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);

    login.mutate(
      { data: { email: email.trim(), password } },
      {
        onSuccess: (result) => {

          if (result?.error === 'account_locked') {
            setError(result.message || 'Account temporarily locked. Try again later.');
            return;
          }

          if (result?.requires_mfa) return;
          void navigate({ to: '/' });
        },
        onError: (err) => setError(errorMessage(err)),
      },
    );
  };

  return (
    <div className={styles.page}>
      <main className={styles.card}>
        <div className={styles.brand}>
          <BrandLogo source="login" size="xl" />
          <h1 className={styles.title}>{branding.loginTitle}</h1>
          <p className={styles.subtitle}>{branding.loginSubtitle}</p>
        </div>

        {mfaChallenge ? (
          <MfaStep userId={mfaChallenge.userId} email={mfaChallenge.email || email} />
        ) : (
          <form className={styles.form} onSubmit={onSubmit} noValidate>
            {error ? (
              <Alert severity="error" title="Sign-in failed">
                {error}
              </Alert>
            ) : null}

            <Field label="Email">
              <Input
                type="email"
                name="email"
                autoComplete="username"
                autoFocus
                required
                value={email}
                onChange={(event) => setEmail(event.target.value)}
              />
            </Field>

            <Field label="Password">
              <Input
                type={showPassword ? 'text' : 'password'}
                name="password"
                autoComplete="current-password"
                required
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </Field>

            <label className={styles.showPassword}>
              <input
                type="checkbox"
                checked={showPassword}
                onChange={(event) => setShowPassword(event.target.checked)}
              />
              Show password
            </label>

            <Button
              type="submit"
              variant="primary"
              fullWidth
              loading={login.isPending || status === 'locked'}
              disabled={!email.trim() || !password || login.isPending}
            >
              Sign in
            </Button>
          </form>
        )}
      </main>
    </div>
  );
}

function MfaStep({ userId, email }: { userId: string; email: string }) {
  const styles = useStyles2(getStyles);
  const verify = useMfaVerify();
  const navigate = useNavigate();
  const setMfaChallenge = useAuthStore((s) => s.setMfaChallenge);
  const [code, setCode] = useState('');
  const [error, setError] = useState<string | null>(null);

  const onSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);
    verify.mutate(
      { data: { user_id: userId, code } },
      {
        onSuccess: () => void navigate({ to: '/' }),
        onError: (err) => setError(errorMessage(err)),
      },
    );
  };

  return (
    <form className={styles.form} onSubmit={onSubmit} noValidate>
      <p className={styles.subtitle}>Enter the 6-digit code from your authenticator app for {email}.</p>
      {error ? (
        <Alert severity="error" title="Verification failed">
          {error}
        </Alert>
      ) : null}
      <Field label="Authentication code">
        <Input
          inputMode="numeric"
          autoComplete="one-time-code"
          maxLength={6}
          autoFocus
          value={code}
          onChange={(event) => setCode(event.target.value.replace(/\D/g, ''))}
        />
      </Field>
      <Button type="submit" variant="primary" fullWidth loading={verify.isPending} disabled={code.length !== 6}>
        Verify
      </Button>
      <Button type="button" variant="secondary" fill="text" fullWidth onClick={() => setMfaChallenge(null)}>
        Back
      </Button>
    </form>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  page: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: '100vh',
    padding: theme.spacing(2),
    background: theme.colors.background.canvas,
  }),
  card: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(3),
    width: '100%',
    maxWidth: 400,
    padding: theme.spacing(4),
    background: theme.colors.background.primary,
    border: `1px solid ${theme.colors.border.medium}`,
    borderRadius: theme.shape.radius.lg,
    boxShadow: theme.shadows.z2,
  }),
  brand: css({ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: theme.spacing(1), textAlign: 'center' }),
  title: css({ margin: 0, fontSize: theme.typography.h4.fontSize, fontWeight: theme.typography.fontWeightMedium }),
  subtitle: css({ margin: 0, color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  form: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  showPassword: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
    cursor: 'pointer',
  }),
});
