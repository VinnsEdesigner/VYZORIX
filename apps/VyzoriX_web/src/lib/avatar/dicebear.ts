import { createAvatar, type Style } from '@dicebear/core';
import {
  adventurer,
  avataaars,
  bigEars,
  bottts,
  croodles,
  dylan,
  funEmoji,
  glass,
  icons,
  identicon,
  initials,
  lorelei,
  micah,
  miniavs,
  notionists,
  openPeeps,
  personas,
  pixelArt,
  rings,
  shapes,
  thumbs,
} from '@dicebear/collection';

export interface AvatarStyle {
  id: string;
  label: string;
  style: Style<Record<string, unknown>>;
}

export const AVATAR_STYLES: AvatarStyle[] = [
  { id: 'notionists', label: 'Notionists', style: notionists },
  { id: 'adventurer', label: 'Adventurer', style: adventurer },
  { id: 'avataaars', label: 'Avataaars', style: avataaars },
  { id: 'lorelei', label: 'Lorelei', style: lorelei },
  { id: 'micah', label: 'Micah', style: micah },
  { id: 'miniavs', label: 'Miniavs', style: miniavs },
  { id: 'open-peeps', label: 'Open Peeps', style: openPeeps },
  { id: 'personas', label: 'Personas', style: personas },
  { id: 'big-ears', label: 'Big Ears', style: bigEars },
  { id: 'croodles', label: 'Croodles', style: croodles },
  { id: 'dylan', label: 'Dylan', style: dylan },
  { id: 'fun-emoji', label: 'Fun Emoji', style: funEmoji },
  { id: 'bottts', label: 'Bottts', style: bottts },
  { id: 'pixel-art', label: 'Pixel Art', style: pixelArt },
  { id: 'thumbs', label: 'Thumbs', style: thumbs },
  { id: 'identicon', label: 'Identicon', style: identicon },
  { id: 'rings', label: 'Rings', style: rings },
  { id: 'shapes', label: 'Shapes', style: shapes },
  { id: 'glass', label: 'Glass', style: glass },
  { id: 'icons', label: 'Icons', style: icons },
  { id: 'initials', label: 'Initials', style: initials },
];

export function avatarStyleById(id: string): AvatarStyle {
  return AVATAR_STYLES.find((item) => item.id === id) ?? AVATAR_STYLES[0]!;
}

export const BACKGROUND_SWATCHES = [
  'transparent',
  '#f43f5e',
  '#e11d48',
  '#fb7185',
  '#f59e0b',
  '#22c55e',
  '#0ea5e9',
  '#6366f1',
  '#a855f7',
  '#171717',
  '#1c1c1c',
  '#e5e5e5',
];

export interface AvatarOptions {
  styleId: string;
  seed: string;
  background: string;
}

function toConfig(options: AvatarOptions, size: number): Record<string, unknown> {
  const config: Record<string, unknown> = { seed: options.seed || 'vyzorix', size };
  if (options.background && options.background !== 'transparent') {
    config.backgroundColor = [options.background.replace('#', '')];
  }
  return config;
}

export function createAvatarDataUri(options: AvatarOptions, size = 128): string {
  const { style } = avatarStyleById(options.styleId);
  return createAvatar(style, toConfig(options, size) as never).toDataUri();
}

export function randomSeed(): string {
  return Math.random().toString(36).slice(2, 10);
}

export async function renderAvatarPng(options: AvatarOptions, size = 512): Promise<Blob> {
  const { style } = avatarStyleById(options.styleId);
  const svg = createAvatar(style, toConfig(options, size) as never).toString();
  const url = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;

  const image = await new Promise<HTMLImageElement>((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = () => reject(new Error('failed to render avatar'));
    img.src = url;
  });

  const canvas = document.createElement('canvas');
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('canvas unavailable');
  ctx.drawImage(image, 0, 0, size, size);

  return new Promise<Blob>((resolve, reject) => {
    canvas.toBlob(
      (blob) => (blob ? resolve(blob) : reject(new Error('failed to encode avatar'))),
      'image/png',
    );
  });
}
