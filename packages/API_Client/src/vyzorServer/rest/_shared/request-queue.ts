export interface RequestQueueConfig {

  staleMs: number;

  maxQueueSize: number;
}

export const DEFAULT_REQUEST_QUEUE_CONFIG: RequestQueueConfig = {
  staleMs: 0,
  maxQueueSize: 100,
};

interface InFlightEntry {
  promise: Promise<unknown>;
}

interface CacheEntry {
  value: unknown;
  expiresAt: number;
}

function stableStringify(value: unknown): string {
  if (value === undefined || value === null) return '';
  if (typeof value === 'string') return value;
  if (typeof value !== 'object') return String(value);
  if (Array.isArray(value)) {
    return '[' + value.map(stableStringify).join(',') + ']';
  }
  const obj = value as Record<string, unknown>;
  const keys = Object.keys(obj).sort();
  return '{' + keys.map((k) => JSON.stringify(k) + ':' + stableStringify(obj[k])).join(',') + '}';
}

function dedupKey(method: string, url: string, payload: unknown, scope?: string): string {
  return `${scope ? scope + '|' : ''}${method.toUpperCase()}:${url}:${stableStringify(payload)}`;
}

export class VyzorixRequestQueue {
  private config: RequestQueueConfig;
  private inFlight: Map<string, InFlightEntry> = new Map();
  private cache: Map<string, CacheEntry> = new Map();

  private stats = { deduped: 0, cached: 0, executed: 0 };

  constructor(config: Partial<RequestQueueConfig> = {}) {
    this.config = { ...DEFAULT_REQUEST_QUEUE_CONFIG, ...config };
  }

  execute<T>(
    method: string,
    url: string,
    payload: unknown,
    executor: () => Promise<T>,
    scope?: string,
    signal?: { aborted: boolean },
  ): Promise<T> {
    const upper = method.toUpperCase();

    if (signal) {
      this.stats.executed += 1;
      return executor();
    }

    const key = dedupKey(upper, url, payload, scope);
    const now = Date.now();

    if (upper === 'GET') {
      const hit = this.cache.get(key);
      if (hit && hit.expiresAt > now) {
        this.stats.cached += 1;
        return Promise.resolve(hit.value as T);
      }
      if (hit) this.cache.delete(key);
    }

    const pending = this.inFlight.get(key);
    if (pending) {
      this.stats.deduped += 1;
      return pending.promise as Promise<T>;
    }

    const promise = this.run<T>(upper, key, executor).finally(() => {
      if (this.inFlight.get(key)?.promise === promise) {
        this.inFlight.delete(key);
      }
    });

    this.inFlight.set(key, { promise });
    this.evictIfNeeded();
    return promise;
  }

  private async run<T>(method: string, key: string, executor: () => Promise<T>): Promise<T> {
    this.stats.executed += 1;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const value: any = await executor();
    if (method === 'GET') {
      this.cache.set(key, { value, expiresAt: Date.now() + this.config.staleMs });
    }
    return value as T;
  }

  private evictIfNeeded(): void {
    while (this.inFlight.size > this.config.maxQueueSize) {
      const oldest = this.inFlight.keys().next().value;
      if (oldest === undefined) break;
      this.inFlight.delete(oldest);
    }
  }

  invalidate(method: string, url: string, payload?: unknown, scope?: string): void {
    this.cache.delete(dedupKey(method, url, payload, scope));
  }

  clear(): void {
    this.cache.clear();

    this.inFlight.clear();
  }

  getStats(): { inFlight: number; cachedEntries: number; deduped: number; executed: number } {
    return { inFlight: this.inFlight.size, cachedEntries: this.cache.size, deduped: this.stats.deduped, executed: this.stats.executed };
  }

  getInFlightCount(): number {
    return this.inFlight.size;
  }
}

const queueState: { queue: VyzorixRequestQueue | null } = { queue: null };

export function getRequestQueue(config?: Partial<RequestQueueConfig>): VyzorixRequestQueue {
  if (!queueState.queue) {
    queueState.queue = new VyzorixRequestQueue(config);
  }
  return queueState.queue;
}

export function resetRequestQueue(): void {
  queueState.queue?.clear();
  queueState.queue = null;
}
