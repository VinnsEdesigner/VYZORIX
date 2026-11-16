/**
 * Scope derivation and payload encoding for the local store.
 *
 * What this is: the store's contents are encoded so that a second person using
 * the same browser profile cannot read the first person's cached data, and so
 * the database file cannot be identified or read by name.
 *
 * What this is NOT: encryption at rest. The key is derived in the page, from a
 * salt in the same localStorage the data lives beside. Anyone with the device,
 * or with script execution in this origin, can recover it. The threat it
 * addresses is a shared browser profile, nothing more. See
 * documents/FRONTEND_LOCAL_STORE_AT_REST.md.
 *
 * The database name and every stored value are derived from a scope string (the
 * signed-in user's id) plus a per-origin random salt.
 */

type Bytes = Uint8Array<ArrayBuffer>;

const SALT_KEY = 'vyzorix.localStore.salt';
const SALT_BYTES = 32;
const IV_BYTES = 16;
const KEY_BITS = 256;

export interface Codec {
  /**
   * Encode with an IV derived from the plaintext, so the same input always
   * produces the same output. Required for columns that are queried by equality
   * (a primary key, an indexed column): a random IV would make them unmatchable.
   */
  encodeDeterministic(value: string): Promise<string>;

  /** Encode with a fresh random IV. Used for payloads, which are never queried. */
  encodeRandom(value: string): Promise<string>;

  decode(value: string): Promise<string>;
}

export function toBase64(bytes: Bytes): string {
  let binary = '';
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary);
}

export function fromBase64(text: string): Bytes {
  const binary = atob(text);
  const out = new Uint8Array(new ArrayBuffer(binary.length));
  for (let i = 0; i < binary.length; i += 1) out[i] = binary.charCodeAt(i);
  return out;
}

function concat(a: Bytes, b: Bytes): Bytes {
  const out = new Uint8Array(a.length + b.length);
  out.set(a, 0);
  out.set(b, a.length);
  return out;
}

/**
 * The per-origin salt, created once and persisted in localStorage.
 *
 * This runs on the main thread: a worker has neither `window` nor
 * `localStorage`, so reading the salt there would fail silently and produce a
 * new random salt on every open — and therefore a new database name each time,
 * which looks exactly like the cache being cleared. The caller passes the salt
 * to the worker.
 */
export function getOrCreateSalt(): Bytes {
  const existing = window.localStorage.getItem(SALT_KEY);
  if (existing) return fromBase64(existing);

  const salt = crypto.getRandomValues(new Uint8Array(new ArrayBuffer(SALT_BYTES)));
  window.localStorage.setItem(SALT_KEY, toBase64(salt));
  return salt;
}

async function deriveBits(scope: string, salt: Bytes, bits: number): Promise<ArrayBuffer> {
  const material = await crypto.subtle.importKey(
    'raw',
    new TextEncoder().encode(scope),
    'PBKDF2',
    false,
    ['deriveBits'],
  );

  return crypto.subtle.deriveBits(
    { name: 'PBKDF2', salt, iterations: 100_000, hash: 'SHA-256' },
    material,
    bits,
  );
}

/**
 * The database file name. Derived, not chosen, so the name itself carries no
 * information and two scopes never collide.
 */
export async function databaseNameFor(scope: string, salt: Bytes): Promise<string> {
  const bits = await deriveBits(scope, salt, 128);
  const hex = [...new Uint8Array(bits)]
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
  return `vyzorix-${hex}.db`;
}

export async function createCodec(scope: string, salt: Bytes): Promise<Codec> {
  const raw = await deriveBits(scope, salt, KEY_BITS);

  const [cipherKey, macKey] = await Promise.all([
    crypto.subtle.importKey('raw', raw, { name: 'AES-CTR' }, false, ['encrypt', 'decrypt']),
    crypto.subtle.importKey('raw', raw, { name: 'HMAC', hash: 'SHA-256' }, false, ['sign']),
  ]);

  const encryptWith = async (plaintext: string, iv: Bytes): Promise<string> => {
    const cipher = await crypto.subtle.encrypt(
      { name: 'AES-CTR', counter: iv, length: 64 },
      cipherKey,
      new TextEncoder().encode(plaintext),
    );
    return toBase64(concat(iv, new Uint8Array(cipher)));
  };

  return {
    encodeRandom: (value) =>
      encryptWith(value, crypto.getRandomValues(new Uint8Array(new ArrayBuffer(IV_BYTES)))),

    encodeDeterministic: async (value) => {
      const mac = await crypto.subtle.sign('HMAC', macKey, new TextEncoder().encode(value));
      return encryptWith(value, new Uint8Array(new Uint8Array(mac).slice(0, IV_BYTES)));
    },

    decode: async (stored) => {
      const rawBytes = fromBase64(stored);
      const iv = rawBytes.slice(0, IV_BYTES);
      const body = rawBytes.slice(IV_BYTES);
      const plain = await crypto.subtle.decrypt(
        { name: 'AES-CTR', counter: iv, length: 64 },
        cipherKey,
        body,
      );
      return new TextDecoder().decode(plain);
    },
  };
}
