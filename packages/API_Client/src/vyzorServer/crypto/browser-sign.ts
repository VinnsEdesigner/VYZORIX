const SIGNING_ALGO = 'HMAC';
const HASH_ALGO = 'SHA-512';

export interface SignHeaders {
  'X-Vyzorix-Timestamp': string;
  'X-Vyzorix-Nonce': string;
  'X-Vyzorix-Signature': string;
}

export function generateNonce(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

function arrayBufferToBase64(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf);
  let binary = '';
  for (let i = 0; i < bytes.length; i++) {
    const byte = bytes[i];
    binary += String.fromCharCode(byte ?? 0);
  }
  return btoa(binary);
}

function encodeUtf8(text: string): Uint8Array {
  const encoded = new TextEncoder().encode(text);

  const out = new Uint8Array(encoded.byteLength);
  out.set(encoded);
  return out;
}

async function importHmacKey(secret: string): Promise<CryptoKey> {
  const keyBytes = encodeUtf8(secret);
  return crypto.subtle.importKey(
    'raw',
    keyBytes.buffer.slice(keyBytes.byteOffset, keyBytes.byteOffset + keyBytes.byteLength) as ArrayBuffer,
    { name: SIGNING_ALGO, hash: HASH_ALGO },
    false,
    ['sign'],
  );
}

export async function computeHttpSignature(
  method: string,
  path: string,
  timestampMs: string,
  nonce: string,
  body: string,
  secret: string,
): Promise<string> {
  const canonical = [method.toUpperCase(), path, nonce, timestampMs].join('\n') + '\n';

  const key = await importHmacKey(secret);
  const data = new Uint8Array(encodeUtf8(canonical).length + encodeUtf8(body).length);
  data.set(encodeUtf8(canonical), 0);
  data.set(encodeUtf8(body), encodeUtf8(canonical).length);

  const signature = await crypto.subtle.sign(
    SIGNING_ALGO,
    key,
    data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength) as ArrayBuffer,
  );
  return arrayBufferToBase64(signature);
}

export async function signHttpRequestBrowser(
  method: string,
  path: string,
  body: string,
  secret: string,
): Promise<SignHeaders> {
  const timestampMs = Date.now().toString();
  const nonce = generateNonce();

  const signature = await computeHttpSignature(
    method,
    path,
    timestampMs,
    nonce,
    body,
    secret,
  );

  return {
    'X-Vyzorix-Timestamp': timestampMs,
    'X-Vyzorix-Nonce': nonce,
    'X-Vyzorix-Signature': signature,
  };
}

export function constantTimeCompare(a: string, b: string): boolean {
  if (a.length !== b.length) {
    return false;
  }
  let result = 0;
  for (let i = 0; i < a.length; i++) {
    result |= a.charCodeAt(i) ^ b.charCodeAt(i);
  }
  return result === 0;
}
