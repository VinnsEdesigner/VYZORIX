import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { ThemeContext, getThemeById } from '@vyzorix/ui';
import { PreferencesTab } from '@/ui/pages/profile/preferences-tab';
import { usePreferencesStore, PREFERENCE_DEFAULTS } from '@/stores/preferences-store';
import { useVyzorI18nStore } from '@/stores/vyzor-i18n-store';
import { createTestQueryClient } from '../helpers/render-hook';

function renderTab() {
  render(
    <QueryClientProvider client={createTestQueryClient()}>
      <ThemeContext.Provider value={getThemeById('dark')}>
        <PreferencesTab />
      </ThemeContext.Provider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  localStorage.clear();
  usePreferencesStore.setState({ ...PREFERENCE_DEFAULTS });
  useVyzorI18nStore.setState({ locale: 'en' });
});

describe('PreferencesTab', () => {
  it('renders the appearance, behaviour and language groups', () => {
    renderTab();
    expect(screen.getByRole('heading', { name: 'Appearance & display' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Behaviour & defaults' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Language & region' })).toBeInTheDocument();
  });

  it('changes density', () => {
    renderTab();
    fireEvent.click(screen.getByRole('button', { name: 'Compact' }));
    expect(usePreferencesStore.getState().density).toBe('compact');
  });

  it('toggles reduce motion', () => {
    renderTab();
    fireEvent.click(screen.getByLabelText('Reduce motion'));
    expect(usePreferencesStore.getState().reduceMotion).toBe(true);
  });

  it('changes rows per page', () => {
    renderTab();
    fireEvent.click(screen.getByRole('button', { name: '100' }));
    expect(usePreferencesStore.getState().tablePageSize).toBe(100);
  });

  it('toggles confirm destructive actions', () => {
    renderTab();
    fireEvent.click(screen.getByLabelText('Confirm destructive actions'));
    expect(usePreferencesStore.getState().confirmDestructive).toBe(false);
  });

  it('shows the accessibility group and not the security group', () => {
    renderTab();
    expect(screen.getByRole('heading', { name: 'Accessibility' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Security & sessions' })).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Require device verification for')).not.toBeInTheDocument();
  });

  it('toggles accessibility options', () => {
    renderTab();
    fireEvent.click(screen.getByLabelText('High contrast'));
    expect(usePreferencesStore.getState().highContrast).toBe(true);
    fireEvent.click(screen.getByLabelText('Always underline links'));
    expect(usePreferencesStore.getState().underlineLinks).toBe(true);
  });
});
