import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { ThemeContext, getThemeById } from '@vyzorix/ui';
import { SecurityTab } from '@/ui/pages/profile/security-tab';
import { usePreferencesStore, PREFERENCE_DEFAULTS } from '@/stores/preferences-store';
import { createTestQueryClient } from '../helpers/render-hook';

function renderTab() {
  render(
    <QueryClientProvider client={createTestQueryClient()}>
      <ThemeContext.Provider value={getThemeById('dark')}>
        <SecurityTab />
      </ThemeContext.Provider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  localStorage.clear();
  usePreferencesStore.setState({ ...PREFERENCE_DEFAULTS });
});

describe('SecurityTab', () => {
  it('renders the sessions and step-up groups', () => {
    renderTab();
    expect(screen.getByRole('heading', { name: 'Sessions' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Step-up verification' })).toBeInTheDocument();
  });

  it('shows the idle-timeout control', () => {
    renderTab();
    expect(screen.getByLabelText('Sign out after inactivity')).toBeInTheDocument();
  });

  it('keeps step-up verification disabled until fingerprint security exists', () => {
    renderTab();
    expect(screen.getByLabelText('Require device verification for')).toBeDisabled();
  });
});
