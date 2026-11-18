import { describe, it, expect } from 'vitest';
import {
  AVATAR_STYLES,
  BACKGROUND_SWATCHES,
  avatarStyleById,
  createAvatarDataUri,
  randomSeed,
} from '@/lib/avatar';

describe('avatar styles', () => {
  it('exposes a broad catalogue of modern styles', () => {
    expect(AVATAR_STYLES.length).toBeGreaterThanOrEqual(15);
    const ids = AVATAR_STYLES.map((style) => style.id);
    expect(ids).toContain('notionists');
    expect(ids).toContain('adventurer');
    expect(ids).toContain('bottts');
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('falls back to the first style for an unknown id', () => {
    expect(avatarStyleById('does-not-exist').id).toBe(AVATAR_STYLES[0]!.id);
  });
});

describe('createAvatarDataUri', () => {
  it('renders an SVG data URI for a seed', () => {
    const uri = createAvatarDataUri({ styleId: 'notionists', seed: 'Oscar', background: 'transparent' });
    expect(uri.startsWith('data:image/svg+xml')).toBe(true);
    expect(uri.length).toBeGreaterThan(200);
  });

  it('is deterministic for the same seed', () => {
    const a = createAvatarDataUri({ styleId: 'adventurer', seed: 'same-seed', background: 'transparent' });
    const b = createAvatarDataUri({ styleId: 'adventurer', seed: 'same-seed', background: 'transparent' });
    expect(a).toBe(b);
  });

  it('differs for a different seed', () => {
    const a = createAvatarDataUri({ styleId: 'adventurer', seed: 'seed-a', background: 'transparent' });
    const b = createAvatarDataUri({ styleId: 'adventurer', seed: 'seed-b', background: 'transparent' });
    expect(a).not.toBe(b);
  });

  it('applies a background colour when one is chosen', () => {
    const plain = createAvatarDataUri({ styleId: 'shapes', seed: 'x', background: 'transparent' });
    const tinted = createAvatarDataUri({ styleId: 'shapes', seed: 'x', background: '#f43f5e' });
    expect(tinted).not.toBe(plain);
    expect(decodeURIComponent(tinted)).toContain('#f43f5e');
  });

  it('offers a set of background swatches', () => {
    expect(BACKGROUND_SWATCHES).toContain('transparent');
    expect(BACKGROUND_SWATCHES.length).toBeGreaterThan(4);
  });
});

describe('randomSeed', () => {
  it('returns a non-empty string', () => {
    expect(randomSeed().length).toBeGreaterThan(0);
  });
});
