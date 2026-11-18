import { describe, it, expect } from 'vitest';
import { SERVER_KEY } from '@/hooks/preferences/use-preferences-sync';
import { PREFERENCE_KEYS_UNDER_TEST } from './preferences-test-helpers';

describe('preference sync keys', () => {
  it('matches the list the backend drift test reads', () => {
    const fromCode = Object.values(SERVER_KEY).sort();
    const fromHelper = [...PREFERENCE_KEYS_UNDER_TEST].sort();
    expect(fromCode).toEqual(fromHelper);
  });

  it('uses snake_case server keys', () => {
    for (const key of Object.values(SERVER_KEY)) {
      expect(key).toBe(key.toLowerCase());
      expect(key).not.toMatch(/[A-Z]/);
    }
  });
});
