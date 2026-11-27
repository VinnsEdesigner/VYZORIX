import { describe, it, expect } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { QueryClientProvider } from '@tanstack/react-query';
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router';
import { ThemeContext, getThemeById } from '@vyzorix/ui';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { createTestQueryClient } from '../helpers/render-hook';
import { SourcesSurface } from '@/ui/pages/admin-provisioning/sources-surface';
import { CredentialsSurface } from '@/ui/pages/admin-provisioning/credentials-surface';
import { ResourcesSurface } from '@/ui/pages/admin-provisioning/resources-surface';
import { ActivitySurface } from '@/ui/pages/admin-provisioning/activity-surface';
import { OverviewSurface } from '@/ui/pages/admin-provisioning/overview-surface';

const { server } = setupIntegrationTest();

function buildRouter(ui: React.ReactElement) {
  const rootRoute = createRootRoute({ component: () => <><Outlet /></> });
  const routeTree = rootRoute.addChildren([
    createRoute({
      getParentRoute: () => rootRoute,
      path: '/',
      component: () => (
        <QueryClientProvider client={createTestQueryClient()}>
          <ThemeContext.Provider value={getThemeById('vyzorix-dark')}>{ui}</ThemeContext.Provider>
        </QueryClientProvider>
      ),
    }),
    createRoute({ getParentRoute: () => rootRoute, path: '/admin/provisioning', component: () => <div /> }),
  ]);

  return createRouter({ routeTree, history: createMemoryHistory({ initialEntries: ['/'] }) });
}

function renderSurface(ui: React.ReactElement) {
  return render(<RouterProvider router={buildRouter(ui)} />);
}

describe('SourcesSurface', () => {
  it('lists the sources the server returns', async () => {
    renderSurface(<SourcesSurface canWrite />);

    await waitFor(() => expect(screen.getByText('Production fleet')).toBeInTheDocument());
    expect(screen.getByText('Staging fleet')).toBeInTheDocument();
  });

  it('shows the teaching empty state when there are no sources', async () => {
    server.use(
      http.get('/v1/provisioning/repositories', () => HttpResponse.json({ repositories: [], total: 0 })),
    );

    renderSurface(<SourcesSurface canWrite />);

    await waitFor(() => expect(screen.getByText('No sources yet')).toBeInTheDocument());
  });

  it('shows the error state when the list fails, with a retry', async () => {
    server.use(
      http.get('/v1/provisioning/repositories', () =>
        HttpResponse.json({ message: 'boom', code: 'INTERNAL' }, { status: 500 }),
      ),
    );

    renderSurface(<SourcesSurface canWrite />);

    await waitFor(() => expect(screen.getByText('Could not load sources')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument();
  });

  it('offers sync and edit to a member who can write', async () => {
    renderSurface(<SourcesSurface canWrite />);

    await waitFor(() => expect(screen.getByText('Production fleet')).toBeInTheDocument());

    expect(screen.getAllByRole('button', { name: /^sync$/i }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole('button', { name: /actions for/i }).length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: /add source/i })).toBeInTheDocument();
  });

  it('hides every write control from a read-only member', async () => {
    renderSurface(<SourcesSurface canWrite={false} />);

    await waitFor(() => expect(screen.getByText('Production fleet')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /add source/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^sync$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /actions for/i })).not.toBeInTheDocument();
  });

  it('distinguishes no matches from no sources', async () => {
    renderSurface(<SourcesSurface canWrite />);
    await waitFor(() => expect(screen.getByText('Production fleet')).toBeInTheDocument());

    fireEvent.change(screen.getByPlaceholderText('Search sources'), {
      target: { value: 'zzzz-no-such-source' },
    });

    await waitFor(() => expect(screen.getByText('No sources match that filter')).toBeInTheDocument());
  });
});

describe('CredentialsSurface', () => {
  it('never renders a token, only health and access', async () => {
    renderSurface(<CredentialsSurface canWrite />);

    await waitFor(() => expect(screen.getByText('GitHub — platform')).toBeInTheDocument());
    expect(screen.queryByText(/ghp_/)).not.toBeInTheDocument();
    expect(screen.getAllByText('Read only').length).toBeGreaterThan(0);
  });

  it('shows the reason a credential is failing', async () => {
    renderSurface(<CredentialsSurface canWrite />);

    await waitFor(() => expect(screen.getByText('the token was rejected')).toBeInTheDocument());
  });

  it('hides check, edit, and delete from a read-only member', async () => {
    renderSurface(<CredentialsSurface canWrite={false} />);

    await waitFor(() => expect(screen.getByText('GitHub — platform')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /check now/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^delete$/i })).not.toBeInTheDocument();
  });

  it('shows the empty state with an add action when writable', async () => {
    server.use(
      http.get('/v1/provisioning/connections', () => HttpResponse.json({ connections: [], total: 0 })),
    );

    renderSurface(<CredentialsSurface canWrite />);

    await waitFor(() => expect(screen.getByText('No credentials yet')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /add a credential/i })).toBeInTheDocument();
  });
});

