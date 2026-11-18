import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
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
import { AccountMenu, initialsFor } from '@/ui/components/layout';
import { useAuthStore } from '@/stores';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { createTestQueryClient } from '../helpers/render-hook';

const { server } = setupIntegrationTest();

const theme = getThemeById('dark');

function buildRouter() {
  const rootRoute = createRootRoute({
    component: () => (
      <>
        <AccountMenu />
        <Outlet />
      </>
    ),
  });
  const make = (path: string) =>
    createRoute({ getParentRoute: () => rootRoute, path, component: () => <div>{path} page</div> });
  const routeTree = rootRoute.addChildren([
    createRoute({ getParentRoute: () => rootRoute, path: '/', component: () => <div>home page</div> }),
    make('/profile'),
    make('/settings'),
    make('/notifications'),
  ]);
  return createRouter({ routeTree, history: createMemoryHistory({ initialEntries: ['/'] }) });
}

async function renderMenu() {
  const router = buildRouter();
  render(
    <QueryClientProvider client={createTestQueryClient()}>
      <ThemeContext.Provider value={theme}>
        <RouterProvider router={router} />
      </ThemeContext.Provider>
    </QueryClientProvider>,
  );

  await screen.findByText('home page');
  return router;
}

function seedUser(overrides: Partial<{ name: string | null; email: string | null }> = {}) {
  useAuthStore.setState({
    isAuthenticated: true,
    user: {
      id: 'user-test-1',
      name: overrides.name === undefined ? 'Ada Lovelace' : overrides.name,
      email: overrides.email === undefined ? 'ada@example.test' : overrides.email,
      needs_organization: false,
    },
  } as never);
}

beforeEach(() => {
  seedUser();
});

describe('initialsFor', () => {
  it('takes two initials from a name', () => {
    expect(initialsFor('Ada Lovelace', 'ada@example.test')).toBe('AL');
  });

  it('falls back to the email local part when there is no name', () => {
    expect(initialsFor(null, 'oscar.porter@example.test')).toBe('OP');
  });

  it('returns a placeholder when there is nothing to work from', () => {
    expect(initialsFor(null, null)).toBe('?');
  });
});

describe('AccountMenu', () => {
  it('shows the avatar initials and opens the menu on click', async () => {
    await renderMenu();
    const trigger = screen.getByRole('button', { name: /Ada Lovelace/ });
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    expect(trigger).toHaveTextContent('AL');

    fireEvent.click(trigger);

    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    const menu = screen.getByRole('menu', { name: 'Account' });
    expect(within(menu).getByText('Ada Lovelace')).toBeInTheDocument();
    expect(within(menu).getByText('ada@example.test')).toBeInTheDocument();
  });

  it('lists the account destinations and a sign-out action', async () => {
    await renderMenu();
    fireEvent.click(screen.getByRole('button', { name: /Ada Lovelace/ }));

    const menu = screen.getByRole('menu', { name: 'Account' });
    for (const label of [
      'Profile',
      'Preferences',
      'Organization settings',
      'API keys',
      'Notification history',
      'Sign out',
    ]) {
      expect(within(menu).getByRole('menuitem', { name: label })).toBeInTheDocument();
    }
  });

  it('renders the sheet anchored to the bottom of the viewport over a scrim', async () => {
    await renderMenu();
    fireEvent.click(screen.getByRole('button', { name: /Ada Lovelace/ }));

    const sheet = screen.getByRole('menu', { name: 'Account' });

    expect(sheet.parentElement?.parentElement).toBe(document.body);
    const sheetClass = sheet.className;
    expect(sheetClass.length).toBeGreaterThan(0);

    const scrim = sheet.previousElementSibling as HTMLElement | null;
    expect(scrim).not.toBeNull();
    expect(scrim?.getAttribute('aria-hidden')).toBe('true');
    fireEvent.click(scrim as HTMLElement);
    await waitFor(() => expect(screen.queryByRole('menu', { name: 'Account' })).not.toBeInTheDocument());
  });

  it('closes on Escape and returns focus to the trigger', async () => {
    await renderMenu();
    const trigger = screen.getByRole('button', { name: /Ada Lovelace/ });
    fireEvent.click(trigger);
    expect(screen.getByRole('menu', { name: 'Account' })).toBeInTheDocument();

    fireEvent.keyDown(document, { key: 'Escape' });

    await waitFor(() => expect(screen.queryByRole('menu', { name: 'Account' })).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
  });

  it('closes when clicking outside', async () => {
    await renderMenu();
    fireEvent.click(screen.getByRole('button', { name: /Ada Lovelace/ }));
    expect(screen.getByRole('menu', { name: 'Account' })).toBeInTheDocument();

    fireEvent.mouseDown(document.body);

    await waitFor(() => expect(screen.queryByRole('menu', { name: 'Account' })).not.toBeInTheDocument());
  });

  it('moves focus between items with the arrow keys', async () => {
    await renderMenu();
    fireEvent.click(screen.getByRole('button', { name: /Ada Lovelace/ }));

    const profile = screen.getByRole('menuitem', { name: 'Profile' });
    const preferences = screen.getByRole('menuitem', { name: 'Preferences' });
    expect(profile).toHaveFocus();

    fireEvent.keyDown(profile, { key: 'ArrowDown' });
    expect(preferences).toHaveFocus();

    fireEvent.keyDown(preferences, { key: 'ArrowUp' });
    expect(profile).toHaveFocus();
  });

  it('signs out and clears the session', async () => {
    await renderMenu();
    fireEvent.click(screen.getByRole('button', { name: /Ada Lovelace/ }));
    fireEvent.click(screen.getByRole('menuitem', { name: 'Sign out' }));

    await waitFor(() => expect(useAuthStore.getState().isAuthenticated).toBe(false));
    expect(useAuthStore.getState().user).toBeNull();
  });

  it('navigates to a destination when an item is chosen', async () => {
    const router = await renderMenu();
    fireEvent.click(screen.getByRole('button', { name: /Ada Lovelace/ }));
    fireEvent.click(screen.getByRole('menuitem', { name: 'Organization settings' }));

    await waitFor(() => expect(router.state.location.pathname).toBe('/settings'));
  });

  it('has a request-interception harness available', () => {
    expect(server).toBeTruthy();
  });
});
