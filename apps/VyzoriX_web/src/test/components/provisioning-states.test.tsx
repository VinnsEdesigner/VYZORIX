import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeContext, getThemeById } from '@vyzorix/ui';
import {
  CredentialError,
  CredentialPending,
  CredentialReady,
  DevicePresenceLine,
  DeviceSyncBlocked,
  DeviceSyncRunning,
  ResourceApplied,
  ResourceDrifted,
  ResourceError,
  ResourceIgnored,
  ResourceMissing,
  ResourceStateBadge,
  SecretMissing,
  SecretReady,
  SecretReadinessList,
  SourceListEmpty,
  SourceListError,
  SourceListNoMatches,
  SourceSyncBadge,
  SyncFailed,
  SyncIdle,
  SyncOutcome,
  SyncPartialFailure,
  SyncRunning,
  SyncSucceeded,
} from '@/ui/pages/admin-provisioning/states';

function withTheme(ui: React.ReactElement) {
  return <ThemeContext.Provider value={getThemeById('vyzorix-dark')}>{ui}</ThemeContext.Provider>;
}

function renderThemed(ui: React.ReactElement) {
  const result = render(withTheme(ui));

  return { ...result, rerender: (next: React.ReactElement) => result.rerender(withTheme(next)) };
}

describe('source list states', () => {
  it('teaches what a source is when there are none', () => {
    renderThemed(<SourceListEmpty />);
    expect(screen.getByText('No sources yet')).toBeInTheDocument();
    expect(screen.getByText(/git repository whose manifests describe your devices/)).toBeInTheDocument();
  });

  it('offers an add action only when the caller can write', () => {
    const { rerender } = renderThemed(<SourceListEmpty />);
    expect(screen.queryByRole('button', { name: /add a source/i })).not.toBeInTheDocument();

    rerender(<SourceListEmpty onAdd={vi.fn()} />);
    expect(screen.getByRole('button', { name: /add a source/i })).toBeInTheDocument();
  });

  it('describes a server failure in words, and offers a retry', () => {
    const onRetry = vi.fn();
    renderThemed(<SourceListError error={new Error('the server failed')} onRetry={onRetry} />);

    expect(screen.getByText('Could not load sources')).toBeInTheDocument();
    screen.getByRole('button', { name: /try again/i }).click();
    expect(onRetry).toHaveBeenCalled();
  });

  it('distinguishes a filtered-out list from an empty one', () => {
    renderThemed(<SourceListNoMatches onClear={vi.fn()} />);
    expect(screen.getByText('No sources match that filter')).toBeInTheDocument();
    expect(screen.queryByText('No sources yet')).not.toBeInTheDocument();
  });
});

describe('sync badges', () => {
  it('names each repository sync state in a word, never only a colour', () => {
    const cases: Array<[Parameters<typeof SourceSyncBadge>[0]['state'], string]> = [
      ['success', 'Synced'],
      ['warning', 'Partial'],
      ['working', 'Syncing'],
      ['error', 'Failed'],
      ['disabled', 'Disabled'],
      ['pending', 'Not synced'],
    ];

    for (const [state, label] of cases) {
      const { unmount } = renderThemed(<SourceSyncBadge state={state} />);
      expect(screen.getByText(label)).toBeInTheDocument();
      unmount();
    }
  });

  it('separates a never-checked credential from a failed one', () => {
    const { rerender } = renderThemed(<CredentialPending />);
    expect(screen.getByText('Not checked yet')).toBeInTheDocument();

    rerender(<CredentialError reason="the token was rejected" />);
    expect(screen.getByText('the token was rejected')).toBeInTheDocument();

    rerender(<CredentialReady host="github.com" />);
    expect(screen.getByText('Reached github.com')).toBeInTheDocument();
  });
});

describe('resource states', () => {
  it('labels every drift state distinctly', () => {
    const cases: Array<[Parameters<typeof ResourceStateBadge>[0]['state'], string]> = [
      ['applied', 'Applied'],
      ['drifted', 'Drifted'],
      ['missing', 'Missing'],
      ['error', 'Failed'],
      ['ignored', 'Out of scope'],
    ];

    for (const [state, label] of cases) {
      const { unmount } = renderThemed(<ResourceStateBadge state={state} />);
      expect(screen.getByText(label)).toBeInTheDocument();
      unmount();
    }
  });

  it('explains what drifted means, because it needs a decision', () => {
    renderThemed(<ResourceDrifted detail="The device was changed outside this source." />);
    expect(screen.getByText('Drifted')).toBeInTheDocument();
    expect(screen.getByText(/changed outside this source/)).toBeInTheDocument();
  });

  it('never describes a missing resource as reverted', () => {
    renderThemed(<ResourceMissing detail="The file was removed from the repository. The device keeps what it runs." />);
    expect(screen.getByText(/keeps what it runs/)).toBeInTheDocument();
  });

  it('does not conflate out-of-scope with missing', () => {
    renderThemed(<ResourceIgnored detail="Outside this source's include/exclude scope. Nothing reverts." />);
    expect(screen.getByText('Out of scope')).toBeInTheDocument();
    expect(screen.queryByText('Missing')).not.toBeInTheDocument();
  });

  it('renders each state component', () => {
    renderThemed(
      <>
        <ResourceApplied detail="in sync" />
        <ResourceError detail="parse error" />
      </>,
    );
    expect(screen.getByText('Applied')).toBeInTheDocument();
    expect(screen.getByText('Failed')).toBeInTheDocument();
    expect(screen.getByText('parse error')).toBeInTheDocument();
  });
});

