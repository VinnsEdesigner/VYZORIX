import { describe, it, expect } from 'vitest';
import { iconRegistry } from '@vyzorix/ui';
import { pageTabs } from '@/ui/components/layout';
import {
  DEFAULT_PROVISIONING_TAB,
  PROVISIONING_PAGE_TABS,
  PROVISIONING_TABS,
  PROVISIONING_TAB_IDS,
  isProvisioningTab,
  resolveProvisioningTab,
} from '@/lib/provisioning/provisioning-tabs';

describe('provisioning tabs', () => {
  it('declares every tab exactly once', () => {
    const ids = PROVISIONING_TABS.map((tab) => tab.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect(ids).toEqual([...PROVISIONING_TAB_IDS]);
  });

  it('gives every tab a label, an icon, and a known icon name', () => {
    for (const tab of PROVISIONING_TABS) {
      expect(tab.text).toBeTruthy();
      expect(tab.icon in iconRegistry).toBe(true);
    }
  });

  it('gives every tab except Get started a subtitle', () => {
    for (const tab of PROVISIONING_TABS) {
      if (tab.id === 'getting-started') {
        expect(tab.subTitle).toBe('');
      } else {
        expect(tab.subTitle.length).toBeGreaterThan(0);
      }
    }
  });

  it('drives the shell tab bar from the same registry', () => {
    expect(pageTabs['/admin/provisioning']).toEqual(PROVISIONING_PAGE_TABS);
    expect(PROVISIONING_PAGE_TABS.map((tab) => tab.id)).toEqual([...PROVISIONING_TAB_IDS]);
  });

  it('lands on the overview when no tab is given', () => {
    expect(DEFAULT_PROVISIONING_TAB).toBe('overview');
    expect(resolveProvisioningTab(undefined).id).toBe('overview');
    expect(resolveProvisioningTab('').id).toBe('overview');
  });

  it('falls back to the overview for an unknown tab rather than rendering nothing', () => {
    expect(resolveProvisioningTab('not-a-tab').id).toBe('overview');
  });

  it('resolves a known tab with its own subtitle', () => {
    expect(resolveProvisioningTab('sources').subTitle).toContain('Git repositories');
  });

  it('rejects a non-tab string in the guard', () => {
    expect(isProvisioningTab('sources')).toBe(true);
    expect(isProvisioningTab('repos')).toBe(false);
    expect(isProvisioningTab(undefined)).toBe(false);
  });

  it('keeps the nine tabs the feature needs, so the set is deliberate', () => {
    expect([...PROVISIONING_TAB_IDS]).toEqual([
      'overview',
      'providers',
      'sources',
      'code',
      'credentials',
      'resources',
      'activity',
      'scheduler',
      'getting-started',
    ]);
  });

  it('lists Providers before Sources, because a source needs one installed', () => {
    const ids = PROVISIONING_TABS.map((tab) => tab.id);
    expect(ids.indexOf('providers')).toBeLessThan(ids.indexOf('sources'));
  });
});
