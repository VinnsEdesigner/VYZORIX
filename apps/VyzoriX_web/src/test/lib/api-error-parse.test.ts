import { describe, expect, it } from 'vitest';
import { parseApiError } from '@vyzorix/api-client';
import { isDuplicateResource, serverErrorInfo } from '@/lib/error/server-error';

function axiosErrorWith(status: number, data: unknown) {
  return {
    isAxiosError: true,
    name: 'AxiosError',
    message: 'Request failed',
    response: { status, statusText: '', data, headers: {}, config: {} },
  } as never;
}

describe('parseApiError', () => {
  it('reads the nested error envelope', () => {
    const err = parseApiError(
      axiosErrorWith(409, {
        error: {
          code: 'RESOURCE_ALREADY_EXISTS',
          message: 'a token with that name already exists on this service account',
          trace_id: 'abc',
          docs_url: 'https://docs.vyzorix.com/errors/RESOURCE_ALREADY_EXISTS',
        },
      }),
    );

    expect(err.message).toBe('a token with that name already exists on this service account');
    expect(err.code).toBe('RESOURCE_ALREADY_EXISTS');
    expect(err.statusCode).toBe(409);
  });

  it('still reads the flat shape', () => {
    const err = parseApiError(
      axiosErrorWith(400, { error: 'invalid_input', message: 'name is required' }),
    );

    expect(err.message).toBe('name is required');
    expect(err.code).toBe('invalid_input');
  });

  it('falls back to the status mapping when the body carries no code', () => {
    const err = parseApiError(axiosErrorWith(404, { message: 'gone' }));
    expect(err.statusCode).toBe(404);
    expect(err.message).toBe('gone');
  });
});

describe('serverErrorInfo / isDuplicateResource', () => {
  it('surfaces the server message and code verbatim', () => {
    const err = parseApiError(
      axiosErrorWith(409, {
        error: { code: 'RESOURCE_ALREADY_EXISTS', message: 'a token with that name already exists' },
      }),
    );

    const info = serverErrorInfo(err);
    expect(info.message).toBe('a token with that name already exists');
    expect(info.code).toBe('RESOURCE_ALREADY_EXISTS');
    expect(info.status).toBe(409);
    expect(isDuplicateResource(err)).toBe(true);
  });

  it('treats a bare 409 as a duplicate', () => {
    expect(isDuplicateResource({ statusCode: 409 })).toBe(true);
  });

  it('does not treat an unrelated error as a duplicate', () => {
    expect(isDuplicateResource({ statusCode: 500, code: 'INTERNAL_ERROR' })).toBe(false);
    expect(isDuplicateResource(null)).toBe(false);
  });
});
