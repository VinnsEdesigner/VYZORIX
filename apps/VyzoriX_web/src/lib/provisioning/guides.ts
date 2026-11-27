import type { EmptyIllustration } from '@/lib/avatar';

export interface GuideSection {
  heading: string;
  body?: string;

  code?: string;

  points?: string[];

  steps?: string[];
}

export interface GuideDef {
  id: GuideId;
  title: string;

  summary: string;
  illustration: EmptyIllustration;

  minutes: number;
  sections: GuideSection[];
}

export const GUIDE_IDS = [
  'providers',
  'sources',
  'browse',
  'resources',
  'device-state',
  'credentials',
  'scheduler',
  'secrets',
] as const;

export type GuideId = (typeof GUIDE_IDS)[number];

const REPO_LAYOUT = `devices/
  dev-0001/
    vyzorix.toml
  dev-0002/
    vyzorix.toml

orgs/
  Dev Organisation/
    alerts.toml`;

const DEVICE_MANIFEST = `[device]
id  = "dev-0001"
org = "org-dev-0001"

[[app]]
name = "dashboard"
cmd  = "/home/apps/dashboard/run"
port = 8080

[app.env]
PORT = "8080"`;

export const GUIDES: readonly GuideDef[] = [
  {
    id: 'providers',
    title: 'Install a git provider',
    summary: 'Authorise GitHub or GitLab once, so VYZORIX holds the grant instead of you pasting tokens.',
    illustration: 'providers',
    minutes: 3,
    sections: [
      {
        heading: 'What a provider is',
        body:
          'A provider is the git host your configuration lives on. Installing one authorises VYZORIX ' +
          'to read from that host. It is not a credential: a credential is one token, a provider is ' +
          'the host that token belongs to.',
      },
      {
        heading: 'Installing',
        steps: [
          'Open the Providers tab and pick a host.',
          'Read the scopes it will ask for — each one is listed with the reason it is needed.',
          'Press Authorise and approve the request on the host.',
        ],
      },
      {
        heading: 'If the host is not available yet',
        body:
          'A provider only appears as installable when the server has its OAuth client configured. ' +
          'If it says "not available yet", the server is missing that client — a self-hoster sets it ' +
          'in the server environment, and the provider page names the variables it needs.',
      },
      {
        heading: 'Prefer a token instead?',
        body:
          'You do not have to install a provider. A credential with a pasted token works just as ' +
          'well and is the right choice for a host VYZORIX has no OAuth support for.',
      },
    ],
  },
  {
    id: 'sources',
    title: 'Add a source',
    summary: 'Point VYZORIX at the repository, branch and folder that describe your devices.',
    illustration: 'sources',
    minutes: 4,
    sections: [
      {
        heading: 'What a source is',
        body:
          'A source is one branch of one repository, read on a schedule. It is the origin of the ' +
          'desired state: every file it declares is compared against what your devices run.',
      },
      {
        heading: 'The layout it expects',
        body: 'The directory decides what a file means, so the layout is the contract:',
        code: REPO_LAYOUT,
      },
      {
        heading: 'What a device file looks like',
        code: DEVICE_MANIFEST,
      },
      {
        heading: 'Things that catch people out',
        points: [
          'device.org must be the organisation ID, not its name.',
          'The folder name is the device ID, and the id inside the file must match it.',
          'An app\'s command must live inside that app\'s own folder.',
          'A URL carrying a password is refused — put the credential in a connection instead.',
          'One branch per source. Tracking a second branch means a second source.',
        ],
      },
      {
        heading: 'Narrowing what a source reads',
        body:
          'Include and exclude are glob patterns matched against the path. Exclude wins over ' +
          'include, and a pattern ending in /* covers the whole subtree. A file that drops out of ' +
          'scope becomes "out of scope", not "missing" — nothing is reverted.',
      },
    ],
  },
  {
    id: 'browse',
    title: 'Browse a repository',
    summary: 'Read the tree, open a file, and see the commit log in the Code tab — without a git client.',
    illustration: 'browse',
    minutes: 3,
    sections: [
      {
        heading: 'What browsing is for',
        body:
          'Browsing is how you answer "what is actually in the repository?" without a git client. ' +
          'It reads the branch the source tracks, so you see the files a sync would see — not a ' +
          'copy the server keeps, and not a cached listing that could be out of date.',
      },
      {
        heading: 'Reading it',
        steps: [
          'Open the Code tab and pick a source.',
          'The Files view starts at the repository root. A folder opens in place; the breadcrumb walks back up.',
          'Open a file to read it, with the commit, author and time it came from.',
          'Switch to History to list the most recent commits on that branch, newest first.',
        ],
      },
      {
        heading: 'Read-only, on purpose',
        body:
          'Browsing needs only the read permission, and it works on a read-only connection. Seeing ' +
          'what is in a repository is harmless; editing it is not. That is why an operator who ' +
          'cannot push can still read every file here.',
      },
      {
        heading: 'Editing from a file',
        body:
          'If the connection has write access and the file is a provisioned resource, an Edit button ' +
          'appears. The edit goes through the same preview-then-commit path as the rest of the ' +
          'console, so you see a diff before anything lands. A file that is not a provisioned ' +
          'resource has no Edit button: there is nothing to apply it to.',
      },
      {
        heading: 'Two limits worth knowing',
        points: [
          'A very large file is truncated and marked as such, rather than streamed whole.',
          'History is a bounded window (the last 50 commits), so a big repository is not cloned in full just to show a log.',
        ],
      },
    ],
  },
  {
    id: 'device-state',
    title: 'Trust the state, not just the database',
    summary: 'Why a resource says "awaiting device", and how presence tells a real success from a stored one.',
    illustration: 'resources',
    minutes: 4,
    sections: [
      {
        heading: 'A deploy is not proof',
        body:
          'When VYZORIX applies a file it compiles, authorises, signs and stores a document. That ' +
          'proves the server did its part. It does not prove the device fetched it — the device may ' +
          'be offline, unregistered, or simply not have polled yet. So the resource is recorded as ' +
          '"awaiting device" until the device itself reports the version.',
      },
      {
        heading: 'What makes it "applied"',
        body:
          'Only the device can say what it is running. It reports its config version on every ' +
          'heartbeat, and the resource becomes "applied" when that report matches what this source ' +
          'signed — and the report is current. A report from a device that is reachable now, or ' +
          'that arrived within the offline window, counts. A week-old report from a device nobody ' +
          'has heard from does not: that is not evidence, it is a stored guess.',
      },
      {
        heading: 'Read the presence line',
        points: [
          'Online — the device holds a live connection right now.',
          'Offline — it is not connected; the last-seen time says how long.',
          'Reachability unknown — the server could not read the device. This is not "offline"; a failed lookup is not a down device.',
          'Reports vN — the config version the device last said it holds.',
        ],
      },
      {
        heading: 'Why this matters',
        body:
          'A state that comes only from a database row can claim a resource was applied when the ' +
          'device never received it. Showing presence beside the state is what lets you tell the ' +
          'difference: "Applied · Online" is a device that confirmed and is still there; "Applied · ' +
          'Offline" is a device that confirmed, then went away. If you need it to converge now, use ' +
          'Sync this device — that is what the action is for.',
      },
    ],
  },
  {
    id: 'resources',
    title: 'Read a resource',
    summary: 'What applied, pending, drifted and missing each mean, and what to do about them.',
    illustration: 'resources',
    minutes: 4,
    sections: [
      {
        heading: 'A resource is one file, for one target',
        body:
          'Files under devices/ are bound to one device; files under orgs/ configure the ' +
          'organisation. Two devices are two resources even when their files are identical.',
      },
      {
        heading: 'The states',
        points: [
          'Applied — the device reports it is running this exact config version.',
          'Awaiting device — the config is signed and stored, but the device has not confirmed it.',
          'Drifted — the device changed after this source applied it.',
          'Missing — the file was removed from the repository; the device keeps what it runs.',
          'Out of scope — the file is now outside the include/exclude patterns.',
          'Failed — the last attempt to apply it failed, and the reason is shown.',
        ],
      },
      {
        heading: 'Why "awaiting device" is not a failure',
        body:
          'A deploy proves the server signed and stored a document. It does not prove the device ' +
          'fetched it — the device may be offline, or may not have polled yet. VYZORIX will not ' +
          'claim a fact it cannot see, so the resource stays pending until the device reports. ' +
          'The presence line beside the state tells you whether the device is reachable now.',
      },
      {
        heading: 'Acting on one resource',
        body:
          'Open a resource to see its signed document, the secrets it declares, and a Sync this ' +
          'device action. That is the right tool after a single device fails: a full pass over ' +
          'every file is a much bigger hammer.',
      },
    ],
  },
  {
    id: 'credentials',
    title: 'Manage credentials',
    summary: 'How tokens are stored, why they can never be read back, and how to check them.',
    illustration: 'credentials',
    minutes: 3,
    sections: [
      {
        heading: 'A credential is write-only',
        body:
          'The token is encrypted at rest and never returned by any route. If you lose it you ' +
          'replace it; you cannot read it back. That is deliberate: a token you can read is a token ' +
          'an attacker can read from a screenshot.',
      },
      {
        heading: 'Editing without re-entering the token',
        body:
          'Leave the token field empty when editing. An empty token means "keep the stored one", ' +
          'which is what makes renaming a credential possible without a secret nobody can read.',
      },
      {
        heading: 'The health check',
        body:
          'Saving runs a check against the host. A failed check still saves the credential, marked ' +
          'with the reason — a typo becomes a red row you can fix, not a silent nothing. Check now ' +
          're-runs it at any time.',
      },
      {
        heading: 'Hosts with a scheme',
        body:
          'A host may be written as "host" or "http://host:port". The scheme is kept when you give ' +
          'one, because an internal git host is a real deployment. A credential still in use by a ' +
          'source cannot be deleted; re-point the source first.',
      },
    ],
  },
  {
    id: 'scheduler',
    title: 'Understand the scheduler',
    summary: 'When sources sync on their own, what is due next, and where the cadence lives.',
    illustration: 'scheduler',
    minutes: 3,
    sections: [
      {
        heading: 'It runs without being asked',
        body:
          'The scheduler looks for sources whose interval has elapsed and syncs them. Without it a ' +
          'source would only ever change when someone pressed Sync.',
      },
      {
        heading: 'The cadence is per source',
        body:
          'Each source carries its own interval and on/off switch, so one can sync every minute ' +
          'while another waits an hour. The Scheduler tab reports and links to Sources; it does not ' +
          'hold a second, competing interval.',
      },
      {
        heading: 'Reading the page',
        points: [
          'Enabled sources — how many are on a schedule at all.',
          'Due now — what the next tick will sync.',
          'Next due — when the next one becomes due.',
          'Check interval — how often the loop looks for work.',
        ],
      },
      {
        heading: 'Minimum interval',
        body: 'A source cannot sync more often than every 60 seconds. A shorter interval is refused on save.',
      },
    ],
  },
  {
    id: 'secrets',
    title: 'Declare secrets safely',
    summary: 'Name the secrets an app needs without ever putting a value in git.',
    illustration: 'resources',
    minutes: 3,
    sections: [
      {
        heading: 'The manifest names them, never carries them',
        body:
          'A manifest lists the secret keys an app requires. The value lives in the secret store on ' +
          'the server, and the device fetches it. Nothing secret is ever committed.',
      },
      {
        heading: 'Checked at apply time',
        body:
          'A required secret that is not set fails that resource with a message naming the app and ' +
          'key. The rest of the source still applies — one missing value does not block the fleet.',
        code: `[app.secrets]
API_KEY = ""`,
      },
      {
        heading: 'Reading readiness',
        body:
          'A resource detail lists each declared secret and whether it is set. It never shows a ' +
          'value. Setting one requires the device to have a secret key registered; without one the ' +
          'server refuses with a clear 409 rather than a server error.',
      },
      {
        heading: 'A value set later does not retro-apply',
        body:
          'The next sync applies it. That is the existing model: the value is fetched by the device, ' +
          'not pushed to it.',
      },
    ],
  },
];

export function guideById(id: string | undefined): GuideDef | undefined {
  return GUIDES.find((guide) => guide.id === id);
}

export function isGuideId(value: string | undefined): value is GuideId {
  return value !== undefined && (GUIDE_IDS as readonly string[]).includes(value);
}

export function guideRoute(id: GuideId): string {
  return `/admin/provisioning/guides/${id}`;
}
