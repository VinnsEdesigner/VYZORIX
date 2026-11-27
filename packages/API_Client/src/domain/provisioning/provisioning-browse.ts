import type {
  CommitJSON,
  FileJSON,
  HistoryJSON,
  TreeEntryJSON,
  TreeJSON,
} from '../../generated/vyzorixUpdateServerAPI.schemas';

export interface RepoEntry {
  path: string;
  name: string;
  kind: 'file' | 'dir';
  size: number;
}

export function repoEntryFromWire(wire: TreeEntryJSON): RepoEntry {
  return {
    path: wire.path ?? '',
    name: wire.name ?? '',
    kind: wire.kind === 'dir' ? 'dir' : 'file',
    size: wire.size ?? 0,
  };
}

export interface RepoTree {

  dir: string;
  entries: RepoEntry[];
}

export function repoTreeFromWire(wire: TreeJSON | undefined): RepoTree {
  return {
    dir: wire?.dir ?? '',
    entries: (wire?.entries ?? []).map(repoEntryFromWire),
  };
}

export interface RepoFile {
  path: string;
  content: string;
  commit: string;
  message: string;
  author: string;
  when: Date | null;

  truncated: boolean;
}

export function repoFileFromWire(wire: FileJSON | undefined): RepoFile {
  return {
    path: wire?.path ?? '',
    content: wire?.content ?? '',
    commit: wire?.commit ?? '',
    message: wire?.message ?? '',
    author: wire?.author ?? '',
    when: wire?.when ? new Date(wire.when) : null,
    truncated: wire?.truncated ?? false,
  };
}

export interface RepoCommit {
  sha: string;
  message: string;
  author: string;
  when: Date | null;
}

export function repoCommitFromWire(wire: CommitJSON): RepoCommit {
  return {
    sha: wire.sha ?? '',
    message: wire.message ?? '',
    author: wire.author ?? '',
    when: wire.when ? new Date(wire.when) : null,
  };
}

export interface RepoHistory {
  commits: RepoCommit[];

  depth: number;
}

export function repoHistoryFromWire(wire: HistoryJSON | undefined): RepoHistory {
  return {
    commits: (wire?.commits ?? []).map(repoCommitFromWire),
    depth: wire?.depth ?? 0,
  };
}

export function pathCrumbs(path: string): Array<{ label: string; path: string }> {
  const parts = path.split('/').filter(Boolean);
  const crumbs: Array<{ label: string; path: string }> = [];

  let accumulated = '';
  for (const part of parts) {
    accumulated = accumulated ? `${accumulated}/${part}` : part;
    crumbs.push({ label: part, path: accumulated });
  }

  return crumbs;
}

export function parentDir(path: string): string {
  const parts = path.split('/').filter(Boolean);
  parts.pop();

  return parts.join('/');
}
