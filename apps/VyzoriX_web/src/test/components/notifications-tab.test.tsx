import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { QueryClientProvider } from '@tanstack/react-query';
import { ThemeContext, getThemeById } from '@vyzorix/ui';
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router';
import { NotificationsTab } from '@/ui/pages/profile/notifications-tab';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { createTestQueryClient } from '../helpers/render-hook';

const { server } = setupIntegrationTest();

function buildRouter() {
  const rootRoute = createRootRoute({ component: () => <Outlet /> });
  const make = (path: string, el: React.ReactNode) =>
    createRoute({ getParentRoute: () => rootRoute, path, component: () => <>{el}</> });
  const routeTree = rootRoute.addChildren([
    make('/profile/notifications/email', <div>email page</div>),
    make('/profile/notifications/slack', <div>slack page</div>),
    make('/notifications', <div>history page</div>),
    make('/', <NotificationsTab />),
  ]);
  return createRouter({ routeTree, history: createMemoryHistory({ initialEntries: ['/'] }) });
}

async function renderTab() {
  render(
    <QueryClientProvider client={createTestQueryClient()}>
      <ThemeContext.Provider value={getThemeById('dark')}>
        <RouterProvider router={buildRouter()} />
      </ThemeContext.Provider>
    </QueryClientProvider>,
  );
  await screen.findByRole('heading', { name: 'Channels' });
}

const SETTINGS = {
  enabled: true,
  channels: ['email'],
  email: { thresholdBreach: true },
  push: {},
  webhook: { enabled: false, url: '', types: [] },
  slack: { connected: false },
};

beforeEach(() => {
  server.use(http.get('/v1/me/notifications', () => HttpResponse.json(SETTINGS)));
});

describe('NotificationsTab', () => {
  it('renders the channel rows and the history row', async () => {
    await renderTab();
    expect(screen.getByRole('button', { name: /Email/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Slack/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Notification history/ })).toBeInTheDocument();
  });

  it('shows the master toggle', async () => {
    await renderTab();
    expect(screen.getByLabelText('Enable notifications')).toBeChecked();
  });

  it('disables the channel toggles when the master switch is off', async () => {
    server.use(
      http.get('/v1/me/notifications', () => HttpResponse.json({ ...SETTINGS, enabled: false })),
    );
    await renderTab();
    expect(screen.getByLabelText('Enable notifications')).not.toBeChecked();
    expect(screen.getByLabelText('Enable Email')).toBeDisabled();
    expect(screen.getByLabelText('Enable Slack')).toBeDisabled();
  });

  it('offers a Send test button once browser push is subscribed', async () => {
    let called = false;
    server.use(
      http.post('/v1/auth/me/notifications/push/test', () => {
        called = true;
        return HttpResponse.json({ delivered: 1 });
      }),
    );
    renderTab();

    await screen.findByRole('heading', { name: 'Channels' });
    expect(screen.queryByRole('button', { name: 'Send test' })).not.toBeInTheDocument();
    expect(called).toBe(false);
  });

  it('opens the email settings route when the whole row is clicked', async () => {
    await renderTab();
    fireEvent.click(screen.getByRole('button', { name: /Email/ }));
    expect(await screen.findByText('email page')).toBeInTheDocument();
  });

  it('opens the slack settings route when the whole row is clicked', async () => {
    await renderTab();
    fireEvent.click(screen.getByRole('button', { name: /Slack/ }));
    expect(await screen.findByText('slack page')).toBeInTheDocument();
  });

  it('opens the notification history route', async () => {
    await renderTab();
    fireEvent.click(screen.getByRole('button', { name: /Notification history/ }));
    expect(await screen.findByText('history page')).toBeInTheDocument();
  });
});
