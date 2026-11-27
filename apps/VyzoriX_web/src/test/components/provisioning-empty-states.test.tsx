import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeContext, getThemeById } from '@vyzorix/ui';
import { emptyStateAvatar } from '@/lib/avatar';
import { EmptySurface, StatusSurface } from '@/ui/pages/admin-provisioning/states';
import { SourceListEmpty } from '@/ui/pages/admin-provisioning/states';

function renderThemed(ui: React.ReactElement) {
  return render(<ThemeContext.Provider value={getThemeById('vyzorix-dark')}>{ui}</ThemeContext.Provider>);
}

describe('emptyStateAvatar', () => {
  it('produces a data URI for every surface', () => {
    for (const kind of ['sources', 'credentials', 'resources', 'activity', 'providers', 'overview'] as const) {
      const uri = emptyStateAvatar(kind);
      expect(uri.startsWith('data:image/svg+xml')).toBe(true);
      expect(uri.length).toBeGreaterThan(200);
    }
  });

  it('is stable for the same surface, so a page does not shuffle its character', () => {
    expect(emptyStateAvatar('sources')).toBe(emptyStateAvatar('sources'));
  });

  it('differs between surfaces, so each empty page is recognisable', () => {
    const distinct = new Set(
      (['sources', 'credentials', 'resources', 'activity', 'providers', 'overview'] as const).map((kind) =>
        emptyStateAvatar(kind),
      ),
    );
    expect(distinct.size).toBe(6);
  });
});

describe('EmptySurface', () => {
  it('renders a large illustration, not a small icon', () => {
    const { container } = renderThemed(<EmptySurface illustration="sources" title="No sources yet" detail="Add one." />);

    const image = container.querySelector('img');
    expect(image).not.toBeNull();
    expect(image?.getAttribute('width')).toBe('260');
    expect(image?.getAttribute('src')?.startsWith('data:image/svg+xml')).toBe(true);

    expect(image?.getAttribute('alt')).toBe('');
  });

  it('carries the title and detail a reader needs', () => {
    renderThemed(<EmptySurface illustration="credentials" title="No credentials yet" detail="Add a token." />);

    expect(screen.getByText('No credentials yet')).toBeInTheDocument();
    expect(screen.getByText('Add a token.')).toBeInTheDocument();
  });

  it('shows an action only when one is supplied', () => {
    const { rerender } = renderThemed(<EmptySurface illustration="sources" title="None" />);
    expect(screen.queryByRole('button')).not.toBeInTheDocument();

    rerender(
      <ThemeContext.Provider value={getThemeById('vyzorix-dark')}>
        <EmptySurface
          illustration="sources"
          title="None"
          actions={<button type="button">Add</button>}
        />
      </ThemeContext.Provider>,
    );
    expect(screen.getByRole('button', { name: 'Add' })).toBeInTheDocument();
  });
});

describe('StatusSurface', () => {
  it('keeps the compact icon shape for failures, so it does not shout', () => {
    const { container } = renderThemed(
      <StatusSurface icon="alert-triangle" tone="error" title="Could not load" detail="Try again." />,
    );

    expect(container.querySelector('img')).toBeNull();
    expect(screen.getByText('Could not load')).toBeInTheDocument();
  });
});

describe('SourceListEmpty', () => {
  it('uses the illustration and offers the add action', () => {
    const { container } = renderThemed(<SourceListEmpty onAdd={() => undefined} />);

    expect(container.querySelector('img')).not.toBeNull();
    expect(screen.getByText('No sources yet')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add a source/i })).toBeInTheDocument();
  });
});
