import { describe, it, expect } from 'vitest';
import { formatBytes, formatNumber, formatTimestamp } from '@/lib/preferences';

describe('formatBytes', () => {
  it('uses binary units by default', () => {
    expect(formatBytes(1024)).toBe('1 KiB');
    expect(formatBytes(1536)).toBe('1.5 KiB');
    expect(formatBytes(0)).toBe('0 B');
  });

  it('uses decimal units when asked', () => {
    expect(formatBytes(1000, 'decimal')).toBe('1 KB');
    expect(formatBytes(1500, 'decimal')).toBe('1.5 KB');
  });

  it('handles negatives and non-finite values', () => {
    expect(formatBytes(-2048)).toBe('-2 KiB');
    expect(formatBytes(Number.NaN)).toBe('—');
  });
});

describe('formatNumber', () => {
  it('groups digits when enabled', () => {
    expect(formatNumber(1000000, true)).toBe('1,000,000');
  });

  it('omits grouping when disabled', () => {
    expect(formatNumber(1000000, false)).toBe('1000000');
  });
});

describe('formatTimestamp', () => {
  const now = 1_700_000_000_000;

  it('formats an absolute time honouring 24h', () => {
    const out = formatTimestamp(now, { timeFormat: '24h', relative: false });
    expect(out).toMatch(/\d{4}/);
  });

  it('formats relative times', () => {
    expect(formatTimestamp(now - 90_000, { relative: true, now })).toBe('2m ago');
    expect(formatTimestamp(now - 5_000, { relative: true, now })).toBe('just now');
    expect(formatTimestamp(now - 7_200_000, { relative: true, now })).toBe('2h ago');
  });

  it('returns a dash for empty input', () => {
    expect(formatTimestamp(null)).toBe('—');
    expect(formatTimestamp(undefined)).toBe('—');
  });
});
