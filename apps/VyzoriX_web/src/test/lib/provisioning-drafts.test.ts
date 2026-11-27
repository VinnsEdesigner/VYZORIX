import { describe, it, expect } from 'vitest';
import {
  formatPatternList,
  inScope,
  parsePatternList,
  validateConnectionDraft,
  validateRepositoryDraft,
  validateRepositoryUrl,
  validateScopePatterns,
} from '@vyzorix/api-client';

describe('inScope', () => {
  it('includes everything when no patterns are declared', () => {
    expect(inScope('devices/a/vyzorix.toml', [], [])).toBe(true);
  });

  it('lets exclude win over include, because the safe reading is do not touch it', () => {
    expect(inScope('devices/a/vyzorix.toml', ['devices/*'], ['devices/a/vyzorix.toml'])).toBe(false);
  });

  it('treats a pattern ending in /* as the whole subtree', () => {
    expect(inScope('orgs/acme/alerts.toml', ['orgs/*'], [])).toBe(true);
    expect(inScope('orgs/acme/deep/nested.toml', ['orgs/*'], [])).toBe(true);
  });

  it('does not let * cross a segment boundary', () => {
    expect(inScope('devices/a/b/vyzorix.toml', ['devices/*/vyzorix.toml'], [])).toBe(false);
    expect(inScope('devices/a/vyzorix.toml', ['devices/*/vyzorix.toml'], [])).toBe(true);
  });

  it('matches a wildcard-less pattern only at that exact path', () => {
    expect(inScope('devices/a/vyzorix.toml', ['devices/a/vyzorix.toml'], [])).toBe(true);
    expect(inScope('devices/a/vyzorix.toml.bak', ['devices/a/vyzorix.toml'], [])).toBe(false);
  });

  it('normalises a leading slash before matching', () => {
    expect(inScope('/devices/a/vyzorix.toml', ['devices/*'], [])).toBe(true);
  });

  it('matches ? against exactly one character', () => {
    expect(inScope('devices/a1/vyzorix.toml', ['devices/a?/vyzorix.toml'], [])).toBe(true);
    expect(inScope('devices/a12/vyzorix.toml', ['devices/a?/vyzorix.toml'], [])).toBe(false);
  });
});

describe('validateScopePatterns', () => {
  it('accepts well-formed patterns', () => {
    expect(validateScopePatterns(['devices/*', 'orgs/*/alerts.toml', '**'])).toBeNull();
  });

  it('refuses an empty pattern', () => {
    expect(validateScopePatterns(['  '])).toContain('must not be empty');
  });

  it('refuses an unterminated character class', () => {
    expect(validateScopePatterns(['devices/[abc/vyzorix.toml'])).toContain('malformed');
  });
});

describe('parsePatternList / formatPatternList', () => {
  it('splits on newlines and drops blanks', () => {
    expect(parsePatternList('devices/*\n\n  orgs/*  \n')).toEqual(['devices/*', 'orgs/*']);
  });

  it('round-trips a list', () => {
    const patterns = ['devices/*', 'orgs/*'];
    expect(parsePatternList(formatPatternList(patterns))).toEqual(patterns);
  });
});

describe('validateRepositoryUrl', () => {
  it('accepts an https git URL', () => {
    expect(validateRepositoryUrl('https://github.com/acme/prod.git')).toBeNull();
  });

  it('accepts an scp-style ssh address', () => {
    expect(validateRepositoryUrl('git@github.com:acme/prod.git')).toBeNull();
  });

  it('refuses a URL carrying credentials, because they belong in a connection', () => {
    expect(validateRepositoryUrl('https://user:token@github.com/acme/prod.git')).toContain(
      'Remove the credentials',
    );
  });

  it('refuses a bare filesystem path', () => {
    expect(validateRepositoryUrl('/srv/git/prod.git')).toContain('https, http or ssh');
  });

  it('refuses an empty URL', () => {
    expect(validateRepositoryUrl('   ')).toContain('required');
  });

  it('accepts an http URL, because the server accepts an internal git host', () => {

    expect(validateRepositoryUrl('http://github.com/acme/prod.git')).toBeNull();
    expect(validateRepositoryUrl('http://127.0.0.1:8088/device-config.git')).toBeNull();
  });

  it('still refuses a scheme it cannot clone', () => {
    expect(validateRepositoryUrl('ftp://example.com/repo.git')).toContain('https, http or ssh');
  });
});

describe('validateRepositoryDraft', () => {
  const valid = {
    name: 'prod',
    url: 'https://github.com/acme/prod.git',
    connectionId: 'c1',
  };

  it('accepts a complete draft', () => {
    expect(validateRepositoryDraft(valid)).toEqual({});
  });

  it('requires a name, a URL, and a connection', () => {
    const errors = validateRepositoryDraft({ name: ' ', url: '', connectionId: '' });

    expect(errors.name).toBeDefined();
    expect(errors.url).toBeDefined();
    expect(errors.connectionId).toBeDefined();
  });

  it('refuses an interval below the server minimum', () => {
    expect(validateRepositoryDraft({ ...valid, intervalSeconds: 30 }).intervalSeconds).toContain('60');
    expect(validateRepositoryDraft({ ...valid, intervalSeconds: 60 }).intervalSeconds).toBeUndefined();
  });

  it('reports a bad include or exclude pattern', () => {
    expect(validateRepositoryDraft({ ...valid, include: ['[bad'] }).include).toBeDefined();
    expect(validateRepositoryDraft({ ...valid, exclude: [''] }).exclude).toBeDefined();
  });
});

describe('validateConnectionDraft', () => {
  const valid = { name: 'deploy-bot', type: 'github', host: 'github.com', token: 'ghp_x' };

  it('accepts a complete draft', () => {
    expect(validateConnectionDraft(valid)).toEqual({});
  });

  it('requires a name, a provider, and a token', () => {
    const errors = validateConnectionDraft({ name: '', type: '', host: '', token: '' });

    expect(errors.name).toBeDefined();
    expect(errors.type).toBeDefined();
    expect(errors.token).toBeDefined();
  });

  it('refuses a host that looks like a path', () => {
    expect(validateConnectionDraft({ ...valid, host: 'github.com/acme' }).host).toContain('hostname');
  });

  it('accepts a host carrying a scheme, which the server stores as-is', () => {

    expect(validateConnectionDraft({ ...valid, host: 'http://127.0.0.1:8088' }).host).toBeUndefined();
    expect(validateConnectionDraft({ ...valid, host: 'https://git.example.com' }).host).toBeUndefined();
  });

  it('still refuses a path after a scheme', () => {
    expect(validateConnectionDraft({ ...valid, host: 'https://git.example.com/team' }).host).toContain(
      'hostname',
    );
  });

  it('allows an empty host, which the server defaults per provider', () => {
    expect(validateConnectionDraft({ ...valid, host: '' }).host).toBeUndefined();
  });
});
