export interface LocalStoreMetrics {

  recordRead(table: string, durationMs: number, rows: number): void;

  recordCapability(capability: 'ready' | 'unsupported' | 'disabled'): void;

  recordInvalidation(reason: 'schema' | 'integrity' | 'cursor' | 'signout' | 'manual'): void;

  recordSize(tables: Record<string, number>): void;

  recordEviction(table: string, removed: number): void;
}

let reporter: LocalStoreMetrics | null = null;

export function setLocalStoreMetrics(next: LocalStoreMetrics | null): void {
  reporter = next;
}

export function localStoreMetrics(): LocalStoreMetrics | null {
  return reporter;
}

export async function measureRead<T>(
  table: string,
  run: () => Promise<T[]>
): Promise<T[]> {
  const started = typeof performance !== 'undefined' ? performance.now() : Date.now();
  const rows = await run();

  if (reporter) {
    const ended = typeof performance !== 'undefined' ? performance.now() : Date.now();
    reporter.recordRead(table, ended - started, rows.length);
  }

  return rows;
}

export function createConsoleMetricsSink(): LocalStoreMetrics {
  const emit = (event: string, detail: Record<string, unknown>): void => {
    console.info('[local-store]', event, detail);
  };

  return {
    recordRead: (table, durationMs, rows) =>
      emit('read', { table, ms: Math.round(durationMs), rows }),
    recordCapability: (capability) => emit('capability', { capability }),
    recordInvalidation: (reason) => emit('invalidation', { reason }),
    recordSize: (tables) => emit('size', tables),
    recordEviction: (table, removed) => emit('eviction', { table, removed }),
  };
}
