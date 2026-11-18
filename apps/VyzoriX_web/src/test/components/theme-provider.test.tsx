import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { QueryClientProvider } from '@tanstack/react-query';
import type { LocalStoreRepository } from '@/lib/local-store';
import { getThemeById } from '@vyzorix/ui';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { createTestQueryClient } from '../helpers/render-hook';

function slowStore(cachedTheme: string): LocalStoreRepository {
  return {
    capability: async () => 'ready',
    open: async () => true,
    isWriter: () => true,
    select: async () => {
      await new Promise((resolve) => setTimeout(resolve, 50));
      return [{ theme: cachedTheme }] as never;
    },
    count: async () => 0,
    upsert: async () => {},
    deleteByKeys: async () => {},
    evictExpired: async () => 0,
    cursor: async () => null,
    setCursor: async () => {},
    clear: async () => {},
    close: async () => {},
  };
}

let store = slowStore('dark');

vi.mock('@/hooks/_shared', () => ({
  useLocalStore: () => store,
  useCurrentUserId: () => 'user-1',
  useCurrentUser: () => ({ id: 'user-1', name: 'T', email: 't@x.com', avatarUrl: null }),
}));

const { server } = setupIntegrationTest();

beforeEach(async () => {
  store = slowStore('dark');
  localStorage.clear();
  const { useThemeStore } = await import('@/stores/theme-store');
  const { useAuthStore } = await import('@/stores');
  useThemeStore.setState({
    themeId: 'dark',
    resolvedThemeId: 'dark',
    theme: getThemeById('dark'),
    followSystem: false,
    themeSettled: false,
  });
  useAuthStore.setState({
    isAuthenticated: true,
    sessionChecked: true,
    user: { id: 'user-1', name: 'T', email: 't@x.com', avatar_url: '', needs_organization: false },
  } as never);
});

describe('ThemeProvider', () => {
  it('applies the server-resolved theme even when a stale cache read lands later', async () => {
    server.use(
      http.get('/v1/me/preferences', () =>
        HttpResponse.json({ preferences: {}, resolved: { theme: 'light' }, theme: 'light' }),
      ),
    );

    const { ThemeProvider } = await import('@/providers/theme-provider');
    const { useThemeStore } = await import('@/stores/theme-store');

    render(
      <QueryClientProvider client={createTestQueryClient()}>
        <ThemeProvider>
          <div>child</div>
        </ThemeProvider>
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(useThemeStore.getState().themeId).toBe('light');
    });

    await new Promise((resolve) => setTimeout(resolve, 120));
    expect(useThemeStore.getState().themeId).toBe('light');
    expect(localStorage.getItem('vyzorix.theme')).toBe('light');
  });
});
