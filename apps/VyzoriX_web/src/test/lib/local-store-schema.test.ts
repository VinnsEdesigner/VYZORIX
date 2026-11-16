import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { rowsPerStatement, MAX_BOUND_VARIABLES } from '@/lib/local-store/protocol';
import { tableDefinition, TABLES, schemaStatements } from '@/lib/local-store/schema';

/** The schema version is read from the environment, so it is tested by reloading the module. */
async function loadSchemaVersion(envValue: string | undefined): Promise<number> {
  vi.resetModules();
  if (envValue === undefined) {
    vi.stubEnv('VITE_LOCAL_STORE_SCHEMA_VERSION', '');
  } else {
    vi.stubEnv('VITE_LOCAL_STORE_SCHEMA_VERSION', envValue);
  }
  const mod = await import('@/lib/local-store/schema');
  return mod.LOCAL_STORE_SCHEMA_VERSION;
}

afterEach(() => {
  vi.unstubAllEnvs();
});

describe('rowsPerStatement', () => {
  it('stays within SQLite\'s bound-variable limit', () => {
    // A wide table binds one value per column per row.
    expect(rowsPerStatement(10) * 10).toBeLessThanOrEqual(MAX_BOUND_VARIABLES);
    expect(rowsPerStatement(1)).toBe(MAX_BOUND_VARIABLES);
  });

  it('never returns zero, so a chunk is always progress', () => {
    // A table wider than the limit would otherwise produce an empty statement.
    expect(rowsPerStatement(MAX_BOUND_VARIABLES + 1)).toBe(1);
    expect(rowsPerStatement(0)).toBe(0);
  });
});

describe('schema', () => {
  it('defines a payload column on every table so a new field needs no migration', () => {
    const statements = schemaStatements();
    for (const table of TABLES) {
      const create = statements.find((s) => s.includes(`"${table.name}"`) && s.startsWith('CREATE TABLE'));
      expect(create, `missing CREATE for ${table.name}`).toBeDefined();
      expect(create).toContain('payload TEXT NOT NULL');
    }
  });

  it('declares the primary key, so the upsert has a conflict target', () => {
    // Without a PRIMARY KEY, ON CONFLICT has nothing to match and every upsert
    // fails with "ON CONFLICT clause does not match any PRIMARY KEY or UNIQUE
    // constraint".
    const statements = schemaStatements();
    for (const table of TABLES) {
      const create = statements.find(
        (s) => s.startsWith('CREATE TABLE') && s.includes(`"${table.name}"`)
      );
      expect(create, `missing CREATE for ${table.name}`).toContain(`"${table.pk}" PRIMARY KEY`);
    }
  });

  it('creates the bookkeeping table first', () => {
    expect(schemaStatements()[0]).toContain('_meta');
  });

  it('indexes every declared index', () => {
    const statements = schemaStatements();
    for (const table of TABLES) {
      for (const column of table.indexes ?? []) {
        expect(
          statements.some((s) => s.includes(`idx_${table.name}_${column}`)),
          `missing index ${table.name}.${column}`
        ).toBe(true);
      }
    }
  });

  it('gives every bounded table an age column', () => {
    for (const table of TABLES.filter((t) => t.bounded)) {
      expect(table.ageColumn, `${table.name} is bounded but has no ageColumn`).toBeTruthy();
      expect(table.columns).toContain(table.ageColumn);
    }
  });

  it('resolves a table by name and rejects an unknown one', () => {
    expect(tableDefinition('devices')?.pk).toBe('id');
    expect(tableDefinition('does_not_exist')).toBeUndefined();
  });
});

describe('schema version from the environment', () => {
  beforeEach(() => {
    vi.resetModules();
  });

  it('uses the environment value when it is a positive integer', async () => {
    expect(await loadSchemaVersion('7')).toBe(7);
  });

  it('falls back to the default when unset', async () => {
    expect(await loadSchemaVersion(undefined)).toBe(2);
  });

  it('falls back when the value is not a number', async () => {
    // A typo must not produce a version no database can match, which would
    // rebuild the cache on every single load.
    expect(await loadSchemaVersion('latest')).toBe(2);
  });

  it('falls back when the value is zero or negative', async () => {
    expect(await loadSchemaVersion('0')).toBe(2);
    expect(await loadSchemaVersion('-3')).toBe(2);
  });

  it('falls back when the value is fractional', async () => {
    expect(await loadSchemaVersion('1.5')).toBe(2);
  });
});
