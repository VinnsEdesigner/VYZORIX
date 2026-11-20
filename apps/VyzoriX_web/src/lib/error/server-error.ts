export interface ServerErrorInfo {

  message: string;

  code: string;

  status?: number;
}

export function serverErrorInfo(error: unknown): ServerErrorInfo {
  if (error == null || typeof error !== 'object') {
    return { message: typeof error === 'string' ? error : '', code: '' };
  }

  const record = error as { message?: unknown; code?: unknown; statusCode?: unknown; status?: unknown };

  const message = typeof record.message === 'string' ? record.message : '';
  const code = typeof record.code === 'string' ? record.code : '';
  const status =
    typeof record.statusCode === 'number'
      ? record.statusCode
      : typeof record.status === 'number'
        ? record.status
        : undefined;

  return { message, code, status };
}

export function isDuplicateResource(error: unknown): boolean {
  const { code, status } = serverErrorInfo(error);
  return code === 'RESOURCE_ALREADY_EXISTS' || status === 409;
}
