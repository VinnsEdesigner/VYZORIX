export function describeUserAgent(ua: string | null | undefined): string {
  if (!ua) return 'Unknown';

  const browser = detectBrowser(ua);
  const os = detectOs(ua);

  if (!browser && !os) return ua;
  if (!browser) return os ?? ua;
  if (!os) return browser;
  return `${browser} · ${os}`;
}

function detectBrowser(ua: string): string | null {

  if (/Edg\//.test(ua)) return 'Edge';
  if (/OPR\/|Opera/.test(ua)) return 'Opera';
  if (/Firefox\//.test(ua)) return 'Firefox';
  if (/Chrome\//.test(ua)) return 'Chrome';
  if (/Safari\//.test(ua)) return 'Safari';
  return null;
}

function detectOs(ua: string): string | null {
  if (/Windows NT/.test(ua)) return 'Windows';
  if (/Android/.test(ua)) return 'Android';
  if (/iPhone|iPad|iPod/.test(ua)) return 'iOS';
  if (/Mac OS X|Macintosh/.test(ua)) return 'macOS';
  if (/Linux/.test(ua)) return 'Linux';
  return null;
}
