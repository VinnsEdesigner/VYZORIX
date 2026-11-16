import { probeLocalStoreCapability, type LocalStoreCapability } from './capability';
import type { OpenResult, SqlValue } from './protocol';
import { BOUNDED_WINDOW_DAYS, TABLES, tableDefinition, type TableDefinition } from './schema';
import { localStoreMetrics, measureRead } from './metrics';
import { getOrCreateSalt, toBase64 } from './scope';

export interface StoredRow {
  [column: string]: unknown;
}

export interface SelectOptions {
  columns?: string[];
  where?: Record<string, SqlValue>;
  orderBy?: { column: string; direction: 'asc' | 'desc' };
  limit?: number;
}

export interface LocalStoreRepository {

  capability(): Promise<LocalStoreCapability>;

  open(): Promise<boolean>;

  isWriter(): boolean;

  select<T = StoredRow>(table: string, options?: SelectOptions): Promise<T[]>;

  count(table: string, where?: Record<string, SqlValue>): Promise<number>;

  upsert(table: string, rows: Record<string, SqlValue>[]): Promise<void>;

  deleteByKeys(table: string, keys: SqlValue[]): Promise<void>;

  evictExpired(): Promise<number>;

  cursor(): Promise<string | null>;

  setCursor(cursor: string): Promise<void>;

  clear(): Promise<void>;

  close(): Promise<void>;
}

export function createNullLocalStore(_scope?: string): LocalStoreRepository {
  return {
    capability: async () => 'unsupported',
    open: async () => false,
    isWriter: () => false,
    select: async () => [],
    count: async () => 0,
    upsert: async () => {},
    deleteByKeys: async () => {},
    evictExpired: async () => 0,
    cursor: async () => null,
    setCursor: async () => {},
    clear: async () => {},
    close: async () => {},
  };
}

function requireTable(name: string): TableDefinition {
  const definition = tableDefinition(name);
  if (!definition) throw new Error(`unknown local store table: ${name}`);
  return definition;
}

function decodeRow(row: Record<string, unknown>): StoredRow {
  const payload = row.payload;
  if (typeof payload !== 'string') return row;
  try {
    return JSON.parse(payload) as StoredRow;
  } catch (_ignored) {

    return row;
  }
}

interface Pending {
  resolve: (value: unknown) => void;
  reject: (error: Error) => void;
}

