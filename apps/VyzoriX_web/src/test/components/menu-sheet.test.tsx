import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { ThemeContext, getThemeById, Menu, Button, OverlayProvider } from '@vyzorix/ui';

describe('Menu in sheet presentation (regression: item clicks silently did nothing)', () => {
  it('invokes the item handler when an item is clicked', async () => {
    const onEdit = vi.fn();
    render(
      <ThemeContext.Provider value={getThemeById('vyzorix-dark')}>
        <OverlayProvider mode="sheet">
          <Menu items={[{ label: 'Edit source', onClick: onEdit }]}>
            <Button aria-label="Actions">…</Button>
          </Menu>
        </OverlayProvider>
      </ThemeContext.Provider>,
    );
    fireEvent.click(screen.getByLabelText('Actions'));
    const item = await screen.findByRole('menuitem', { name: /edit source/i });
    fireEvent.mouseDown(item);
    fireEvent.click(item);
    expect(onEdit).toHaveBeenCalled();
  });

  it('still closes when clicking genuinely outside', async () => {
    render(
      <ThemeContext.Provider value={getThemeById('vyzorix-dark')}>
        <OverlayProvider mode="sheet">
          <Menu items={[{ label: 'Edit source', onClick: vi.fn() }]}>
            <Button aria-label="Actions">…</Button>
          </Menu>
        </OverlayProvider>
      </ThemeContext.Provider>,
    );
    fireEvent.click(screen.getByLabelText('Actions'));
    await screen.findByRole('menuitem', { name: /edit source/i });
    fireEvent.mouseDown(document.body);
    await waitFor(() => expect(screen.queryByRole('menuitem')).not.toBeInTheDocument());
  });
});
