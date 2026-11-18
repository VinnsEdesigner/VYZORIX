import { getSigningKey } from '../_shared/rest-client';
import { signHttpRequestBrowser } from '../../crypto/browser-sign';

function simpleHashPayload(data: unknown): string {
  const str = typeof data === 'string' ? data : (data === undefined ? '' : JSON.stringify(data)) ?? '';
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i);
    hash = (hash << 5) - hash + char;
    hash = hash & hash;
  }
  return Math.abs(hash).toString(36);
}

export interface GraphQLBatchedOperation {
  query: string;
  variables?: Record<string, unknown>;
  operationName?: string;
}

export interface BatchConfig {

  windowMs: number;

  maxBatchSize: number;

  enableREST: boolean;

  enableGraphQL: boolean;
}

const DEFAULT_CONFIG: BatchConfig = {
  windowMs: 50,
  maxBatchSize: 10,
  enableREST: true,
  enableGraphQL: true,
};

interface BatchedGraphQLOperation {
  query: string;
  variables?: Record<string, unknown>;
  operationName?: string;
  callers: {
    resolve: (value: unknown) => void;
    reject: (error: Error) => void;
    operationName?: string;
  }[];
  timer?: ReturnType<typeof setTimeout>;
  executed: boolean;
}

export class GraphQLBatcher {
  private pendingOperations: Map<string, BatchedGraphQLOperation> = new Map();
  private config: BatchConfig;
  private batchUrl: string = '';
  private authToken: string = '';

  constructor(config: Partial<BatchConfig> = {}) {
    this.config = { ...DEFAULT_CONFIG, ...config };
  }

  configure(baseUrl: string, orgId: string, authToken: string): void {
    this.batchUrl = `${baseUrl}/${orgId}/graphql/batch`;
    this.authToken = authToken;
  }

  private getBatchKey(query: string, variables?: Record<string, unknown>): string {
    return simpleHashPayload({ query, variables });
  }

  async execute<T>(
    query: string,
    variables: Record<string, unknown> | undefined,
    operationName: string | undefined,
    _executor: () => Promise<T>
  ): Promise<T> {
    if (!this.config.enableGraphQL || !this.batchUrl) {
      return _executor();
    }

    const batchKey = this.getBatchKey(query, variables);

    const existing = this.pendingOperations.get(batchKey);
    if (existing && existing.query === query && JSON.stringify(existing.variables) === JSON.stringify(variables)) {
      return new Promise<T>((resolve, reject) => {
        existing.callers.push({
          resolve: resolve as (value: unknown) => void,
          reject,
          operationName,
        });
      });
    }

    return new Promise<T>((resolve, reject) => {
      const batchedOp: BatchedGraphQLOperation = {
        query,
        variables,
        operationName,
        callers: [{ resolve: resolve as (value: unknown) => void, reject, operationName }],
        timer: setTimeout(() => {
          this.flushBatch();
        }, this.config.windowMs),
        executed: false,
      };

      this.pendingOperations.set(batchKey, batchedOp);
    });
  }

  private async flushBatch(): Promise<void> {
    const operations: GraphQLBatchedOperation[] = [];
    const keyedOperations: { key: string; op: BatchedGraphQLOperation }[] = [];

    for (const [key, op] of this.pendingOperations.entries()) {
      if (op.executed) continue;
      op.executed = true;
      if (op.timer) clearTimeout(op.timer);

      operations.push({
        query: op.query,
        variables: op.variables,
        operationName: op.operationName,
      });
      keyedOperations.push({ key, op });

      if (operations.length >= this.config.maxBatchSize) break;
    }

    if (operations.length === 0) return;

    this.pendingOperations.clear();

    try {
      const body = JSON.stringify(operations);
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
      };
      if (this.authToken) {
        headers['Authorization'] = `Bearer ${this.authToken}`;
      }

      const signingKey = getSigningKey();
      if (signingKey) {
        const signHeaders = await signHttpRequestBrowser('POST', this.batchUrl, body, signingKey);
        headers['X-Vyzorix-Timestamp'] = signHeaders['X-Vyzorix-Timestamp'];
        headers['X-Vyzorix-Nonce'] = signHeaders['X-Vyzorix-Nonce'];
        headers['X-Vyzorix-Signature'] = signHeaders['X-Vyzorix-Signature'];
      }

      const response = await fetch(this.batchUrl, {
        method: 'POST',
        headers,
        body,
      });

      if (!response.ok) {
        throw new Error(`Batch request failed: ${response.status}`);
      }

      const results = await response.json() as { data?: unknown; errors?: { message: string }[] }[];

      for (let i = 0; i < keyedOperations.length; i++) {
        const entry = keyedOperations[i];
        if (!entry) continue;
        const { op } = entry;
        const result = results[i];

        if (!result) {
          for (const caller of op.callers) {
            caller.reject(new Error('Batch response missing result'));
          }
          continue;
        }

        if (result.errors && result.errors.length > 0) {
          const firstError = result.errors[0];
          const err = new Error(firstError?.message ?? 'GraphQL batch error');
          for (const caller of op.callers) {
            caller.reject(err);
          }
        } else {
          for (const caller of op.callers) {
            caller.resolve(result);
          }
        }
      }
    } catch (error) {
      const err = error instanceof Error ? error : new Error(String(error));
      for (const { op } of keyedOperations) {
        for (const caller of op.callers) {
          caller.reject(err);
        }
      }
    }
  }

  clear(): void {
    for (const op of this.pendingOperations.values()) {
      if (op.timer) clearTimeout(op.timer);
      const error = new Error('GraphQL batcher cleared');
      for (const caller of op.callers) {
        caller.reject(error);
      }
    }
    this.pendingOperations.clear();
  }

  getPendingCount(): number {
    return this.pendingOperations.size;
  }
}

const batcherState = {
  graphql: null as GraphQLBatcher | null,
};

export function getGraphQLBatcher(config?: Partial<BatchConfig>): GraphQLBatcher {
  if (!batcherState.graphql) {
    batcherState.graphql = new GraphQLBatcher(config);
  }
  return batcherState.graphql;
}

export function resetBatchers(): void {
  if (batcherState.graphql) {
    batcherState.graphql.clear();
  }
  batcherState.graphql = null;
}
