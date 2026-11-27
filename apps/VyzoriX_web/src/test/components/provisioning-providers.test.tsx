import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
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
import { ProvidersSurface } from '@/ui/pages/admin-provisioning/providers-surface';
import { ProviderDetail } from '@/ui/pages/admin-provisioning/provider-detail';

const { server } = setupIntegrationTest();

function renderRouted(ui: React.ReactElement) {
  const rootRoute = createRootRoute({ component: () => <Outlet /> });
  const page = () => (
    <QueryClientProvider client={createTestQueryClient()}>
      <ThemeContext.Provider value={getThemeById('vyzorix-dark')}>{ui}</ThemeContext.Provider>
    </QueryClientProvider>
  );

  const tree = rootRoute.addChildren([
    createRoute({ getParentRoute: () => rootRoute, path: '/', component: page }),
    createRoute({ getParentRoute: () => rootRoute, path: '/admin/provisioning', component: page }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: '/admin/provisioning/providers/$providerId',
      component: page,
    }),
  ]);

  return render(
    <RouterProvider
      router={createRouter({ routeTree: tree, history: createMemoryHistory({ initialEntries: ['/'] }) })}
    />,
  );
}

describe('ProvidersSurface', () => {
  it('lists GitHub and GitLab, both not installed by default', async () => {
    server.use(http.get('/v1/provisioning/connections', () => HttpResponse.json({ connections: [], total: 0 })));

    renderRouted(<ProvidersSurface canWrite />);

    await waitFor(() => expect(screen.getByText('GitHub')).toBeInTheDocument());
    expect(screen.getByText('GitLab')).toBeInTheDocument();
    expect(screen.getAllByText('Not installed')).toHaveLength(2);
  });

  it('marks a provider installed once a credential of that type exists', async () => {
    server.use(
      http.get('/v1/provisioning/connections', () =>
        HttpResponse.json({
          connections: [{ id: 'c1', name: 'x', type: 'github', host: 'github.com', status: 'ready' }],
          total: 1,
        }),
      ),
    );

    renderRouted(<ProvidersSurface canWrite />);

    await waitFor(() => expect(screen.getByText('Installed')).toBeInTheDocument());
    expect(screen.getAllByText('Not installed')).toHaveLength(1);
  });

  it('links each provider to its own configuration page', async () => {
    server.use(http.get('/v1/provisioning/connections', () => HttpResponse.json({ connections: [], total: 0 })));

    renderRouted(<ProvidersSurface canWrite />);

    await waitFor(() => expect(screen.getByText('GitHub')).toBeInTheDocument());
    const links = screen.getAllByRole('link', { name: /configure/i });
    expect(links.map((link) => link.getAttribute('href'))).toEqual([
      '/admin/provisioning/providers/github',
      '/admin/provisioning/providers/gitlab',
    ]);
  });

  it('hides the install control from a read-only member', async () => {
    server.use(http.get('/v1/provisioning/connections', () => HttpResponse.json({ connections: [], total: 0 })));

    renderRouted(<ProvidersSurface canWrite={false} />);

    await waitFor(() => expect(screen.getByText('GitHub')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /install/i })).not.toBeInTheDocument();
  });
});

describe('ProviderDetail', () => {
  it('shows GitHub’s scopes on its own page', async () => {
    renderRouted(<ProviderDetail providerId="github" canWrite />);

    expect(await screen.findByRole('heading', { name: 'GitHub' })).toBeInTheDocument();
    expect(screen.getByText('read:user')).toBeInTheDocument();
    expect(screen.getByText('user:email')).toBeInTheDocument();
  });

  it('does not show operator-only OAuth app setup to an end user', async () => {
    renderRouted(<ProviderDetail providerId="github" canWrite />);
    await screen.findByRole('heading', { name: 'GitHub' });

    expect(screen.queryByText(/Setting up the OAuth app/i)).not.toBeInTheDocument();
    expect(screen.queryByText('GITHUB_OAUTH_CLIENT_ID')).not.toBeInTheDocument();
    expect(screen.queryByText('GITHUB_OAUTH_CLIENT_SECRET')).not.toBeInTheDocument();
    expect(screen.queryByText(/Copy the client ID/i)).not.toBeInTheDocument();
  });

  it('shows GitLab’s different scopes, proving the pages are separate', async () => {
    renderRouted(<ProviderDetail providerId="gitlab" canWrite />);

    expect(await screen.findByRole('heading', { name: 'GitLab' })).toBeInTheDocument();
    expect(screen.getByText('read_repository')).toBeInTheDocument();
    expect(screen.queryByText('read:user')).not.toBeInTheDocument();
    expect(screen.queryByText('GITLAB_OAUTH_CLIENT_ID')).not.toBeInTheDocument();
  });

  it('refuses to pretend GitLab is installable, and says why', async () => {
    renderRouted(<ProviderDetail providerId="gitlab" canWrite />);

    expect(await screen.findByText('Not available yet')).toBeInTheDocument();
    expect(screen.getByText(/no GitLab OAuth handler/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /authorise with gitlab/i })).toBeDisabled();
  });

  it('offers an authorise action for GitHub', async () => {
    renderRouted(<ProviderDetail providerId="github" canWrite />);

    expect(await screen.findByRole('button', { name: /authorise with github/i })).toBeEnabled();
  });

  it('does not offer authorise to a read-only member', async () => {
    renderRouted(<ProviderDetail providerId="github" canWrite={false} />);

    await screen.findByRole('heading', { name: 'GitHub' });
    expect(screen.queryByRole('button', { name: /authorise with github/i })).not.toBeInTheDocument();
    expect(screen.getByText(/authorising needs admin authority/)).toBeInTheDocument();
  });
});
