import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { QueryClientProvider } from '@tanstack/react-query';
import { ThemeContext, getThemeById } from '@vyzorix/ui';
import { NotificationHistoryPage } from '@/ui/pages/profile/notification-history-page';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { createTestQueryClient } from '../helpers/render-hook';

const { server } = setupIntegrationTest();

const HISTORY = {
  entries: [
    { id: 'n1', eventType: 'threshold_breach', channel: 'email', sentAt: 1767312000000, success: true },
    { id: 'n2', eventType: 'device_offline', channel: 'slack', sentAt: 1767225600000, success: false, errorMsg: 'channel_not_found' },
  ],
  total: 2,
  unread: 2,
  limit: 25,
  offset: 0,
};

function renderPage() {
  render(
    <QueryClientProvider client={createTestQueryClient()}>
      <ThemeContext.Provider value={getThemeById('dark')}>
        <NotificationHistoryPage />
      </ThemeContext.Provider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  server.use(http.get('/v1/auth/me/notifications/history', () => HttpResponse.json(HISTORY)));
});

describe('NotificationHistoryPage', () => {
  it('lists delivered notifications with their outcome', async () => {
    renderPage();
    expect(await screen.findByText('Threshold breach')).toBeInTheDocument();
    expect(screen.getByText('Device offline')).toBeInTheDocument();

    const list = screen.getByRole('list');
    expect(within(list).getByText('Sent')).toBeInTheDocument();
    expect(within(list).getByText('Failed')).toBeInTheDocument();
  });

  it('shows the failure reason', async () => {
    renderPage();
    expect(await screen.findByText('channel_not_found')).toBeInTheDocument();
  });

  it('shows an empty state when there is no history', async () => {
    server.use(
      http.get('/v1/auth/me/notifications/history', () =>
        HttpResponse.json({ entries: [], total: 0, limit: 25, offset: 0 }),
      ),
    );
    renderPage();
    expect(await screen.findByText('No notifications have been sent yet.')).toBeInTheDocument();
  });

  it('shows the unread count and the toolbar options', async () => {
    renderPage();
    await screen.findByText('Threshold breach');
    expect(screen.getByText('2 unread')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Mark all read' })).toBeEnabled();
    expect(screen.getByRole('button', { name: /Export/ })).toBeEnabled();
    expect(screen.getByLabelText('Live')).not.toBeChecked();
  });

  it('marks all read through the API', async () => {
    let called = false;
    server.use(
      http.post('/v1/auth/me/notifications/history/read', () => {
        called = true;
        return HttpResponse.json({ unread: 0 });
      }),
    );
    renderPage();
    await screen.findByText('Threshold breach');
    fireEvent.click(screen.getByRole('button', { name: 'Mark all read' }));
    await waitFor(() => expect(called).toBe(true));
  });

  it('sends the channel filter to the API', async () => {
    let url = '';
    server.use(
      http.get('/v1/auth/me/notifications/history', ({ request }) => {
        url = new URL(request.url).searchParams.get('channel') ?? '';
        return HttpResponse.json(HISTORY);
      }),
    );
    renderPage();
    await screen.findByText('Threshold breach');
    fireEvent.click(screen.getByRole('button', { name: 'Slack' }));
    await waitFor(() => expect(url).toBe('slack'));
  });
});