export function createWorkerLocalStore(scope: string): LocalStoreRepository {
  let worker: Worker | null = null;
  let nextId = 1;
  let openPromise: Promise<OpenResult> | null = null;
  let writer = false;
  const pending = new Map<number, Pending>();

  function send<T>(request: Record<string, unknown>): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      if (!worker) {
        reject(new Error('local store worker is not running'));
        return;
      }
      const id = nextId++;
      pending.set(id, { resolve: resolve as (value: unknown) => void, reject });
      worker.postMessage({ ...request, id });
    });
  }

  function resolveResponse(data: {
    rows?: unknown[];
    value?: string | number | null;
    changes?: number;
  }): unknown {
    if (data.value !== undefined) return data.value;
    if (data.changes !== undefined) return data.changes;
    return data.rows ?? [];
  }

  function failAll(reason: Error): void {
    for (const [, p] of pending) p.reject(reason);
    pending.clear();
  }

  function ensureWorker(): Promise<OpenResult> {
    if (openPromise) return openPromise;

    openPromise = (async () => {
      worker = new Worker(new URL('./db.worker.ts', import.meta.url), { type: 'module' });

      worker.onmessage = (event: MessageEvent) => {
        const data = event.data as {
          id: number;
          ok: boolean;
          rows?: unknown[];
          error?: string;
          value?: string | number | null;
          result?: OpenResult;
        };
        const entry = pending.get(data.id);
        if (!entry) return;
        pending.delete(data.id);

        if (data.ok) {
          entry.resolve(resolveResponse(data));
        } else {
          entry.reject(new Error(data.error ?? 'local store request failed'));
        }
      };

      worker.onerror = () => {
        failAll(new Error('local store worker failed'));
        openPromise = null;
        worker = null;
      };

      const opened = await send<OpenResult[]>({
        op: 'open',
        scope,
        salt: toBase64(getOrCreateSalt()),
      });
      const result = Array.isArray(opened) ? opened[0] : opened;
      writer = result?.isMaster ?? false;
      return result ?? { isMaster: false, schemaVersion: 0, database: '' };
    })().catch((error) => {
      openPromise = null;
      throw error;
    });

    return openPromise as Promise<OpenResult>;
  }

  return {
    capability: probeLocalStoreCapability,

    async open(): Promise<boolean> {
      try {
        await ensureWorker();
        return writer;
      } catch {
        return false;
      }
    },

    isWriter: () => writer,

    async select<T = StoredRow>(table: string, options: SelectOptions = {}): Promise<T[]> {
      try {
        requireTable(table);
        await ensureWorker();
        const rows = await measureRead(table, async () => {
          const raw = (await send<Record<string, unknown>[]>({
            op: 'select',
            table,
            columns: options.columns,
            where: options.where,
            orderBy: options.orderBy,
            limit: options.limit,
          })) ?? [];
          return raw.map(decodeRow);
        });
        return rows as T[];
      } catch (_ignored) {
        return [];
      }
    },

    async count(table: string, where: Record<string, SqlValue> = {}): Promise<number> {
      try {
        requireTable(table);
        await ensureWorker();
        const value = await send<number>({ op: 'count', table, where });
        return typeof value === 'number' ? value : 0;
      } catch (_ignored) {
        return 0;
      }
    },

    async upsert(table: string, rows: Record<string, SqlValue>[]): Promise<void> {
      if (rows.length === 0) return;
      const definition = requireTable(table);
      const columns = [...definition.columns, 'payload'];

      const now = Date.now();

      const values = rows.map((row) =>
        columns.map((column) => {
          if (column === 'payload') return JSON.stringify(row);
          if (column === 'fetched_at') return (row[column] as SqlValue) ?? now;
          return row[column] ?? null;
        })
      );

      try {
        await ensureWorker();
        await send({ op: 'bulkInsert', table: definition.name, columns, rows: values });
      } catch (_ignored) {
        void _ignored;
      }
    },

    async deleteByKeys(table: string, keys: SqlValue[]): Promise<void> {
      if (keys.length === 0) return;
      const definition = requireTable(table);
      try {
        await ensureWorker();
        await send({ op: 'deleteByKeys', table: definition.name, pk: definition.pk, keys });
      } catch (_ignored) {
        void _ignored;
      }
    },

    async evictExpired(): Promise<number> {
      const cutoff = Date.now() - BOUNDED_WINDOW_DAYS * 24 * 60 * 60 * 1000;
      let removed = 0;

      for (const table of TABLES.filter((t) => t.bounded && t.ageColumn)) {
        try {
          await ensureWorker();
          const changes = await send<number>({
            op: 'evict',
            table: table.name,
            ageColumn: table.ageColumn as string,
            olderThan: cutoff,
          });
          if (typeof changes === 'number' && changes > 0) {
            removed += changes;
            localStoreMetrics()?.recordEviction(table.name, changes);
          }
        } catch (_ignored) {
          void _ignored;
        }
      }

      return removed;
    },

    async cursor(): Promise<string | null> {
      try {
        await ensureWorker();
        const value = await send<string | null>({ op: 'getMeta', key: 'sync_cursor' });
        return typeof value === 'string' && value !== '' ? value : null;
      } catch (_ignored) {
        return null;
      }
    },

    async setCursor(cursor: string): Promise<void> {
      try {
        await ensureWorker();
        await send({ op: 'setMeta', key: 'sync_cursor', value: cursor });
      } catch (_ignored) {
        void _ignored;
      }
    },

    async clear(): Promise<void> {
      try {
        await ensureWorker();
        await send({ op: 'clear' });
        localStoreMetrics()?.recordInvalidation('manual');
      } catch (_ignored) {
        void _ignored;
      }
    },

    async close(): Promise<void> {
      try {
        if (worker) await send({ op: 'close' });
      } catch (_ignored) {
        void _ignored;
      } finally {
        worker?.terminate();
        worker = null;
        openPromise = null;
        writer = false;
        failAll(new Error('local store closed'));
      }
    },
  };
}
