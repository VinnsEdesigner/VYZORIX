import type { IconName } from '@vyzorix/ui';

export interface ProviderDef {
  id: ProviderId;
  name: string;

  host: string;

  summary: string;
  icon: IconName;

  scopes: Array<{ scope: string; reason: string }>;
}

export const PROVIDER_IDS = ['github', 'gitlab'] as const;
export type ProviderId = (typeof PROVIDER_IDS)[number];

export const PROVIDERS: readonly ProviderDef[] = [
  {
    id: 'github',
    name: 'GitHub',
    host: 'github.com',
    summary: 'Read repositories and open pull requests against GitHub.',
    icon: 'globe',
    scopes: [
      { scope: 'read:user', reason: 'Read the account the authorisation belongs to.' },
      { scope: 'user:email', reason: 'Match the account to your VYZORIX user by email.' },
      { scope: 'repo', reason: 'Read private repositories, and open pull requests when you edit a manifest.' },
    ],
  },
  {
    id: 'gitlab',
    name: 'GitLab',
    host: 'gitlab.com',
    summary: 'Read repositories and open merge requests against GitLab.',
    icon: 'globe',
    scopes: [
      { scope: 'read_user', reason: 'Read the account the authorisation belongs to.' },
      { scope: 'read_repository', reason: 'Read the repositories this source syncs.' },
      { scope: 'api', reason: 'Open merge requests when you edit a manifest from the console.' },
    ],
  },
];

export function providerById(id: string | undefined): ProviderDef | undefined {
  return PROVIDERS.find((provider) => provider.id === id);
}

export function isProviderId(value: string | undefined): value is ProviderId {
  return value !== undefined && (PROVIDER_IDS as readonly string[]).includes(value);
}

export function providerInstallable(id: ProviderId): boolean {
  return id === 'github';
}

export function providerRoute(id: ProviderId): string {
  return `/admin/provisioning/providers/${id}`;
}
