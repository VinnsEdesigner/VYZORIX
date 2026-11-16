export type SqlValue = string | number | null | Uint8Array;

export interface OpenRequest {
  id: number;
  op: 'open';
  scope: string;
  salt: string;
}

export interface SelectRequest {
  id: number;
  op: 'select';
  table: string;

  columns?: string[];

  where?: Record<string, SqlValue>;

  orderBy?: { column: string; direction: 'asc' | 'desc' };
  limit?: number;
}

export interface CountRequest {
  id: number;
  op: 'count';
  table: string;
  where?: Record<string, SqlValue>;
}

export interface BulkInsertRequest {
  id: number;
  op: 'bulkInsert';
  table: string;
  columns: string[];
  rows: SqlValue[][];
}

export interface DeleteByKeysRequest {
  id: number;
  op: 'deleteByKeys';
  table: string;
  pk: string;
  keys: SqlValue[];
}

export interface EvictRequest {
  id: number;
  op: 'evict';
  table: string;
  ageColumn: string;
  olderThan: number;
}

export interface GetMetaRequest {
  id: number;
  op: 'getMeta';
  key: string;
}

export interface SetMetaRequest {
  id: number;
  op: 'setMeta';
  key: string;
  value: string;
}

export interface ClearRequest {
  id: number;
  op: 'clear';
}

export interface CloseRequest {
  id: number;
  op: 'close';
}

export type WorkerRequest =
  | OpenRequest
  | SelectRequest
  | CountRequest
  | BulkInsertRequest
  | DeleteByKeysRequest
  | EvictRequest
  | GetMetaRequest
  | SetMetaRequest
  | ClearRequest
  | CloseRequest;

export interface WorkerSuccess {
  id: number;
  ok: true;
  rows?: unknown[];

  changes?: number;

  value?: string | number | null;
}

export interface WorkerFailure {
  id: number;
  ok: false;
  error: string;
}

export type WorkerResponse = WorkerSuccess | WorkerFailure;

export interface OpenResult {
  isMaster: boolean;
  schemaVersion: number;
  database: string;
}

export const MAX_BOUND_VARIABLES = 30000;

export function rowsPerStatement(columnCount: number): number {
  if (columnCount <= 0) return 0;
  return Math.max(1, Math.floor(MAX_BOUND_VARIABLES / columnCount));
}
