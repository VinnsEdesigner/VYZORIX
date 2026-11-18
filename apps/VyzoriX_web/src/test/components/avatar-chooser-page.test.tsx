import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';

vi.mock('@/lib/avatar', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/avatar')>();
  return {
    ...actual,
    renderAvatarPng: vi.fn(async () => new Blob(['png'], { type: 'image/png' })),
  };
});
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
import { AvatarChooserPage } from '@/ui/pages/profile/avatar-chooser-page';
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
    make('/profile', <div>profile</div>),
    make('/profile/avatar', <AvatarChooserPage />),
  ]);
  return createRouter({ routeTree, history: createMemoryHistory({ initialEntries: ['/profile/avatar'] }) });
}

async function renderChooser() {
  render(
    <QueryClientProvider client={createTestQueryClient()}>
      <ThemeContext.Provider value={theme}>
        <RouterProvider router={buildRouter()} />
      </ThemeContext.Provider>
    </QueryClientProvider>,
  );
  await screen.findByRole('img', { name: 'Avatar preview' });
}

beforeEach(() => {
  useAuthStore.setState({
    isAuthenticated: true,
    user: { id: 'user-test-1', name: 'Test User', email: 'test@vyzorix.com', avatar_url: '', needs_organization: false },
  } as never);
});

describe('AvatarChooserPage', () => {
  it('shows a preview seeded from the user name', async () => {
    await renderChooser();
    const preview = screen.getByRole('img', { name: 'Avatar preview' });
    expect(preview).toHaveAttribute('src');
    expect(preview.getAttribute('src')).toContain('data:image/svg+xml');
  });

  it('lists style tiles and marks the selected one', async () => {
    await renderChooser();
    const tiles = screen.getAllByRole('button', { pressed: true });
    expect(tiles.length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: /Notionists/ })).toBeInTheDocument();
  });

  it('changes the style when a tile is clicked', async () => {
    await renderChooser();
    const bottts = screen.getByRole('button', { name: /Bottts/ });
    fireEvent.click(bottts);
    expect(bottts).toHaveAttribute('aria-pressed', 'true');
  });

  it('filters the style catalogue', async () => {
    await renderChooser();
    fireEvent.click(screen.getByRole('button', { name: 'Abstract' }));
    expect(screen.getByRole('button', { name: /Identicon/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Adventurer/ })).not.toBeInTheDocument();
  });

  it('shuffles the seed', async () => {
    await renderChooser();
    const seedInput = screen.getByPlaceholderText('A word, a name, anything') as HTMLInputElement;
    const before = seedInput.value;
    fireEvent.click(screen.getByRole('button', { name: 'Shuffle' }));
    expect((screen.getByPlaceholderText('A word, a name, anything') as HTMLInputElement).value).not.toBe(before);
  });

  it('saves the rendered avatar through the upload endpoint', async () => {
    let uploaded = false;
    server.use(
      http.post('/v1/auth/me/avatar', () => {
        uploaded = true;
        return HttpResponse.json({ id: 'user-test-1', name: 'Test User', email: 'test@vyzorix.com', avatar_url: '/avatars/me.png' });
      }),
    );

    await renderChooser();
    fireEvent.click(screen.getByRole('button', { name: 'Save avatar' }));

    await waitFor(() => expect(uploaded).toBe(true));
  });
});
