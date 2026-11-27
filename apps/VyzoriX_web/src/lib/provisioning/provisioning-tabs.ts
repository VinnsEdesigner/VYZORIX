import type { IconName } from '@vyzorix/ui';

export interface ProvisioningTabDef {
  id: ProvisioningTabId;
  text: string;
  icon: IconName;

  subTitle: string;
}

export const PROVISIONING_TAB_IDS = [
  'overview',
  'providers',
  'sources',
  'code',
  'credentials',
  'resources',
  'activity',
  'scheduler',
  'getting-started',
] as const;

export type ProvisioningTabId = (typeof PROVISIONING_TAB_IDS)[number];

export const PROVISIONING_TABS: readonly ProvisioningTabDef[] = [
  {
    id: 'overview',
    text: 'Overview',
    icon: 'gauge',
    subTitle: 'The state of every source, and what changed recently',
  },
  {
    id: 'providers',
    text: 'Providers',
    icon: 'globe',
    subTitle: 'Git hosts VYZORIX can read from, and how they are authorised',
  },
  {
    id: 'sources',
    text: 'Sources',
    icon: 'folder',
    subTitle: 'Git repositories whose manifests define your devices',
  },
  {
    id: 'code',
    text: 'Code',
    icon: 'code',
    subTitle: 'Browse a source\'s repository, read a file, and commit an edit',
  },
  {
    id: 'credentials',
    text: 'Credentials',
    icon: 'key',
    subTitle: 'Git tokens VYZORIX reads sources with',
  },
  {
    id: 'resources',
    text: 'Resources',
    icon: 'box',
    subTitle: 'Every manifest a source owns, and whether it has drifted',
  },
  {
    id: 'activity',
    text: 'Activity',
    icon: 'activity',
    subTitle: 'Who changed what, and whether it was a person or a machine',
  },
  {
    id: 'scheduler',
    text: 'Scheduler',
    icon: 'calendar-clock',
    subTitle: 'When sources sync on their own, and what is due next',
  },
  {
    id: 'getting-started',
    text: 'Get started',
    icon: 'book-open',
    subTitle: '',
  },
];

export const PROVISIONING_PAGE_TABS = PROVISIONING_TABS.map((tab) => ({ id: tab.id, text: tab.text }));

export const DEFAULT_PROVISIONING_TAB: ProvisioningTabId = 'overview';

export function isProvisioningTab(value: string | undefined): value is ProvisioningTabId {
  return value !== undefined && (PROVISIONING_TAB_IDS as readonly string[]).includes(value);
}

export function resolveProvisioningTab(value: string | undefined): ProvisioningTabDef {
  const id = isProvisioningTab(value) ? value : DEFAULT_PROVISIONING_TAB;

  return PROVISIONING_TABS.find((tab) => tab.id === id) ?? PROVISIONING_TABS[0]!;
}
