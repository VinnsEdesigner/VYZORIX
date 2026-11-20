import { createAvatar, type Style } from '@dicebear/core';
import { bottts, identicon, shapes } from '@dicebear/collection';

const SERVICE_ACCOUNT_STYLES: Array<{ id: string; style: Style<Record<string, unknown>> }> = [
  { id: 'bottts', style: bottts },
  { id: 'identicon', style: identicon },
  { id: 'shapes', style: shapes },
];

const SERVICE_ACCOUNT_BACKGROUNDS = ['1c1c1c', '242424', '2a2a2a', '1f1f1f', '212121'];

function hash(value: string): number {
  let h = 0x811c9dc5;
  for (let i = 0; i < value.length; i += 1) {
    h ^= value.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return h >>> 0;
}

export function serviceAccountAvatar(seed: string, size = 64): string {
  const key = seed || 'service-account';
  const h = hash(key);
  const { style } = SERVICE_ACCOUNT_STYLES[h % SERVICE_ACCOUNT_STYLES.length]!;
  const background = SERVICE_ACCOUNT_BACKGROUNDS[(h >>> 8) % SERVICE_ACCOUNT_BACKGROUNDS.length]!;

  return createAvatar(style, {
    seed: key,
    size,
    backgroundColor: [background],
  }).toDataUri();
}
