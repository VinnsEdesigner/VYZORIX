import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/react';
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router';
import { QueryClientProvider } from '@tanstack/react-query';
import { ThemeContext, getThemeById } from '@vyzorix/ui';
import { AppShell, Page, type NavSection } from '@/ui/components/layout';
import { createTestQueryClient } from '../helpers/render-hook';

const theme = getThemeById('dark');

const testNav: NavSection[] = [
  {
    id: 'devices',
    label: 'Devices',
    items: [
      { id: 'home', text: 'Home', url: '/', icon: 'home' },
      { id: 'devices', text: 'Devices', url: '/devices', icon: 'smartphone' },
    ],
  },
  {
    id: 'operations',
    label: 'Operations',
    items: [
      {
        id: 'logs',
        text: 'Logs',
        url: '/logs',
        icon: 'file-text',
        children: [
          { id: 'entries', text: 'Entries', url: '/logs?tab=entries' },
          { id: 'volume', text: 'Volume', url: '/logs?tab=volume' },
        ],
      },
    ],
  },
];

function buildRouter(initialPath: string) {
  const rootRoute = createRootRoute({
    component: () => (
      <AppShell nav={testNav}>
        <Outlet />
      </AppShell>
    ),
  });
  const indexRoute = createRoute({ getParentRoute: () => rootRoute, path: '/', component: () => <div>home page</div> });
  const devicesRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/devices',
    component: () => <div>devices page</div>,
  });
  const logsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/logs',
    component: () => <div>logs page</div>,
  });

  const routeTree = rootRoute.addChildren([indexRoute, devicesRoute, logsRoute]);
  return createRouter({ routeTree, history: createMemoryHistory({ initialEntries: [initialPath] }) });
}

function renderAt(path: string) {
  const router = buildRouter(path);
  render(

    <QueryClientProvider client={createTestQueryClient()}>
      <ThemeContext.Provider value={theme}>
        <RouterProvider router={router} />
      </ThemeContext.Provider>
    </QueryClientProvider>,
  );
  return router;
}

describe('AppShell', () => {
  it('renders the page content with the navigation closed by default', async () => {
    renderAt('/devices');
    expect(await screen.findByText('devices page')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Toggle menu' })).toHaveAttribute('aria-expanded', 'false');
    expect(document.getElementById('app-navigation')).toHaveAttribute('inert');
  });

  it('shows the page label in the breadcrumb', async () => {
    renderAt('/devices');
    await screen.findByText('devices page');
    expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toHaveTextContent('Devices');
  });

  it('renders the page tabs for a tabbed page', async () => {
    renderAt('/logs?tab=entries');
    await screen.findByText('logs page');
    const tabs = screen.getByRole('navigation', { name: 'Page tabs' });
    expect(tabs).toHaveTextContent('Entries');
    expect(tabs).toHaveTextContent('Volume');
  });

  it('opens the mega menu from the top bar and closes it with Escape', async () => {
    renderAt('/');
    await screen.findByText('home page');

    expect(screen.getByRole('button', { name: 'Toggle menu' })).toHaveAttribute('aria-expanded', 'false');
    expect(document.getElementById('app-navigation')).toHaveAttribute('inert');

    fireEvent.click(screen.getByRole('button', { name: 'Toggle menu' }));
    expect(screen.getByRole('button', { name: 'Close menu' })).toHaveAttribute('aria-expanded', 'true');
    expect(document.getElementById('app-navigation')).not.toHaveAttribute('inert');
    expect(screen.getByRole('navigation', { name: 'Main navigation' })).toHaveTextContent('Logs');

    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.getByRole('button', { name: 'Toggle menu' })).toHaveAttribute('aria-expanded', 'false');
    expect(document.getElementById('app-navigation')).toHaveAttribute('inert');
  });

  it('starts every group collapsed and expands one on click', async () => {
    renderAt('/');
    await screen.findByText('home page');
    fireEvent.click(screen.getByRole('button', { name: 'Toggle menu' }));

    const head = screen.getByRole('button', { name: 'Logs' });
    expect(head).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('link', { name: 'Entries' })).not.toBeInTheDocument();

    fireEvent.click(head);
    expect(screen.getByRole('button', { name: 'Logs' })).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('link', { name: 'Entries' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Logs' }));
    expect(screen.getByRole('button', { name: 'Logs' })).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('link', { name: 'Entries' })).not.toBeInTheDocument();
  });

  it('keeps the active group expanded', async () => {
    renderAt('/logs?tab=volume');
    await screen.findByText('logs page');
    fireEvent.click(screen.getByRole('button', { name: 'Toggle menu' }));

    const nav = screen.getByRole('navigation', { name: 'Main navigation' });
    expect(screen.getByRole('button', { name: 'Logs' })).toHaveAttribute('aria-expanded', 'true');
    expect(within(nav).getByRole('link', { name: 'Volume' })).toHaveAttribute('aria-current', 'page');
  });

  it('provides a skip link to the content', async () => {
    renderAt('/');
    await screen.findByText('home page');
    expect(screen.getByText('Skip to main content')).toHaveAttribute('href', '#pageContent');
  });
});

describe('Page', () => {
  it('renders the title and sub-title as a heading', () => {
    render(
      <ThemeContext.Provider value={theme}>
        <Page title="Alerts" subTitle="Rules and state">
          body
        </Page>
      </ThemeContext.Provider>,
    );
    expect(screen.getByRole('heading', { level: 1, name: 'Alerts' })).toBeInTheDocument();
    expect(screen.getByText('Rules and state')).toBeInTheDocument();
    expect(screen.getByText('body')).toBeInTheDocument();
  });

  it('renders actions beside the title', () => {
    render(
      <ThemeContext.Provider value={theme}>
        <Page title="Devices" actions={<button type="button">action-slot</button>}>
          body
        </Page>
      </ThemeContext.Provider>,
    );
    expect(screen.getByRole('button', { name: 'action-slot' })).toBeInTheDocument();
  });
});
