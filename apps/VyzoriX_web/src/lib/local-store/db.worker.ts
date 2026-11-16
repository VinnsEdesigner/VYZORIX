import sqlite3InitModule, { type Database } from '@sqlite.org/sqlite-wasm';
import {
  LOCAL_STORE_SCHEMA_VERSION,
  META_TABLE,
  TABLES,
  schemaStatements,
  tableDefinition,
  type TableDefinition,
} from './schema';
import { createCodec, databaseNameFor, fromBase64, type Codec } from './scope';
import {
  rowsPerStatement,
  type CountRequest,
  type EvictRequest,
  type SelectRequest,
  type SqlValue,
  type WorkerRequest,
  type WorkerResponse,
} from './protocol';

const VFS_NAME = 'opfs-sahpool';
const LOCK_NAME = 'vyzorix-local-store';
const LOCK_RETRY_MS = 3000;
const LOCK_RETRY_INTERVAL_MS = 100;

let db: Database | null = null;
let isMaster = false;
let pool: { unlink: (name: string) => boolean; getFileNames: () => string[] } | null = null;
let databaseFile = '';
let codec: Codec | null = null;
let scope = '';
let saltBase64 = '';

function applyPragmas(handle: Database): void {
  handle.exec('PRAGMA journal_mode = WAL');
  handle.exec('PRAGMA wal_autocheckpoint = 1000');
  handle.exec('PRAGMA locking_mode = NORMAL');
  handle.exec('PRAGMA synchronous = NORMAL');
  handle.exec('PRAGMA temp_store = FILE');
  handle.exec('PRAGMA cache_size = -131072');
  handle.exec('PRAGMA cache_spill = ON');
}

function readUserVersion(handle: Database): number {
  const rows = handle.exec('PRAGMA user_version', { rowMode: 'array', returnValue: 'resultRows' }) as SqlValue[][];
  const first = rows[0]?.[0];
  return typeof first === 'number' ? first : 0;
}

function integrityOk(handle: Database): boolean {
  const quick = handle.exec('PRAGMA quick_check', { rowMode: 'array', returnValue: 'resultRows' }) as SqlValue[][];
  if (quick[0]?.[0] === 'ok') return true;

  const full = handle.exec('PRAGMA integrity_check', { rowMode: 'array', returnValue: 'resultRows' }) as SqlValue[][];
  return full[0]?.[0] === 'ok';
}

function createSchema(handle: Database): void {
  for (const statement of schemaStatements()) {
    handle.exec(statement);
  }
  handle.exec(`PRAGMA user_version = ${LOCAL_STORE_SCHEMA_VERSION}`);
}

function rebuild(handle: Database): void {
  const tables = handle.exec(
    "SELECT name FROM sqlite_master WHERE type = 'table'",
    { rowMode: 'array', returnValue: 'resultRows' }
  ) as SqlValue[][];

  for (const row of tables) {
    const name = row[0];
    if (typeof name === 'string' && !name.startsWith('sqlite_')) {
      handle.exec(`DROP TABLE IF EXISTS "${name}"`);
    }
  }

  createSchema(handle);
}

async function openDatabase(): Promise<void> {
  if (!isMaster) {
    return;
  }

  const sqlite3 = await sqlite3InitModule();

  const installed = await sqlite3.installOpfsSAHPoolVfs({
    name: VFS_NAME,
    initialCapacity: 6,

    clearOnInit: false,
  });
  pool = installed as unknown as typeof pool;

  const salt = fromBase64(saltBase64);
  codec = await createCodec(scope, salt);
  databaseFile = await databaseNameFor(scope, salt);

  const handle = new installed.OpfsSAHPoolDb(databaseFile);

  applyPragmas(handle);

  const needsRebuild =
    readUserVersion(handle) !== LOCAL_STORE_SCHEMA_VERSION || !integrityOk(handle);

  if (needsRebuild) {
    rebuild(handle);
  } else {
    createSchema(handle);
  }

  db = handle;
}

