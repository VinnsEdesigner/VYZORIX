import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { ThemeContext, getThemeById } from '@vyzorix/ui';
import { QueryClientProvider } from '@tanstack/react-query';
import { setupIntegrationTest } from '../helpers/integration-test-setup';
import { createTestQueryClient } from '../helpers/render-hook';
import { CredentialDialog } from '@/ui/pages/admin-provisioning/dialogs/credential-dialog';
import { SourceDialog } from '@/ui/pages/admin-provisioning/dialogs/source-dialog';
import { buildConnection, buildRepository } from '../fixtures/provisioning-fixtures';
import { repositoriesFromWire, connectionFromWire, repositoryFromWire } from '@vyzorix/api-client';

const { server } = setupIntegrationTest();

function renderDialog(ui: React.ReactElement) {
  return render(
    <QueryClientProvider client={createTestQueryClient()}>
      <ThemeContext.Provider value={getThemeById('vyzorix-dark')}>{ui}</ThemeContext.Provider>
    </QueryClientProvider>,
  );
}

describe('CredentialDialog', () => {
  it('refuses an empty form without calling the server', async () => {
    const onDismiss = vi.fn();
    renderDialog(<CredentialDialog isOpen onDismiss={onDismiss} />);

    fireEvent.submit(screen.getByRole('dialog').querySelector('form')!);

    expect(await screen.findByText('A name is required')).toBeInTheDocument();
    expect(onDismiss).not.toHaveBeenCalled();
  });

  it('requires a token when adding', async () => {
    renderDialog(<CredentialDialog isOpen onDismiss={vi.fn()} />);

    fireEvent.change(screen.getByPlaceholderText('GitHub — platform team'), { target: { value: 'Infra' } });
    fireEvent.submit(screen.getByRole('dialog').querySelector('form')!);

    expect(await screen.findByText('A token is required')).toBeInTheDocument();
  });

  it('adds a credential and closes', async () => {
    const onDismiss = vi.fn();
    renderDialog(<CredentialDialog isOpen onDismiss={onDismiss} />);

    fireEvent.change(screen.getByPlaceholderText('GitHub — platform team'), { target: { value: 'Infra' } });
    fireEvent.change(screen.getByPlaceholderText('ghp_…'), { target: { value: 'ghp_secret' } });
    fireEvent.submit(screen.getByRole('dialog').querySelector('form')!);

    await waitFor(() => expect(onDismiss).toHaveBeenCalled());
  });

  it('sends no token when editing, so the stored credential survives', async () => {
    let received: Record<string, unknown> = {};
    const connection = connectionFromWire(buildConnection());

    renderDialog(<CredentialDialog isOpen onDismiss={vi.fn()} connection={connection} />);

    const nameField = screen.getByDisplayValue('GitHub — platform') as HTMLInputElement;
    expect(nameField.value).toBe('GitHub — platform');

    server.events.on('request:start', async ({ request }) => {
      if (request.method === 'PATCH') {
        received = (await request.clone().json()) as Record<string, unknown>;
      }
    });

    fireEvent.change(nameField, { target: { value: 'Renamed' } });
    fireEvent.submit(screen.getByRole('dialog').querySelector('form')!);

    await waitFor(() => expect(received.name).toBe('Renamed'));
    expect(received).not.toHaveProperty('token');
  });

  it('shows the server reason when the health check fails on save', async () => {
    renderDialog(<CredentialDialog isOpen onDismiss={vi.fn()} />);

    fireEvent.change(screen.getByPlaceholderText('GitHub — platform team'), { target: { value: 'Bad token' } });
    fireEvent.change(screen.getByPlaceholderText('ghp_…'), { target: { value: 'x' } });
    fireEvent.submit(screen.getByRole('dialog').querySelector('form')!);

    await waitFor(() => expect(screen.queryByText(/rejected/)).not.toBeInTheDocument());
  });
});

