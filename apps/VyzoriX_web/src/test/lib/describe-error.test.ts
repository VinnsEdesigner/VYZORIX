import { describe, it, expect } from 'vitest';
import { describeError } from '@/lib/error';

describe('describeError', () => {
  it('names a 429 as a rate limit, with the status', () => {
    const err = Object.assign(new Error('too many requests'), { statusCode: 429 });
    expect(describeError(err)).toMatch(/429/);
    expect(describeError(err)).toMatch(/too many/i);
  });

  it('reads statusCode (ApiError) as well as status (axios)', () => {
    const api = Object.assign(new Error('nope'), { statusCode: 403 });
    const axios = Object.assign(new Error('nope'), { status: 403 });
    expect(describeError(api)).toMatch(/403/);
    expect(describeError(axios)).toMatch(/403/);
  });

  it('explains a 403 as a permission problem', () => {
    const err = Object.assign(new Error('forbidden'), { statusCode: 403 });
    expect(describeError(err)).toMatch(/not permitted/i);
  });

  it('explains a 500 as a server failure, not the user\u2019s fault', () => {
    const err = Object.assign(new Error('boom'), { statusCode: 500 });
    expect(describeError(err)).toMatch(/server failed/i);
    expect(describeError(err)).toMatch(/500/);
  });

  it('includes the server message when there is one', () => {
    const err = Object.assign(new Error('invalid week start'), { statusCode: 400 });
    expect(describeError(err)).toContain('invalid week start');
    expect(describeError(err)).toContain('400');
  });

  it('falls back to the message when there is no status', () => {
    expect(describeError(new Error('no organization selected'))).toBe('no organization selected');
  });

  it('accepts a bare string', () => {
    expect(describeError('something odd')).toBe('something odd');
  });

  it('never returns an empty string', () => {
    for (const value of [null, undefined, {}, 0, '']) {
      expect(describeError(value).length).toBeGreaterThan(0);
    }
  });
});