async function tryAcquireLock(): Promise<boolean> {
  return new Promise((resolve) => {
    void navigator.locks
      .request(LOCK_NAME, { mode: 'exclusive', ifAvailable: true }, (lock) => {
        if (lock === null) {
          resolve(false);
          return;
        }

        resolve(true);

        return new Promise<void>(() => {});
      })
      .catch(() => resolve(false));
  });
}

function acquireWriterLock(): Promise<boolean> {
  // ifAvailable never queues, so nothing can be left pending to acquire the lock
  // later — which is what made a timed-out request leak it. The cost is that a
  // transiently-held lock reads as unavailable, and a page reload is exactly
  // that: the previous worker's terminate() is asynchronous, so its lock is
  // still held for a moment. Retry within a small budget rather than giving up.
  const deadline = Date.now() + LOCK_RETRY_MS;

  const attempt = async (): Promise<boolean> => {
    if (await tryAcquireLock()) return true;
    if (Date.now() >= deadline) return false;
    await new Promise((r) => setTimeout(r, LOCK_RETRY_INTERVAL_MS));
    return attempt();
  };

  return attempt();
}

async function encodeValue(value: SqlValue): Promise<SqlValue> {
  if (value === null || typeof value === 'number' || value instanceof Uint8Array) {
    return value;
  }
  if (!codec) throw new Error('codec is not ready');
  return codec.encodeDeterministic(value);
}

async function encodePayload(value: SqlValue): Promise<SqlValue> {
  if (typeof value !== 'string') return value;
  if (!codec) throw new Error('codec is not ready');
  return codec.encodeRandom(value);
}

async function decodeRow(row: Record<string, unknown>): Promise<Record<string, unknown>> {
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(row)) {
    if (key === 'payload' && typeof value === 'string') {
      if (!codec) throw new Error('codec is not ready');
      const json = await codec.decode(value);
      try {
        out.data = JSON.parse(json);
      } catch {
        out.data = json;
      }
      continue;
    }
    out[key] = value;
  }
  return out;
}

async function decodeRows(rows: Record<string, unknown>[]): Promise<Record<string, unknown>[]> {
  const out: Record<string, unknown>[] = [];
  for (const row of rows) out.push(await decodeRow(row));
  return out;
}

function requireWriter(): void {
  if (!isMaster) throw new Error('local store is read-only in this tab');
}

async function handleBulkInsert(table: string, columns: string[], rows: SqlValue[][]): Promise<number> {
  if (!db) throw new Error('database is not open');

  const definition = tableDefinition(table);
  if (!definition) throw new Error(`unknown local store table: ${table}`);

  const perStatement = rowsPerStatement(columns.length);
  if (perStatement === 0) return 0;

  const columnList = columns.map((c) => `"${c}"`).join(', ');
  const rowPlaceholder = `(${columns.map(() => '?').join(', ')})`;
  const conflictTarget = `"${definition.pk}"`;
  const updates = columns
    .filter((c) => c !== definition.pk)
    .map((c) => `"${c}" = excluded."${c}"`)
    .join(', ');

  const encoded: SqlValue[][] = [];
  for (const row of rows) {
    const out: SqlValue[] = [];
    for (let i = 0; i < row.length; i += 1) {
      const value = row[i] ?? null;
      out.push(columns[i] === 'payload' ? await encodePayload(value) : await encodeValue(value));
    }
    encoded.push(out);
  }

  let changes = 0;

  db.exec('BEGIN');
  try {
    for (let offset = 0; offset < encoded.length; offset += perStatement) {
      const chunk = encoded.slice(offset, offset + perStatement);
      const placeholders = chunk.map(() => rowPlaceholder).join(', ');
      const sql =
        `INSERT INTO "${definition.name}" (${columnList}) VALUES ${placeholders} ` +
        `ON CONFLICT(${conflictTarget}) DO UPDATE SET ${updates}`;

      db.exec(sql, { bind: chunk.flat() });
      changes += chunk.length;
    }
    db.exec('COMMIT');
  } catch (error) {
    db.exec('ROLLBACK');
    throw error;
  }

  return changes;
}

