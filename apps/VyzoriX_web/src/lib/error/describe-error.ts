export function describeError(error: unknown): string {
  const status = getStatus(error);
  const detail = getMessage(error);

  if (status === 429) {
    return `Too many requests (429). Wait a moment and try again.${detail ? ` ${detail}` : ''}`;
  }
  if (status === 403) {
    return `Not permitted (403). You may not have access to this organisation.${detail ? ` ${detail}` : ''}`;
  }
  if (status === 401) {
    return `Not signed in (401). Sign in again and retry.${detail ? ` ${detail}` : ''}`;
  }
  if (status !== undefined && status >= 500) {
    return `The server failed (${status}).${detail ? ` ${detail}` : ''}`;
  }
  if (status !== undefined) {
    return detail ? `${detail} (${status})` : `The request failed (${status}).`;
  }
  if (detail) {
    return detail;
  }
  return 'The request failed before it reached the server.';
}

function getStatus(error: unknown): number | undefined {
  if (error == null || typeof error !== 'object') return undefined;
  const record = error as { statusCode?: unknown; status?: unknown };
  if (typeof record.statusCode === 'number') return record.statusCode;
  if (typeof record.status === 'number') return record.status;
  return undefined;
}

function getMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === 'string') return error;
  return '';
}
