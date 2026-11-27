const DEFAULT_SCHEMA_VERSION = 2;

function readSchemaVersion(): number {
  const raw = import.meta.env.VITE_LOCAL_STORE_SCHEMA_VERSION;
  if (raw === undefined || raw === '') return DEFAULT_SCHEMA_VERSION;

  const parsed = Number(raw);
  if (!Number.isInteger(parsed) || parsed <= 0) return DEFAULT_SCHEMA_VERSION;

  return parsed;
}

export const LOCAL_STORE_SCHEMA_VERSION = readSchemaVersion();

export const META_TABLE = '_meta';

export interface TableDefinition {
  name: string;

  pk: string;
  columns: string[];

  indexes?: string[];

  bounded?: boolean;

  ageColumn?: string;
}

export const TABLES: TableDefinition[] = [
  {
    name: 'devices',
    pk: 'id',
    columns: ['id', 'organization_id', 'name', 'online', 'tier', 'fetched_at'],
    indexes: ['organization_id'],
  },
  {
    name: 'alerts',
    pk: 'id',
    columns: ['id', 'organization_id', 'enabled', 'fetched_at'],
    indexes: ['organization_id'],
  },
  {
    name: 'contact_points',
    pk: 'id',
    columns: ['id', 'organization_id', 'fetched_at'],
    indexes: ['organization_id'],
  },
  {
    name: 'logs',
    pk: 'id',
    columns: ['id', 'device_id', 'organization_id', 'at', 'fetched_at'],
    indexes: ['device_id', 'at'],
    bounded: true,
    ageColumn: 'at',
  },
  {
    name: 'telemetry',
    pk: 'id',
    columns: ['id', 'device_id', 'organization_id', 'at', 'fetched_at'],
    indexes: ['device_id', 'at'],
    bounded: true,
    ageColumn: 'at',
  },
  {
    name: 'events',
    pk: 'id',
    columns: ['id', 'device_id', 'organization_id', 'at', 'fetched_at'],
    indexes: ['device_id', 'at'],
    bounded: true,
    ageColumn: 'at',
  },
  {
    name: 'command_history',
    pk: 'id',
    columns: ['id', 'device_id', 'organization_id', 'at', 'fetched_at'],
    indexes: ['device_id', 'at'],
    bounded: true,
    ageColumn: 'at',
  },
  {

    name: 'user_prefs',
    pk: 'user_id',
    columns: ['user_id', 'theme', 'updated_at'],
  },
  {

    name: 'provisioning_tree',
    pk: 'key',
    columns: ['key', 'repository_id', 'dir', 'organization_id', 'fetched_at'],
    indexes: ['repository_id'],
  },
  {

    name: 'provisioning_file',
    pk: 'key',
    columns: ['key', 'repository_id', 'path', 'organization_id', 'fetched_at'],
    indexes: ['repository_id'],
  },
  {

    name: 'provisioning_resources',
    pk: 'key',
    columns: ['key', 'repository_id', 'organization_id', 'fetched_at'],
    indexes: ['repository_id'],
  },
  {

    name: 'provisioning_history',
    pk: 'key',
    columns: ['key', 'repository_id', 'organization_id', 'fetched_at'],
    indexes: ['repository_id'],
  },
];

export const BOUNDED_WINDOW_DAYS = 7;

export function createTableSql(table: TableDefinition): string {
  const columns = table.columns.map((c) => (c === table.pk ? `"${c}" PRIMARY KEY` : `"${c}"`)).join(', ');
  return `CREATE TABLE IF NOT EXISTS "${table.name}" (
    ${columns},
    payload TEXT NOT NULL
  )`;
}

export function createIndexSql(table: TableDefinition, column: string): string {
  return `CREATE INDEX IF NOT EXISTS "idx_${table.name}_${column}" ON "${table.name}" ("${column}")`;
}

export function createMetaTableSql(): string {
  return `CREATE TABLE IF NOT EXISTS "${META_TABLE}" (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
  )`;
}

export function schemaStatements(): string[] {
  const statements = [createMetaTableSql()];

  for (const table of TABLES) {
    statements.push(createTableSql(table));
    statements.push(createIndexSql(table, table.pk));
    for (const column of table.indexes ?? []) {
      statements.push(createIndexSql(table, column));
    }
  }

  return statements;
}

export function tableDefinition(name: string): TableDefinition | undefined {
  return TABLES.find((t) => t.name === name);
}