async function handleDeleteByKeys(table: string, pk: string, keys: SqlValue[]): Promise<number> {
  if (!db) throw new Error('database is not open');

  const definition = tableDefinition(table);
  if (!definition || definition.pk !== pk) throw new Error(`unknown local store table: ${table}`);

  const encoded: SqlValue[] = [];
  for (const key of keys) encoded.push(await encodeValue(key));

  const placeholders = encoded.map(() => '?').join(', ');
  db.exec(`DELETE FROM "${definition.name}" WHERE "${pk}" IN (${placeholders})`, { bind: encoded });

  return encoded.length;
}

function quote(identifier: string): string {
  return `"${identifier.replace(/"/g, '""')}"`;
}

function requireTable(name: string): TableDefinition {
  const definition = tableDefinition(name);
  if (!definition) throw new Error(`unknown local store table: ${name}`);
  return definition;
}

function requireColumn(definition: TableDefinition, column: string): string {
  if (!definition.columns.includes(column)) {
    throw new Error(`unknown column ${column} on ${definition.name}`);
  }
  return column;
}

async function handleSelect(request: SelectRequest): Promise<unknown[]> {
  if (!db) throw new Error('database is not open');
  const definition = requireTable(request.table);

  const columns =
    request.columns && request.columns.length > 0
      ? request.columns.map((c) => quote(requireColumn(definition, c))).join(', ')
      : 'payload';

  const params: SqlValue[] = [];
  let sql = `SELECT ${columns} FROM ${quote(definition.name)}`;

  if (request.where && Object.keys(request.where).length > 0) {
    const clauses: string[] = [];
    for (const [column, value] of Object.entries(request.where)) {
      params.push(await encodeValue(value));
      clauses.push(`${quote(requireColumn(definition, column))} = ?`);
    }
    sql += ` WHERE ${clauses.join(' AND ')}`;
  }

  if (request.orderBy) {
    const column = requireColumn(definition, request.orderBy.column);
    const direction = request.orderBy.direction === 'desc' ? 'DESC' : 'ASC';
    sql += ` ORDER BY ${quote(column)} ${direction}`;
  }

  if (typeof request.limit === 'number' && request.limit > 0) {
    sql += ' LIMIT ?';
    params.push(request.limit);
  }

  const rows = db.exec(sql, { bind: params, rowMode: 'object', returnValue: 'resultRows' }) as Record<string, unknown>[];
  return decodeRows(rows);
}

async function handleCount(request: CountRequest): Promise<number> {
  if (!db) throw new Error('database is not open');
  const definition = requireTable(request.table);

  const params: SqlValue[] = [];
  let sql = `SELECT COUNT(*) AS n FROM ${quote(definition.name)}`;

  if (request.where && Object.keys(request.where).length > 0) {
    const clauses: string[] = [];
    for (const [column, value] of Object.entries(request.where)) {
      params.push(await encodeValue(value));
      clauses.push(`${quote(requireColumn(definition, column))} = ?`);
    }
    sql += ` WHERE ${clauses.join(' AND ')}`;
  }

  const rows = db.exec(sql, { bind: params, rowMode: 'array', returnValue: 'resultRows' }) as SqlValue[][];
  const n = rows[0]?.[0];
  return typeof n === 'number' ? n : 0;
}

async function handleEvict(request: EvictRequest): Promise<number> {
  requireWriter();
  if (!db) throw new Error('database is not open');

  const definition = requireTable(request.table);
  const ageColumn = requireColumn(definition, request.ageColumn);

  const before = await handleCount({ id: 0, op: 'count', table: definition.name });
  db.exec(`DELETE FROM ${quote(definition.name)} WHERE ${quote(ageColumn)} < ?`, {
    bind: [request.olderThan],
  });
  const after = await handleCount({ id: 0, op: 'count', table: definition.name });

  return before - after;
}

