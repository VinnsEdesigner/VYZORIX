import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { QueryClientProvider } from '@tanstack/react-query';
import { ThemeContext, getThemeById } from '@vyzorix/ui';
import { EmailChannelPage } from '@/ui/pages/profile/email-channel-page';
import { useAuthStore } from '@/stores';
import { useToastStore } from '@/stores/toast-store';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { createTestQueryClient } from '../helpers/render-hook';

const { server } = setupIntegrationTest();

const BASE = {
  enabled: true,
  channels: ['email'],
  email: { thresholdBreach: true, deviceOffline: true, deviceOnline: false, updateAvailable: false, commandFailed: false, registrationRequest: false },
  push: {},
  webhook: { enabled: false, url: '', types: [] },
  slack: { connected: false },
};

function renderPage() {
  render(
    <QueryClientProvider client={createTestQueryClient()}>
      <ThemeContext.Provider value={getThemeById('dark')}>
        <EmailChannelPage />
      </ThemeContext.Provider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useToastStore.setState({ toasts: [] });
  useAuthStore.setState({
    isAuthenticated: true,
    user: { id: 'user-test-1', name: 'Test User', email: 'test@vyzorix.com', avatar_url: '', needs_organization: false },
  } as never);
  server.use(http.get('/v1/me/notifications', () => HttpResponse.json(BASE)));
});

describe('EmailChannelPage', () => {
  it('defaults to the account email', async () => {
    renderPage();
    expect(await screen.findByLabelText('My account email')).toBeChecked();
    expect(screen.getByText('test@vyzorix.com')).toBeInTheDocument();
  });

  it('sends a custom address when chosen', async () => {
    let body: unknown = null;
    server.use(
      http.patch('/v1/me/notifications', async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(BASE);
      }),
    );
    renderPage();
    await screen.findByLabelText('A different email address');

    fireEvent.click(screen.getByLabelText('A different email address'));
    fireEvent.change(screen.getByLabelText('Notification email address'), { target: { value: 'alerts@example.com' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save address' }));

    await waitFor(() => expect(body).toEqual({ emailAddress: 'alerts@example.com' }));
  });

  it('surfaces the server rejection of an invalid address', async () => {
    server.use(
      http.patch('/v1/me/notifications', () =>
        HttpResponse.json(
          { error: { code: 'VALIDATION_FAILED', message: 'email domain does not accept mail' } },
          { status: 400 },
        ),
      ),
    );
    renderPage();
    await screen.findByLabelText('A different email address');

    fireEvent.click(screen.getByLabelText('A different email address'));
    fireEvent.change(screen.getByLabelText('Notification email address'), {
      target: { value: 'someone@this-domain-does-not-exist.invalid' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save address' }));

    await waitFor(() =>
      expect(
        useToastStore.getState().toasts.some((toast) => toast.title === 'Could not save the email address'),
      ).toBe(true),
    );
  });

  it('does not offer a command-failed email event', async () => {
    renderPage();
    await screen.findByRole('heading', { name: 'Email events' });
    expect(screen.queryByLabelText('Command failed')).not.toBeInTheDocument();
  });
});
