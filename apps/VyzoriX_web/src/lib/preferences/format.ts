import type { ByteUnits, TimeFormat } from '@/stores/preferences-store';

const BINARY_UNITS = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'];
const DECIMAL_UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];

export function formatBytes(bytes: number, units: ByteUnits = 'binary'): string {
  if (!Number.isFinite(bytes)) return '—';
  const base = units === 'binary' ? 1024 : 1000;
  const table = units === 'binary' ? BINARY_UNITS : DECIMAL_UNITS;
  const sign = bytes < 0 ? '-' : '';
  let value = Math.abs(bytes);
  let index = 0;
  while (value >= base && index < table.length - 1) {
    value /= base;
    index += 1;
  }
  const rounded = index === 0 ? Math.round(value) : Math.round(value * 10) / 10;
  return `${sign}${rounded} ${table[index]}`;
}

export function formatNumber(value: number, thousandsSeparator = true): string {
  if (!Number.isFinite(value)) return '—';
  return thousandsSeparator ? value.toLocaleString('en-US') : String(value);
}

export interface TimestampOptions {
  timeFormat?: TimeFormat;
  relative?: boolean;
  now?: number;
}

export function formatTimestamp(ms: number | null | undefined, options: TimestampOptions = {}): string {
  if (!ms) return '—';
  const { timeFormat = '24h', relative = false, now = Date.now() } = options;

  if (relative) {
    const deltaSeconds = Math.round((now - ms) / 1000);
    const abs = Math.abs(deltaSeconds);
    if (abs < 45) return 'just now';
    if (abs < 3600) return `${Math.round(deltaSeconds / 60)}m ago`;
    if (abs < 86400) return `${Math.round(deltaSeconds / 3600)}h ago`;
    if (abs < 604800) return `${Math.round(deltaSeconds / 86400)}d ago`;
    return `${Math.round(deltaSeconds / 604800)}w ago`;
  }

  return new Date(ms).toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: timeFormat === '12h',
  });
}
