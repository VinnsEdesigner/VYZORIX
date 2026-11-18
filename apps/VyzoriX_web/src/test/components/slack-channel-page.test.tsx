import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { QueryClientProvider } from '@tanstack/react-query';
import { ThemeContext, getThemeById } from '@vyzorix/ui';
import { SlackChannelPage } from '@/ui/pages/profile/slack-channel-page';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { createTestQueryClient } from '../helpers/render-hook';

const { server } = setupIntegrationTest();

const NOT_CONNECTED = {
  enabled: true,
  channels: ['email'],
  email: {},
  push: {},
  webhook: { enabled: false, url: '', types: [] },
  slack: { connected: false },
};

const CONNECTED = {
  ...NOT_CONNECTED,
  channels: ['email', 'slack'],
  slack: { connected: true, teamName: 'Acme', channelId: 'C1', channelName: 'alerts' },
};

function renderPage() {
  render(
    <QueryClientProvider client={createTestQueryClient()}>
      <ThemeContext.Provider value={getThemeById('dark')}>
        <SlackChannelPage />
      </ThemeContext.Provider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  server.use(http.get('/v1/me/notifications', () => HttpResponse.json(NOT_CONNECTED)));
});

describe('SlackChannelPage', () => {
  it('offers Connect Slack when not linked', async () => {
    renderPage();
    expect(await screen.findByRole('button', { name: 'Connect Slack' })).toBeInTheDocument();
  });

  it('shows the workspace and channel picker once linked', async () => {
    server.use(
      http.get('/v1/me/notifications', () => HttpResponse.json(CONNECTED)),
      http.get('/v1/auth/me/notifications/slack/channels', () =>
        HttpResponse.json({ channels: [{ id: 'C1', name: 'alerts' }, { id: 'C2', name: 'ops' }] }),
      ),
    );
    renderPage();

    expect(await screen.findByText('Acme')).toBeInTheDocument();
    expect(screen.getByText('Connected')).toBeInTheDocument();
    expect(screen.getByLabelText('Post to channel')).toBeInTheDocument();
  });

  it('fetches the channel list for the linked workspace', async () => {
    let fetched = false;
    server.use(
      http.get('/v1/me/notifications', () => HttpResponse.json(CONNECTED)),
      http.get('/v1/auth/me/notifications/slack/channels', () => {
        fetched = true;
        return HttpResponse.json({ channels: [{ id: 'C1', name: 'alerts' }] });
      }),
    );
    renderPage();
    await screen.findByLabelText('Post to channel');
    await new Promise((resolve) => setTimeout(resolve, 30));
    expect(fetched).toBe(true);
  });
});
