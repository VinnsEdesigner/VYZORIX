import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
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
import { ProfilePage } from '@/ui/pages/profile/profile-page';
import { describeUserAgent } from '@/lib/session/describe-user-agent';
import { useAuthStore } from '@/stores';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { createTestQueryClient } from '../helpers/render-hook';

const { server } = setupIntegrationTest();

const theme = getThemeById('dark');

function buildRouter() {
  const rootRoute = createRootRoute({ component: () => <Outlet /> });
  const make = (path: string, el: React.ReactNode) =>
    createRoute({ getParentRoute: () => rootRoute, path, component: () => <>{el}</> });
  const routeTree = rootRoute.addChildren([
    make('/', <div>home</div>),
    make('/profile', <ProfilePage />),
  ]);
  return createRouter({ routeTree, history: createMemoryHistory({ initialEntries: ['/profile'] }) });
}

async function renderProfile() {
  const router = buildRouter();
  render(
    <QueryClientProvider client={createTestQueryClient()}>
      <ThemeContext.Provider value={theme}>
        <RouterProvider router={router} />
      </ThemeContext.Provider>
    </QueryClientProvider>,
  );

  await screen.findAllByText('test@vyzorix.com');

  await waitFor(() => {
    const joined = screen.getByText('Joined').parentElement;
    expect(joined?.textContent).toMatch(/\d{4}/);
  });
}

function seedUser(overrides: Partial<{ name: string | null; email: string | null; avatar_url: string }> = {}) {
  useAuthStore.setState({
    isAuthenticated: true,
    user: {
      id: 'user-test-1',
      name: overrides.name === undefined ? 'Test User' : overrides.name,
      email: overrides.email === undefined ? 'test@vyzorix.com' : overrides.email,
      avatar_url: overrides.avatar_url ?? '',
      needs_organization: false,
    },
  } as never);
}

beforeEach(() => {
  seedUser();
});

describe('describeUserAgent', () => {
  it('names the browser and OS', () => {
    expect(describeUserAgent('Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/153.0 Safari/537.36')).toBe(
      'Chrome · Linux',
    );
  });

  it('prefers Edge over the Chrome token it embeds', () => {
    expect(describeUserAgent('Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 Chrome/120 Safari/537.36 Edg/120')).toBe(
      'Edge · Windows',
    );
  });

  it('falls back to the raw string when nothing is recognised', () => {
    expect(describeUserAgent('curl/8.0')).toBe('curl/8.0');
  });

  it('handles a missing value', () => {
    expect(describeUserAgent(null)).toBe('Unknown');
  });
});

