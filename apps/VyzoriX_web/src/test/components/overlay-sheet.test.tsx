import { describe, it, expect, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import {
  Combobox,
  Menu,
  OverlayProvider,
  ThemeContext,
  getThemeById,
  useOverlay,
} from '@vyzorix/ui';

function renderThemed(ui: React.ReactElement, themeId = 'dark') {
  return render(<ThemeContext.Provider value={getThemeById(themeId)}>{ui}</ThemeContext.Provider>);
}

const options = [
  { value: 'viewer', label: 'Viewer' },
  { value: 'operator', label: 'Operator' },
];

function PresentationProbe() {
  const { presentation } = useOverlay();

  return <span data-testid="presentation">{presentation}</span>;
}

describe('OverlayProvider', () => {
  it('defaults to the sheet presentation outside a provider', () => {
    renderThemed(<PresentationProbe />);
    expect(screen.getByTestId('presentation')).toHaveTextContent('sheet');
  });

  it('reports the sheet presentation when mode is sheet', () => {
    renderThemed(
      <OverlayProvider mode="sheet">
        <PresentationProbe />
      </OverlayProvider>,
    );
    expect(screen.getByTestId('presentation')).toHaveTextContent('sheet');
  });

  it('reports the popover presentation when mode is popover', () => {
    renderThemed(
      <OverlayProvider mode="popover">
        <PresentationProbe />
      </OverlayProvider>,
    );
    expect(screen.getByTestId('presentation')).toHaveTextContent('popover');
  });

  it('resolves auto to a sheet on a narrow viewport', async () => {
    const original = window.matchMedia;
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: query.includes('max-width'),
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));

    renderThemed(
      <OverlayProvider mode="auto" sheetBelow="md">
        <PresentationProbe />
      </OverlayProvider>,
    );

    await waitFor(() => expect(screen.getByTestId('presentation')).toHaveTextContent('sheet'));
    window.matchMedia = original;
  });

  it('resolves auto to a popover on a wide viewport', async () => {
    const original = window.matchMedia;
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));

    renderThemed(
      <OverlayProvider mode="auto">
        <PresentationProbe />
      </OverlayProvider>,
    );

    await waitFor(() => expect(screen.getByTestId('presentation')).toHaveTextContent('popover'));
    window.matchMedia = original;
  });
});

describe('Combobox sheet presentation', () => {
  it('opens a dialog sheet instead of an anchored popover', () => {
    renderThemed(
      <OverlayProvider mode="sheet">
        <Combobox options={options} value={null} onChange={() => {}} aria-label="Role" />
      </OverlayProvider>,
    );

    fireEvent.click(screen.getByLabelText('Role'));

    const sheet = screen.getByRole('dialog', { name: 'Role' });
    expect(sheet).toBeInTheDocument();
    expect(screen.getByRole('listbox')).toBeInTheDocument();
  });

  it('keeps the popover path when the provider says popover', () => {
    renderThemed(
      <OverlayProvider mode="popover">
        <Combobox options={options} value={null} onChange={() => {}} aria-label="Role" />
      </OverlayProvider>,
    );

    fireEvent.click(screen.getByLabelText('Role'));

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('listbox')).toBeInTheDocument();
  });

  it('selecting an option closes the sheet and reports the value', () => {
    const onChange = vi.fn();

    renderThemed(
      <OverlayProvider mode="sheet">
        <Combobox options={options} value={null} onChange={onChange} aria-label="Role" />
      </OverlayProvider>,
    );

    fireEvent.click(screen.getByLabelText('Role'));
    fireEvent.click(screen.getByText('Operator'));

    expect(onChange).toHaveBeenCalledWith('operator');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('dismisses the sheet on Escape', () => {
    renderThemed(
      <OverlayProvider mode="sheet">
        <Combobox options={options} value={null} onChange={() => {}} aria-label="Role" />
      </OverlayProvider>,
    );

    fireEvent.click(screen.getByLabelText('Role'));
    expect(screen.getByRole('dialog')).toBeInTheDocument();

    fireEvent.keyDown(document, { key: 'Escape' });

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
});

describe('Menu sheet presentation', () => {
  const items = [
    { label: 'Rename', onClick: vi.fn() },
    { label: 'Delete', onClick: vi.fn(), destructive: true },
  ];

  it('renders its actions in a sheet', () => {
    renderThemed(
      <OverlayProvider mode="sheet">
        <Menu items={items}>
          <button type="button">Open</button>
        </Menu>
      </OverlayProvider>,
    );

    fireEvent.click(screen.getByText('Open'));

    expect(screen.getByRole('dialog', { name: 'Actions' })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: /Rename/ })).toBeInTheDocument();
  });

  it('runs the action and closes the sheet', () => {
    const onSelect = vi.fn();

    renderThemed(
      <OverlayProvider mode="sheet">
        <Menu items={[{ label: 'Rename', onClick: onSelect }]}>
          <button type="button">Open</button>
        </Menu>
      </OverlayProvider>,
    );

    fireEvent.click(screen.getByText('Open'));
    fireEvent.click(screen.getByRole('menuitem', { name: /Rename/ }));

    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
});
