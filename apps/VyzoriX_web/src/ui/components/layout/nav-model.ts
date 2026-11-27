import { type IconName } from '@vyzorix/ui';

export interface NavChild {
  id: string;
  text: string;
  url: string;
}

export interface NavItem {
  id: string;
  text: string;
  url: string;
  icon?: IconName;
  children?: NavChild[];
}

export interface NavSection {
  id: string;
  label: string;
  items: NavItem[];
}

export interface PageTabDef {
  id: string;
  text: string;
}

export function stripTrailingSlash(value: string): string {
  return value.length > 1 ? value.replace(/\/$/, '') : value;
}

export function isNavItemActive(item: NavItem, pathname: string): boolean {
  const url = stripTrailingSlash(item.url);
  const path = stripTrailingSlash(pathname);

  if (url === '') {
    return path === '';
  }
  return path === url || path.startsWith(`${url}/`);
}

export function findActiveNavItem(sections: NavSection[], pathname: string): NavItem | undefined {
  for (const section of sections) {
    for (const item of section.items) {
      if (isNavItemActive(item, pathname)) return item;
    }
  }
  return undefined;
}

export function findActiveSection(sections: NavSection[], pathname: string): NavSection | undefined {
  return sections.find((section) => section.items.some((item) => isNavItemActive(item, pathname)));
}

export function splitUrl(url: string): { path: string; tab: string | null } {
  const [path = '', query = ''] = url.split('?');
  return { path: stripTrailingSlash(path), tab: new URLSearchParams(query).get('tab') };
}

export function isNavChildActive(child: NavChild, pathname: string, tab: string | undefined): boolean {
  const target = splitUrl(child.url);
  return target.path === stripTrailingSlash(pathname) && target.tab === (tab ?? null);
}

export interface NavTrail {
  section?: NavSection;
  item?: NavItem;
  child?: NavChild;
}

export function resolveNavTrail(sections: NavSection[], pathname: string): NavTrail {
  const path = stripTrailingSlash(pathname);

  for (const section of sections) {
    for (const item of section.items) {
      for (const child of item.children ?? []) {

        const childPath = splitUrl(child.url).path;
        if (path === childPath || path.startsWith(`${childPath}/`)) {
          return { section, item, child };
        }
      }
      if (isNavItemActive(item, path)) {
        return { section, item };
      }
    }
  }

  return {};
}

export function navTrailLabels(trail: NavTrail, childIsTab = false): string[] {
  const { section, item, child } = trail;
  if (!item) return [];

  if (item.url === '/') return [item.text];

  const labels: string[] = [];
  if (section) labels.push(section.label);
  labels.push(item.text);
  if (child && !childIsTab) labels.push(child.text);

  return labels.filter((label, index) => index === 0 || label !== labels[index - 1]);
}

export function humanisePath(pathname: string): string {
  const segment = stripTrailingSlash(pathname).split('/').filter(Boolean).pop();
  if (!segment) return 'Home';
  return segment
    .split('-')
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

export interface BackTarget {
  to: string;
  tab?: string;
}

export function backTargetFor(pathname: string): BackTarget | null {
  const path = stripTrailingSlash(pathname);
  if (path.startsWith('/profile/notifications/')) {
    return { to: '/profile', tab: 'notifications' };
  }
  if (path === '/notifications') {
    return { to: '/profile', tab: 'notifications' };
  }
  return null;
}