describe('ProfilePage', () => {
  it('shows the avatar initials and identity details', async () => {
    await renderProfile();

    expect(screen.getAllByText('Test User').length).toBeGreaterThan(0);
    expect(screen.getByText('TU')).toBeInTheDocument();
    expect(screen.getAllByText('test@vyzorix.com').length).toBeGreaterThan(0);
  });

  it('shows the current session IP and browser', async () => {
    await renderProfile();

    expect(await screen.findByText('192.168.1.1')).toBeInTheDocument();
    expect(screen.getByText(/Chrome/)).toBeInTheDocument();
  });

  it('shows the activity counts from the summary endpoint', async () => {
    await renderProfile();

    const sessionsStat = await screen.findByText('Active sessions');
    await waitFor(() =>
      expect(within(sessionsStat.parentElement as HTMLElement).getByText('1')).toBeInTheDocument(),
    );
    const actionsStat = screen.getByText('Actions');
    await waitFor(() =>
      expect(within(actionsStat.parentElement as HTMLElement).getByText('42')).toBeInTheDocument(),
    );
  });

  it('hides the display-name editor until Edit is clicked', async () => {
    await renderProfile();

    expect(screen.queryByDisplayValue('Test User')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Edit profile' }));

    expect(await screen.findByDisplayValue('Test User')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
  });

  it('shows the locked account email and full name in the editor', async () => {
    await renderProfile();
    fireEvent.click(screen.getByRole('button', { name: 'Edit profile' }));

    const editCard = screen.getByRole('heading', { name: 'Profile' }).closest('section') as HTMLElement;
    expect(within(editCard).getByText('test@vyzorix.com')).toBeInTheDocument();
    expect(within(editCard).getByText('Full name')).toBeInTheDocument();

    expect(within(editCard).queryAllByRole('textbox').length).toBe(1);
  });

  it('saves the display name and collapses the editor', async () => {
    await renderProfile();
    fireEvent.click(screen.getByRole('button', { name: 'Edit profile' }));

    const input = await screen.findByDisplayValue('Test User');
    fireEvent.change(input, { target: { value: 'Ada Lovelace' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument());
  });

  it('labels the upload control "Upload profile picture" when there is no picture', async () => {
    await renderProfile();
    expect(screen.getByRole('button', { name: 'Upload profile picture' })).toBeInTheDocument();
  });

  it('keeps the upload control disabled until Edit profile is clicked', async () => {
    await renderProfile();

    const uploadButton = screen.getByRole('button', { name: 'Upload profile picture' });
    expect(uploadButton).toBeDisabled();

    fireEvent.click(screen.getByRole('button', { name: 'Edit profile' }));

    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Upload profile picture' })).toBeEnabled(),
    );
  });

  it('does not upload directly: picking a file opens the editor instead', async () => {
    await renderProfile();
    fireEvent.click(screen.getByRole('button', { name: 'Edit profile' }));

    const input = screen.getByTestId('avatar-input') as HTMLInputElement;
    const file = new File(['x'], 'photo.png', { type: 'image/png' });
    fireEvent.change(input, { target: { files: [file] } });

    expect(await screen.findByRole('dialog', { name: 'Edit profile picture' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save picture' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Discard' })).toBeInTheDocument();
  });

  it('discards the preview without uploading', async () => {
    await renderProfile();
    fireEvent.click(screen.getByRole('button', { name: 'Edit profile' }));

    const input = screen.getByTestId('avatar-input') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [new File(['x'], 'photo.png', { type: 'image/png' })] } });

    await screen.findByRole('dialog', { name: 'Edit profile picture' });
    fireEvent.click(screen.getByRole('button', { name: 'Discard' }));

    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: 'Edit profile picture' })).not.toBeInTheDocument(),
    );
  });

  it('labels the upload control "Change profile picture" when one exists', async () => {

    seedUser({ avatar_url: '/avatars/me.png' });
    server.use(
      http.get('/v1/auth/me', () =>
        HttpResponse.json({
          id: 'user-test-1',
          email: 'test@vyzorix.com',
          name: 'Test User',
          avatar_url: '/avatars/me.png',
          created_at: 1767225600000,
          last_login_at: 1767312000000,
          mfa_enabled: false,
          email_verified: true,
          needs_organization: false,
          organizations: [],
        }),
      ),
    );
    await renderProfile();
    expect(await screen.findByRole('button', { name: 'Change profile picture' })).toBeInTheDocument();
  });

  it('keeps the security control unlinked (disabled)', async () => {
    await renderProfile();
    expect(screen.getByRole('button', { name: 'Manage' })).toBeDisabled();
  });

  it('gates Choose an avatar behind Edit profile', async () => {
    await renderProfile();

    const chooseButtons = screen.getAllByRole('button', { name: 'Choose an avatar' });
    expect(chooseButtons.length).toBeGreaterThan(0);
    chooseButtons.forEach((button) => expect(button).toBeDisabled());

    fireEvent.click(screen.getByRole('button', { name: 'Edit profile' }));

    await waitFor(() => {
      screen.getAllByRole('button', { name: 'Choose an avatar' }).forEach((button) => expect(button).toBeEnabled());
    });
  });

  it('opens the full picture in a dialog when the avatar is clicked', async () => {
    seedUser({ avatar_url: '/avatars/me.png' });
    server.use(
      http.get('/v1/auth/me', () =>
        HttpResponse.json({
          id: 'user-test-1',
          email: 'test@vyzorix.com',
          name: 'Test User',
          avatar_url: '/avatars/me.png',
          created_at: 1767225600000,
          last_login_at: 1767312000000,
          mfa_enabled: false,
          email_verified: true,
          needs_organization: false,
          organizations: [],
        }),
      ),
    );
    await renderProfile();

    fireEvent.click(await screen.findByRole('button', { name: 'View full profile picture' }));

    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    expect(screen.getByRole('img', { name: /profile picture/ })).toHaveAttribute('src', '/avatars/me.png');

    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });
});