async function handleGetMeta(key: string): Promise<string | null> {
  if (!db) throw new Error('database is not open');
  const rows = db.exec(`SELECT value FROM ${quote(META_TABLE)} WHERE key = ?`, {
    bind: [key],
    rowMode: 'array',
    returnValue: 'resultRows',
  }) as SqlValue[][];
  const value = rows[0]?.[0];
  if (typeof value !== 'string') return null;
  if (!codec) throw new Error('codec is not ready');
  return codec.decode(value);
}

async function handleSetMeta(key: string, value: string): Promise<void> {
  requireWriter();
  if (!db) throw new Error('database is not open');
  db.exec(
    `INSERT INTO ${quote(META_TABLE)} (key, value) VALUES (?, ?) ` +
      `ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
    { bind: [key, codec ? await codec.encodeRandom(value) : value] }
  );
}

function handleClear(): void {
  requireWriter();
  if (!db) throw new Error('database is not open');

  for (const table of TABLES) {
    db.exec(`DELETE FROM ${quote(table.name)}`);
  }
  db.exec(`DELETE FROM ${quote(META_TABLE)}`);
}

async function dispatch(request: WorkerRequest): Promise<WorkerResponse> {
  try {
    switch (request.op) {
      case 'open':
        await openWithScope(request.scope, request.salt);
        return {
          id: request.id,
          ok: true,
          rows: [{ isMaster, schemaVersion: LOCAL_STORE_SCHEMA_VERSION, database: databaseFile }],
        };

      case 'select':
        return { id: request.id, ok: true, rows: await handleSelect(request) };

      case 'count':
        return { id: request.id, ok: true, value: await handleCount(request) };

      case 'bulkInsert':
        requireWriter();
        return {
          id: request.id,
          ok: true,
          changes: await handleBulkInsert(request.table, request.columns, request.rows),
        };

      case 'deleteByKeys':
        requireWriter();
        return {
          id: request.id,
          ok: true,
          changes: await handleDeleteByKeys(request.table, request.pk, request.keys),
        };

      case 'evict':
        return { id: request.id, ok: true, changes: await handleEvict(request) };

      case 'getMeta':
        return { id: request.id, ok: true, value: await handleGetMeta(request.key) };

      case 'setMeta':
        await handleSetMeta(request.key, request.value);
        return { id: request.id, ok: true };

      case 'clear':
        handleClear();
        return { id: request.id, ok: true };

      case 'close':
        db?.close();
        db = null;
        return { id: request.id, ok: true };

      default:
        return { id: (request as { id: number }).id, ok: false, error: 'unknown operation' };
    }
  } catch (error) {
    return {
      id: request.id,
      ok: false,
      error: error instanceof Error ? error.message : String(error),
    };
  }
}

let queue: Promise<void> = Promise.resolve();

let opened = false;

async function init(): Promise<void> {
  isMaster = await acquireWriterLock();
}

async function dropPlaintextDatabase(): Promise<void> {
  if (!pool || !isMaster) return;
  for (const name of pool.getFileNames()) {
    if (name === 'vyzorix.db' || name.startsWith('vyzorix.db-')) {
      try {
        pool.unlink(name);
      } catch {
        void 0;
      }
    }
  }
}

async function openWithScope(next: string, nextSalt: string): Promise<void> {
  if (opened) return;
  opened = true;
  scope = next;
  saltBase64 = nextSalt;
  await openDatabase();
  await dropPlaintextDatabase();
}

queue = init()
  .then(() => {
    self.postMessage({ type: 'ready', isMaster });
  })
  .catch((error: unknown) => {
    self.postMessage({
      type: 'init-failed',
      error: error instanceof Error ? error.message : String(error),
    });
  });

self.onmessage = (event: MessageEvent<WorkerRequest>) => {
  const request = event.data;
  queue = queue.then(async () => {
    self.postMessage(await dispatch(request));
  });
};
