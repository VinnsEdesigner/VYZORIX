import { MIN_SYNC_INTERVAL_SECONDS } from './provisioning-vocabulary';

export interface ConnectionDraft {
  name: string;
  type: string;
  host: string;
  token: string;
}

export interface RepositoryDraft {
  name: string;
  url: string;
  connectionId: string;
  intervalSeconds?: number;
  include?: readonly string[];
  exclude?: readonly string[];
}

function matchesScopePattern(pattern: string, path: string): boolean {
  if (matchSingleSegment(pattern, path)) return true;

  if (pattern.endsWith('/*')) {
    const prefix = pattern.slice(0, -2);
    return path === prefix || path.startsWith(`${prefix}/`);
  }

  return false;
}

function matchSingleSegment(pattern: string, path: string): boolean {
  const patternParts = pattern.split('/');
  const pathParts = path.split('/');
  if (patternParts.length !== pathParts.length) return false;

  return patternParts.every((segment, i) => wildcardMatch(segment, pathParts[i] ?? ''));
}

function wildcardMatch(pattern: string, value: string): boolean {
  const escaped = pattern
    .replace(/[.+^${}()|[\]\\]/g, '\\$&')
    .replace(/\*/g, '[^/]*')
    .replace(/\?/g, '[^/]');

  return new RegExp(`^${escaped}$`).test(value);
}

function normaliseScopePath(value: string): string {
  const parts: string[] = [];
  for (const part of value.trim().split('/')) {
    if (!part || part === '.') continue;
    if (part === '..') {
      parts.pop();
      continue;
    }
    parts.push(part);
  }

  return parts.join('/');
}

export function inScope(path: string, include: readonly string[], exclude: readonly string[]): boolean {
  const normalised = normaliseScopePath(path);
  if (exclude.some((pattern) => matchesScopePattern(pattern.trim(), normalised))) return false;
  if (include.length === 0) return true;

  return include.some((pattern) => matchesScopePattern(pattern.trim(), normalised));
}

export function validateScopePatterns(patterns: readonly string[]): string | null {
  for (const pattern of patterns) {
    if (!pattern.trim()) return 'A pattern must not be empty';
    if (!isValidGlob(pattern.trim())) return `Pattern “${pattern}” is malformed`;
  }

  return null;
}

function isValidGlob(pattern: string): boolean {
  let escaped = false;
  for (const ch of pattern) {
    if (escaped) {
      escaped = false;
      continue;
    }
    if (ch === '\\') {
      escaped = true;
      continue;
    }
    if (ch === '[') return pattern.includes(']', pattern.indexOf('['));
  }

  return true;
}

const SCP_URL = /^[\w.-]+@[\w.-]+:[\w./-]+$/;
const PROTOCOL_URL = /^(ssh|git|https?):\/\//i;

const ALLOWED_PROTOCOLS = new Set(['https:', 'http:', 'ssh:', 'git:']);

export function validateRepositoryUrl(raw: string): string | null {
  const url = raw.trim();
  if (!url) return 'A repository URL is required';
  if (!SCP_URL.test(url) && !PROTOCOL_URL.test(url)) return 'Use an https, http or ssh git URL';

  if (PROTOCOL_URL.test(url)) {
    try {
      const parsed = new URL(url);
      if (parsed.username || parsed.password) {
        return 'Remove the credentials from the URL and store them in a connection';
      }
      if (!ALLOWED_PROTOCOLS.has(parsed.protocol)) {
        return 'Use an https, http or ssh git URL';
      }
    } catch {
      return 'That is not a valid URL';
    }
  }

  return null;
}

export function validateConnectionDraft(draft: ConnectionDraft): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!draft.name.trim()) errors.name = 'A name is required';
  if (!draft.type) errors.type = 'Choose a provider';
  if (!draft.token.trim()) errors.token = 'A token is required';

  const host = draft.host.trim().replace(/^https?:\/\//i, '');
  if (draft.host.trim() && /[ /]/.test(host)) errors.host = 'Use a hostname, not a path';

  return errors;
}

export function validateRepositoryDraft(draft: RepositoryDraft): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!draft.name.trim()) errors.name = 'A name is required';

  const urlError = validateRepositoryUrl(draft.url);
  if (urlError) errors.url = urlError;
  if (!draft.connectionId) errors.connectionId = 'Choose a connection';

  if (draft.intervalSeconds !== undefined && draft.intervalSeconds < MIN_SYNC_INTERVAL_SECONDS) {
    errors.intervalSeconds = `The interval must be at least ${MIN_SYNC_INTERVAL_SECONDS} seconds`;
  }

  const includeError = validateScopePatterns(draft.include ?? []);
  if (includeError) errors.include = includeError;

  const excludeError = validateScopePatterns(draft.exclude ?? []);
  if (excludeError) errors.exclude = excludeError;

  return errors;
}

export function parsePatternList(text: string): string[] {
  return text
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

export function formatPatternList(patterns: readonly string[]): string {
  return patterns.join('\n');
}
