import { describe, it, expect } from 'vitest';
import {
  findActiveNavItem,
  findActiveSection,
  humanisePath,
  isNavItemActive,
  isNavChildActive,
  navTrailLabels,
  resolveNavTrail,
  stripTrailingSlash,
  type NavChild,
  type NavItem,
  type NavSection,
} from '@/ui/components/layout';

const homeItem: NavItem = { id: 'home', text: 'Home', url: '/' };
const devicesItem: NavItem = { id: 'devices', text: 'Devices', url: '/devices' };
const dashboard: NavSection = { id: 'devices', label: 'Devices', items: [homeItem, devicesItem] };
const entriesChild: NavChild = { id: 'entries', text: 'Entries', url: '/logs?tab=entries' };
const ops: NavSection = {
  id: 'operations',
  label: 'Operations',
  items: [
    {
      id: 'logs',
      text: 'Logs',
      url: '/logs',
      children: [
        { id: 'entries', text: 'Entries', url: '/logs?tab=entries' },
        { id: 'volume', text: 'Volume', url: '/logs?tab=volume' },
      ],
    },
  ],
};
const sections: NavSection[] = [dashboard, ops];

describe('stripTrailingSlash', () => {
  it('drops a trailing slash but keeps the root', () => {
    expect(stripTrailingSlash('/logs/')).toBe('/logs');
    expect(stripTrailingSlash('/')).toBe('/');
  });
});

describe('isNavItemActive', () => {
  it('matches the root only at the root', () => {
    expect(isNavItemActive(homeItem, '/')).toBe(true);
    expect(isNavItemActive(homeItem, '/logs')).toBe(false);
  });

  it('stays active for a nested url', () => {
    expect(isNavItemActive(devicesItem, '/devices/abc')).toBe(true);
  });

  it('does not match a url that merely shares a prefix', () => {
    expect(isNavItemActive(devicesItem, '/devices-archive')).toBe(false);
  });
});

describe('findActiveNavItem', () => {
  it('finds the item across sections', () => {
    expect(findActiveNavItem(sections, '/logs')?.id).toBe('logs');
  });

  it('returns undefined when nothing matches', () => {
    expect(findActiveNavItem(sections, '/nope')).toBeUndefined();
  });
});

describe('findActiveSection', () => {
  it('finds the section that owns the active item', () => {
    expect(findActiveSection(sections, '/logs')?.id).toBe('operations');
    expect(findActiveSection(sections, '/devices')?.id).toBe('devices');
  });
});

describe('isNavChildActive', () => {
  it('matches only when the url and tab both agree', () => {
    expect(isNavChildActive(entriesChild, '/logs', 'entries')).toBe(true);
    expect(isNavChildActive(entriesChild, '/logs', 'volume')).toBe(false);
    expect(isNavChildActive(entriesChild, '/devices', 'entries')).toBe(false);
  });
});

describe('resolveNavTrail', () => {
  const admin: NavSection = {
    id: 'administration',
    label: 'Administration',
    items: [
      {
        id: 'general',
        text: 'General',
        url: '/admin/default-preferences',
        children: [
          { id: 'default-preferences', text: 'Default preferences', url: '/admin/default-preferences' },
          { id: 'provisioning', text: 'Provisioning', url: '/admin/provisioning' },
        ],
      },
    ],
  };
  const logs: NavSection = {
    id: 'operations',
    label: 'Operations',
    items: [
      {
        id: 'logs',
        text: 'Logs',
        url: '/logs',
        children: [
          { id: 'entries', text: 'Entries', url: '/logs?tab=entries' },
          { id: 'volume', text: 'Volume', url: '/logs?tab=volume' },
        ],
      },
    ],
  };
  const sections: NavSection[] = [dashboard, logs, admin];

  it('prefers the child when a child url matches', () => {
    const trail = resolveNavTrail(sections, '/admin/provisioning');
    expect(trail.section?.label).toBe('Administration');
    expect(trail.item?.text).toBe('General');
    expect(trail.child?.text).toBe('Provisioning');
  });

  it('returns just the item when no child matches', () => {
    const trail = resolveNavTrail(sections, '/devices');
    expect(trail.item?.text).toBe('Devices');
    expect(trail.child).toBeUndefined();
  });

  it('returns nothing for a path that is not in the nav', () => {
    expect(resolveNavTrail(sections, '/nope')).toEqual({});
  });
});

describe('navTrailLabels', () => {
  const section: NavSection = { id: 'devices', label: 'Devices', items: [] };
  const devicesItem: NavItem = { id: 'devices', text: 'Devices', url: '/devices' };
  const adminSection: NavSection = { id: 'administration', label: 'Administration', items: [] };
  const generalItem: NavItem = { id: 'general', text: 'General', url: '/admin/default-preferences' };
  const provisioningChild: NavChild = { id: 'provisioning', text: 'Provisioning', url: '/admin/provisioning' };
  const homeItem: NavItem = { id: 'home', text: 'Home', url: '/' };

  it('collapses a section and item with the same name', () => {
    expect(navTrailLabels({ section, item: devicesItem })).toEqual(['Devices']);
  });

  it('keeps the child for a group whose children are pages', () => {
    expect(navTrailLabels({ section: adminSection, item: generalItem, child: provisioningChild })).toEqual([
      'Administration',
      'General',
      'Provisioning',
    ]);
  });

  it('drops the child when the children are the page tabs', () => {
    expect(navTrailLabels({ section: adminSection, item: generalItem, child: provisioningChild }, true)).toEqual([
      'Administration',
      'General',
    ]);
  });

  it('shows only the label for the root', () => {
    expect(navTrailLabels({ section, item: homeItem })).toEqual(['Home']);
  });

  it('is empty when the page is not in the nav', () => {
    expect(navTrailLabels({})).toEqual([]);
  });
});

describe('humanisePath', () => {
  it('turns a slug into a title-cased label', () => {
    expect(humanisePath('/admin/default-preferences')).toBe('Default Preferences');
  });

  it('falls back to Home for the root', () => {
    expect(humanisePath('/')).toBe('Home');
  });
});

describe('resolveNavTrail — detail routes', () => {

  const adminUsers: NavItem = {
    id: 'users-access',
    text: 'Users and access',
    url: '/admin/users',
    children: [
      { id: 'users', text: 'Users', url: '/admin/users' },
      { id: 'service-accounts', text: 'Service accounts', url: '/admin/service-accounts' },
    ],
  };
  const admin: NavSection = { id: 'administration', label: 'Administration', items: [adminUsers] };

  it('resolves a child link exactly', () => {
    const trail = resolveNavTrail([admin], '/admin/service-accounts');
    expect(trail.child?.text).toBe('Service accounts');
    expect(navTrailLabels(trail)).toEqual(['Administration', 'Users and access', 'Service accounts']);
  });

  it('resolves a detail route beneath a child link', () => {
    const trail = resolveNavTrail([admin], '/admin/service-accounts/abc-123');
    expect(trail.child?.text).toBe('Service accounts');
    expect(navTrailLabels(trail)).toEqual(['Administration', 'Users and access', 'Service accounts']);
  });

  it('does not treat a sibling path as a descendant', () => {
    const trail = resolveNavTrail([admin], '/admin/service-accounts-archive');
    expect(trail.child).toBeUndefined();
  });
});
