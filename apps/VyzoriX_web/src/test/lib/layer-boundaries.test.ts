import { describe, it, expect } from 'vitest';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

const SRC = join(process.cwd(), 'src');
const API_CLIENT = /from\s+['"]@vyzorix\/api-client(?:\/[^'"]*)?['"]/;

function walk(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      out.push(...walk(full));
    } else if (/\.(ts|tsx)$/.test(entry)) {
      out.push(full);
    }
  }
  return out;
}

function offenders(layer: string): string[] {
  return walk(join(SRC, layer))
    .filter((file) => API_CLIENT.test(readFileSync(file, 'utf8')))
    .map((file) => file.slice(SRC.length + 1));
}

describe('layer boundaries', () => {
  it('no component or route imports the API client directly', () => {
    expect(offenders('ui')).toEqual([]);
    expect(offenders('routes')).toEqual([]);
  });

  it('lib does not import the API client, except the one error facade that re-exports it', () => {
    const allowed = ['lib/error/index.ts'];
    expect(offenders('lib').filter((file) => !allowed.includes(file))).toEqual([]);
  });
});
