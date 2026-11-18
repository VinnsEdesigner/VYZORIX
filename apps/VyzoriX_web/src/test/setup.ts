import '@testing-library/jest-dom/vitest';
import { afterEach, beforeEach, vi } from 'vitest';
import { cleanup } from '@testing-library/react';
import { getDefaultVyzorI18n } from '@/lib/i18n';

vi.stubGlobal('matchMedia', (query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: vi.fn(),
  removeListener: vi.fn(),
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
  dispatchEvent: vi.fn(),
}));

class ProgressEventPolyfill extends Event {
  lengthComputable = false;
  loaded = 0;
  total = 0;
  constructor(type: string, init: ProgressEventInit = {}) {
    super(type, init);
    this.lengthComputable = init.lengthComputable ?? false;
    this.loaded = init.loaded ?? 0;
    this.total = init.total ?? 0;
  }
}

if (typeof globalThis.ProgressEvent === 'undefined') {
  (globalThis as Record<string, unknown>).ProgressEvent = ProgressEventPolyfill;
}
if (typeof window !== 'undefined' && typeof window.ProgressEvent === 'undefined') {
  (window as unknown as Record<string, unknown>).ProgressEvent = ProgressEventPolyfill;
}

afterEach(() => {
  cleanup();
});

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  vi.stubEnv('VITE_API_URL', '/api');

  getDefaultVyzorI18n();
});
