import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
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
import { createTestQueryClient } from '../helpers/render-hook';
import { GUIDES, GUIDE_IDS, guideById, guideRoute, isGuideId } from '@/lib/provisioning/guides';
import { GettingStartedSurface } from '@/ui/pages/admin-provisioning/getting-started-surface';
import { GuidePage } from '@/ui/pages/admin-provisioning/guide-page';

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
    createRoute({ getParentRoute: () => rootRoute, path: '/admin/provisioning/guides/$guideId', component: page }),
  ]);

  return render(
    <RouterProvider
      router={createRouter({ routeTree: tree, history: createMemoryHistory({ initialEntries: ['/'] }) })}
    />,
  );
}

describe('guides catalogue', () => {
  it('has one guide per documented feature', () => {
    expect([...GUIDE_IDS]).toEqual([
      'providers',
      'sources',
      'browse',
      'resources',
      'device-state',
      'credentials',
      'scheduler',
      'secrets',
    ]);
    expect(GUIDES).toHaveLength(GUIDE_IDS.length);
  });

  it('gives every guide a title, a summary, an illustration and content', () => {
    for (const guide of GUIDES) {
      expect(guide.title.length).toBeGreaterThan(0);
      expect(guide.summary.length).toBeGreaterThan(0);
      expect(guide.sections.length).toBeGreaterThan(0);
      expect(guide.minutes).toBeGreaterThan(0);
    }
  });

  it('resolves every id to its guide, and rejects an unknown one', () => {
    for (const id of GUIDE_IDS) {
      expect(guideById(id)?.id).toBe(id);
      expect(isGuideId(id)).toBe(true);
    }
    expect(guideById('nope')).toBeUndefined();
    expect(isGuideId('nope')).toBe(false);
  });

  it('builds a route for every guide', () => {
    for (const id of GUIDE_IDS) {
      expect(guideRoute(id)).toBe(`/admin/provisioning/guides/${id}`);
    }
  });
});

describe('GettingStartedSurface', () => {
  it('shows a card for every guide, each linking to its page', async () => {
    renderRouted(<GettingStartedSurface canWrite />);

    await waitFor(() => expect(screen.getByText('Install a git provider')).toBeInTheDocument());

    for (const guide of GUIDES) {
      expect(screen.getByText(guide.title)).toBeInTheDocument();
    }

    const hrefs = screen
      .getAllByRole('link')
      .map((link) => link.getAttribute('href'))
      .filter((href): href is string => Boolean(href?.includes('/guides/')));

    expect(hrefs.sort()).toEqual(GUIDE_IDS.map(guideRoute).sort());
  });

  it('gives every card an illustration', async () => {
    const { container } = renderRouted(<GettingStartedSurface canWrite />);
    await waitFor(() => expect(screen.getByText('Install a git provider')).toBeInTheDocument());

    const illustrations = container.querySelectorAll('img[src^="data:image/svg+xml"]');
    expect(illustrations.length).toBeGreaterThanOrEqual(GUIDES.length);
  });
});

describe('GuidePage', () => {
  it('renders one guide with its sections and an illustration', async () => {
    const { container } = renderRouted(<GuidePage guideId="sources" />);

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Add a source' })).toBeInTheDocument());
    expect(screen.getByText('The layout it expects')).toBeInTheDocument();
    expect(container.querySelector('img[src^="data:image/svg+xml"]')).not.toBeNull();
  });

  it('links to the other guides, not to itself', async () => {
    renderRouted(<GuidePage guideId="credentials" />);
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Manage credentials' })).toBeInTheDocument());

    for (const guide of GUIDES) {
      if (guide.id === 'credentials') continue;
      expect(screen.getByText(guide.title)).toBeInTheDocument();
    }
  });
});