describe('device presence', () => {
  it('says online and offline plainly', () => {
    const { unmount } = renderThemed(<DevicePresenceLine online={true} />);
    expect(screen.getByText('Online')).toBeInTheDocument();
    unmount();

    renderThemed(<DevicePresenceLine online={false} />);
    expect(screen.getByText('Offline')).toBeInTheDocument();
  });

  it('never calls an unknown device offline', () => {
    // A null presence is "we could not read it", which is not the same as "it is down".
    renderThemed(<DevicePresenceLine online={null} />);
    expect(screen.getByText('Reachability unknown')).toBeInTheDocument();
    expect(screen.queryByText('Offline')).not.toBeInTheDocument();
  });

  it('shows what the device last reported', () => {
    renderThemed(<DevicePresenceLine online={false} reportedVersion={7} />);
    expect(screen.getByText(/reports v7/)).toBeInTheDocument();
  });
});

describe('sync states', () => {
  it('explains what a sync does before it is run', () => {
    renderThemed(<SyncIdle />);
    expect(screen.getByText('No sync has run yet')).toBeInTheDocument();
    expect(screen.getByText(/Nothing is written until you run one/)).toBeInTheDocument();
  });

  it('reports the applied count with correct pluralisation', () => {
    const { rerender } = renderThemed(<SyncSucceeded applied={1} />);
    expect(screen.getByText('Applied 1 manifest')).toBeInTheDocument();

    rerender(<SyncSucceeded applied={3} />);
    expect(screen.getByText('Applied 3 manifests')).toBeInTheDocument();
  });

  it('says everything was already up to date when nothing applied', () => {
    renderThemed(<SyncSucceeded applied={0} />);
    expect(screen.getByText('Everything was already up to date')).toBeInTheDocument();
  });

  it('reports a partial failure as both counts, not as success', () => {
    renderThemed(<SyncPartialFailure applied={2} failed={1} />);
    expect(screen.getByText('Applied 2, failed 1')).toBeInTheDocument();
    expect(screen.queryByText(/^Applied 2$/)).not.toBeInTheDocument();
  });

  it('reports a total failure', () => {
    renderThemed(<SyncFailed error={new Error('the remote is unreachable')} />);
    expect(screen.getByText('The sync failed')).toBeInTheDocument();
  });

  it('shows a running indicator, so a sync cannot be started twice', () => {
    renderThemed(<SyncRunning />);
    expect(screen.getByText('Syncing…')).toBeInTheDocument();
  });

  it('renders the phase through one component rather than an inline chain', () => {
    const { rerender } = renderThemed(<SyncOutcome phase="succeeded" applied={4} />);
    expect(screen.getByText('Applied 4 manifests')).toBeInTheDocument();

    rerender(<SyncOutcome phase="partial" applied={4} failed={2} />);
    expect(screen.getByText('Applied 4, failed 2')).toBeInTheDocument();

    rerender(<SyncOutcome phase="failed" error={new Error('boom')} />);
    expect(screen.getByText('The sync failed')).toBeInTheDocument();

    rerender(<SyncOutcome phase="idle" />);
    expect(screen.getByText('No sync has run yet')).toBeInTheDocument();
  });

  it('names the device being synced', () => {
    renderThemed(<DeviceSyncRunning deviceId="living-room" />);
    expect(screen.getByText(/Syncing living-room/)).toBeInTheDocument();
  });

  it('says why a sync button is disabled instead of looking broken', () => {
    renderThemed(<DeviceSyncBlocked reason="A repository sync is running." />);
    expect(screen.getByText('Sync unavailable')).toBeInTheDocument();
    expect(screen.getByText('A repository sync is running.')).toBeInTheDocument();
  });
});

describe('secret readiness', () => {
  it('names each declared key and whether it is set, and never a value', () => {
    renderThemed(
      <SecretReadinessList
        readiness={{
          secrets: [
            { app: 'dashboard', key: 'DB_URL', set: true },
            { app: 'dashboard', key: 'API_KEY', set: false },
          ],
          total: 2,
          missing: 1,
        }}
      />,
    );

    expect(screen.getByText('DB_URL')).toBeInTheDocument();
    expect(screen.getByText('API_KEY')).toBeInTheDocument();
    expect(screen.getByText('Set')).toBeInTheDocument();
    expect(screen.getByText('Not set')).toBeInTheDocument();
  });

  it('says plainly when a manifest declares nothing', () => {
    renderThemed(<SecretReadinessList readiness={{ secrets: [], total: 0, missing: 0 }} />);
    expect(screen.getByText('This manifest declares no secrets.')).toBeInTheDocument();
  });

  it('renders the two states individually', () => {
    renderThemed(
      <>
        <SecretReady app="dashboard" secretKey="DB_URL" />
        <SecretMissing app="dashboard" secretKey="API_KEY" />
      </>,
    );
    expect(screen.getByText('DB_URL')).toBeInTheDocument();
    expect(screen.getByText('API_KEY')).toBeInTheDocument();
  });
});
