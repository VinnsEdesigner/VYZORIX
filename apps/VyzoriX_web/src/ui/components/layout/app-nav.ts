import { type NavSection, type PageTabDef } from './nav-model';
import { PROVISIONING_PAGE_TABS } from '@/lib/provisioning/provisioning-tabs';

export type { PageTabDef };

export const pageTabs: Record<string, PageTabDef[]> = {
  '/': [{ id: 'overview', text: 'Overview' }],
  '/devices': [{ id: 'overview', text: 'Overview' }],
  '/apps': [
    { id: 'deployed', text: 'Deployed' },
    { id: 'catalog', text: 'Catalog' },
    { id: 'exposure', text: 'Exposure' },
  ],
  '/permissions': [
    { id: 'members', text: 'Members' },
    { id: 'review', text: 'Review' },
    { id: 'invitations', text: 'Invitations' },
    { id: 'audit', text: 'Audit' },
  ],
  '/telemetry': [{ id: 'overview', text: 'Overview' }],
  '/updates': [
    { id: 'overview', text: 'Overview' },
    { id: 'versions', text: 'Release versions' },
    { id: 'pushes', text: 'Push history' },
  ],
  '/alerts': [{ id: 'overview', text: 'Overview' }],
  '/logs': [
    { id: 'entries', text: 'Entries' },
    { id: 'volume', text: 'Volume' },
  ],
  '/terminal': [
    { id: 'session', text: 'Session' },
    { id: 'files', text: 'Files' },
    { id: 'recordings', text: 'Recordings' },
    { id: 'limits', text: 'Access & limits' },
  ],
  '/profile': [
    { id: 'profile', text: 'Profile' },
    { id: 'preferences', text: 'Preferences' },
    { id: 'notifications', text: 'Notifications' },
    { id: 'security', text: 'Security' },
  ],
  '/admin/default-preferences': [{ id: 'overview', text: 'Overview' }],
  '/admin/provisioning': PROVISIONING_PAGE_TABS,
  '/admin/users': [{ id: 'overview', text: 'Overview' }],
  '/admin/service-accounts': [{ id: 'overview', text: 'Overview' }],
  '/admin/authentication': [{ id: 'overview', text: 'Overview' }],
};

export const appNav: NavSection[] = [
  {
    id: 'devices',
    label: 'Devices',
    items: [
      { id: 'home', text: 'Home', url: '/', icon: 'home' },
      { id: 'devices', text: 'Devices', url: '/devices', icon: 'smartphone' },
      {
        id: 'apps',
        text: 'Apps',
        url: '/apps',
        icon: 'box',
        children: [
          { id: 'deployed', text: 'Deployed', url: '/apps?tab=deployed' },
          { id: 'catalog', text: 'Catalog', url: '/apps?tab=catalog' },
          { id: 'exposure', text: 'Exposure', url: '/apps?tab=exposure' },
        ],
      },
      {
        id: 'permissions',
        text: 'Permissions',
        url: '/permissions',
        icon: 'shield',
        children: [
          { id: 'members', text: 'Members', url: '/permissions?tab=members' },
          { id: 'review', text: 'Review', url: '/permissions?tab=review' },
          { id: 'invitations', text: 'Invitations', url: '/permissions?tab=invitations' },
          { id: 'audit', text: 'Audit', url: '/permissions?tab=audit' },
        ],
      },
      { id: 'telemetry', text: 'Telemetry', url: '/telemetry', icon: 'activity' },
    ],
  },
  {
    id: 'operations',
    label: 'Operations',
    items: [
      {
        id: 'updates',
        text: 'Updates',
        url: '/updates',
        icon: 'download',
        children: [
          { id: 'overview', text: 'Overview', url: '/updates?tab=overview' },
          { id: 'versions', text: 'Release versions', url: '/updates?tab=versions' },
          { id: 'pushes', text: 'Push history', url: '/updates?tab=pushes' },
        ],
      },
      { id: 'alerts', text: 'Alerts', url: '/alerts', icon: 'bell' },
      {
        id: 'logs',
        text: 'Logs',
        url: '/logs',
        icon: 'file-text',
        children: [
          { id: 'entries', text: 'Entries', url: '/logs?tab=entries' },
          { id: 'volume', text: 'Volume', url: '/logs?tab=volume' },
        ],
      },
      {
        id: 'terminal',
        text: 'Terminal',
        url: '/terminal',
        icon: 'terminal',
        children: [
          { id: 'session', text: 'Session', url: '/terminal?tab=session' },
          { id: 'files', text: 'Files', url: '/terminal?tab=files' },
          { id: 'recordings', text: 'Recordings', url: '/terminal?tab=recordings' },
          { id: 'limits', text: 'Access & limits', url: '/terminal?tab=limits' },
        ],
      },
    ],
  },
  {
    id: 'administration',
    label: 'Administration',
    items: [
      {
        id: 'general',
        text: 'General',
        url: '/admin/default-preferences',
        icon: 'settings',
        children: [
          { id: 'default-preferences', text: 'Organisation defaults', url: '/admin/default-preferences' },
          { id: 'provisioning', text: 'Provisioning', url: '/admin/provisioning' },
        ],
      },
      {
        id: 'users-and-access',
        text: 'Users and access',
        url: '/admin/users',
        icon: 'users',
        children: [
          { id: 'users', text: 'Users', url: '/admin/users' },
          { id: 'service-accounts', text: 'Service accounts', url: '/admin/service-accounts' },
        ],
      },
      { id: 'authentication', text: 'Authentication', url: '/admin/authentication', icon: 'log-in' },
    ],
  },
];
