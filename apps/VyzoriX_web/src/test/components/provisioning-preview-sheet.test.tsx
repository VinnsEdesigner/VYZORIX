import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeContext, getThemeById, OverlayProvider } from '@vyzorix/ui';
import { PreviewSheet } from '@/ui/pages/admin-provisioning/preview-sheet';
import type { SyncJob } from '@/hooks/provisioning';

function renderSheet(props: Parameters<typeof PreviewSheet>[0]) {
  return render(
    <ThemeContext.Provider value={getThemeById('vyzorix-dark')}>
      <OverlayProvider mode="sheet">
        <PreviewSheet {...props} />
      </OverlayProvider>
    </ThemeContext.Provider>,
  );
}

const job: SyncJob = {
  id: 'job-1',
  repoId: 'repo-1',
  state: 'success',
  error: '',
  dryRun: true,
  applied: 2,
  skipped: 1,
  failed: 0,
  startedAt: null,
  finishedAt: null,
  results: [
    { resourceId: 'r1', path: 'devices/dev-0001/vyzorix.toml', outcome: 'applied', message: 'would apply' },
    { resourceId: 'r2', path: 'devices/dev-0002/vyzorix.toml', outcome: 'unchanged', message: '' },
    { resourceId: 'r3', path: 'orgs/Dev Organisation/alerts.toml', outcome: 'applied', message: '3 rules' },
  ],
};

describe('PreviewSheet', () => {
  it('shows the per-file outcomes a toast would have thrown away', () => {
    renderSheet({ job, repositoryName: 'Device configuration', onClose: vi.fn(), onApply: vi.fn(), isApplying: false });

    expect(screen.getByText('Preview: Device configuration')).toBeInTheDocument();
    expect(screen.getByText('devices/dev-0001/vyzorix.toml')).toBeInTheDocument();
    expect(screen.getByText('orgs/Dev Organisation/alerts.toml')).toBeInTheDocument();
    expect(screen.getByText('3 rules')).toBeInTheDocument();
  });

  it('summarises the counts', () => {
    renderSheet({ job, repositoryName: 'x', onClose: vi.fn(), onApply: vi.fn(), isApplying: false });

    expect(screen.getByText('Files to apply')).toBeInTheDocument();
    expect(screen.getByText('Files unchanged')).toBeInTheDocument();
    expect(screen.getByText('Files to fail')).toBeInTheDocument();
  });

  it('says plainly that nothing was written', () => {
    renderSheet({ job, repositoryName: 'x', onClose: vi.fn(), onApply: vi.fn(), isApplying: false });

    expect(screen.getByText('Nothing has been written.')).toBeInTheDocument();
  });

  it('offers the real sync as a separate action', () => {
    const onApply = vi.fn();
    renderSheet({ job, repositoryName: 'x', onClose: vi.fn(), onApply, isApplying: false });

    screen.getByRole('button', { name: 'Apply for real' }).click();
    expect(onApply).toHaveBeenCalled();
  });

  it('refuses to apply when the preview found failures', () => {
    const failing: SyncJob = {
      ...job,
      failed: 1,
      results: [{ resourceId: 'r9', path: 'devices/dev-0009/vyzorix.toml', outcome: 'failed', message: 'parse error' }],
    };
    renderSheet({ job: failing, repositoryName: 'x', onClose: vi.fn(), onApply: vi.fn(), isApplying: false });

    const button = screen.getByRole('button', { name: /fix the failures first/i });
    expect(button).toBeDisabled();
    expect(screen.getByText('parse error')).toBeInTheDocument();
  });

  it('is closed when there is no preview', () => {
    renderSheet({ job: undefined, repositoryName: 'x', onClose: vi.fn(), onApply: vi.fn(), isApplying: false });

    expect(screen.queryByText('Nothing has been written.')).not.toBeInTheDocument();
  });
});
