import { describe, it, expect } from 'vitest';
import { filterOptions, isNewGroup, itemToString } from '@vyzorix/ui';

const options = [
  { value: '', label: 'Default' },
  { value: 'dark', label: 'Vyzorix Dark' },
  { value: 'light', label: 'Vyzorix Light' },
  { value: 'tron', label: 'Tron', group: 'Experimental' },
];

describe('itemToString', () => {
  it('prefers the label', () => {
    expect(itemToString({ value: 'dark', label: 'Vyzorix Dark' })).toBe('Vyzorix Dark');
  });

  it('falls back to the value', () => {
    expect(itemToString({ value: 'dark' })).toBe('dark');
  });

  it('is empty for null or undefined', () => {
    expect(itemToString(null)).toBe('');
    expect(itemToString(undefined)).toBe('');
  });
});

describe('filterOptions', () => {
  it('returns everything for an empty query', () => {
    expect(filterOptions(options, '')).toHaveLength(4);
    expect(filterOptions(options, '   ')).toHaveLength(4);
  });

  it('matches on label, case-insensitively', () => {
    expect(filterOptions(options, 'dark').map((o) => o.value)).toEqual(['dark']);
    expect(filterOptions(options, 'TRON').map((o) => o.value)).toEqual(['tron']);
  });

  it('matches on description', () => {
    const withDesc = [{ value: 'a', label: 'A', description: 'Inherit the organisation default' }];
    expect(filterOptions(withDesc, 'inherit')).toHaveLength(1);
  });

  it('returns an empty list when nothing matches', () => {
    expect(filterOptions(options, 'zzz')).toHaveLength(0);
  });
});

describe('isNewGroup', () => {
  it('is true for the first option of a group', () => {
    expect(isNewGroup({ value: 'a', group: 'G' })).toBe(true);
  });

  it('is false for a second option in the same group', () => {
    expect(isNewGroup({ value: 'b', group: 'G' }, { value: 'a', group: 'G' })).toBe(false);
  });

  it('is true when the group changes', () => {
    expect(isNewGroup({ value: 'b', group: 'H' }, { value: 'a', group: 'G' })).toBe(true);
  });

  it('is true when leaving a group for an ungrouped option', () => {
    expect(isNewGroup({ value: 'b' }, { value: 'a', group: 'G' })).toBe(true);
  });
});
