import { createAvatar, type Style } from '@dicebear/core';
import { bottts, lorelei, micah, notionists, openPeeps, personas } from '@dicebear/collection';

type EmptyIllustration =
  | 'sources'
  | 'credentials'
  | 'resources'
  | 'activity'
  | 'providers'
  | 'overview'
  | 'serviceAccounts'
  | 'scheduler'
  | 'browse';

interface StyleChoice {
  style: Style<Record<string, unknown>>;
  seed: string;
}

const CHOICES: Record<EmptyIllustration, StyleChoice> = {
  sources: { style: notionists, seed: 'vyzorix-sources' },
  credentials: { style: lorelei, seed: 'vyzorix-credentials' },
  resources: { style: micah, seed: 'vyzorix-resources' },
  activity: { style: openPeeps, seed: 'vyzorix-activity' },
  providers: { style: personas, seed: 'vyzorix-providers' },
  overview: { style: notionists, seed: 'vyzorix-overview' },
  serviceAccounts: { style: bottts, seed: 'vyzorix-service-accounts' },
  scheduler: { style: micah, seed: 'vyzorix-scheduler' },
  browse: { style: notionists, seed: 'vyzorix-browse' },
};

export function emptyStateAvatar(kind: EmptyIllustration, size = 600): string {
  const choice = CHOICES[kind];

  return createAvatar(choice.style, {
    seed: choice.seed,
    size,
  }).toDataUri();
}

export type { EmptyIllustration };
