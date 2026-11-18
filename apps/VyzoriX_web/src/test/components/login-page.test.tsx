import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
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
import { LoginPage } from '@/ui/pages/login/login-page';
import { useAuthStore } from '@/stores';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { createTestQueryClient } from '../helpers/render-hook';

const { server } = setupIntegrationTest();

const theme = getThemeById('dark');

function buildRouter() {
  const rootRoute = createRootRoute({ component: () => <Outlet /> });
  const tree = rootRoute.addChildren([
    createRoute({ getParentRoute: () => rootRoute, path: '/', component: () => <div>home page</div> }),
    createRoute({ getParentRoute: () => rootRoute, path: '/login', component: () => <LoginPage /> }),
  ]);
  return createRouter({ routeTree: tree, history: createMemoryHistory({ initialEntries: ['/login'] }) });
}

function renderLogin() {
  const router = buildRouter();
  render(
    <QueryClientProvider client={createTestQueryClient()}>
      <ThemeContext.Provider value={theme}>
        <RouterProvider router={router} />
      </ThemeContext.Provider>
    </QueryClientProvider>,
  );
  return router;
}

beforeEach(() => {
  useAuthStore.setState({
    isAuthenticated: false,
    user: null,
    accessToken: null,
    refreshToken: null,
    organizationId: null,
    mfaChallenge: null,
  } as never);
});

describe('LoginPage', () => {
  it('renders the email and password fields and a sign-in button', async () => {
    renderLogin();
    expect(await screen.findByLabelText("Email")).toBeInTheDocument();
    expect(screen.getByLabelText("Password")).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeInTheDocument();
  });

  it('disables sign-in until both fields are filled', async () => {
    renderLogin();
    const button = await screen.findByRole('button', { name: 'Sign in' });
    expect(button).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Email"), { target: { value: 'dev@vyzorix.local' } });
    expect(button).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Password"), { target: { value: 'DevPassword123!' } });
    expect(button).toBeEnabled();
  });

  it('signs in with valid credentials and authenticates the store', async () => {
    renderLogin();
    fireEvent.change(await screen.findByLabelText("Email"), { target: { value: 'dev@vyzorix.local' } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: 'DevPassword123!' } });
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    await waitFor(() => expect(useAuthStore.getState().isAuthenticated).toBe(true));

    expect(useAuthStore.getState().user?.email).toBe('test@vyzorix.com');
  });

  it('shows an error when the credentials are rejected', async () => {

    server.use(
      http.post('/v1/auth/login', () =>
        HttpResponse.json({ error: 'invalid_credentials', message: 'Invalid email or password' }, { status: 401 }),
      ),
    );

    renderLogin();
    fireEvent.change(await screen.findByLabelText("Email"), { target: { value: 'dev@vyzorix.local' } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: 'wrong-password' } });
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByText(/invalid email or password/i)).toBeInTheDocument();
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
  });
});
