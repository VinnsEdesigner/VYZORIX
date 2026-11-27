import { describe, it, expect } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { useSelection, LONG_PRESS_MS } from '@/ui/components/shared/use-long-press';
import { languageForPath } from '@/ui/components/shared/code-block';

describe('useSelection', () => {
  it('starts with nothing selected', () => {
    const { result } = renderHook(() => useSelection());

    expect(result.current.count).toBe(0);
    expect(result.current.isSelected('a')).toBe(false);
  });

  it('toggles one id on and off', () => {
    const { result } = renderHook(() => useSelection());

    act(() => result.current.toggle('a'));
    expect(result.current.isSelected('a')).toBe(true);
    expect(result.current.count).toBe(1);

    act(() => result.current.toggle('a'));
    expect(result.current.isSelected('a')).toBe(false);
    expect(result.current.count).toBe(0);
  });

  it('selects all of a list in one action', () => {
    const { result } = renderHook(() => useSelection());

    act(() => result.current.selectAll(['a', 'b', 'c']));

    expect(result.current.count).toBe(3);
    expect(result.current.allSelected(['a', 'b', 'c'])).toBe(true);
  });

  it('reports allSelected false for an empty list, not vacuously true', () => {
    const { result } = renderHook(() => useSelection());

    expect(result.current.allSelected([])).toBe(false);
  });

  it('clears a selection', () => {
    const { result } = renderHook(() => useSelection());

    act(() => result.current.selectAll(['a', 'b']));
    act(() => result.current.clear());

    expect(result.current.count).toBe(0);
  });

  it('keys on ids, so a refetch that reorders rows keeps the right ones selected', () => {
    const { result } = renderHook(() => useSelection());

    act(() => result.current.toggle('event-2'));

    expect(result.current.isSelected('event-2')).toBe(true);
    expect(result.current.isSelected('event-1')).toBe(false);
  });
});

describe('languageForPath', () => {
  it('maps a manifest to toml', () => {
    expect(languageForPath('devices/dev-0001/vyzorix.toml')).toBe('toml');
  });

  it('maps the compiled .v output to toml too', () => {

    expect(languageForPath('devices/dev-0001/vyzorix.v')).toBe('toml');
  });

  it('maps common config and docs extensions', () => {
    expect(languageForPath('a/b.json')).toBe('json');
    expect(languageForPath('a/b.yaml')).toBe('yaml');
    expect(languageForPath('README.md')).toBe('markdown');
    expect(languageForPath('run.sh')).toBe('shellscript');
  });

  it('falls back to text for an unknown or extensionless file', () => {
    expect(languageForPath('LICENSE')).toBe('text');
    expect(languageForPath('a/b.unknownext')).toBe('text');
  });

  it('maps a filename with no extension by its name, not its dot', () => {

    expect(languageForPath('CMakeLists.txt')).toBe('makefile');
    expect(languageForPath('go.mod')).toBe('go');
    expect(languageForPath('Dockerfile')).toBe('dockerfile');
    expect(languageForPath('Makefile')).toBe('makefile');
    expect(languageForPath('.env')).toBe('dotenv');
  });

  it('treats a known-but-unhighlightable filename as plain text', () => {

    expect(languageForPath('.gitignore')).toBe('text');
    expect(languageForPath('go.sum')).toBe('text');
  });

  it('does not read a leading dot as an extension separator', () => {
    expect(languageForPath('.somehiddenfile')).toBe('text');
  });

  it('maps the extended language set', () => {
    const cases: Array<[string, string]> = [
      ['a.jsonc', 'jsonc'],
      ['a.json5', 'json5'],
      ['a.ini', 'ini'],
      ['a.cfg', 'ini'],
      ['a.ps1', 'powershell'],
      ['a.bat', 'bat'],
      ['a.mdx', 'mdx'],
      ['a.scss', 'scss'],
      ['a.less', 'less'],
      ['a.vue', 'vue'],
      ['a.svelte', 'svelte'],
      ['a.sql', 'sql'],
      ['a.graphql', 'graphql'],
      ['a.proto', 'proto'],
      ['a.xml', 'xml'],
      ['a.svg', 'xml'],
      ['a.tf', 'terraform'],
      ['a.hcl', 'hcl'],
      ['a.py', 'python'],
      ['a.rs', 'rust'],
      ['a.java', 'java'],
      ['a.kt', 'kotlin'],
      ['a.swift', 'swift'],
      ['a.rb', 'ruby'],
      ['a.php', 'php'],
      ['a.cs', 'csharp'],
      ['a.cpp', 'cpp'],
      ['a.c', 'c'],
      ['a.m', 'objective-c'],
      ['a.lua', 'lua'],
      ['a.ex', 'elixir'],
      ['a.erl', 'erlang'],
      ['a.hs', 'haskell'],
      ['a.scala', 'scala'],
      ['a.clj', 'clojure'],
      ['a.dart', 'dart'],
      ['a.patch', 'diff'],
    ];

    for (const [path, expected] of cases) {
      expect(languageForPath(path), path).toBe(expected);
    }
  });

  it('maps every extension it claims to a language Shiki bundles', async () => {

    const { bundledLanguages } = await import('shiki');

    const paths = [
      'a.toml', 'a.json', 'a.jsonc', 'a.json5', 'a.yaml', 'a.ini', 'a.env', 'a.md', 'a.mdx',
      'a.sh', 'a.ps1', 'a.bat', 'a.dockerfile', 'a.mk', 'a.go', 'a.ts', 'a.tsx', 'a.js', 'a.jsx',
      'a.css', 'a.scss', 'a.sass', 'a.less', 'a.html', 'a.vue', 'a.svelte', 'a.astro', 'a.sql',
      'a.graphql', 'a.proto', 'a.xml', 'a.tf', 'a.hcl', 'a.nginx', 'a.py', 'a.rb', 'a.php',
      'a.java', 'a.kt', 'a.scala', 'a.groovy', 'a.clj', 'a.rs', 'a.c', 'a.cpp', 'a.cs', 'a.m',
      'a.swift', 'a.dart', 'a.lua', 'a.pl', 'a.r', 'a.ex', 'a.erl', 'a.hs', 'a.diff',
    ];

    for (const path of paths) {
      const lang = languageForPath(path);
      if (lang === 'text') continue;
      expect(lang in bundledLanguages, `${path} → ${lang} is not bundled`).toBe(true);
    }
  });

  it('is case-insensitive on the extension', () => {
    expect(languageForPath('a/B.TOML')).toBe('toml');
  });
});

describe('long-press timing constant', () => {
  it('is a human-scale hold, not a click', () => {

    expect(LONG_PRESS_MS).toBeGreaterThanOrEqual(400);
    expect(LONG_PRESS_MS).toBeLessThanOrEqual(800);
  });
});