describe('ResourcesSurface', () => {
  it('separates device resources from org files by kind', async () => {
    renderSurface(<ResourcesSurface canWrite />);

    await waitFor(() => expect(screen.getByText('living-room')).toBeInTheDocument());
    expect(screen.getAllByText('orgs/acme/alerts.toml').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Alerts').length).toBeGreaterThan(0);
  });

  it('shows each drift state as its own label', async () => {
    renderSurface(<ResourcesSurface canWrite />);

    await waitFor(() => expect(screen.getByText('Drifted')).toBeInTheDocument());
    expect(screen.getAllByText('Applied').length).toBeGreaterThan(0);
  });

  it('shows device presence beside the state, so a row says what the server can see', async () => {
    renderSurface(<ResourcesSurface canWrite />);

    await waitFor(() => expect(screen.getByText('living-room')).toBeInTheDocument());
    // The fixture's device resource is online; the org file has no device and shows no presence.
    expect(screen.getAllByText('Online').length).toBeGreaterThan(0);
  });

  it('opens the detail view for a resource', async () => {
    renderSurface(<ResourcesSurface canWrite />);
    await waitFor(() => expect(screen.getByText('living-room')).toBeInTheDocument());

    screen.getAllByRole('button', { name: /^view$/i })[0]!.click();

    await waitFor(() => expect(screen.getAllByText('Sync this device').length).toBeGreaterThan(0));
    expect(screen.getByText('Declared secrets')).toBeInTheDocument();
    expect(screen.getByText('Signed document')).toBeInTheDocument();
  });

  it('omits the per-device sync for a read-only member', async () => {
    renderSurface(<ResourcesSurface canWrite={false} />);
    await waitFor(() => expect(screen.getByText('living-room')).toBeInTheDocument());

    screen.getAllByRole('button', { name: /^view$/i })[0]!.click();

    await waitFor(() => expect(screen.getByText('Declared secrets')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /sync this device/i })).not.toBeInTheDocument();
  });

  it('shows the signed document for a device resource', async () => {
    renderSurface(<ResourcesSurface canWrite />);
    await waitFor(() => expect(screen.getByText('living-room')).toBeInTheDocument());

    screen.getAllByRole('button', { name: /^view$/i })[0]!.click();

    await waitFor(() => expect(screen.getByText(/signed v/)).toBeInTheDocument());
  });
});

describe('ActivitySurface', () => {
  it('names who caused each action, including a machine', async () => {
    renderSurface(<ActivitySurface canWrite />);

    await waitFor(() => expect(screen.getByText('dev@vyzorix.local')).toBeInTheDocument());
    expect(screen.getByText('Git provider')).toBeInTheDocument();
    expect(screen.getByText('Service account')).toBeInTheDocument();
  });

  it('shows the empty state when nothing has happened', async () => {
    server.use(
      http.get('/v1/provisioning/events', () => HttpResponse.json({ events: [], total: 0 })),
    );

    renderSurface(<ActivitySurface canWrite />);

    await waitFor(() => expect(screen.getByText('Nothing has happened yet')).toBeInTheDocument());
  });
});

describe('OverviewSurface', () => {
  it('surfaces failing sources and drifted resources', async () => {
    server.use(
      http.get('/v1/provisioning/repositories', () =>
        HttpResponse.json({
          repositories: [
            {
              id: 'r1',
              name: 'broken',
              title: 'Broken source',
              sync_state: 'error',
              last_error: 'the remote is unreachable',
              stats: { applied: 0, drifted: 0, missing: 0, error: 0, ignored: 0, total: 0 },
            },
            {
              id: 'r2',
              name: 'drifting',
              title: 'Drifting source',
              sync_state: 'success',
              stats: { applied: 2, drifted: 1, missing: 0, error: 0, ignored: 0, total: 3 },
            },
          ],
          total: 2,
        }),
      ),
    );

    renderSurface(<OverviewSurface canWrite />);

    await waitFor(() => expect(screen.getByText('Sources that are failing')).toBeInTheDocument());
    expect(screen.getByText('Devices that drifted from their source')).toBeInTheDocument();
    expect(screen.getByText(/the remote is unreachable/)).toBeInTheDocument();
  });

  it('says everything is fine when nothing is wrong', async () => {

    server.use(
      http.get('/v1/provisioning/repositories', () =>
        HttpResponse.json({
          repositories: [
            {
              id: 'r1',
              name: 'healthy',
              title: 'Healthy source',
              sync_state: 'success',
              stats: { applied: 3, drifted: 0, missing: 0, error: 0, ignored: 0, total: 3 },
            },
          ],
          total: 1,
        }),
      ),
    );

    renderSurface(<OverviewSurface canWrite />);

    await waitFor(() =>
      expect(screen.getByText('Every source is syncing, and nothing has drifted.')).toBeInTheDocument(),
    );
  });

  it('shows the setup prompt when there are no sources at all', async () => {
    server.use(
      http.get('/v1/provisioning/repositories', () => HttpResponse.json({ repositories: [], total: 0 })),
    );

    renderSurface(<OverviewSurface canWrite />);

    await waitFor(() => expect(screen.getByText('Provisioning is not set up')).toBeInTheDocument());
  });
});
