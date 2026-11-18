import { describe, it, expect } from 'vitest';
import { iconRegistry } from '@vyzorix/ui';
import { appNav, pageTabs } from '@/ui/components/layout';

describe('appNav', () => {
  it('has sections, each with a label and items', () => {
    expect(appNav.length).toBeGreaterThan(0);
    for (const section of appNav) {
      expect(section.label).toBeTruthy();
      expect(section.items.length).toBeGreaterThan(0);
    }
  });

  it('gives every item a unique id, a label, a url and a known icon', () => {
    const ids: string[] = [];
    for (const section of appNav) {
      for (const item of section.items) {
        ids.push(item.id);
        expect(item.text).toBeTruthy();
        expect(item.url.startsWith('/')).toBe(true);
        expect(item.icon && item.icon in iconRegistry).toBe(true);
      }
    }
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('points Home at the root, so the root is the landing page', () => {
    expect(appNav[0]?.items[0]?.url).toBe('/');
  });

  it('gives every child a url under /admin or its parent page', () => {
    for (const section of appNav) {
      for (const item of section.items) {
        for (const child of item.children ?? []) {
          const underParent = child.url.startsWith(item.url);
          const underAdmin = child.url.startsWith('/admin/');
          expect(underParent || underAdmin).toBe(true);
        }
      }
    }
  });
});

describe('pageTabs', () => {
  it('has an Overview tab for the root', () => {
    expect(pageTabs['/']).toEqual([{ id: 'overview', text: 'Overview' }]);
  });

  it('gives the grouped pages more than one tab', () => {
    expect(pageTabs['/logs']?.length).toBeGreaterThan(1);
    expect(pageTabs['/terminal']?.length).toBeGreaterThan(1);
  });
});