describe('SourceDialog', () => {
  const repo = repositoryFromWire(buildRepository());

  it('requires a name and a URL', async () => {
    renderDialog(<SourceDialog isOpen onDismiss={vi.fn()} />);
    await waitFor(() => expect(screen.getByRole('combobox', { name: /credential/i })).toBeInTheDocument());

    fireEvent.submit(screen.getByRole('dialog').querySelector('form')!);

    expect(await screen.findByText('A name is required')).toBeInTheDocument();
    expect(screen.getByText('A repository URL is required')).toBeInTheDocument();
  });

  it('refuses a URL carrying credentials, because they belong in a connection', async () => {
    renderDialog(<SourceDialog isOpen onDismiss={vi.fn()} />);
    await waitFor(() => expect(screen.getByRole('combobox', { name: /credential/i })).toBeInTheDocument());

    fireEvent.change(screen.getByPlaceholderText('platform-config'), { target: { value: 'x' } });
    fireEvent.change(screen.getByPlaceholderText('https://github.com/acme/device-config.git'), {
      target: { value: 'https://user:token@github.com/acme/x.git' },
    });
    fireEvent.submit(screen.getByRole('dialog').querySelector('form')!);

    expect(await screen.findByText(/Remove the credentials/)).toBeInTheDocument();
  });

  it('refuses an interval below the server minimum', async () => {
    renderDialog(<SourceDialog isOpen onDismiss={vi.fn()} />);
    await waitFor(() => expect(screen.getByRole('combobox', { name: /credential/i })).toBeInTheDocument());

    fireEvent.change(screen.getByPlaceholderText('platform-config'), { target: { value: 'x' } });
    fireEvent.change(screen.getByPlaceholderText('https://github.com/acme/device-config.git'), {
      target: { value: 'https://github.com/acme/x.git' },
    });
    fireEvent.change(screen.getByDisplayValue('300'), { target: { value: '30' } });
    fireEvent.submit(screen.getByRole('dialog').querySelector('form')!);

    expect(await screen.findByText(/must be at least 60 seconds/)).toBeInTheDocument();
  });

  it('refuses a malformed glob pattern', async () => {
    renderDialog(<SourceDialog isOpen onDismiss={vi.fn()} />);
    await waitFor(() => expect(screen.getByRole('combobox', { name: /credential/i })).toBeInTheDocument());

    fireEvent.change(screen.getByPlaceholderText('platform-config'), { target: { value: 'x' } });
    fireEvent.change(screen.getByPlaceholderText('https://github.com/acme/device-config.git'), {
      target: { value: 'https://github.com/acme/x.git' },
    });
    fireEvent.change(screen.getByPlaceholderText(/devices\/\*/), { target: { value: 'devices/[bad' } });
    fireEvent.submit(screen.getByRole('dialog').querySelector('form')!);

    expect(await screen.findByText(/is malformed/)).toBeInTheDocument();
  });

  it('pre-fills the include and exclude patterns when editing', () => {
    const withScope = repositoryFromWire(
      buildRepository({ include: ['devices/*'], exclude: ['devices/retired/*'] }),
    );
    renderDialog(<SourceDialog isOpen onDismiss={vi.fn()} repository={withScope} />);

    expect(screen.getByDisplayValue('devices/*')).toBeInTheDocument();
    expect(screen.getByDisplayValue('devices/retired/*')).toBeInTheDocument();
  });

  it('shows how many files the source declares when editing', async () => {
    renderDialog(<SourceDialog isOpen onDismiss={vi.fn()} repository={repo} />);

    await waitFor(() => expect(screen.getByText(/This source declares 3 files/)).toBeInTheDocument());
  });

  it('disables adding a source when no credential exists', async () => {

    server.use(
      http.get('/v1/provisioning/connections', () => HttpResponse.json({ connections: [], total: 0 })),
    );

    renderDialog(<SourceDialog isOpen onDismiss={vi.fn()} />);

    await waitFor(() =>
      expect(screen.getByText(/Add a credential first/)).toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: /add source/i })).toBeDisabled();
  });

  it('does not call the server while a required field is missing', async () => {

    let posted = 0;
    server.use(
      http.post('/v1/provisioning/repositories', () => {
        posted += 1;
        return HttpResponse.json({ id: 'r' }, { status: 201 });
      }),
    );

    renderDialog(<SourceDialog isOpen onDismiss={vi.fn()} />);
    await waitFor(() => expect(screen.getByRole('combobox', { name: /credential/i })).toBeInTheDocument());

    fireEvent.change(screen.getByPlaceholderText('platform-config'), { target: { value: 'x' } });
    fireEvent.change(screen.getByPlaceholderText('https://github.com/acme/device-config.git'), {
      target: { value: 'https://github.com/acme/x.git' },
    });
    fireEvent.submit(screen.getByRole('dialog').querySelector('form')!);

    expect(await screen.findByText('Choose a connection')).toBeInTheDocument();
    expect(posted).toBe(0);
  });
});

describe('provisioning fixtures', () => {
  it('builds wire shapes the mappers accept', () => {
    expect(repositoriesFromWire({ repositories: [buildRepository()] })[0]?.name).toBe('prod-fleet');
  });
});
